package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"my-bilibili/experiments/playback/mpvprobe"
)

func (c *client) resources(ctx context.Context) (mpvprobe.Resources, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return mpvprobe.Resources{}, err
	}
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "resources ") {
			var r mpvprobe.Resources
			if json.Unmarshal([]byte(strings.TrimPrefix(n.Label, "resources ")), &r) == nil {
				return r, nil
			}
		}
	}
	return mpvprobe.Resources{}, errors.New("native resource observation absent")
}
