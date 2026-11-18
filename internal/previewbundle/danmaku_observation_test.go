package previewbundle

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

func activeDanmakuObservation() Observation {
	o := validObservation()
	o.State.Playback.State, o.State.Playback.Position = playback.Paused, 2*time.Second
	o.Danmaku = DanmakuObservation{
		Generation: 1, Revision: 2, Position: 2 * time.Second,
		Width: 640, Height: 360, Lanes: 10,
		Settings: danmaku.DefaultSettings(), Stats: danmaku.Stats{Input: 64, Loaded: 64, Overflow: 32},
		Active: 4, Layouts: 4,
		Items: []DanmakuItemObservation{
			{Index: 0, Mode: danmaku.FixedTop, Lane: 0, Progress: .5, X: 260, Y: 2, Width: 120, Height: 30},
			{Index: 1, Mode: danmaku.ScrollRTL, Lane: 2, Progress: .125, X: 535, Y: 70, Width: 200, Height: 30},
			{Index: 2, Mode: danmaku.ScrollLTR, Lane: 3, Progress: .0625, X: -110, Y: 104, Width: 160, Height: 30},
			{Index: 3, Mode: danmaku.FixedBottom, Lane: 9, Progress: .0625, X: 230, Y: 328, Width: 180, Height: 30},
		},
	}
	return o
}

func emptyDanmakuItems(o *Observation) {
	o.Danmaku.Active = 0
	o.Danmaku.Items = []DanmakuItemObservation{}
}

func closedDanmakuObservation() Observation {
	o := activeDanmakuObservation()
	o.Danmaku = DanmakuObservation{
		Generation: 1, Revision: 2, Closed: true,
		Settings: danmaku.Settings{Visible: false, Opacity: .4, FontSize: 30, Speed: 1.5},
		Items:    []DanmakuItemObservation{},
	}
	return o
}

func marshalDanmakuObservation(t *testing.T, o Observation) []byte {
	t.Helper()
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireDanmakuRoundTrip(t *testing.T, o Observation) {
	t.Helper()
	got, err := ParseObservation(marshalDanmakuObservation(t, o))
	if err != nil || !reflect.DeepEqual(got, o) {
		t.Fatalf("observation changed: error=%v, got=%+v, want=%+v", err, got, o)
	}
}

func requireInvalidDanmakuObservation(t *testing.T, data []byte) {
	t.Helper()
	got, err := ParseObservation(data)
	if err == nil || err.Error() != "invalid preview observation" || !reflect.DeepEqual(got, Observation{}) {
		t.Fatalf("invalid observation accepted, partially returned or disclosed: error=%v, got=%+v", err, got)
	}
}

func TestDanmakuObservationPreservesValidStateAndCounts(t *testing.T) {
	for _, state := range []playback.State{playback.Ready, playback.Paused, playback.Playing} {
		for _, rate := range []float64{.5, 1, 2, 3} {
			t.Run(string(state)+"-rate-"+strconv.FormatFloat(rate, 'f', -1, 64), func(t *testing.T) {
				o := activeDanmakuObservation()
				o.State.Playback.State, o.State.Playback.Rate, o.State.Rate = state, rate, rate
				requireDanmakuRoundTrip(t, o)
			})
		}
	}
	for _, code := range []danmaku.ErrorCode{danmaku.NoError, danmaku.Invalid, danmaku.Stale, danmaku.IsClosed, danmaku.Unavailable} {
		o := activeDanmakuObservation()
		o.Danmaku.Error = code
		requireDanmakuRoundTrip(t, o)
	}
	for name, mutate := range map[string]func(*Observation){
		"hidden":              func(o *Observation) { o.Danmaku.Settings.Visible = false },
		"zero-opacity":        func(o *Observation) { o.Danmaku.Settings.Opacity = 0 },
		"generation-mismatch": func(o *Observation) { o.Danmaku.Generation++ },
		"retired-time":        func(o *Observation) { o.Danmaku.Position = 0 },
		"layout-count":        func(o *Observation) { o.Danmaku.Layouts = 16 },
		"zero-width":          func(o *Observation) { o.Danmaku.Width, o.Danmaku.Lanes = 0, 0 },
		"zero-height":         func(o *Observation) { o.Danmaku.Height, o.Danmaku.Lanes = 0, 0 },
		"large-height":        func(o *Observation) { o.Danmaku.Height, o.Danmaku.Lanes = 1e300, 16 },
		"all-empty-text": func(o *Observation) {
			o.Danmaku.Stats = danmaku.Stats{Input: 64, SkippedEmpty: 64}
		},
	} {
		t.Run(name, func(t *testing.T) {
			o := activeDanmakuObservation()
			emptyDanmakuItems(&o)
			mutate(&o)
			requireDanmakuRoundTrip(t, o)
		})
	}
	for _, state := range []playback.State{playback.Idle, playback.Loading, playback.Failed, playback.Ended, playback.Closed} {
		o := activeDanmakuObservation()
		emptyDanmakuItems(&o)
		o.State.Playback.State = state
		requireDanmakuRoundTrip(t, o)
	}
	for _, opacity := range []float64{.4, 1} {
		o := activeDanmakuObservation()
		o.Danmaku.Settings.Opacity = opacity
		requireDanmakuRoundTrip(t, o)
	}
	requireDanmakuRoundTrip(t, validObservation())
	requireDanmakuRoundTrip(t, closedDanmakuObservation())
}

func TestDanmakuObservationRejectsContradictions(t *testing.T) {
	mutations := map[string]func(*Observation){
		"unknown-error": func(o *Observation) { o.Danmaku.Error = "private-sentinel" },
		"negative-position": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Position = -1
		},
		"negative-width": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Width, o.Danmaku.Lanes = -1, 0
		},
		"negative-height": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Width, o.Danmaku.Height, o.Danmaku.Lanes = 0, -1, 0
		},
		"opacity-low":  func(o *Observation) { o.Danmaku.Settings.Opacity = -.1 },
		"opacity-high": func(o *Observation) { o.Danmaku.Settings.Opacity = 1.1 },
		"font-low": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Width, o.Danmaku.Lanes, o.Danmaku.Settings.FontSize = 0, 0, 15
		},
		"font-high": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Width, o.Danmaku.Lanes, o.Danmaku.Settings.FontSize = 0, 0, 41
		},
		"speed-low":  func(o *Observation) { o.Danmaku.Settings.Speed = .4 },
		"speed-high": func(o *Observation) { o.Danmaku.Settings.Speed = 2.1 },
		"input-cap": func(o *Observation) {
			o.Danmaku.Stats.Input, o.Danmaku.Stats.Loaded = 65, 65
		},
		"negative-input": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Stats = danmaku.Stats{Input: -1, SkippedEmpty: -1}
		},
		"negative-loaded": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Stats = danmaku.Stats{Input: 64, Loaded: -1, SkippedEmpty: 65}
		},
		"loaded-exceeds-input": func(o *Observation) {
			o.Danmaku.Stats.Loaded, o.Danmaku.Stats.SkippedEmpty = 65, -1
		},
		"empty-accounting":        func(o *Observation) { o.Danmaku.Stats.SkippedEmpty = 1 },
		"negative-overflow":       func(o *Observation) { o.Danmaku.Stats.Overflow = -1 },
		"overflow-exceeds-loaded": func(o *Observation) { o.Danmaku.Stats.Overflow = 65 },
		"negative-layouts":        func(o *Observation) { o.Danmaku.Layouts = -1 },
		"layout-cap":              func(o *Observation) { o.Danmaku.Layouts = 17 },
		"negative-active":         func(o *Observation) { o.Danmaku.Active = -1 },
		"active-count":            func(o *Observation) { o.Danmaku.Active++ },
		"lane-count": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Lanes = 9
		},
		"negative-lanes": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Lanes = -1
		},
		"lane-cap": func(o *Observation) {
			emptyDanmakuItems(o)
			o.Danmaku.Height, o.Danmaku.Lanes = 17*34, 17
		},
		"hidden-active":              func(o *Observation) { o.Danmaku.Settings.Visible = false },
		"zero-opacity-active":        func(o *Observation) { o.Danmaku.Settings.Opacity = 0 },
		"closed-active":              func(o *Observation) { o.Danmaku.Closed = true },
		"generation-mismatch-active": func(o *Observation) { o.Danmaku.Generation++ },
		"media-time-mismatch":        func(o *Observation) { o.Danmaku.Position++ },
		"unknown-mode": func(o *Observation) {
			o.Danmaku.Items[0].Mode, o.Danmaku.Items[0].Lane, o.Danmaku.Items[0].Y = 2, 4, 138
		},
		"negative-index":  func(o *Observation) { o.Danmaku.Items[0].Index = -1 },
		"index-at-input":  func(o *Observation) { o.Danmaku.Items[0].Index = 64 },
		"duplicate-index": func(o *Observation) { o.Danmaku.Items[1].Index = 0 },
		"duplicate-lane": func(o *Observation) {
			o.Danmaku.Items[2].Lane, o.Danmaku.Items[2].Y = 2, 70
		},
		"negative-lane": func(o *Observation) {
			o.Danmaku.Items[0].Lane, o.Danmaku.Items[0].Y = -1, -32
		},
		"lane-at-count": func(o *Observation) {
			o.Danmaku.Items[0].Lane, o.Danmaku.Items[0].Y = 10, 362
		},
		"top-lane-pool": func(o *Observation) {
			o.Danmaku.Items[0].Lane, o.Danmaku.Items[0].Y = 4, 138
		},
		"scroll-lane-pool": func(o *Observation) {
			o.Danmaku.Items[1].Lane, o.Danmaku.Items[1].Y = 1, 36
		},
		"bottom-lane-pool": func(o *Observation) {
			o.Danmaku.Items[3].Lane, o.Danmaku.Items[3].Y = 7, 240
		},
		"negative-progress": func(o *Observation) { o.Danmaku.Items[0].Progress = -.1 },
		"interval-end":      func(o *Observation) { o.Danmaku.Items[0].Progress = 1 },
		"zero-shaped-width": func(o *Observation) {
			o.Danmaku.Items[0].Width, o.Danmaku.Items[0].X = 0, 320
		},
		"negative-shaped-width": func(o *Observation) {
			o.Danmaku.Items[0].Width, o.Danmaku.Items[0].X = -2, 321
		},
		"zero-shaped-height": func(o *Observation) {
			o.Danmaku.Items[0].Height, o.Danmaku.Items[0].Y = 0, 17
		},
		"negative-shaped-height": func(o *Observation) {
			o.Danmaku.Items[0].Height, o.Danmaku.Items[0].Y = -2, 18
		},
		"rtl-x":    func(o *Observation) { o.Danmaku.Items[1].X++ },
		"ltr-x":    func(o *Observation) { o.Danmaku.Items[2].X++ },
		"fixed-x":  func(o *Observation) { o.Danmaku.Items[0].X++ },
		"lane-y":   func(o *Observation) { o.Danmaku.Items[1].Y++ },
		"bottom-y": func(o *Observation) { o.Danmaku.Items[3].Y++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			o := activeDanmakuObservation()
			requireDanmakuRoundTrip(t, o)
			mutate(&o)
			requireInvalidDanmakuObservation(t, marshalDanmakuObservation(t, o))
		})
	}
	for _, state := range []playback.State{playback.Idle, playback.Loading, playback.Failed, playback.Ended, playback.Closed} {
		t.Run("inactive-"+string(state), func(t *testing.T) {
			o := activeDanmakuObservation()
			o.State.Playback.State = state
			requireInvalidDanmakuObservation(t, marshalDanmakuObservation(t, o))
		})
	}
}

func TestDanmakuObservationClosedRelease(t *testing.T) {
	for name, mutate := range map[string]func(*DanmakuObservation){
		"layouts":  func(d *DanmakuObservation) { d.Layouts = 1 },
		"stats":    func(d *DanmakuObservation) { d.Stats = danmaku.Stats{Input: 1, Loaded: 1, Overflow: 1} },
		"position": func(d *DanmakuObservation) { d.Position = 1 },
		"width":    func(d *DanmakuObservation) { d.Width = 1 },
		"height":   func(d *DanmakuObservation) { d.Height = 1 },
		"lanes":    func(d *DanmakuObservation) { d.Lanes = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			o := closedDanmakuObservation()
			requireDanmakuRoundTrip(t, o)
			mutate(&o.Danmaku)
			requireInvalidDanmakuObservation(t, marshalDanmakuObservation(t, o))
		})
	}
}

func TestDanmakuObservationGoldenGeometry(t *testing.T) {
	for _, tc := range []struct {
		name                string
		width, height, font float64
		lanes               int
		item                DanmakuItemObservation
	}{
		{"one-rtl", 640, 40, 24, 1, DanmakuItemObservation{Mode: danmaku.ScrollRTL, Progress: .25, X: 455, Y: 5, Width: 100, Height: 30}},
		{"one-ltr", 640, 40, 24, 1, DanmakuItemObservation{Mode: danmaku.ScrollLTR, Progress: .25, X: 85, Y: 5, Width: 100, Height: 30}},
		{"one-top", 640, 40, 24, 1, DanmakuItemObservation{Mode: danmaku.FixedTop, Progress: .25, X: 270, Y: 5, Width: 100, Height: 30}},
		{"one-bottom", 640, 40, 24, 1, DanmakuItemObservation{Mode: danmaku.FixedBottom, Progress: .25, X: 270, Y: 5, Width: 100, Height: 30}},
		{"two-top", 640, 80, 24, 2, DanmakuItemObservation{Mode: danmaku.FixedTop, X: 270, Y: 2, Width: 100, Height: 30}},
		{"two-bottom", 640, 80, 24, 2, DanmakuItemObservation{Mode: danmaku.FixedBottom, Lane: 1, X: 270, Y: 48, Width: 100, Height: 30}},
		{"two-scroll", 640, 80, 24, 2, DanmakuItemObservation{Mode: danmaku.ScrollRTL, Lane: 1, Progress: .25, X: 455, Y: 48, Width: 100, Height: 30}},
		{"three-scroll", 640, 112, 24, 3, DanmakuItemObservation{Mode: danmaku.ScrollRTL, Lane: 1, Progress: .25, X: 455, Y: 36, Width: 100, Height: 30}},
		{"three-bottom", 640, 112, 24, 3, DanmakuItemObservation{Mode: danmaku.FixedBottom, Lane: 2, X: 270, Y: 80, Width: 100, Height: 30}},
		{"capped-bottom", 640, 800, 24, 16, DanmakuItemObservation{Mode: danmaku.FixedBottom, Lane: 15, X: 270, Y: 768, Width: 100, Height: 30}},
		{"long-tall-clipped", 640, 40, 24, 1, DanmakuItemObservation{Mode: danmaku.FixedTop, X: -180, Y: -5, Width: 1000, Height: 50}},
		{"small-font", 640, 80, 16, 3, DanmakuItemObservation{Mode: danmaku.FixedBottom, Lane: 2, X: 270, Y: 58, Width: 100, Height: 20}},
		{"large-font", 640, 120, 40, 2, DanmakuItemObservation{Mode: danmaku.ScrollLTR, Lane: 1, Progress: .25, X: 85, Y: 68, Width: 100, Height: 50}},
		{"fractional-size", 637.5, 80.5, 24, 2, DanmakuItemObservation{Mode: danmaku.FixedBottom, Lane: 1, X: 268.75, Y: 48.5, Width: 100, Height: 30}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := activeDanmakuObservation()
			o.Danmaku.Width, o.Danmaku.Height, o.Danmaku.Lanes = tc.width, tc.height, tc.lanes
			o.Danmaku.Settings.FontSize = tc.font
			o.Danmaku.Active, o.Danmaku.Layouts, o.Danmaku.Items = 1, 1, []DanmakuItemObservation{tc.item}
			for _, speed := range []float64{.5, 2} {
				o.Danmaku.Settings.Speed = speed
				requireDanmakuRoundTrip(t, o)
			}
		})
	}
}

func TestDanmakuObservationMaximumActiveAndExclusiveLanes(t *testing.T) {
	o := activeDanmakuObservation()
	o.Danmaku.Height, o.Danmaku.Lanes, o.Danmaku.Active, o.Danmaku.Layouts = 800, 16, 16, 16
	o.Danmaku.Items = []DanmakuItemObservation{}
	for lane, top := range []float64{0, 34, 68, 102, 136, 170, 204, 238, 272, 306, 340, 374, 664, 698, 732, 766} {
		mode := danmaku.ScrollRTL
		if lane < 4 {
			mode = danmaku.FixedTop
		} else if lane >= 12 {
			mode = danmaku.FixedBottom
		}
		o.Danmaku.Items = append(o.Danmaku.Items, DanmakuItemObservation{
			Index: lane, Mode: mode, Lane: lane, Progress: .5, X: 260, Y: top + 2, Width: 120, Height: 30,
		})
	}
	requireDanmakuRoundTrip(t, o)
	o.Danmaku.Items = append(o.Danmaku.Items, o.Danmaku.Items[0])
	o.Danmaku.Active++
	requireInvalidDanmakuObservation(t, marshalDanmakuObservation(t, o))

	// Two independently valid items in one lane violate both exclusivity and
	// Active <= Lanes. No count mismatch or ineligible coordinate masks the rule.
	o = activeDanmakuObservation()
	o.Danmaku.Height, o.Danmaku.Lanes, o.Danmaku.Active, o.Danmaku.Layouts = 40, 1, 2, 2
	o.Danmaku.Items = []DanmakuItemObservation{
		{Index: 0, Mode: danmaku.FixedTop, X: 270, Y: 5, Width: 100, Height: 30},
		{Index: 1, Mode: danmaku.FixedBottom, X: 270, Y: 5, Width: 100, Height: 30},
	}
	requireInvalidDanmakuObservation(t, marshalDanmakuObservation(t, o))
}

func TestDanmakuObservationStrictFieldsAndEnvelope(t *testing.T) {
	data := marshalDanmakuObservation(t, activeDanmakuObservation())
	testRequiredObservationFields(t, data)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"Danmaku": string(root["Danmaku"]), "Generation": "1", "Opacity": "0.9", "Input": "64", "Index": "0",
	} {
		needle := strconv.Quote(key) + ":"
		for name, replacement := range map[string]string{
			"case-alias": strconv.Quote(strings.ToLower(key)) + ":",
			"duplicate":  needle + value + "," + needle,
			"unknown":    "\"private-field\":\"private-sentinel\"," + needle,
		} {
			t.Run(key+"-"+name, func(t *testing.T) {
				bad := strings.Replace(string(data), needle, replacement, 1)
				if bad == string(data) {
					t.Fatal("test did not mutate the intended field")
				}
				requireInvalidDanmakuObservation(t, []byte(bad))
			})
		}
	}
	for _, field := range []string{"Text", "Color", "Location", "AccountID", "Layout"} {
		bad := strings.Replace(string(data), "\"Index\":", strconv.Quote(field)+":\"private-sentinel\",\"Index\":", 1)
		requireInvalidDanmakuObservation(t, []byte(bad))
	}
	for _, field := range []string{
		"\"Width\":640", "\"Height\":360", "\"Opacity\":0.9", "\"FontSize\":24", "\"Speed\":1",
		"\"Progress\":0.5", "\"X\":260", "\"Y\":2", "\"Width\":120", "\"Height\":30",
	} {
		name, _, ok := strings.Cut(field, ":")
		if !ok || !strings.Contains(string(data), field) {
			t.Fatal("numeric test does not name an existing field", field)
		}
		for _, invalidNumber := range []string{"1e309", "-1e309", "NaN", "\"NaN\"", "Infinity"} {
			bad := strings.Replace(string(data), field, name+":"+invalidNumber, 1)
			requireInvalidDanmakuObservation(t, []byte(bad))
		}
	}
	atLimit := append(append([]byte{}, data...), []byte(strings.Repeat(" ", 65536-len(data)))...)
	if _, err := ParseObservation(atLimit); err != nil {
		t.Fatal("valid observation at the existing envelope bound rejected", err)
	}
	requireInvalidDanmakuObservation(t, append(atLimit, ' '))
}
