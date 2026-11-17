package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"

	"my-bilibili/internal/danmaku"
)

type danmakuCapture struct {
	state          observed
	surface        rectangle
	pixels         *image.NRGBA
	scaleX, scaleY float64
}

// Keep the whole fitted image for independent glyph inspection and compare its
// upper 65%, which contains the original top/scroll entries without transport
// controls. This is a pixel check, never an OCR or font-fallback claim.
func danmakuPixels(data []byte, surface, content rectangle) (*image.NRGBA, *image.NRGBA, float64, float64, error) {
	if _, err := pictureDigest(data, surface, content); err != nil {
		return nil, nil, 0, 0, err
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, 0, 0, errors.New("danmaku capture decode failed")
	}
	sx, sy := float64(im.Bounds().Dx())/content.Width, float64(im.Bounds().Dy())/content.Height
	bounds := image.Rect(int(math.Floor(surface.X*sx)), int(math.Floor(surface.Y*sy)), int(math.Ceil((surface.X+surface.Width)*sx)), int(math.Ceil((surface.Y+surface.Height)*sy)))
	if bounds.Empty() || !bounds.In(im.Bounds()) {
		return nil, nil, 0, 0, errors.New("fitted danmaku crop is outside the native capture")
	}
	full := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(full, full.Bounds(), im, bounds.Min, draw.Src)
	band := image.NewNRGBA(image.Rect(0, 0, full.Bounds().Dx(), int(float64(full.Bounds().Dy())*.65)))
	draw.Draw(band, band.Bounds(), full, image.Point{}, draw.Src)
	return full, band, sx, sy, nil
}

func writeDanmakuPNG(path string, im image.Image) error {
	var b bytes.Buffer
	if png.Encode(&b, im) != nil || os.WriteFile(path, b.Bytes(), 0600) != nil {
		return errors.New("danmaku crop could not be saved")
	}
	return nil
}

func samePreparedDanmaku(a, b observed) bool {
	return danmakuPaused(a) && danmakuPaused(b) && sameDanmakuWindow(a, b) && a.State.Playback.Position == b.State.Playback.Position && a.Danmaku.Active == b.Danmaku.Active && a.Danmaku.Layouts == b.Danmaku.Layouts && slices.Equal(a.Danmaku.Items, b.Danmaku.Items)
}

func (c *client) danmakuPicture(ctx context.Context, dir, name string) (danmakuCapture, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return danmakuCapture{}, err
	}
	o, err := observation(nodes)
	if err != nil {
		return danmakuCapture{}, err
	}
	if !danmakuPaused(o) || !popupsClosed(nodes) {
		return danmakuCapture{}, errors.New("danmaku capture requires settled pause with every popup closed")
	}
	w, err := c.identify(ctx)
	if err != nil {
		return danmakuCapture{}, err
	}
	surface, err := danmakuGeometry(nodes, o, w.ContentBounds)
	if err != nil {
		return danmakuCapture{}, err
	}
	windowName := name + "-window.png"
	if _, err = c.capturePicture(ctx, dir, windowName, surface, w.ContentBounds); err != nil {
		return danmakuCapture{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, windowName))
	if err != nil {
		return danmakuCapture{}, errors.New("saved danmaku capture unavailable")
	}
	full, pixels, sx, sy, err := danmakuPixels(data, surface, w.ContentBounds)
	if err != nil {
		return danmakuCapture{}, err
	}
	if err = writeDanmakuPNG(filepath.Join(dir, name+".png"), full); err != nil {
		return danmakuCapture{}, err
	}
	if err = writeDanmakuPNG(filepath.Join(dir, name+"-crop.png"), pixels); err != nil {
		return danmakuCapture{}, err
	}
	after, err := c.snapshot(ctx)
	if err != nil {
		return danmakuCapture{}, err
	}
	if !samePreparedDanmaku(o, after) {
		return danmakuCapture{}, errors.New("paused native or danmaku state changed during capture")
	}
	return danmakuCapture{state: o, surface: surface, pixels: pixels, scaleX: sx, scaleY: sy}, nil
}

func equalDanmakuPixels(a, b danmakuCapture) bool {
	return sameRectangle(a.surface, b.surface) && a.scaleX == b.scaleX && a.scaleY == b.scaleY && a.pixels.Bounds() == b.pixels.Bounds() && bytes.Equal(a.pixels.Pix, b.pixels.Pix)
}

func hiddenDanmakuPixels(visible, hidden danmakuCapture) (int, error) {
	a, b := visible.state, hidden.state
	if !danmakuPaused(a) || !danmakuPaused(b) || !a.Danmaku.Settings.Visible || a.Danmaku.Settings.Opacity == 0 || b.Danmaku.Settings.Visible && b.Danmaku.Settings.Opacity > 0 || b.Danmaku.Active != 0 || b.Danmaku.Layouts != 0 || a.State.Playback.Generation != b.State.Playback.Generation || a.State.Playback.Position != b.State.Playback.Position || a.Danmaku.Revision != b.Danmaku.Revision || !sameRectangle(visible.surface, hidden.surface) || visible.scaleX != hidden.scaleX || visible.scaleY != hidden.scaleY || visible.pixels.Bounds() != hidden.pixels.Bounds() {
		return 0, errors.New("visible and hidden crops do not share one paused video frame")
	}
	var boxes []image.Rectangle
	var title image.Rectangle
	for _, item := range a.Danmaku.Items {
		// One-DIP shadow plus a two-pixel antialias/rounded-crop margin. Nothing
		// outside these observed layout bounds may change when text is hidden.
		box := image.Rect(int(math.Floor(item.X*visible.scaleX))-2, int(math.Floor(item.Y*visible.scaleY))-2, int(math.Ceil((item.X+item.Width+1)*visible.scaleX))+2, int(math.Ceil((item.Y+item.Height+1)*visible.scaleY))+2).Intersect(visible.pixels.Bounds())
		boxes = append(boxes, box)
		if item.Index == 0 && item.Mode == danmaku.FixedTop {
			title = box
		}
	}
	changed, titleChanged := 0, 0
	for y := 0; y < visible.pixels.Bounds().Dy(); y++ {
		for x := 0; x < visible.pixels.Bounds().Dx(); x++ {
			if visible.pixels.NRGBAAt(x, y) == hidden.pixels.NRGBAAt(x, y) {
				continue
			}
			point, inside := image.Pt(x, y), false
			for _, box := range boxes {
				inside = inside || point.In(box)
			}
			if !inside {
				return 0, errors.New("video pixels outside observed text changed while hiding danmaku")
			}
			changed++
			if point.In(title) {
				titleChanged++
			}
		}
	}
	if titleChanged == 0 {
		return 0, errors.New("original Chinese title pixels did not disappear in the hidden crop")
	}
	return changed, nil
}
