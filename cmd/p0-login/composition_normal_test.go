//go:build !e2e

package main

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestNormalLoginExcludesAutomation(t *testing.T) {
	if syntheticBuild {
		t.Fatal("normal build allows synthetic mode")
	}
	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal("inspect normal dependencies failed")
	}
	if bytes.Contains(out, []byte("plugins/automation")) {
		t.Fatal("normal login includes capture automation")
	}
	if _, err := compose(options{synthetic: true}); err == nil {
		t.Fatal("normal composition accepted fixture")
	}
}
