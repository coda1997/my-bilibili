//go:build !darwin

package main

import (
	"my-bilibili/internal/playback"
	"time"
)

func doubleClickInterval() (time.Duration, error) { return 0, playback.Unsupported }
