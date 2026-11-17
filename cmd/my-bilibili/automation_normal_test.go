//go:build !e2e

package main

import (
	"context"
	"my-bilibili/internal/fixture"
	"testing"
)

func TestNormalSourceIgnoresAutomationEnvironment(t *testing.T) {
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", "delayed")
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	got, cleanup, err := automationSource(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got != source {
		t.Fatal("normal source was replaced by automation")
	}
	r, err := got.Resolve(context.Background(), "part-1")
	in := r.Input
	if err != nil || in.URL != "" || in.FilePath == "" {
		t.Fatal(in, err)
	}
}
