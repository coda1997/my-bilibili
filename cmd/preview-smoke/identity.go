package main

import (
	"debug/buildinfo"
	"errors"
	"runtime"
	"runtime/debug"

	"my-bilibili/internal/previewbundle"
)

func verifyAssignedApp(path, expected string) error {
	hash, err := previewbundle.Hash(path)
	if err != nil || hash != expected {
		return errors.New("assigned preview app identity mismatch")
	}
	return verifyE2EApp(path)
}

// Normal now restores the real account by default. Check build metadata before
// process creation; waiting for a missing automation socket would be too late.
func verifyE2EApp(path string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil || !e2eApplication(info) {
		return errors.New("assigned preview app is not the synthetic e2e build")
	}
	return nil
}

func e2eApplication(info *debug.BuildInfo) bool {
	if info == nil || info.Path != "my-bilibili/cmd/my-bilibili" {
		return false
	}
	want := map[string]string{"-tags": "e2e", "GOOS": "darwin", "GOARCH": runtime.GOARCH}
	for _, setting := range info.Settings {
		expected, required := want[setting.Key]
		if !required {
			continue
		}
		if setting.Value != expected {
			return false
		}
		delete(want, setting.Key)
	}
	return len(want) == 0
}
