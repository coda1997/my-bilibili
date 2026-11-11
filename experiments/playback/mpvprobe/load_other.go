//go:build !darwin || !arm64

package mpvprobe

import "my-bilibili/experiments/playback/contract"

func load(string) (*nativeAPI, error) { return nil, contract.Unsupported }
