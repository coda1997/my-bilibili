package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/desktop"
	"my-bilibili/internal/playback"
)

type layoutPlayer struct {
	app.Player
	state app.State
}

func (p layoutPlayer) State() app.State { return p.state }
func (p layoutPlayer) Tick()            {}

func TestLayoutUsesActualAccessibleDesktopGeometry(t *testing.T) {
	for _, width := range []int{1120, 800} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			height := 760
			if width == 800 {
				height = 600
			}
			s := completeObservation(t).State
			s.Playback.State = playback.Paused
			view := &desktop.View{Player: layoutPlayer{state: s}, Fullscreen: func() bool { return false }, DoubleClickInterval: 500 * time.Millisecond}
			ui := gui.NewTester(view.Render, width, height)
			var nodes []node
			for _, element := range []struct{ key, label string }{{"preview-video", "Video surface"}, {"list-part-part-1", fmt.Sprintf("P1 · %s · playing", s.Video.Parts[0].Title)}} {
				a, ok := ui.Access(element.label)
				if !ok {
					t.Fatal("accessible desktop element absent", element.label)
				}
				nodes = append(nodes, node{Key: element.key, Label: element.label, Bounds: rectangle{X: float64(a.Bounds.X), Y: float64(a.Bounds.Y), Width: float64(a.Bounds.W), Height: float64(a.Bounds.H)}})
			}
			// The actual mygo accessibility tree omits the unroled layout Row;
			// only the visible video group and part button are protocol nodes.
			c := pipeClient(t, func(_ int, method string) any {
				if method == "windows" {
					return map[string]any{"windows": []window{{ID: 1, Title: "My Bilibili Preview", Kind: "content", Visible: true, ContentBounds: rectangle{Width: float64(width), Height: float64(height)}}}}
				}
				return map[string]any{"nodes": nodes}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()
			if err := c.layout(ctx, width >= 1048); err != nil {
				t.Fatal("actual accessible layout rejected", err)
			}
		})
	}
}
