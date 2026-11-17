//go:build !darwin || !arm64

package mpv

import "my-bilibili/internal/playback"

func load(string) (*nativeAPI, error) { return nil, playback.Unsupported }
