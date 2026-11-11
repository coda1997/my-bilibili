package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownStagePrecedesAnyArtifactAccess(t *testing.T) {
	for _, name := range []string{"", "full", "preview", "CAPABILITIES", "SURFACE"} {
		r := report{Stage: name}
		if err := run(context.Background(), "/absent-app", "/absent-out", "/absent-fixtures", "/absent-manifest", &r); err == nil || err.Error() != "unknown replacement stage: choose surface or capabilities" {
			t.Fatal(name, err)
		}
	}
}

func TestRuntimeIdentityDetectsDependencyChanges(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "library")
	dep := filepath.Join(dir, "dependency")
	for _, p := range []string{lib, dep} {
		if err := os.WriteFile(p, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := sha256.Sum256([]byte("synthetic"))
	digest := hex.EncodeToString(h[:])
	m := runtimeManifest{SourceCommit: "b0841e8c4d156b8403dc1876c4cbd9011eb583f3", API: "2.6", Library: artifact{lib, digest}, Dependencies: []artifact{{dep, digest}}}
	b, _ := json.Marshal(m)
	path := filepath.Join(dir, "manifest.json")
	_ = os.WriteFile(path, b, 0600)
	if _, _, err := verifyRuntime(path); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(dep, []byte("changed"), 0600)
	if _, _, err := verifyRuntime(path); err == nil {
		t.Fatal("changed native dependency accepted")
	}
	m.API = "2.5"
	b, _ = json.Marshal(m)
	_ = os.WriteFile(path, b, 0600)
	if _, _, err := verifyRuntime(path); err == nil {
		t.Fatal("incompatible API identity accepted")
	}
}
