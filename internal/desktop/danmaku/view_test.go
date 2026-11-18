package danmakuview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"reflect"
	"runtime"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

const canvasX, canvasY = 80, 60

var canvasColor = gui.RGB(35, 57, 79)

type renderHarness struct {
	view          *View
	snapshot      playback.Snapshot
	width, height float32
	ui            *gui.Tester
}

func installed(t *testing.T, entries []danmaku.Entry) *View {
	t.Helper()
	v := New()
	if err := v.Reset(1); err != nil {
		t.Fatal(err)
	}
	if err := v.SetEntries(1, entries); err != nil {
		t.Fatal(err)
	}
	return v
}

func comment(mode danmaku.Mode, text string) danmaku.Entry {
	return danmaku.Entry{Mode: mode, Text: text, Color: 0xffffff}
}

func paused(position time.Duration) playback.Snapshot {
	return playback.Snapshot{Generation: 1, State: playback.Paused, Position: position, Rate: 1}
}

func newHarness(v *View, width, height float32) *renderHarness {
	h := &renderHarness{view: v, snapshot: paused(2 * time.Second), width: width, height: height}
	h.ui = gui.NewTester(func(c *gui.Context) {
		gui.Box(c).Fill().Background(canvasColor).Children(func() {
			// These containers deliberately have no clip: only the renderer may
			// prevent a long or tall glyph from escaping the video and its lane.
			gui.Box(c).Absolute().Left(canvasX).Top(canvasY).Size(h.width, h.height).Children(func() {
				h.view.Render(c, h.snapshot, h.width, h.height)
			})
		})
	}, 640, 480)
	return h
}

func painted(draw func(*gui.Painter, gui.Rect), width, height float32) *image.RGBA {
	ui := gui.NewTester(func(c *gui.Context) {
		gui.Box(c).Fill().Background(canvasColor).Children(func() {
			gui.Box(c).Absolute().Left(canvasX).Top(canvasY).Size(width, height).Draw(draw)
		})
	}, 640, 480)
	return cloneImage(ui.Image())
}

func cloneImage(source *image.RGBA) *image.RGBA {
	copy := *source
	copy.Pix = bytes.Clone(source.Pix)
	return &copy
}

func changedPixels(a, b *image.RGBA, inside image.Rectangle) int {
	count := 0
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		for x := a.Rect.Min.X; x < a.Rect.Max.X; x++ {
			if image.Pt(x, y).In(inside) && a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				count++
			}
		}
	}
	return count
}

func noPixelsOutside(t *testing.T, got, blank *image.RGBA, allowed image.Rectangle) {
	t.Helper()
	for y := got.Rect.Min.Y; y < got.Rect.Max.Y; y++ {
		for x := got.Rect.Min.X; x < got.Rect.Max.X; x++ {
			if !image.Pt(x, y).In(allowed) && got.RGBAAt(x, y) != blank.RGBAAt(x, y) {
				t.Fatalf("paint escaped %v at (%d,%d)", allowed, x, y)
			}
		}
	}
}

func configure(t *testing.T, v *View, change func(*danmaku.Settings)) {
	t.Helper()
	settings := v.Settings()
	change(&settings)
	if err := v.Configure(settings); err != nil {
		t.Fatal(err)
	}
}

func TestStateDefaultsCopiesAndPlainData(t *testing.T) {
	v := New()
	if s := v.State(); s.Settings != danmaku.DefaultSettings() || s.Items == nil || s.Generation != 0 || s.Active != 0 || s.Closed {
		t.Fatalf("initial state: %+v", s)
	}
	v = installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "private comment is not observable")})
	v.prepare(paused(time.Second), 400, 272)
	s := v.State()
	if s.Active != 1 || s.Layouts != 1 || len(s.Items) != 1 {
		t.Fatalf("prepared state: %+v", s)
	}
	s.Items[0].X = 9999
	if v.State().Items[0].X == 9999 {
		t.Fatal("State aliases owned item data")
	}
	raw, err := json.Marshal(v.State())
	if err != nil || bytes.Contains(raw, []byte("private comment")) {
		t.Fatalf("unsafe snapshot: %s (%v)", raw, err)
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[State](), reflect.TypeFor[ItemState]()} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Name == "Text" || f.Name == "Color" || f.Type.Kind() == reflect.Pointer || f.Type.Kind() == reflect.Func {
				t.Fatalf("unexpected observable field: %v", f)
			}
		}
	}
}

func TestMeasuredGeometryAndMediaTime(t *testing.T) {
	entries := []danmaku.Entry{
		comment(danmaku.FixedTop, "中文弹幕测试"),
		comment(danmaku.ScrollRTL, "a long right-to-left comment 中文"),
		comment(danmaku.ScrollLTR, "逆向弹幕"),
		comment(danmaku.FixedBottom, "底部"),
	}
	v := installed(t, entries)
	calls := 0
	v.shape = func(text string, font gui.Font) shapedText {
		calls++
		if font.Size != 24 || font.Weight != 600 || font.Lang != "zh-Hans" || font.LineHeight != 1.25 || font.Family != "" {
			t.Fatalf("font: %+v", font)
		}
		return shapeText(text, font)
	}
	h := newHarness(v, 400, 272)
	s := v.State()
	if s.Active != 4 || s.Layouts != 4 || s.Lanes != 8 {
		t.Fatalf("geometry state: %+v", s)
	}
	wantLanes := []int{0, 2, 3, 7}
	wantTops := []float64{0, 68, 102, 238}
	for i, item := range s.Items {
		layout := gui.ShapeText(entries[i].Text, gui.Font{Size: 24, Weight: 600, Lang: "zh-Hans", LineHeight: 1.25}, 0)
		w, height := layout.Size()
		progress, x := .5, (400-float64(w))/2
		if item.Mode == danmaku.ScrollRTL {
			progress, x = .25, 400-.25*(400+float64(w))
		}
		if item.Mode == danmaku.ScrollLTR {
			progress, x = .25, -float64(w)+.25*(400+float64(w))
		}
		want := ItemState{Index: i, Mode: entries[i].Mode, Lane: wantLanes[i], Progress: progress, X: x, Y: wantTops[i] + (34-float64(height))/2, Width: float64(w), Height: float64(height)}
		if item != want {
			t.Fatalf("item %d: %+v want %+v", i, item, want)
		}
	}
	before := cloneImage(h.ui.Image())
	h.ui.Advance(3 * time.Second)
	h.snapshot.Rate = 2
	h.ui.Frame()
	if !reflect.DeepEqual(s, v.State()) || !bytes.Equal(before.Pix, h.ui.Image().Pix) || calls != 4 {
		t.Fatal("paused repaint/rate change moved or reshaped text")
	}
	h.snapshot.Position = 3 * time.Second
	h.snapshot.State = playback.Playing
	h.ui.Frame()
	advanced := v.State()
	if advanced.Items[1].X >= s.Items[1].X || advanced.Items[2].X <= s.Items[2].X || calls != 4 {
		t.Fatal("media-position progression did not move both directions without reshaping")
	}
	h.snapshot = paused(2 * time.Second)
	h.ui.Frame()
	if !reflect.DeepEqual(s, v.State()) || !bytes.Equal(before.Pix, h.ui.Image().Pix) {
		t.Fatal("seeking back did not restore the same prepared frame and pixels")
	}
}

func TestVisibilityOpacityAndColorPixels(t *testing.T) {
	t.Run("portable", func(t *testing.T) { checkVisibilityOpacityAndColorPixels(t, "MMMM中文弹幕测试") })
	// The renderer uses system fonts; a headless Linux image may lack Han glyphs.
	if runtime.GOOS != "linux" {
		t.Run("Chinese", func(t *testing.T) { checkVisibilityOpacityAndColorPixels(t, "中文弹幕测试") })
	}
}

func checkVisibilityOpacityAndColorPixels(t *testing.T, text string) {
	t.Helper()
	entry := comment(danmaku.FixedTop, text)
	entry.Color = 0xff4050
	v := installed(t, []danmaku.Entry{entry})
	h := newHarness(v, 400, 136)
	visible := cloneImage(h.ui.Image())
	configure(t, v, func(s *danmaku.Settings) { s.Visible = false })
	h.ui.Frame()
	blank := cloneImage(h.ui.Image())
	if changedPixels(visible, blank, visible.Rect) < 40 || v.State().Active != 0 || len(v.layouts) != 0 {
		t.Fatal("visible text did not paint, or hiding retained layouts")
	}
	configure(t, v, func(s *danmaku.Settings) { s.Visible, s.Opacity = true, 0 })
	h.ui.Frame()
	if !bytes.Equal(blank.Pix, h.ui.Image().Pix) || v.State().Layouts != 0 {
		t.Fatal("zero opacity still painted or retained active layouts")
	}
	for _, opacity := range []float64{.25, 1} {
		configure(t, v, func(s *danmaku.Settings) { s.Opacity = opacity })
		h.ui.Frame()
		got := cloneImage(h.ui.Image())
		item := v.State().Items[0]
		layout := gui.ShapeText(entry.Text, gui.Font{Size: 24, Weight: 600, Lang: "zh-Hans", LineHeight: 1.25}, 0)
		want := painted(func(p *gui.Painter, r gui.Rect) {
			x, y := r.X+float32(item.X), r.Y+float32(item.Y)
			p.TextLayout(layout, x+1, y+1, gui.RGBA(0, 0, 0, float32(opacity)))
			p.TextLayout(layout, x, y, gui.RGBA(255, 64, 80, float32(opacity)))
		}, 400, 136)
		if !bytes.Equal(got.Pix, want.Pix) || changedPixels(got, blank, got.Rect) < 40 {
			t.Fatalf("foreground/shadow color or opacity differs at %v", opacity)
		}
	}
}

func TestVideoAndLaneClipping(t *testing.T) {
	t.Run("long text and shadow", func(t *testing.T) {
		v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "MMMMMMMMMMMMMMMMMMMMMMMM")})
		h := newHarness(v, 80, 136)
		got := cloneImage(h.ui.Image())
		blank := painted(func(*gui.Painter, gui.Rect) {}, 80, 136)
		allowed := image.Rect(canvasX, canvasY, canvasX+80, canvasY+34)
		if changedPixels(got, blank, allowed) < 40 {
			t.Fatal("clipping test has no visible text")
		}
		noPixelsOutside(t, got, blank, allowed)
	})
	t.Run("tall layout in a valid small lane", func(t *testing.T) {
		v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "MMMM")})
		configure(t, v, func(s *danmaku.Settings) { s.FontSize = 16 })
		v.shape = func(text string, font gui.Font) shapedText {
			font.Size = 80
			return shapeText(text, font)
		}
		h := newHarness(v, 400, 144)
		got := cloneImage(h.ui.Image())
		blank := painted(func(*gui.Painter, gui.Rect) {}, 400, 144)
		allowed := image.Rect(canvasX, canvasY, canvasX+400, canvasY+24)
		if changedPixels(got, blank, allowed) < 40 {
			t.Fatal("lane test has no visible part of tall text")
		}
		noPixelsOutside(t, got, blank, allowed)
	})
}

func TestDrawingClipsActualSurfaceBounds(t *testing.T) {
	v := installed(t, []danmaku.Entry{comment(danmaku.FixedBottom, "MMMMMMM")})
	frame := v.prepare(paused(time.Second), 200, 136)
	// The native callback receives the actual laid-out rectangle. A shorter
	// rectangle must clip even a lane prepared for the requested 136 DIPs.
	got := painted(v.draw(frame), 200, 120)
	blank := painted(func(*gui.Painter, gui.Rect) {}, 200, 120)
	allowed := image.Rect(canvasX, canvasY, canvasX+200, canvasY+120)
	if changedPixels(got, blank, allowed) < 10 {
		t.Fatal("surface test has no visible part of bottom text")
	}
	noPixelsOutside(t, got, blank, allowed)
}

func TestActiveLayoutReuseEvictionAndRetirement(t *testing.T) {
	entries := make([]danmaku.Entry, 40)
	for i := range entries {
		entries[i] = comment(danmaku.ScrollRTL, fmt.Sprintf("interval %d", i))
		entries[i].At = time.Duration(i) * 9 * time.Second
	}
	v := installed(t, entries)
	shapes := 0
	v.shape = func(text string, font gui.Font) shapedText {
		shapes++
		return shapeText(text, font)
	}
	for i := range entries {
		sample := paused(entries[i].At + time.Second)
		v.prepare(sample, 400, 272)
		v.prepare(sample, 400, 272)
		if len(v.layouts) != 1 || v.State().Layouts != 1 || shapes != i+1 {
			t.Fatalf("interval %d retained %d layouts, reported %d, shaped %d", i, len(v.layouts), v.State().Layouts, shapes)
		}
	}
	v.prepare(paused(400*time.Second), 400, 272)
	if len(v.layouts) != 0 || v.State().Active != 0 {
		t.Fatal("inactive layouts were retained")
	}
	entries = []danmaku.Entry{comment(danmaku.FixedTop, "short")}
	if err := v.SetEntries(1, entries); err != nil {
		t.Fatal(err)
	}
	v.prepare(paused(time.Second), 400, 272)
	old := v.State()
	before := shapes
	entries[0].Text = "a much longer replacement at the same input index"
	if err := v.SetEntries(1, entries); err != nil {
		t.Fatal(err)
	}
	if len(v.layouts) != 0 || v.State().Layouts != 0 {
		t.Fatal("replacement kept previous layouts before rendering")
	}
	v.prepare(paused(time.Second), 400, 272)
	if shapes != before+1 || v.State().Revision != old.Revision+1 || v.State().Items[0].Width <= old.Items[0].Width {
		t.Fatal("replacement reused stale text geometry")
	}
	before = shapes
	configure(t, v, func(s *danmaku.Settings) { s.FontSize = 36 })
	if len(v.layouts) != 0 || v.State().Layouts != 0 {
		t.Fatal("font change kept old layout references")
	}
	v.prepare(paused(time.Second), 400, 272)
	if shapes != before+1 || v.State().Items[0].Height <= old.Items[0].Height {
		t.Fatal("font change did not shape the new size")
	}
	if err := v.Reset(2); err != nil {
		t.Fatal(err)
	}
	if len(v.layouts) != 0 || v.State().Stats != (danmaku.Stats{}) || v.State().Generation != 2 || v.Settings().FontSize != 36 {
		t.Fatal("new generation did not retire the old frame while keeping settings")
	}
}

func TestSixteenActiveLayoutsDoNotAccumulate(t *testing.T) {
	entries := make([]danmaku.Entry, 16)
	for i := range entries {
		mode := []danmaku.Mode{danmaku.FixedTop, danmaku.FixedBottom, danmaku.ScrollRTL, danmaku.ScrollLTR}[i/4]
		entries[i] = comment(mode, fmt.Sprintf("visible %d", i))
	}
	v := installed(t, entries)
	first := v.prepare(paused(time.Second), 640, 600)
	if len(first.items) != 16 || len(v.layouts) != 16 || v.State().Layouts != 16 {
		t.Fatalf("full active frame: %+v", v.State())
	}
	if err := v.SetEntries(1, entries); err != nil {
		t.Fatal(err)
	}
	second := v.prepare(paused(time.Second), 640, 600)
	if len(v.layouts) != 16 || v.State().Layouts != 16 || len(first.items)+len(second.items) > 32 {
		t.Fatal("outgoing/current layout ownership accumulated")
	}
	for key := range v.layouts {
		if key.revision != v.State().Revision {
			t.Fatal("old revision still retained by view")
		}
	}
}

func TestPaintIsImmutableAndClosedCallbackCannotPaint(t *testing.T) {
	v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "retained callback")})
	shapes := 0
	v.shape = func(text string, font gui.Font) shapedText {
		shapes++
		return shapeText(text, font)
	}
	frame := v.prepare(paused(time.Second), 400, 136)
	callback := v.draw(frame)
	state := v.State()
	first := painted(callback, 400, 136)
	second := painted(callback, 400, 136)
	blank := painted(func(*gui.Painter, gui.Rect) {}, 400, 136)
	if shapes != 1 || !reflect.DeepEqual(state, v.State()) || !bytes.Equal(first.Pix, second.Pix) || bytes.Equal(first.Pix, blank.Pix) {
		t.Fatal("repeated paint mutated preparation, did not paint or changed pixels")
	}
	v.Close()
	closed := v.State()
	if len(v.layouts) != 0 || !closed.Closed || closed.Items == nil || len(closed.Items) != 0 || closed.Active != 0 || closed.Layouts != 0 || closed.Stats != (danmaku.Stats{}) || closed.Width != 0 || closed.Height != 0 || closed.Position != 0 || closed.Lanes != 0 || closed.Generation != state.Generation || closed.Revision != state.Revision || closed.Settings != state.Settings {
		t.Fatalf("close did not release its owned frame: %+v (cache %d)", closed, len(v.layouts))
	}
	// Repaint the real previously prepared closure, not an already-empty
	// newly prepared frame. The lifetime guard is independently necessary.
	if got := painted(callback, 400, 136); !bytes.Equal(blank.Pix, got.Pix) || shapes != 1 {
		t.Fatal("an outgoing callback painted after Close")
	}
	v.Close()
	for _, action := range []func() error{
		func() error { return v.Reset(2) },
		func() error { return v.SetEntries(1, nil) },
		func() error { return v.SetUnavailable(1) },
		func() error { return v.Configure(danmaku.DefaultSettings()) },
	} {
		if err := action(); err != danmaku.IsClosed || !reflect.DeepEqual(closed, v.State()) {
			t.Fatalf("closed write changed state: %v %+v", err, v.State())
		}
	}
}

func TestClosedPlaybackSampleClosesView(t *testing.T) {
	v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "closing")})
	h := newHarness(v, 400, 136)
	if len(v.layouts) == 0 {
		t.Fatal("test never acquired a layout")
	}
	h.snapshot.State = playback.Closed
	h.ui.Frame()
	if s := v.State(); !s.Closed || s.Layouts != 0 || len(v.layouts) != 0 || s.Stats != (danmaku.Stats{}) {
		t.Fatalf("closed sample: %+v", s)
	}
	if err := v.Reset(2); err != danmaku.IsClosed {
		t.Fatal("Closed playback did not terminally close view")
	}
}

func TestPersistentErrorsAreOperationSpecific(t *testing.T) {
	entries := []danmaku.Entry{comment(danmaku.FixedTop, "still usable")}
	v := installed(t, entries)
	h := newHarness(v, 400, 136)
	bad := []danmaku.Entry{comment(danmaku.FixedTop, "bad\x00text")}
	if err := v.SetEntries(1, bad); err != danmaku.Invalid {
		t.Fatal(err)
	}
	if err := v.Configure(v.Settings()); err != nil || v.State().Error != danmaku.Invalid {
		t.Fatal("unrelated config success cleared the entry error")
	}
	h.ui.Frame()
	if v.State().Active != 1 || v.State().Error != danmaku.Invalid {
		t.Fatal("invalid replacement removed prior valid text or the persistent error")
	}
	settings := v.Settings()
	settings.FontSize = 0
	if err := v.Configure(settings); err != danmaku.Invalid {
		t.Fatal(err)
	}
	if err := v.SetEntries(1, entries); err != nil || v.State().Error != danmaku.Invalid {
		t.Fatal("unrelated entry success cleared the configuration error")
	}
	if err := v.Configure(v.Settings()); err != nil || v.State().Error != danmaku.NoError {
		t.Fatal("successful corresponding operation did not clear its error")
	}
	if err := v.SetUnavailable(1); err != nil || v.State().Error != danmaku.Unavailable || v.State().Stats.Input != 0 || len(v.layouts) != 0 {
		t.Fatal("unavailable did not publish an empty window")
	}
	if err := v.Configure(v.Settings()); err != nil || v.State().Error != danmaku.Unavailable {
		t.Fatal("unrelated config erased unavailable installation")
	}
	s := v.State()
	for _, action := range []func() error{
		func() error { return v.SetEntries(2, entries) },
		func() error { return v.SetUnavailable(2) },
	} {
		if err := action(); err != danmaku.Stale || !reflect.DeepEqual(s, v.State()) {
			t.Fatal("stale operation changed visible state")
		}
	}
	if err := v.Reset(1); err != nil || !reflect.DeepEqual(s, v.State()) {
		t.Fatal("equal Reset was not idempotent")
	}
	if err := v.Reset(2); err != nil || v.State().Error != danmaku.NoError {
		t.Fatal("new generation did not clear persistent errors")
	}
	if err := v.SetEntries(1, entries); err != danmaku.Stale || v.State().Stats.Input != 0 {
		t.Fatal("old generation could install text after Reset")
	}
}

func TestInvalidLayoutsSkipOnlyTheBadItemAndRecover(t *testing.T) {
	for _, invalid := range []struct {
		name string
		bad  func(shapedText) shapedText
	}{
		{"nil", func(s shapedText) shapedText { s.layout = nil; return s }},
		{"zero width", func(s shapedText) shapedText { s.width = 0; return s }},
		{"zero height", func(s shapedText) shapedText { s.height = 0; return s }},
		{"nan width", func(s shapedText) shapedText { s.width = float32(math.NaN()); return s }},
		{"infinite height", func(s shapedText) shapedText { s.height = float32(math.Inf(1)); return s }},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "good"), comment(danmaku.FixedBottom, "bad")})
			broken := true
			v.shape = func(text string, font gui.Font) shapedText {
				s := shapeText(text, font)
				if broken && text == "bad" {
					return invalid.bad(s)
				}
				return s
			}
			h := newHarness(v, 400, 136)
			if s := v.State(); s.Active != 1 || s.Layouts != 1 || s.Items[0].Index != 0 || s.Error != danmaku.Unavailable {
				t.Fatalf("layout failure was not isolated: %+v", s)
			}
			blank := painted(func(*gui.Painter, gui.Rect) {}, 400, 136)
			if changedPixels(h.ui.Image(), blank, image.Rect(canvasX, canvasY+20, canvasX+400, canvasY+34)) == 0 {
				t.Fatal("good sibling did not paint")
			}
			broken = false
			h.ui.Frame()
			if s := v.State(); s.Error != danmaku.NoError || s.Active != 2 || s.Layouts != 2 {
				t.Fatalf("valid next frame did not recover: %+v", s)
			}
		})
	}
}

func TestPersistentErrorWinsOverTransientLayoutFailure(t *testing.T) {
	v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "bad layout")})
	v.shape = func(string, gui.Font) shapedText { return shapedText{} }
	settings := v.Settings()
	settings.Opacity = -1
	if err := v.Configure(settings); err != danmaku.Invalid {
		t.Fatal(err)
	}
	v.prepare(paused(time.Second), 400, 136)
	if v.State().Error != danmaku.Invalid {
		t.Fatal("transient layout failure overwrote persistent invalid input")
	}
	v.shape = shapeText
	v.prepare(paused(time.Second), 400, 136)
	if v.State().Error != danmaku.Invalid || v.State().Active != 1 {
		t.Fatal("successful layout cleared an unrelated persistent error")
	}
	if err := v.Configure(v.Settings()); err != nil || v.State().Error != danmaku.NoError {
		t.Fatal("corresponding success could not clear recovered error")
	}
	v.prepare(paused(time.Second), float32(math.NaN()), 136)
	if v.State().Error != danmaku.Invalid || v.State().Active != 0 || len(v.layouts) != 0 {
		t.Fatal("invalid frame retained a drawable state")
	}
	v.prepare(paused(time.Second), 400, 136)
	if v.State().Error != danmaku.NoError || v.State().Active != 1 {
		t.Fatal("valid frame did not recover geometry error")
	}
}

func TestSurfaceAndErrorLabelPassThrough(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(fmt.Sprint(unavailable), func(t *testing.T) {
			v := installed(t, []danmaku.Entry{comment(danmaku.FixedTop, "overlay")})
			if unavailable {
				if err := v.SetUnavailable(1); err != nil {
					t.Fatal(err)
				}
			}
			clicks := 0
			ui := gui.NewTester(func(c *gui.Context) {
				gui.Box(c).Fill().Children(func() {
					if gui.Button(c.Key("underlying-video"), "Under video").Absolute().Left(0).Top(0).Size(400, 136).Clicked() {
						clicks++
					}
					v.Render(c, paused(time.Second), 400, 136)
				})
			}, 500, 300)
			ui.ClickAt(250, 90)
			if clicks != 1 {
				t.Fatal("surface intercepted underlying video input")
			}
			if unavailable {
				r, ok := ui.Find("Danmaku unavailable")
				if !ok {
					t.Fatal("safe error label missing")
				}
				ui.ClickAt(r.X+r.W/2, r.Y+r.H/2)
				if clicks != 2 {
					t.Fatal("error label intercepted underlying video input")
				}
			}
			if ui.Focused("Danmaku overlay") || ui.Focused("Danmaku unavailable") {
				t.Fatal("overlay acquired keyboard focus")
			}
		})
	}
}
