package main

import (
	"github.com/ebitengine/purego/objc"
	"math"
	"my-bilibili/internal/playback"
	"time"
)

func doubleClickInterval() (time.Duration, error) {
	seconds := objc.Send[float64](objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("doubleClickInterval"))
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 5 {
		return 0, playback.Media
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
