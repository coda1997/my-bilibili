//go:build !e2e

package main

import (
	"testing"

	"my-bilibili/internal/catalog"
)

func TestNormalClientModeCannotReadSyntheticEnvironment(t *testing.T) {
	t.Setenv("MY_BILIBILI_E2E_CLIENT_SCENARIO", "history-corrupt")
	t.Setenv("MY_BILIBILI_E2E_HISTORY_DIR", t.TempDir())
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", "delayed")
	e2e, scene, err := clientBuildMode()
	if err != nil || e2e || scene != "" {
		t.Fatal("normal build used synthetic environment")
	}
	if _, err := composeClient("", clientOptions{scenario: "search"}); err != catalog.Invalid {
		t.Fatal("normal composition admitted a synthetic selector")
	}
}
