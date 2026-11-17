//go:build e2e

package main

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestSyntheticLoginExcludesLiveAdapters(t *testing.T) {
	if !syntheticBuild {
		t.Fatal("e2e mode missing")
	}
	cmd := exec.Command("go", "list", "-tags=e2e", "-deps", ".")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal("inspect e2e dependencies failed")
	}
	for _, p := range []string{"my-bilibili/internal/adapters/credentials", "my-bilibili/internal/adapters/bilibili/account"} {
		if bytes.Contains(out, []byte(p)) {
			t.Fatal("e2e includes real credential/service adapter")
		}
	}
	for _, o := range []options{{begin: true}, {restoreCheck: true}, {synthetic: true, begin: true}} {
		if _, err := compose(o); err == nil {
			t.Fatal("e2e live mode accepted")
		}
	}
}
