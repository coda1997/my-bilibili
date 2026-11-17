package danmaku

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"unsafe"

	"my-bilibili/internal/playback"
)

func comment(at time.Duration, mode Mode, text string) Entry {
	return Entry{At: at, Mode: mode, Text: text, Color: 0xffffff}
}

func installed(tb testing.TB, entries []Entry) *Timeline {
	tb.Helper()
	timeline := New()
	if err := timeline.Reset(1); err != nil {
		tb.Fatal(err)
	}
	if err := timeline.SetEntries(1, entries); err != nil {
		tb.Fatal(err)
	}
	return timeline
}

func frameAt(tb testing.TB, timeline *Timeline, at time.Duration, width, height float64) Frame {
	tb.Helper()
	frame, err := timeline.Frame(playback.Snapshot{Generation: 1, State: playback.Playing, Position: at, Rate: 1}, width, height)
	if err != nil || frame.Items == nil {
		tb.Fatal("missing complete frame", err)
	}
	return frame
}

func TestDefaultAndGenerationOwnership(t *testing.T) {
	timeline := New()
	if timeline.Settings() != (Settings{Visible: true, Opacity: .9, FontSize: 24, Speed: 1}) {
		t.Fatal("unexpected default settings")
	}
	initial, err := timeline.Frame(playback.Snapshot{}, 800, 450)
	if err != nil || initial.Generation != 0 || initial.Revision != 0 || initial.Closed || initial.Items == nil || len(initial.Items) != 0 {
		t.Fatal("construction invented bound content", initial, err)
	}
	if err := timeline.SetEntries(1, nil); err != Stale {
		t.Fatal("entries installed before Reset", err)
	}
	if err := timeline.Reset(0); err != Invalid {
		t.Fatal("zero generation accepted", err)
	}
	if err := timeline.Reset(1); err != nil {
		t.Fatal(err)
	}
	entries := []Entry{comment(0, FixedTop, "原始弹幕")}
	if err := timeline.SetEntries(1, entries); err != nil {
		t.Fatal(err)
	}
	before := frameAt(t, timeline, 0, 800, 450)
	if err := timeline.Reset(1); err != nil || !reflect.DeepEqual(before, frameAt(t, timeline, 0, 800, 450)) {
		t.Fatal("same generation reset was not idempotent", err)
	}
	for _, generation := range []uint64{0, 2} {
		want := Stale
		if generation == 0 {
			want = Invalid
		}
		if err := timeline.SetEntries(generation, []Entry{comment(0, FixedTop, "other")}); err != want || !reflect.DeepEqual(before, frameAt(t, timeline, 0, 800, 450)) {
			t.Fatal("mismatched entries changed the accepted generation", generation, err)
		}
	}
	settings := Settings{Visible: false, Opacity: .3, FontSize: 30, Speed: 1.5}
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	if err := timeline.Reset(2); err != nil {
		t.Fatal(err)
	}
	newFrame, err := timeline.Frame(playback.Snapshot{Generation: 2, State: playback.Ready}, 800, 450)
	if err != nil || newFrame.Generation != 2 || newFrame.Revision != before.Revision+1 || newFrame.Settings != settings || newFrame.Stats != (Stats{}) || len(newFrame.Items) != 0 {
		t.Fatal("new generation retained old data or lost settings", newFrame, err)
	}
	if err := timeline.Reset(1); err != Stale || timeline.generation != 2 {
		t.Fatal("older reset replaced current generation", err)
	}
	if err := timeline.SetEntries(1, entries); err != Stale || len(timeline.entries) != 0 {
		t.Fatal("older installation revived old text", err)
	}
}

func TestNormalizationCopiesAndOriginalIndices(t *testing.T) {
	backing := strings.Repeat("x", 1<<20) + "原文"
	text := backing[len(backing)-len("原文"):]
	entries := []Entry{
		comment(time.Second, FixedTop, " \r\n中文\t\u00a0弹幕\u2028\u2029 "),
		comment(0, FixedTop, " \t\v\f\u0085\u2003\u2028 "),
		comment(0, FixedTop, text),
		comment(0, FixedBottom, "e\u0301👩\u200d💻 <字>"),
	}
	timeline := installed(t, entries)
	frame := frameAt(t, timeline, time.Second, 800, 800)
	if frame.Stats != (Stats{Input: 4, Loaded: 3, SkippedEmpty: 1}) || len(frame.Items) != 3 {
		t.Fatal("normalization lost empty accounting", frame.Stats)
	}
	wantIndex := []int{2, 3, 0}
	wantText := []string{"原文", "e\u0301👩\u200d💻 <字>", "中文 弹幕"}
	for i, item := range frame.Items {
		if item.Index != wantIndex[i] || item.Text != wantText[i] {
			t.Fatal("normalization, order or original index changed", item)
		}
	}
	if unsafe.StringData(frame.Items[0].Text) == unsafe.StringData(text) {
		t.Fatal("normalized text retains the caller's large backing buffer")
	}
	entries[2] = comment(20*time.Second, ScrollRTL, "mutated input")
	frame.Items[0].Text, frame.Items[0].Color, frame.Stats.Input = "mutated output", 0, 999
	again := frameAt(t, timeline, time.Second, 800, 800)
	if again.Items[0].Text != "原文" || again.Items[0].Color != 0xffffff || again.Stats.Input != 4 {
		t.Fatal("input/output ownership escaped")
	}
}

func repeatedEntries(count int, text string) []Entry {
	entries := make([]Entry, count)
	for i := range entries {
		entries[i] = comment(0, ScrollRTL, text)
	}
	return entries
}

func TestInputValidationIsTransactionalAndIncludesEmptyText(t *testing.T) {
	cases := map[string][]Entry{
		"count":              repeatedEntries(MaxEntries+1, "x"),
		"single text bytes":  {comment(0, ScrollRTL, strings.Repeat("x", MaxTextBytes+1))},
		"multibyte bytes":    {comment(0, ScrollRTL, strings.Repeat("中", 342))},
		"aggregate bytes":    repeatedEntries(MaxWindowBytes/MaxTextBytes+1, strings.Repeat("x", MaxTextBytes)),
		"empty bytes count":  repeatedEntries(MaxWindowBytes/MaxTextBytes+1, strings.Repeat(" ", MaxTextBytes)),
		"utf8":               {comment(0, ScrollRTL, "\xff")},
		"control":            {comment(0, ScrollRTL, "a\x00b")},
		"delete control":     {comment(0, ScrollRTL, "\x7f")},
		"mode zero":          {comment(0, 0, "x")},
		"mode unsupported":   {comment(0, 2, "x")},
		"empty invalid mode": {comment(0, 0, "")},
		"negative time":      {comment(-time.Nanosecond, ScrollRTL, "")},
		"time headroom":      {comment(maxStart+time.Nanosecond, ScrollRTL, "")},
		"color":              {{Mode: ScrollRTL, Text: "", Color: 0x1000000}},
	}
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			timeline := installed(t, []Entry{comment(0, FixedTop, "保留")})
			before := frameAt(t, timeline, time.Second, 800, 450)
			// A valid prefix must not be partially installed before a later error.
			input := append([]Entry{comment(0, FixedBottom, "不能部分替换")}, invalid...)
			if err := timeline.SetEntries(1, input); err != Invalid {
				t.Fatal("invalid external entry accepted", err)
			}
			if !reflect.DeepEqual(before, frameAt(t, timeline, time.Second, 800, 450)) {
				t.Fatal("failed replacement changed accepted data")
			}
		})
	}
}

func TestInclusiveInputBoundsAndOverflowSafeTime(t *testing.T) {
	timeline := installed(t, repeatedEntries(MaxEntries, "x"))
	if frameAt(t, timeline, 0, 800, 450).Stats.Loaded != MaxEntries {
		t.Fatal("exact count bound rejected")
	}
	if err := timeline.SetEntries(1, repeatedEntries(MaxWindowBytes/MaxTextBytes, strings.Repeat("x", MaxTextBytes))); err != nil {
		t.Fatal("exact text bounds rejected", err)
	}
	if err := timeline.SetEntries(1, []Entry{comment(maxStart, ScrollRTL, "末尾")}); err != nil {
		t.Fatal("safe last start rejected", err)
	}
	settings := timeline.Settings()
	settings.Speed = .5
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	frame := frameAt(t, timeline, maxStart+8*time.Second, 800, 34)
	if len(frame.Items) != 1 || frame.Items[0].Progress != .5 {
		t.Fatal("largest interval overflowed", frame.Items)
	}
	if got := frameAt(t, timeline, time.Duration(1<<63-1), 800, 34); len(got.Items) != 0 {
		t.Fatal("half-open maximum interval retained its end")
	}
}

func TestReplacementNeverAppendsOrAccumulatesStatistics(t *testing.T) {
	entries := []Entry{comment(0, FixedTop, "相同"), comment(0, FixedTop, ""), comment(0, FixedTop, "相同")}
	timeline := installed(t, entries)
	before := frameAt(t, timeline, time.Second, 800, 800)
	if len(before.Items) != 2 || before.Items[0].Index != 0 || before.Items[1].Index != 2 {
		t.Fatal("repeated content was incorrectly deduplicated", before.Items)
	}
	for i := 1; i <= 3; i++ {
		if err := timeline.SetEntries(1, entries); err != nil {
			t.Fatal(err)
		}
		got := frameAt(t, timeline, time.Second, 800, 800)
		if got.Revision != before.Revision+uint64(i) || got.Stats != before.Stats || !reflect.DeepEqual(got.Items, before.Items) {
			t.Fatal("replacement accumulated entries or changed their identity", got)
		}
	}
	if err := timeline.SetEntries(1, nil); err != nil {
		t.Fatal(err)
	}
	if got := frameAt(t, timeline, time.Second, 800, 800); got.Items == nil || len(got.Items) != 0 || got.Stats != (Stats{}) {
		t.Fatal("empty replacement retained comments", got)
	}
}

func TestSettingsValidationAndVisibility(t *testing.T) {
	timeline := installed(t, []Entry{comment(0, ScrollRTL, "弹幕")})
	before := frameAt(t, timeline, time.Second, 800, 450)
	for name, values := range map[string][]float64{
		"opacity": {math.NaN(), math.Inf(1), math.Inf(-1), -.01, 1.01},
		"font":    {math.NaN(), math.Inf(1), math.Inf(-1), 15.99, 40.01},
		"speed":   {math.NaN(), math.Inf(1), math.Inf(-1), .499, 2.001},
	} {
		for i, value := range values {
			t.Run(fmt.Sprintf("%s-%d", name, i), func(t *testing.T) {
				settings := timeline.Settings()
				switch name {
				case "opacity":
					settings.Opacity = value
				case "font":
					settings.FontSize = value
				case "speed":
					settings.Speed = value
				}
				if err := timeline.Configure(settings); err != Invalid || !reflect.DeepEqual(before, frameAt(t, timeline, time.Second, 800, 450)) {
					t.Fatal("invalid settings changed the frame", err)
				}
			})
		}
	}
	for _, settings := range []Settings{
		{Visible: true, Opacity: 0, FontSize: 16, Speed: .5},
		{Visible: false, Opacity: 1, FontSize: 40, Speed: 2},
	} {
		if err := timeline.Configure(settings); err != nil {
			t.Fatal("inclusive settings boundary rejected", err)
		}
		frame := frameAt(t, timeline, time.Second, 800, 450)
		if len(frame.Items) != 0 || frame.Stats.Loaded != 1 || frame.Settings != settings {
			t.Fatal("hidden/transparent frame lost admission or showed items", frame)
		}
	}
	if err := timeline.Configure(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, frameAt(t, timeline, time.Second, 800, 450)) {
		t.Fatal("re-enabling did not return to current media position")
	}
}

func TestGeometryValidationAndLaneClamp(t *testing.T) {
	timeline := installed(t, repeatedEntries(80, "字"))
	before := frameAt(t, timeline, time.Second, 800, 450)
	scheduleBefore := timeline.schedule
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, geometry := range [][2]float64{{value, 450}, {800, value}} {
			got, err := timeline.Frame(playback.Snapshot{Generation: 1, State: playback.Playing}, geometry[0], geometry[1])
			if err != Invalid || got.Items == nil || timeline.schedule != scheduleBefore || !reflect.DeepEqual(before, frameAt(t, timeline, time.Second, 800, 450)) {
				t.Fatal("invalid geometry mutated the accepted window", err)
			}
		}
	}
	if _, err := timeline.Frame(playback.Snapshot{Position: -1}, 800, 450); err != Invalid {
		t.Fatal("negative media position accepted", err)
	}
	for _, geometry := range [][2]float64{{0, 450}, {800, 0}, {800, 33.999}} {
		got := frameAt(t, timeline, 0, geometry[0], geometry[1])
		if got.Lanes != 0 || len(got.Items) != 0 || got.Stats.Overflow != 80 {
			t.Fatal("zero-lane geometry invented capacity", got)
		}
	}
	huge := frameAt(t, timeline, 0, math.MaxFloat64, math.MaxFloat64)
	if huge.Lanes != 16 || len(huge.Items) != 8 || huge.Stats.Overflow != 72 {
		t.Fatal("finite huge dimensions were not clamped before integer conversion", huge.Lanes, len(huge.Items), huge.Stats)
	}
}

func TestExactLanePoolsOrderAndCoordinates(t *testing.T) {
	entries := []Entry{
		comment(0, FixedTop, "top-0"), comment(0, FixedTop, "top-1"),
		comment(0, FixedBottom, "bottom-0"), comment(0, FixedBottom, "bottom-1"),
		comment(0, ScrollRTL, "rtl-0"), comment(0, ScrollLTR, "ltr-0"),
		comment(0, ScrollRTL, "rtl-1"), comment(0, ScrollLTR, "ltr-1"),
	}
	for _, tc := range []struct {
		lanes   int
		height  float64
		indices []int
		laneIDs []int
		tops    []float64
	}{
		{0, 30, []int{}, []int{}, []float64{}},
		{1, 44, []int{0}, []int{0}, []float64{5}},
		{2, 78, []int{0, 2}, []int{0, 1}, []float64{0, 44}},
		{3, 112, []int{0, 2, 4}, []int{0, 2, 1}, []float64{0, 78, 34}},
		{4, 146, []int{0, 2, 4, 5}, []int{0, 3, 1, 2}, []float64{0, 112, 34, 68}},
		{7, 248, []int{0, 2, 4, 5, 6, 7}, []int{0, 6, 1, 2, 3, 4}, []float64{0, 214, 34, 68, 102, 136}},
		{16, 1000, []int{0, 1, 2, 3, 4, 5, 6, 7}, []int{0, 1, 15, 14, 4, 5, 6, 7}, []float64{0, 34, 966, 932, 136, 170, 204, 238}},
	} {
		t.Run(fmt.Sprint(tc.lanes), func(t *testing.T) {
			timeline := installed(t, entries)
			frame := frameAt(t, timeline, 0, 800, tc.height)
			if frame.Lanes != tc.lanes || frame.LineHeight != 34 || len(frame.Items) != len(tc.indices) || frame.Stats.Overflow != len(entries)-len(tc.indices) {
				t.Fatal("wrong lane capacity or overflow", frame)
			}
			for i, item := range frame.Items {
				if item.Index != tc.indices[i] || item.Lane != tc.laneIDs[i] || item.Top != tc.tops[i] || item.Progress != 0 {
					t.Fatal("first-free lane order/coordinate mismatch", item, tc)
				}
			}
		})
	}
	shared := installed(t, []Entry{comment(0, ScrollRTL, "first"), comment(0, ScrollLTR, "second"), comment(0, FixedTop, "third")})
	got := frameAt(t, shared, 0, 800, 68)
	if len(got.Items) != 2 || got.Items[0].Lane != 0 || got.Items[1].Lane != 1 || got.Stats.Overflow != 1 {
		t.Fatal("two-lane scrolling did not reserve the shared pool", got)
	}
}

func TestAllLaneCountsBoundActiveItemsAndHaveNoOverlap(t *testing.T) {
	entries := make([]Entry, 0, 96)
	for _, mode := range []Mode{FixedTop, FixedBottom, ScrollRTL} {
		for range 32 {
			entries = append(entries, comment(0, mode, "密集"))
		}
	}
	timeline := installed(t, entries)
	for lanes := 0; lanes <= 20; lanes++ {
		frame := frameAt(t, timeline, time.Second, 800, float64(lanes*34))
		want := min(lanes, 16)
		if frame.Lanes != want || len(frame.Items) != want || frame.Stats.Overflow != len(entries)-want {
			t.Fatal("dense input escaped active capacity", lanes, frame)
		}
		seen := map[int]bool{}
		for _, item := range frame.Items {
			if seen[item.Lane] || item.Lane < 0 || item.Lane >= frame.Lanes || item.Top < 0 || item.Top+frame.LineHeight > frame.Height {
				t.Fatal("lane overlap or invalid bounds", lanes, item)
			}
			seen[item.Lane] = true
		}
	}
}

func TestWholeLifetimeAdmissionAndHalfOpenIntervals(t *testing.T) {
	timeline := installed(t, []Entry{
		comment(12*time.Second, ScrollLTR, "last"),
		comment(0, ScrollRTL, "first"),
		comment(4*time.Second, FixedTop, "must drop"),
		comment(8*time.Second, FixedBottom, "middle"),
	})
	for _, tc := range []struct {
		at       time.Duration
		index    int
		progress float64
	}{
		{0, 1, 0}, {4 * time.Second, 1, .5},
		{8*time.Second - time.Nanosecond, 1, float64(8*time.Second-time.Nanosecond) / float64(8*time.Second)},
		{8 * time.Second, 3, 0}, {10 * time.Second, 3, .5},
		{12 * time.Second, 0, 0}, {16 * time.Second, 0, .5},
	} {
		frame := frameAt(t, timeline, tc.at, 800, 34)
		if len(frame.Items) != 1 || frame.Items[0].Index != tc.index || frame.Items[0].Progress != tc.progress || frame.Stats.Overflow != 1 {
			t.Fatal("whole-life admission or half-open interval violated", tc, frame)
		}
	}
	if frame := frameAt(t, timeline, 20*time.Second, 800, 34); len(frame.Items) != 0 || frame.Stats.Overflow != 1 {
		t.Fatal("ended entry remained or overflow accumulated", frame)
	}
	if err := timeline.Configure(Settings{Visible: true, Opacity: .9, FontSize: 24, Speed: 2}); err != nil {
		t.Fatal(err)
	}
	frame := frameAt(t, timeline, 4*time.Second, 800, 34)
	if frame.Stats.Overflow != 0 || len(frame.Items) != 1 || frame.Items[0].Index != 2 || frame.Items[0].Progress != 0 {
		t.Fatal("speed did not rebuild admission at the same media position", frame)
	}
}

func TestSampledPositionIsTheOnlyClockAndSeekIsDeterministic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timeline := installed(t, []Entry{comment(0, ScrollRTL, "暂停")})
		before := frameAt(t, timeline, 2*time.Second, 800, 450)
		time.Sleep(3 * time.Second)
		for _, state := range []playback.State{playback.Playing, playback.Paused, playback.Ready} {
			for _, rate := range []float64{.5, 1, 2, 3} {
				got, err := timeline.Frame(playback.Snapshot{Generation: 1, State: state, Position: 2 * time.Second, Rate: rate}, 800, 450)
				if err != nil || !reflect.DeepEqual(before, got) || got.Items[0].Progress != .25 {
					t.Fatal("wall time, state or native rate moved fixed-position comments", state, rate, got, err)
				}
			}
		}
		_ = frameAt(t, timeline, 7*time.Second, 800, 450)
		_ = frameAt(t, timeline, time.Second, 800, 450)
		if !reflect.DeepEqual(before, frameAt(t, timeline, 2*time.Second, 800, 450)) {
			t.Fatal("seek back/forward accumulated or changed admission")
		}
	})
}

func TestOnlyLiveMatchingPlaybackShowsComments(t *testing.T) {
	timeline := installed(t, []Entry{comment(0, FixedTop, "live")})
	for _, state := range []playback.State{playback.Idle, playback.Loading, playback.Failed, playback.Ended, playback.Closed, "unknown"} {
		got, err := timeline.Frame(playback.Snapshot{Generation: 1, State: state, Position: time.Second}, 800, 450)
		if err != nil || len(got.Items) != 0 || got.Items == nil {
			t.Fatal("inactive playback showed comments", state, got, err)
		}
	}
	for _, generation := range []uint64{0, 2} {
		got, err := timeline.Frame(playback.Snapshot{Generation: generation, State: playback.Playing, Position: time.Second}, 800, 450)
		if err != nil || len(got.Items) != 0 {
			t.Fatal("mismatched playback showed old comments", generation, got, err)
		}
	}
	if got := frameAt(t, timeline, time.Second, 800, 450); len(got.Items) != 1 {
		t.Fatal("inactive observation mutated the installed window")
	}
}

func TestScheduleRebuildsOnlyWhenAdmissionChanges(t *testing.T) {
	timeline := installed(t, []Entry{comment(0, ScrollRTL, "字")})
	_ = frameAt(t, timeline, time.Second, 800, 450)
	previous := timeline.schedule
	builds := 0
	sample := func(width, height float64, wantBuild bool) Frame {
		t.Helper()
		frame := frameAt(t, timeline, time.Second, width, height)
		changed := timeline.schedule != previous
		if changed {
			builds++
		}
		if changed != wantBuild {
			t.Fatal("schedule rebuild disagrees with admission change", changed, wantBuild)
		}
		previous = timeline.schedule
		return frame
	}
	sample(1000, 451, false)
	settings := timeline.Settings()
	settings.Visible, settings.Opacity = false, .2
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	sample(1000, 451, false)
	settings.FontSize = 24.1 // Line height 35, still 12 lanes at height 451.
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	// 450/34 initially yields 13 lanes, so the changed font yields 12.
	sample(1000, 451, true)
	settings.FontSize = 24.2 // Same line height and lane count.
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	sample(1000, 451, false)
	settings.Speed = 1.1
	if err := timeline.Configure(settings); err != nil {
		t.Fatal(err)
	}
	sample(1000, 451, true)
	if err := timeline.SetEntries(1, []Entry{comment(0, ScrollRTL, "替换")}); err != nil {
		t.Fatal(err)
	}
	sample(1000, 451, true)
	sample(1000, 70, true)
	if builds != 4 {
		t.Fatal("unexpected schedule-build count", builds)
	}
}

func TestCloseDropsOwnedDataAndIsTerminal(t *testing.T) {
	timeline := installed(t, []Entry{comment(0, FixedTop, "关闭")})
	before := frameAt(t, timeline, time.Second, 800, 450)
	if timeline.entries == nil || timeline.schedule == nil {
		t.Fatal("close test did not establish owned data")
	}
	timeline.Close()
	timeline.Close()
	if timeline.entries != nil || timeline.schedule != nil || timeline.stats != (Stats{}) {
		t.Fatal("close retained data/schedule ownership")
	}
	got, err := timeline.Frame(playback.Snapshot{Position: -1}, math.NaN(), math.Inf(1))
	if err != nil || !got.Closed || got.Generation != before.Generation || got.Revision != before.Revision || got.Settings != before.Settings || got.Position != 0 || got.Width != 0 || got.Height != 0 || got.LineHeight != 0 || got.Lanes != 0 || got.Stats != (Stats{}) || got.Items == nil || len(got.Items) != 0 {
		t.Fatal("closed frame is not a terminal empty descriptor", got, err)
	}
	for _, err := range []error{timeline.Reset(2), timeline.SetEntries(1, nil), timeline.Configure(DefaultSettings())} {
		if err != IsClosed {
			t.Fatal("closed timeline accepted a write", err)
		}
	}
	if IsClosed.Error() != "danmaku-closed" || Invalid.Error() != "invalid-danmaku" || Stale.Error() != "stale-generation" || Unavailable.Error() != "danmaku-unavailable" || NoError.Error() != "" {
		t.Fatal("error codes are not the fixed safe vocabulary")
	}
}

func maximumTimeline(tb testing.TB) *Timeline {
	tb.Helper()
	entries := make([]Entry, MaxEntries)
	for i := range entries {
		entries[i] = comment(time.Duration(i)*8*time.Second, ScrollRTL, "大量条目")
	}
	return installed(tb, entries)
}

func TestMaximumWindowUsesCachedBinarySearchSchedule(t *testing.T) {
	started := time.Now()
	timeline := maximumTimeline(t)
	lastPosition := time.Duration(MaxEntries-1)*8*time.Second + 4*time.Second
	frame := frameAt(t, timeline, lastPosition, 800, 1000)
	if len(frame.Items) != 1 || frame.Items[0].Index != MaxEntries-1 || frame.Items[0].Progress != .5 || frame.Stats.Overflow != 0 {
		t.Fatal("maximum window lookup lost the last entry", frame)
	}
	refs := 0
	for _, lane := range timeline.schedule.lanes {
		refs += len(lane)
	}
	if refs != MaxEntries {
		t.Fatal("schedule did not retain the bounded full window", refs)
	}
	prepared := time.Since(started)
	prior, builds := timeline.schedule, 0
	started = time.Now()
	for i := range 10000 {
		frameAt(t, timeline, time.Duration(i)*8*time.Second+4*time.Second, 800, 1000)
		if timeline.schedule != prior {
			builds++
			prior = timeline.schedule
		}
	}
	if builds != 0 {
		t.Fatal("per-frame lookup rebuilt the full schedule", builds)
	}
	elapsed := time.Since(started)
	allocations := testing.AllocsPerRun(100, func() { frameAt(t, timeline, lastPosition, 800, 1000) })
	t.Logf("40000-entry normalization/schedule: %s; 10000 cached lookups: %s; additional schedule builds: %d; frame allocations: %.0f", prepared, elapsed, builds, allocations)
}

func BenchmarkFrameMaximumInput(b *testing.B) {
	timeline := maximumTimeline(b)
	frameAt(b, timeline, 0, 800, 1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		frameAt(b, timeline, time.Duration(i%MaxEntries)*8*time.Second+4*time.Second, 800, 1000)
	}
}
