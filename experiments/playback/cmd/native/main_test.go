package main

import (
	"os"
	"path/filepath"
	"testing"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

func TestLauncherSetupBeforeNativeLoad(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "synthetic-library")
	// This is deliberately not a loadable library: configuration must not open it.
	if err := os.WriteFile(library, []byte("not a native library"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"", "http://127.0.0.1:1234", "http://127.0.0.1:1234/", "http://[::1]:1234"} {
		t.Run(base, func(t *testing.T) {
			v, err := newView(dir, base, library)
			if err != nil {
				t.Fatal(err)
			}
			defer v.Close()
			if v.Controller.Active() || v.Controller.Snapshot().State != contract.Idle || mpvprobe.Owned() != (mpvprobe.Resources{}) {
				t.Fatal("configuration acquired native resources or opened media")
			}
			if base == "" {
				if len(v.Sources) != 1 || v.Sources[0].Input.FilePath != filepath.Join(dir, "combined.mp4") {
					t.Fatal("local-only launch requires no HTTP server")
				}
			} else {
				for _, source := range v.Sources {
					if err := source.Input.Validate(); err != nil {
						t.Fatal(source.Key, err)
					}
				}
				if base == "http://127.0.0.1:1234/" && v.Sources[1].Input.URL != "http://127.0.0.1:1234/plain/combined.mp4" {
					t.Fatal("trailing slash corrupted the fixture route")
				}
			}
		})
	}
	for _, invalid := range []string{"", "relative-library", dir, filepath.Join(dir, "absent")} {
		if v, err := newView(dir, "", invalid); err == nil || v != nil {
			t.Fatal("invalid library accepted", invalid)
		}
	}
	for _, invalid := range []string{
		"https://example.com", "http://192.0.2.1:1234", "http://localhost:1234",
		"http://user@127.0.0.1:1234", "http://127.0.0.1:1234/path",
		"http://127.0.0.1:1234/%2F", "http://127.0.0.1:1234?key=value",
		"http://127.0.0.1:1234?", "http://127.0.0.1:1234#fragment",
		"http://127.0.0.1:1234#", "http://127.0.0.1:65536", "://malformed",
	} {
		if v, err := newView(dir, invalid, library); err == nil || v != nil {
			t.Fatal("invalid fixture origin accepted", invalid)
		}
	}
}
