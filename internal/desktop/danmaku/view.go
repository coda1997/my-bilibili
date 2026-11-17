// Package danmakuview owns retained text and controls for a media-time timeline.
// View and its methods belong to the UI thread; drawing never advances time.
package danmakuview

import (
	"math"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

type ItemState struct {
	Index                         int
	Mode                          danmaku.Mode
	Lane                          int
	Progress, X, Y, Width, Height float64
}

type State struct {
	Generation, Revision uint64
	Position             time.Duration
	Width, Height        float64
	Lanes                int
	Settings             danmaku.Settings
	Stats                danmaku.Stats
	Active, Layouts      int
	Items                []ItemState
	Error                danmaku.ErrorCode
	Closed               bool
}

type layoutKey struct {
	generation, revision uint64
	index                int
	fontSize             float64
}

type shapedText struct {
	layout        *gui.TextLayout
	width, height float32
}

type paintItem struct {
	layout    *gui.TextLayout
	x, y, top float32
	color     gui.Color
}

type preparedFrame struct {
	lineHeight float32
	shadow     gui.Color
	items      []paintItem
}

// View is constructed with New. It retains only the current active layouts,
// never a past prepared frame or a drawing callback.
type View struct {
	timeline                            *danmaku.Timeline
	layouts                             map[layoutKey]shapedText
	state                               State
	entryError, configError, frameError danmaku.ErrorCode
	closed                              bool
	shape                               func(string, gui.Font) shapedText
}

func New() *View {
	return &View{
		timeline: danmaku.New(),
		state:    State{Settings: danmaku.DefaultSettings(), Items: []ItemState{}},
		shape:    shapeText,
	}
}

func shapeText(text string, font gui.Font) shapedText {
	layout := gui.ShapeText(text, font, 0)
	if layout == nil {
		return shapedText{}
	}
	w, h := layout.Size()
	return shapedText{layout: layout, width: w, height: h}
}

func (v *View) Reset(generation uint64) error {
	previous := v.state.Generation
	if err := v.timeline.Reset(generation); err != nil {
		return err
	}
	if generation > previous {
		v.entryError, v.configError, v.frameError = danmaku.NoError, danmaku.NoError, danmaku.NoError
		v.retire()
	}
	return nil
}

func (v *View) SetEntries(generation uint64, entries []danmaku.Entry) error {
	if err := v.timeline.SetEntries(generation, entries); err != nil {
		if err == danmaku.Invalid && generation > 0 && generation == v.state.Generation {
			v.entryError = danmaku.Invalid
			v.setError()
		}
		return err
	}
	v.entryError = danmaku.NoError
	v.retire()
	return nil
}

func (v *View) SetUnavailable(generation uint64) error {
	if err := v.SetEntries(generation, nil); err != nil {
		return err
	}
	v.entryError = danmaku.Unavailable
	v.setError()
	return nil
}

func (v *View) Settings() danmaku.Settings { return v.timeline.Settings() }

func (v *View) Configure(settings danmaku.Settings) error {
	previous := v.Settings()
	if err := v.timeline.Configure(settings); err != nil {
		if err == danmaku.Invalid {
			v.configError = danmaku.Invalid
			v.setError()
		}
		return err
	}
	v.configError = danmaku.NoError
	if settings != previous {
		v.retire()
	} else {
		v.setError()
	}
	return nil
}

// retire publishes an empty prepared frame immediately, including when a
// mutation happens after the surface was built but before observation. The
// zero geometry and position are valid model inputs, so Frame cannot fail.
func (v *View) retire() {
	v.layouts = nil
	frame, _ := v.timeline.Frame(playback.Snapshot{}, 0, 0)
	v.state = frameState(frame)
	v.setError()
}

func frameState(frame danmaku.Frame) State {
	return State{
		Generation: frame.Generation, Revision: frame.Revision, Position: frame.Position,
		Width: frame.Width, Height: frame.Height, Lanes: frame.Lanes,
		Settings: frame.Settings, Stats: frame.Stats, Closed: frame.Closed, Items: []ItemState{},
	}
}

func (v *View) setError() {
	switch {
	case v.configError != danmaku.NoError:
		v.state.Error = v.configError
	case v.entryError != danmaku.NoError:
		v.state.Error = v.entryError
	default:
		v.state.Error = v.frameError
	}
}

func positive(value float32) bool {
	return value > 0 && !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (v *View) prepare(snapshot playback.Snapshot, width, height float32) preparedFrame {
	if snapshot.State == playback.Closed {
		v.Close()
	}
	frame, err := v.timeline.Frame(snapshot, float64(width), float64(height))
	v.state = frameState(frame)
	v.frameError = danmaku.NoError
	if err != nil {
		v.layouts = nil
		v.frameError = danmaku.Invalid
		v.setError()
		return preparedFrame{}
	}

	prepared := preparedFrame{
		lineHeight: float32(frame.LineHeight),
		shadow:     gui.RGBA(0, 0, 0, float32(frame.Settings.Opacity)),
		items:      make([]paintItem, 0, len(frame.Items)),
	}
	active := make(map[layoutKey]shapedText, len(frame.Items))
	for _, item := range frame.Items {
		key := layoutKey{frame.Generation, frame.Revision, item.Index, frame.Settings.FontSize}
		text, ok := v.layouts[key]
		if !ok {
			text = v.shape(item.Text, gui.Font{Size: float32(frame.Settings.FontSize), Weight: 600, Lang: "zh-Hans", LineHeight: 1.25})
			if text.layout == nil || !positive(text.width) || !positive(text.height) {
				v.frameError = danmaku.Unavailable
				continue
			}
		}
		active[key] = text
		w, h := float64(text.width), float64(text.height)
		x := (frame.Width - w) / 2
		switch item.Mode {
		case danmaku.ScrollRTL:
			x = frame.Width - item.Progress*(frame.Width+w)
		case danmaku.ScrollLTR:
			x = -w + item.Progress*(frame.Width+w)
		}
		y := item.Top + (frame.LineHeight-h)/2
		v.state.Items = append(v.state.Items, ItemState{
			Index: item.Index, Mode: item.Mode, Lane: item.Lane, Progress: item.Progress,
			X: x, Y: y, Width: w, Height: h,
		})
		prepared.items = append(prepared.items, paintItem{
			layout: text.layout, x: float32(x), y: float32(y), top: float32(item.Top),
			color: gui.RGBA(uint8(item.Color>>16), uint8(item.Color>>8), uint8(item.Color), float32(frame.Settings.Opacity)),
		})
	}
	// Replacing the map evicts every inactive layout. The outgoing callback can
	// hold its immutable <=16 layouts until mygo retires that frame.
	v.layouts = active
	v.state.Active, v.state.Layouts = len(prepared.items), len(active)
	v.setError()
	return prepared
}

func (v *View) draw(frame preparedFrame) func(*gui.Painter, gui.Rect) {
	return func(p *gui.Painter, rect gui.Rect) {
		// mygo may retain an outgoing callback. This is the only mutable state
		// consulted by painting, and both Close and paint belong to the UI owner.
		if v.closed {
			return
		}
		p.Clip(rect, 0, func() {
			for _, item := range frame.items {
				lane := gui.Rect{X: rect.X, Y: rect.Y + item.top, W: rect.W, H: frame.lineHeight}
				p.Clip(lane, 0, func() {
					x, y := rect.X+item.x, rect.Y+item.y
					p.TextLayout(item.layout, x+1, y+1, frame.shadow)
					p.TextLayout(item.layout, x, y, item.color)
				})
			}
		})
	}
}

func (v *View) Render(c *gui.Context, snapshot playback.Snapshot, width, height float32) {
	frame := v.prepare(snapshot, width, height)
	gui.Box(c.Key("danmaku-surface")).Absolute().Left(0).Top(0).
		Size(float32(v.state.Width), float32(v.state.Height)).PassThrough().
		Label("Danmaku overlay").Draw(v.draw(frame))
	if v.state.Error != danmaku.NoError {
		gui.Text(c.Key("danmaku-error"), "Danmaku unavailable").Absolute().Left(8).Top(8).
			FontSize(13).TextColor(gui.RGB(255, 230, 238)).PassThrough()
	}
}

func (v *View) State() State {
	state := v.state
	state.Items = append([]ItemState{}, state.Items...)
	return state
}

func (v *View) Close() {
	if v.closed {
		return
	}
	v.closed = true
	v.timeline.Close()
	v.entryError, v.configError, v.frameError = danmaku.NoError, danmaku.NoError, danmaku.NoError
	v.retire()
}
