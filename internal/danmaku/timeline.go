package danmaku

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/internal/playback"
)

const maxStart = time.Duration(1<<63-1) - 16*time.Second

type indexedEntry struct {
	Entry
	index int
}

type interval struct {
	entry int
	end   time.Duration
}

type schedule struct {
	lanes     [MaxLanes][]interval
	laneCount int
	overflow  int
}

// Timeline is constructed with New. It retains one copied comment window and
// its lane schedule; it owns no clock, worker, I/O or native playback resource.
type Timeline struct {
	generation, revision uint64
	settings             Settings
	entries              []indexedEntry
	stats                Stats
	schedule             *schedule
	closed               bool
}

func New() *Timeline { return &Timeline{settings: DefaultSettings()} }

func (t *Timeline) Reset(generation uint64) error {
	if t.closed {
		return IsClosed
	}
	if generation == 0 {
		return Invalid
	}
	if generation < t.generation {
		return Stale
	}
	if generation == t.generation {
		return nil
	}
	t.generation = generation
	t.revision++
	t.entries, t.schedule, t.stats = nil, nil, Stats{}
	return nil
}

func (t *Timeline) SetEntries(generation uint64, entries []Entry) error {
	if t.closed {
		return IsClosed
	}
	if generation == 0 {
		return Invalid
	}
	if generation != t.generation {
		return Stale
	}
	if len(entries) > MaxEntries {
		return Invalid
	}
	textBytes := 0
	// Validate the whole replacement, including empty text, before allocating
	// owned copies or changing the installed window.
	for _, e := range entries {
		textBytes += len(e.Text)
		if len(e.Text) > MaxTextBytes || textBytes > MaxWindowBytes || !utf8.ValidString(e.Text) || e.At < 0 || e.At > maxStart || e.Color > 0xffffff {
			return Invalid
		}
		switch e.Mode {
		case ScrollRTL, FixedBottom, FixedTop, ScrollLTR:
		default:
			return Invalid
		}
		for _, r := range e.Text {
			if !unicode.IsSpace(r) && unicode.IsControl(r) {
				return Invalid
			}
		}
	}
	normalized := make([]indexedEntry, 0, len(entries))
	stats := Stats{Input: len(entries)}
	for i, e := range entries {
		e.Text = normalize(e.Text)
		if e.Text == "" {
			stats.SkippedEmpty++
			continue
		}
		normalized = append(normalized, indexedEntry{Entry: e, index: i})
	}
	slices.SortFunc(normalized, func(a, b indexedEntry) int {
		if order := cmp.Compare(a.At, b.At); order != 0 {
			return order
		}
		return cmp.Compare(a.index, b.index)
	})
	stats.Loaded = len(normalized)
	t.entries, t.stats, t.schedule = normalized, stats, nil
	t.revision++
	return nil
}

// normalize only receives validated text. Building a new string also prevents
// a small comment from retaining a large caller-owned transport buffer.
func normalize(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	space := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			space = out.Len() > 0
			continue
		}
		if space {
			out.WriteByte(' ')
			space = false
		}
		out.WriteRune(r)
	}
	return out.String()
}

func (t *Timeline) Configure(settings Settings) error {
	if t.closed {
		return IsClosed
	}
	if !finite(settings.Opacity) || settings.Opacity < 0 || settings.Opacity > 1 ||
		!finite(settings.FontSize) || settings.FontSize < 16 || settings.FontSize > 40 ||
		!finite(settings.Speed) || settings.Speed < .5 || settings.Speed > 2 {
		return Invalid
	}
	if settings.Speed != t.settings.Speed {
		t.schedule = nil
	}
	t.settings = settings
	return nil
}

func (t *Timeline) Settings() Settings { return t.settings }

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func (t *Timeline) Frame(snapshot playback.Snapshot, width, height float64) (Frame, error) {
	frame := Frame{Generation: t.generation, Revision: t.revision, Settings: t.settings, Items: []Item{}, Closed: t.closed}
	if t.closed {
		return frame, nil
	}
	if !finite(width) || !finite(height) || width < 0 || height < 0 || snapshot.Position < 0 {
		return frame, Invalid
	}
	frame.Position, frame.Width, frame.Height = snapshot.Position, width, height
	frame.LineHeight = math.Ceil(t.settings.FontSize*1.25) + 4
	if width > 0 {
		frame.Lanes = int(min(float64(MaxLanes), math.Floor(height/frame.LineHeight)))
	}
	if t.schedule == nil || t.schedule.laneCount != frame.Lanes {
		t.schedule = t.buildSchedule(frame.Lanes)
	}
	frame.Stats = t.stats
	frame.Stats.Overflow = t.schedule.overflow
	if snapshot.Generation != t.generation || !t.settings.Visible || t.settings.Opacity == 0 {
		return frame, nil
	}
	switch snapshot.State {
	case playback.Ready, playback.Paused, playback.Playing:
	default:
		return frame, nil
	}
	// Each lane's non-overlapping intervals have strictly increasing ends.
	// Only the first end after the sampled position can still be active.
	type active struct {
		Item
		at time.Duration
	}
	var current [MaxLanes]active
	count := 0
	for laneID := 0; laneID < frame.Lanes; laneID++ {
		lane := t.schedule.lanes[laneID]
		i := sort.Search(len(lane), func(i int) bool { return lane[i].end > snapshot.Position })
		if i == len(lane) {
			continue
		}
		span := lane[i]
		e := t.entries[span.entry]
		if e.At > snapshot.Position {
			continue
		}
		current[count] = active{Item: Item{Index: e.index, Mode: e.Mode, Text: e.Text, Color: e.Color, Lane: laneID,
			Top: laneTop(laneID, frame.Lanes, height, frame.LineHeight), Progress: float64(snapshot.Position-e.At) / float64(span.end-e.At)}, at: e.At}
		count++
	}
	slices.SortFunc(current[:count], func(a, b active) int {
		if order := cmp.Compare(a.at, b.at); order != 0 {
			return order
		}
		return cmp.Compare(a.Index, b.Index)
	})
	frame.Items = make([]Item, count)
	for i := range count {
		frame.Items[i] = current[i].Item
	}
	return frame, nil
}

func (t *Timeline) buildSchedule(lanes int) *schedule {
	s := &schedule{laneCount: lanes}
	var available [MaxLanes]time.Duration
	for i, e := range t.entries {
		start, end, step := lanePool(e.Mode, lanes)
		chosen := -1
		for lane := start; lane != end; lane += step {
			if available[lane] <= e.At {
				chosen = lane
				break
			}
		}
		if chosen < 0 {
			s.overflow++
			continue
		}
		lifetime := 8 * time.Second
		if e.Mode == FixedTop || e.Mode == FixedBottom {
			lifetime = 4 * time.Second
		}
		until := e.At + time.Duration(float64(lifetime)/t.settings.Speed)
		s.lanes[chosen] = append(s.lanes[chosen], interval{entry: i, end: until})
		available[chosen] = until
	}
	return s
}

func lanePool(mode Mode, lanes int) (start, end, step int) {
	if lanes == 0 {
		return 0, 0, 1
	}
	if lanes == 1 {
		return 0, 1, 1
	}
	if lanes == 2 {
		switch mode {
		case FixedTop:
			return 0, 1, 1
		case FixedBottom:
			return 1, 2, 1
		default:
			return 0, 2, 1
		}
	}
	fixed := max(1, lanes/4)
	switch mode {
	case FixedTop:
		return 0, fixed, 1
	case FixedBottom:
		return lanes - 1, lanes - fixed - 1, -1
	default:
		return fixed, lanes - fixed, 1
	}
}

func laneTop(lane, lanes int, height, lineHeight float64) float64 {
	if lanes == 1 {
		return (height - lineHeight) / 2
	}
	if lanes == 2 {
		if lane == 0 {
			return 0
		}
		return height - lineHeight
	}
	if lane >= lanes-max(1, lanes/4) {
		return height - float64(lanes-lane)*lineHeight
	}
	return float64(lane) * lineHeight
}

func (t *Timeline) Close() {
	t.closed = true
	t.entries, t.schedule, t.stats = nil, nil, Stats{}
}
