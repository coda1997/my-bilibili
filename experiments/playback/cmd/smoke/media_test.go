package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

func TestRenderingOracleRequiresFixtureNotLabels(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 640, 360))
	encode := func() []byte {
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if rendered(encode()) {
		t.Fatal("blank frame accepted")
	}
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{32, 192, 96, 255}), image.Point{}, draw.Src)
	if rendered(encode()) {
		t.Fatal("green field without rectangles accepted")
	}
	draw.Draw(im, image.Rect(80, 70, 240, 190), image.NewUniform(color.RGBA{240, 48, 48, 255}), image.Point{}, draw.Src)
	draw.Draw(im, image.Rect(380, 160, 560, 240), image.NewUniform(color.RGBA{48, 96, 240, 255}), image.Point{}, draw.Src)
	if !rendered(encode()) {
		t.Fatal("synthetic picture not recognized")
	}
	if rendered([]byte("invalid")) {
		t.Fatal("invalid PNG accepted")
	}
}
