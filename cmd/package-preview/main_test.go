package main

import (
	"io"
	"testing"
)

func TestPackageRequiresExplicitFreshOutput(t *testing.T) {
	for _, args := range [][]string{nil, {"extra"}, {"-unknown"}} {
		if err := command(args, io.Discard, io.Discard); err == nil {
			t.Fatal("accepted invalid package arguments", args)
		}
	}
}
