package main

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"my-bilibili/internal/previewbundle"
)

func TestSyntheticAppIdentityMustBeKnownBeforeLaunch(t *testing.T) {
	base := debug.BuildInfo{Path: "my-bilibili/cmd/my-bilibili", Settings: []debug.BuildSetting{{Key: "-tags", Value: "e2e"}, {Key: "GOOS", Value: "darwin"}, {Key: "GOARCH", Value: runtime.GOARCH}}}
	if !e2eApplication(&base) {
		t.Fatal("assigned e2e metadata rejected")
	}
	if e2eApplication(nil) {
		t.Fatal("absent identity accepted")
	}
	for _, change := range []func(*debug.BuildInfo){
		func(b *debug.BuildInfo) { b.Path = "my-bilibili/cmd/p0-login" },
		func(b *debug.BuildInfo) { b.Settings[0].Value = "" },
		func(b *debug.BuildInfo) { b.Settings[0].Value = "not-e2e" },
		func(b *debug.BuildInfo) { b.Settings[1].Value = "linux" },
		func(b *debug.BuildInfo) { b.Settings[2].Value = "unknown" },
		func(b *debug.BuildInfo) { b.Settings = b.Settings[1:] },
	} {
		b := base
		b.Settings = append([]debug.BuildSetting{}, base.Settings...)
		change(&b)
		if e2eApplication(&b) {
			t.Fatal("normal, foreign or mismatched build accepted")
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if verifyE2EApp(executable) == nil {
		t.Fatal("this test executable became an application launch candidate")
	}
	hash, err := previewbundle.Hash(executable)
	if err != nil {
		t.Fatal(err)
	}
	if verifyAssignedApp(executable, hash) == nil {
		t.Fatal("a matching hash bypassed the e2e application identity")
	}
	if err := verifyAssignedApp(executable, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatal("assigned digest was not enforced")
	}
}
