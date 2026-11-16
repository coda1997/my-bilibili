package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

func TestPictureMustExistInsideVideoSurface(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 800, 600))
	video := rectangle{X: 100, Y: 100, Width: 640, Height: 360}
	content := rectangle{Width: 800, Height: 600}
	encode := func() []byte {
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	// Colorful metadata/cover above a black video must not establish picture.
	for i, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		draw.Draw(im, image.Rect(i*200, 0, (i+1)*200, 80), image.NewUniform(c), image.Point{}, draw.Src)
	}
	if _, err := pictureDigest(encode(), video, content); err == nil {
		t.Fatal("cover falsely established video picture")
	}
	for i, c := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		draw.Draw(im, image.Rect(100+i*213, 100, 100+(i+1)*213, 460), image.NewUniform(c), image.Point{}, draw.Src)
	}
	a, err := pictureDigest(encode(), video, content)
	if err != nil {
		t.Fatal(err)
	}
	im.Set(200, 200, color.White)
	b, err := pictureDigest(encode(), video, content)
	if err != nil || a == b {
		t.Fatal("frame change not detected", err)
	}
	for _, bad := range []rectangle{{}, {X: -2000, Y: 0, Width: 640, Height: 360}, {X: 700, Y: 500, Width: 640, Height: 360}} {
		if _, err := pictureDigest(encode(), bad, content); err == nil {
			t.Fatal("invalid video geometry accepted")
		}
	}
	if _, err := pictureDigest([]byte("not png"), video, content); err == nil {
		t.Fatal("invalid capture accepted")
	}
}
