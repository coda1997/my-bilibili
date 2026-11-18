//go:build e2e

package main

import (
	"context"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
)

// The actual resolver must first accept its fixed synthetic DASH envelope. Only
// then does this e2e-only boundary replace its input with a pinned local asset.
type syntheticNativeSource struct {
	app.Source
	fixture app.Source
}

func (s *syntheticNativeSource) Resolve(ctx context.Context, request app.ResolveRequest) (app.Resolution, error) {
	resolved, err := s.Source.Resolve(ctx, request)
	if err != nil {
		return app.Resolution{}, err
	}
	var fixturePart catalog.PartID
	switch request.Part {
	case "101", "201", "301":
		fixturePart = "part-1"
	case "102", "202", "302":
		fixturePart = "part-2"
	default:
		return app.Resolution{}, catalog.Invalid
	}
	local, err := s.fixture.Resolve(ctx, app.ResolveRequest{Part: fixturePart})
	if err != nil {
		return app.Resolution{}, err
	}
	resolved.Input = local.Input
	return resolved, nil
}
