package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/previewbundle"
)

func syntheticDanmakuCapture(t *testing.T, hidden bool) danmakuCapture {
	t.Helper()
	o := danmakuSample(t, 2*time.Second)
	surface := rectangle{X: 80, Y: 100, Width: 640, Height: 360, complete: true}
	_, pixels, sx, sy, err := danmakuPixels(fullscreenPNG(t, 800, 600), surface, rectangle{Width: 800, Height: 600})
	if err != nil {
		t.Fatal(err)
	}
	if hidden {
		o.Danmaku.Settings.Visible = false
		o.Danmaku.Active, o.Danmaku.Layouts = 0, 0
		o.Danmaku.Items = []previewbundle.DanmakuItemObservation{}
	} else {
		// Synthetic text-like pixels inside the observed title bounds. This is
		// deliberately no Chinese-recognition fixture or native glyph claim.
		draw.Draw(pixels, image.Rect(260, 6, 263, 23), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(pixels, image.Rect(260, 12, 279, 15), image.NewUniform(color.White), image.Point{}, draw.Src)
	}
	return danmakuCapture{state: o, surface: surface, pixels: pixels, scaleX: sx, scaleY: sy}
}

func TestDanmakuPixelProofRequiresDisappearingTitleAndStableBackground(t *testing.T) {
	visible, hidden := syntheticDanmakuCapture(t, false), syntheticDanmakuCapture(t, true)
	if changed, err := hiddenDanmakuPixels(visible, hidden); err != nil || changed == 0 {
		t.Fatal("same-frame title removal rejected", changed, err)
	}
	for name, mutate := range map[string]func(*danmakuCapture, *danmakuCapture){
		"identical": func(a, b *danmakuCapture) {
			copy(b.pixels.Pix, a.pixels.Pix)
		},
		"background changed": func(_ *danmakuCapture, b *danmakuCapture) {
			b.pixels.SetNRGBA(1, 1, color.NRGBA{A: 255})
		},
		"still visible": func(_ *danmakuCapture, b *danmakuCapture) {
			b.state.Danmaku.Settings.Visible = true
		},
		"still active": func(_ *danmakuCapture, b *danmakuCapture) { b.state.Danmaku.Active++ },
		"retained layouts": func(_ *danmakuCapture, b *danmakuCapture) {
			b.state.Danmaku.Layouts++
		},
		"different media frame": func(_ *danmakuCapture, b *danmakuCapture) {
			b.state.State.Playback.Position++
			b.state.Danmaku.Position++
		},
		"different crop":  func(_ *danmakuCapture, b *danmakuCapture) { b.surface.X++ },
		"different scale": func(_ *danmakuCapture, b *danmakuCapture) { b.scaleX++ },
		"title absent": func(a, _ *danmakuCapture) {
			a.state.Danmaku.Items = a.state.Danmaku.Items[1:]
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, b := syntheticDanmakuCapture(t, false), syntheticDanmakuCapture(t, true)
			mutate(&a, &b)
			if _, err := hiddenDanmakuPixels(a, b); err == nil {
				t.Fatal("unrelated or absent text pixels accepted as overlay proof")
			}
		})
	}
}

func TestDanmakuPausedPixelsAreExact(t *testing.T) {
	a, b := syntheticDanmakuCapture(t, false), syntheticDanmakuCapture(t, false)
	if !equalDanmakuPixels(a, b) {
		t.Fatal("identical paused pixels rejected")
	}
	b.pixels.SetNRGBA(260, 8, color.NRGBA{A: 255})
	if equalDanmakuPixels(a, b) {
		t.Fatal("one changed paused text pixel accepted")
	}
	copy(b.pixels.Pix, a.pixels.Pix)
	b.surface.Y++
	if equalDanmakuPixels(a, b) {
		t.Fatal("moved video crop accepted as identical pause")
	}
}

func TestDanmakuCropExcludesPageAndTransportButKeepsVideoTitle(t *testing.T) {
	surface, content := rectangle{X: 80, Y: 100, Width: 640, Height: 360}, rectangle{Width: 800, Height: 600}
	data := fullscreenPNG(t, 800, 600)
	full, band, _, _, err := danmakuPixels(data, surface, content)
	if err != nil || full.Bounds() != image.Rect(0, 0, 640, 360) || band.Bounds() != image.Rect(0, 0, 640, 234) {
		t.Fatal("wrong fitted/cropped pixel bounds", err)
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	modified := image.NewRGBA(im.Bounds())
	draw.Draw(modified, im.Bounds(), im, image.Point{}, draw.Src)
	modified.Set(10, 10, color.White)   // page furniture
	modified.Set(100, 450, color.White) // transport inside the lower video region
	encode := func() []byte {
		var b bytes.Buffer
		if png.Encode(&b, modified) != nil {
			t.Fatal("encode failed")
		}
		return b.Bytes()
	}
	_, after, _, _, err := danmakuPixels(encode(), surface, content)
	if err != nil || !bytes.Equal(band.Pix, after.Pix) {
		t.Fatal("page/transport pixels contaminated the comparison", err)
	}
	modified.Set(340, 110, color.White) // actual top-title region
	_, after, _, _, err = danmakuPixels(encode(), surface, content)
	if err != nil || bytes.Equal(band.Pix, after.Pix) {
		t.Fatal("top-title pixels were omitted", err)
	}
	if _, _, _, _, err = danmakuPixels(data, rectangle{X: -1, Y: 100, Width: 640, Height: 360}, content); err == nil {
		t.Fatal("full fitted crop outside the native capture accepted")
	}
	if _, _, _, _, err = danmakuPixels([]byte("private-image-sentinel"), surface, content); err == nil || strings.Contains(err.Error(), "private-image-sentinel") {
		t.Fatal("malformed PNG accepted or raw payload leaked", err)
	}
}

func TestDanmakuCaptureRequiresClosedPopupsAndStableObservedFrame(t *testing.T) {
	for _, mode := range []string{"valid", "popup", "drift"} {
		t.Run(mode, func(t *testing.T) {
			o := danmakuSample(t, 2*time.Second)
			trees, captures := 0, 0
			c := pipeClient(t, func(_ int, method string) any {
				switch method {
				case "tree":
					trees++
					sample := o
					if trees > 1 && mode == "drift" {
						sample = danmakuSample(t, 2080*time.Millisecond)
					}
					data, _ := json.Marshal(sample)
					bounds := rectangle{X: 80, Y: 100, Width: 640, Height: 360}
					nodes := []node{{Key: "e2e-observation", Label: string(data)}, {Key: "preview-video", Bounds: bounds}, {Key: "danmaku-surface", Bounds: bounds}}
					if mode == "popup" {
						nodes = append(nodes, node{Key: "danmaku-popup"})
					}
					return map[string]any{"nodes": nodes}
				case "windows":
					return windowResult{Windows: []window{{ID: 1, Title: "My Bilibili Preview", Kind: "content", Visible: true, ContentBounds: rectangle{Width: 800, Height: 600}}}}
				case "capture":
					captures++
					return map[string]any{"png": fullscreenPNG(t, 800, 600)}
				}
				return nil
			})
			dir := t.TempDir()
			_, err := c.danmakuPicture(t.Context(), dir, "original")
			if (err == nil) != (mode == "valid") {
				t.Fatal("capture pre/postcondition did not discriminate", err)
			}
			if mode == "popup" && captures != 0 {
				t.Fatal("capture attempted with a popup covering the fixture")
			}
			if mode == "valid" {
				for _, name := range []string{"original-window.png", "original.png", "original-crop.png"} {
					if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
						t.Fatal("independent visual evidence missing", err)
					}
				}
			}
		})
	}
}
