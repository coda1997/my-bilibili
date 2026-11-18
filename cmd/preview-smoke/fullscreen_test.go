package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"my-bilibili/internal/playback"
)

type fullscreenFixture struct {
	mu                                sync.Mutex
	mode                              string
	state                             observed
	actions, trees, windows, captures int
	phaseTrees, phaseWindows          int
	premature                         bool
	pendingCallback                   bool
	ordinary, enlarged                rectangle
	normalPNG, fullPNG                []byte
	captureDelay                      time.Duration
}

func fullscreenPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, width, height))
	for i, col := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		draw.Draw(im, image.Rect(i*width/3, 0, (i+1)*width/3, height), image.NewUniform(col), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

var fullscreenImages struct {
	sync.Once
	normal, full []byte
}

func newFullscreenFixture(t *testing.T, mode string) *fullscreenFixture {
	fullscreenImages.Do(func() {
		fullscreenImages.normal = fullscreenPNG(t, 1120, 760)
		fullscreenImages.full = fullscreenPNG(t, 1710, 1073)
	})
	o := completeObservation(t)
	o.Danmaku.Closed = false
	o.State.Playback.State = playback.Paused
	o.State.Playback.Error = playback.NoError
	o.State.Playback.Duration = 30 * time.Second
	o.State.Playback.Position = 5 * time.Second
	return &fullscreenFixture{mode: mode, state: o, ordinary: rectangle{X: 335, Y: 145, Width: 1120, Height: 760}, enlarged: rectangle{X: 0, Y: 34, Width: 1710, Height: 1073}, normalPNG: fullscreenImages.normal, fullPNG: fullscreenImages.full}
}
func (f *fullscreenFixture) reply(r popupRequest) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case "act", "input":
		if f.pendingCallback {
			f.premature = true
		}
		if f.mode == "delayed-exit" && f.actions == 2 && f.phaseWindows < 7 {
			f.premature = true
		}
		if (f.mode == "transient-entry" || f.mode == "ordinary-full-layout" || f.mode == "before-picture" || f.mode == "capture-contradiction" || f.mode == "incoherent" || f.mode == "moving-full-geometry") && f.actions == 1 {
			f.premature = true
		}
		if f.mode == "before-next-entry" && f.actions == 2 {
			f.premature = true
		}
		f.actions++
		f.phaseTrees = 0
		f.phaseWindows = 0
		f.pendingCallback = f.mode == "callback-entry-delay" && f.actions%2 == 1 || f.mode == "callback-exit-delay" && f.actions%2 == 0 || f.mode == "callback-never" && f.actions == 2
		if f.mode == "action-error" {
			return map[string]any{"error": map[string]any{"code": "failed", "message": "PRIVATE NATIVE"}}
		}
		return popupOK()
	case "windows":
		f.windows++
		f.phaseWindows++
		b := f.ordinary
		if f.actions%2 == 1 || f.mode == "delayed-exit" && f.actions == 2 && f.phaseWindows < 7 {
			b = f.enlarged
		}
		if f.mode == "transient-entry" || f.mode == "ordinary-full-layout" {
			b = f.ordinary
		}
		if f.mode == "capture-contradiction" && f.captures > 0 || f.mode == "escape-contradiction" && f.captures >= 3 {
			b = f.ordinary
		}
		if f.mode == "before-next-entry" && f.actions == 2 && f.phaseTrees > 2 {
			b = f.enlarged
		}
		if f.mode == "moving-full-geometry" && f.actions == 1 {
			b.X = float64(f.phaseWindows % 2)
		}
		return map[string]any{"result": map[string]any{"windows": []window{{ID: 1, Title: "My Bilibili Preview", Kind: "content", Visible: true, ContentBounds: b}}}}
	case "tree":
		f.trees++
		f.phaseTrees++
		full := f.actions%2 == 1
		if f.mode == "capture-contradiction" && f.captures > 0 || f.mode == "escape-contradiction" && f.captures >= 3 {
			full = false
		}
		if (f.mode == "transient-entry" && f.phaseTrees > 1 || f.mode == "before-picture" && f.phaseTrees > 2) && f.actions == 1 {
			full = false
		}
		if f.mode == "incoherent" && f.actions == 1 {
			full = f.phaseTrees%2 == 1
		}
		if f.mode == "before-next-entry" && f.actions == 2 && f.phaseTrees > 2 {
			full = true
		}
		oldFullLayout := f.mode == "exit-render-lag" && f.actions == 2 && f.phaseTrees <= 4
		o := f.state
		o.Fullscreen = full
		label, _ := json.Marshal(o)
		video := rectangle{X: 24, Y: 100, Width: 792, Height: 445.5}
		button := rectangle{X: 720, Y: 490, Width: 80, Height: 32}
		text := "Fullscreen"
		if full || oldFullLayout {
			video = rectangle{Width: 1710, Height: 1073}
			button = rectangle{X: 1600, Y: 1000, Width: 80, Height: 32}
			text = "Exit full"
		}
		if full && (f.mode == "transient-entry" || f.mode == "ordinary-full-layout") {
			video = rectangle{Width: 1120, Height: 760}
			button = rectangle{X: 1000, Y: 690, Width: 80, Height: 32}
		}
		completed := f.actions
		if f.pendingCallback {
			if f.phaseTrees >= 9 && f.mode != "callback-never" {
				f.pendingCallback = false
			} else {
				completed--
			}
		}
		native, _ := json.Marshal(nativeFullscreenTrace{SchemaVersion: 1, Sequence: uint64(f.actions + completed), Requests: uint64(f.actions), DidEnter: uint64((completed + 1) / 2), DidExit: uint64(completed / 2), Events: []nativeFullscreenEvent{}})
		nodes := []node{{Key: "e2e-observation", Label: string(label)}, {Key: "preview-video", Bounds: video}, {Key: "fullscreen-toggle", Label: text, Bounds: button}}
		if !full && !oldFullLayout {
			nodes = append(nodes, node{Key: "list-part-part-1", Bounds: rectangle{X: 850, Y: 154, Width: 232, Height: 32}})
		}
		nodes = append(nodes, node{Key: "e2e-fullscreen", Label: string(native)})
		return map[string]any{"result": map[string]any{"nodes": nodes}}
	case "capture":
		time.Sleep(f.captureDelay)
		f.captures++
		data := f.fullPNG
		if f.mode == "ordinary-full-layout" || f.mode == "transient-entry" {
			data = f.normalPNG
		}
		return map[string]any{"result": map[string]any{"png": data}}
	}
	return popupOK()
}
func (f *fullscreenFixture) counts() (int, int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actions, f.captures, f.premature
}

func TestFullscreenCyclesWaitForRestoredExitGeometry(t *testing.T) {
	f := newFullscreenFixture(t, "delayed-exit")
	c, trace := popupClient(t, f.reply)
	// Each transition owns its production deadline; a whole-cycle test limit
	// must not expire while a valid capture is still being processed.
	ctx := t.Context()
	r := &report{}
	if _, err := c.fullscreenCycles(ctx, t.TempDir(), r, f.state); err != nil {
		t.Fatal(err)
	}
	actions, captures, premature := f.counts()
	if premature {
		t.Fatal("next entry sent before native content bounds restored")
	}
	if actions != 6 || captures != 3 || trace.count("input", "") != 1 || trace.count("act", "fullscreen-toggle") != 5 || len(r.Checks) != 3 {
		t.Fatalf("three cycles/final Escape changed: actions=%d captures=%d checks=%d", actions, captures, len(r.Checks))
	}
}
func TestFullscreenCyclesRejectContradictoryEntry(t *testing.T) {
	for _, mode := range []string{"transient-entry", "ordinary-full-layout", "incoherent", "moving-full-geometry", "before-picture", "capture-contradiction"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newFullscreenFixture(t, mode)
				c, _ := popupClient(t, f.reply)
				ctx, cancel := context.WithTimeout(t.Context(), 900*time.Millisecond)
				defer cancel()
				start := time.Now()
				r := &report{}
				_, err := c.fullscreenCycles(ctx, t.TempDir(), r, f.state)
				actions, captures, premature := f.counts()
				if mode == "before-picture" && captures != 0 {
					t.Fatal("captured after entry precondition became false")
				}
				if premature || actions != 1 {
					t.Fatalf("reverse action sent from contradictory observation: actions=%d err=%v", actions, err)
				}
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 1600*time.Millisecond || len(r.Checks) != 0 {
					t.Fatalf("noncompletion ignored shared deadline or recorded PASS: %v", err)
				}
				if mode == "capture-contradiction" && (captures != 1 || !strings.Contains(err.Error(), "fullscreen cycle 1 entry-post-capture")) {
					t.Fatalf("did not reject contradictory post-capture observation: captures=%d err=%v", captures, err)
				}
			})
		})
	}
}
func TestFullscreenCyclesActionErrorAndCancellation(t *testing.T) {
	for _, mode := range []string{"action-error", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := newFullscreenFixture(t, mode)
			c, _ := popupClient(t, f.reply)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			_, err := c.fullscreenCycles(ctx, t.TempDir(), &report{}, f.state)
			actions, _, _ := f.counts()
			if mode == "cancel" {
				if !errors.Is(err, context.Canceled) || actions != 0 {
					t.Fatalf("cancellation sent an action: %v", err)
				}
			} else if err == nil || strings.Contains(err.Error(), "PRIVATE") || actions != 1 {
				t.Fatalf("action failure retried or leaked: %v", err)
			}
		})
	}
}

func TestFullscreenCyclesPollMixedExitFrame(t *testing.T) {
	f := newFullscreenFixture(t, "exit-render-lag")
	c, _ := popupClient(t, f.reply)
	// Each transition owns its production deadline; a whole-cycle test limit
	// must not expire while a valid capture is still being processed.
	ctx := t.Context()
	r := &report{}
	if _, err := c.fullscreenCycles(ctx, t.TempDir(), r, f.state); err != nil {
		t.Fatalf("legal cached-false/old-full-layout transient rejected: %v", err)
	}
	if len(r.Checks) != 3 {
		t.Fatal("lost completing sequence")
	}
}
func TestFullscreenCyclesRecheckBeforeNextEntry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFullscreenFixture(t, "before-next-entry")
		c, _ := popupClient(t, f.reply)
		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		r := &report{}
		_, err := c.fullscreenCycles(ctx, t.TempDir(), r, f.state)
		actions, captures, premature := f.counts()
		if premature || actions != 2 || captures != 1 || len(r.Checks) != 1 || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "fullscreen cycle 2 entry-precondition") {
			t.Fatalf("next entry lacked restored ordinary precondition: actions=%d captures=%d checks=%d err=%v", actions, captures, len(r.Checks), err)
		}
	})
}

func TestFullscreenCyclesRejectUnavailableGeometry(t *testing.T) {
	for _, where := range []string{"content", "video", "button", "part"} {
		for _, invalid := range []string{"missing-x", "null-y", "zero-width", "negative-height"} {
			t.Run(where+"/"+invalid, func(t *testing.T) {
				f := newFullscreenFixture(t, "")
				c, _ := popupClient(t, func(r popupRequest) map[string]any {
					result := f.reply(r)
					if where == "content" && r.Method != "windows" || where != "content" && r.Method != "tree" {
						return result
					}
					data, _ := json.Marshal(result)
					var raw map[string]any
					_ = json.Unmarshal(data, &raw)
					var bounds map[string]any
					if where == "content" {
						bounds = raw["result"].(map[string]any)["windows"].([]any)[0].(map[string]any)["ContentBounds"].(map[string]any)
					} else {
						key := map[string]string{"video": "preview-video", "button": "fullscreen-toggle", "part": "list-part-part-1"}[where]
						for _, n := range raw["result"].(map[string]any)["nodes"].([]any) {
							node := n.(map[string]any)
							if node["key"] == key {
								bounds = node["bounds"].(map[string]any)
							}
						}
					}
					switch invalid {
					case "missing-x":
						delete(bounds, "X")
					case "null-y":
						bounds["Y"] = nil
					case "zero-width":
						bounds["Width"] = 0
					case "negative-height":
						bounds["Height"] = -1
					}
					return raw
				})
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				_, err := c.fullscreenCycles(ctx, t.TempDir(), &report{}, f.state)
				actions, _, _ := f.counts()
				if err == nil || errors.Is(err, context.DeadlineExceeded) || actions != 0 {
					t.Fatalf("unavailable required geometry not rejected immediately: actions=%d err=%v", actions, err)
				}
			})
		}
	}
}
func TestFullscreenCyclesRejectAmbiguousNodesAndSessionChange(t *testing.T) {
	for _, mode := range []string{"duplicate", "missing-observation", "changed-session"} {
		t.Run(mode, func(t *testing.T) {
			f := newFullscreenFixture(t, "")
			c, _ := popupClient(t, func(r popupRequest) map[string]any {
				response := f.reply(r)
				if r.Method == "tree" {
					nodes := response["result"].(map[string]any)["nodes"].([]node)
					switch mode {
					case "duplicate":
						nodes = append(nodes, nodes[2])
					case "missing-observation":
						nodes = nodes[1:]
					case "changed-session":
						o := f.state
						o.State.Playback.Generation++
						data, _ := json.Marshal(o)
						nodes[0].Label = string(data)
					}
					response["result"].(map[string]any)["nodes"] = nodes
				}
				return response
			})
			_, err := c.fullscreenCycles(context.Background(), t.TempDir(), &report{}, f.state)
			actions, _, _ := f.counts()
			if err == nil || actions != 0 {
				t.Fatalf("ambiguous/changed state sent action: %v", err)
			}
		})
	}
}

func TestFullscreenCyclesRecheckBeforeEscape(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFullscreenFixture(t, "escape-contradiction")
		c, trace := popupClient(t, f.reply)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		r := &report{}
		_, err := c.fullscreenCycles(ctx, t.TempDir(), r, f.state)
		actions, captures, _ := f.counts()
		if actions != 5 || captures != 3 || trace.count("input", "") != 0 || len(r.Checks) != 2 || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "fullscreen cycle 3 entry-post-capture") {
			t.Fatalf("Escape sent without post-picture precondition: actions=%d captures=%d err=%v", actions, captures, err)
		}
	})
}

func TestFullscreenCyclesPostCaptureErrorAndCancellation(t *testing.T) {
	for _, mode := range []string{"rpc", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := newFullscreenFixture(t, "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c, _ := popupClient(t, func(r popupRequest) map[string]any {
				_, captures, _ := f.counts()
				if captures > 0 && r.Method == "tree" && mode == "rpc" {
					return map[string]any{"error": map[string]any{"code": "failed", "message": "PRIVATE NATIVE"}}
				}
				result := f.reply(r)
				if r.Method == "capture" && mode == "cancel" {
					cancel()
				}
				return result
			})
			_, err := c.fullscreenCycles(ctx, t.TempDir(), &report{}, f.state)
			actions, _, _ := f.counts()
			if err == nil || actions != 1 || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("postcapture failure sent action or leaked: %v", err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost canceled identity: %v", err)
			}
		})
	}
}

type fullscreenDeadlineConn struct {
	net.Conn
	mu      sync.Mutex
	current time.Time
}

func (c *fullscreenDeadlineConn) SetDeadline(d time.Time) error {
	c.mu.Lock()
	c.current = d
	c.mu.Unlock()
	return c.Conn.SetDeadline(d)
}
func (c *fullscreenDeadlineConn) deadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}
func TestFullscreenCyclesPictureSharesEntryDeadline(t *testing.T) {
	f := newFullscreenFixture(t, "")
	var wire *fullscreenDeadlineConn
	deadlines := make(chan time.Time, 100)
	c, _ := popupClient(t, func(r popupRequest) map[string]any {
		actions, captures, _ := f.counts()
		if r.Method == "act" || actions == 1 {
			deadlines <- wire.deadline()
		}
		if captures > 0 && r.Method == "tree" {
			return map[string]any{"error": map[string]any{"code": "failed"}}
		}
		return f.reply(r)
	})
	wire = &fullscreenDeadlineConn{Conn: c.conn}
	c.conn = wire
	c.callLimit = 10 * time.Second
	start := time.Now()
	_, err := c.fullscreenCycles(context.Background(), t.TempDir(), &report{}, f.state)
	if err == nil {
		t.Fatal("postcapture RPC failure lost")
	}
	first := <-deadlines
	if first.After(start.Add(5100 * time.Millisecond)) {
		t.Fatal("entry budget expanded")
	}
	count := 1
	for len(deadlines) > 0 {
		if got := <-deadlines; !got.Equal(first) {
			t.Fatalf("entry/picture/reversal checks reset deadline: %s vs %s", got, first)
		}
		count++
	}
	if count < 10 {
		t.Fatal("entry observations/capture not exercised")
	}
}

func TestFullscreenCyclesRequireRenderedCorrespondence(t *testing.T) {
	for _, mode := range []string{"layout", "label", "disabled", "offscreen", "outside-control"} {
		t.Run(mode, func(t *testing.T) {
			f := newFullscreenFixture(t, "")
			c, _ := popupClient(t, func(r popupRequest) map[string]any {
				response := f.reply(r)
				actions, _, _ := f.counts()
				if r.Method == "tree" && actions == 1 {
					nodes := response["result"].(map[string]any)["nodes"].([]node)
					switch mode {
					case "layout":
						nodes[1].Bounds = rectangle{X: 24, Y: 100, Width: 792, Height: 445.5}
						nodes[2].Bounds = rectangle{X: 720, Y: 490, Width: 80, Height: 32}
					case "label":
						nodes[2].Label = "Fullscreen"
					case "disabled", "offscreen":
						nodes[2].States = []string{mode}
					case "outside-control":
						nodes[2].Bounds.X = 1800
					}
				}
				return response
			})
			ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
			defer cancel()
			_, err := c.fullscreenCycles(ctx, t.TempDir(), &report{}, f.state)
			actions, captures, _ := f.counts()
			if !errors.Is(err, context.DeadlineExceeded) || actions != 1 || captures != 0 {
				t.Fatalf("noncorresponding rendered state passed: actions=%d captures=%d err=%v", actions, captures, err)
			}
		})
	}
}

func TestFullscreenCyclesWaitForNativeCompletion(t *testing.T) {
	for _, test := range []struct {
		name, mode   string
		captureDelay time.Duration
	}{
		{name: "callback-entry-delay", mode: "callback-entry-delay"},
		{name: "callback-exit-delay", mode: "callback-exit-delay"},
		// Three 500ms entry waits and three 900ms captures already exceed the
		// former 4s test-wide limit, but each transition remains within 5s.
		{name: "slow-capture", mode: "callback-exit-delay", captureDelay: 900 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFullscreenFixture(t, test.mode)
			f.captureDelay = test.captureDelay
			c, _ := popupClient(t, f.reply)
			r := &report{}
			_, err := c.fullscreenCycles(t.Context(), t.TempDir(), r, f.state)
			actions, captures, premature := f.counts()
			if premature {
				t.Fatal("sent next transition before native completion despite matching geometry and rendered state")
			}
			if err != nil || actions != 6 || captures != 3 || len(r.Checks) != 3 {
				t.Fatalf("cycles did not complete: %v actions=%d captures=%d", err, actions, captures)
			}
		})
	}
}
func TestFullscreenCyclesMissingNativeCompletionStops(t *testing.T) {
	f := newFullscreenFixture(t, "callback-never")
	// Entry must finish before the missing exit callback's deadline starts.
	// This valid capture plus the entry wait exceeds the former 1.2s parent.
	f.captureDelay = 900 * time.Millisecond
	c, _ := popupClient(t, f.reply)
	r := &report{}
	_, err := c.fullscreenCycles(t.Context(), t.TempDir(), r, f.state)
	actions, captures, premature := f.counts()
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "fullscreen cycle 1 exit-correspondence") || premature || actions != 2 || captures != 1 || len(r.Checks) != 0 {
		t.Fatalf("missing callback accepted or wrong phase expired: %v actions=%d captures=%d premature=%v checks=%d", err, actions, captures, premature, len(r.Checks))
	}
}
