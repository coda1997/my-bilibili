//go:build !darwin

package main

import (
	"my-bilibili/experiments/playback/contract"
	"time"
)

func doubleClickInterval() (time.Duration, error) { return 0, contract.Unsupported }
