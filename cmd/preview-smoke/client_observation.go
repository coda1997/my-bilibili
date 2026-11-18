package main

import (
	"context"
	"time"

	"my-bilibili/internal/previewbundle"
)

type clientObserved = previewbundle.ClientObservation

func clientObservation(nodes []node) (clientObserved, error) {
	n, err := uniqueNode(nodes, "e2e-client-observation")
	if err != nil {
		return clientObserved{}, err
	}
	return previewbundle.ParseClientObservation([]byte(n.Label))
}

func (c *client) clientSnapshot(ctx context.Context) (clientObserved, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return clientObserved{}, err
	}
	return clientObservation(nodes)
}

func (c *client) clientUntil(ctx context.Context, limit time.Duration, predicate func(clientObserved) bool) (clientObserved, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	end, _ := ctx.Deadline()
	var o clientObserved
	for {
		if err := ctx.Err(); err != nil {
			return o, err
		}
		var err error
		o, err = c.clientSnapshot(ctx)
		if err != nil {
			return o, err
		}
		if err := ctx.Err(); err != nil {
			return o, err
		}
		if !time.Now().Before(end) {
			return o, context.DeadlineExceeded
		}
		if predicate(o) {
			return o, nil
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return o, err
		}
	}
}
