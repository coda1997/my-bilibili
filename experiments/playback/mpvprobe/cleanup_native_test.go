//go:build darwin && arm64 && nativecleanup

package mpvprobe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

// This explicit native suite must run uncached with a provenance-verified
// MY_BILIBILI_MPV_LIBRARY. Ordinary tests neither load libmpv nor run this suite.
// Each child remains alive beyond CoreAudio's seven-second idle timer after
// closing its final session: zero Go counters alone missed the original crash.
func TestNativeCoreAudioCleanup(t *testing.T) {
	library := os.Getenv("MY_BILIBILI_MPV_LIBRARY")
	if !filepath.IsAbs(library) {
		t.Fatal("set MY_BILIBILI_MPV_LIBRARY to the verified absolute runtime path")
	}
	if scenario := os.Getenv("MY_BILIBILI_COREAUDIO_SCENARIO"); scenario != "" {
		nativeCleanupScenario(t, library, scenario)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"paused-close", "playing-close", "pause-resume", "idle-expiry", "reopen"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestNativeCoreAudioCleanup$", "-test.v", "-test.timeout=30s")
			cmd.Env = append(os.Environ(), "MY_BILIBILI_COREAUDIO_SCENARIO="+scenario)
			output, err := cmd.CombinedOutput()
			t.Logf("native subprocess:\n%s", output)
			if err != nil {
				t.Fatalf("native cleanup did not survive: %v (deadline: %v)", err, ctx.Err())
			}
		})
	}
}

func nativeCleanupScenario(t *testing.T, library, scenario string) {
	t.Helper()
	fixture, err := filepath.Abs("../testdata/combined.mp4")
	if err != nil {
		t.Fatal(err)
	}
	cycles := 1
	if scenario == "reopen" {
		cycles = 3
	}
	for cycle := 0; cycle < cycles; cycle++ {
		e, err := (Opener{Library: library}).Open(context.Background(), contract.Input{FilePath: fixture})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = e.Close() })
		nativeCleanupAwait(t, e, 10*time.Second, func(s contract.Snapshot) bool { return s.State == contract.Ready })
		play := func() {
			before := e.Snapshot().Position
			if err := e.Command(contract.Command{Kind: contract.Play}); err != nil {
				t.Fatal(err)
			}
			nativeCleanupAwait(t, e, 2*time.Second, func(s contract.Snapshot) bool {
				return s.State == contract.Playing && s.Position >= before+300*time.Millisecond
			})
		}
		switch scenario {
		case "paused-close", "reopen":
		case "playing-close":
			play()
		case "pause-resume":
			for n := 0; n < 4; n++ {
				play()
				if err := e.Command(contract.Command{Kind: contract.Pause}); err != nil {
					t.Fatal(err)
				}
				nativeCleanupAwait(t, e, 2*time.Second, func(s contract.Snapshot) bool { return s.State == contract.Paused })
			}
		case "idle-expiry":
			// Let the real idle deadline expire, then verify native progression
			// resumes before closing; physical sound is a separate observation.
			time.Sleep(8 * time.Second)
			play()
		default:
			t.Fatal("unknown native cleanup scenario")
		}
		started := time.Now()
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(started)
		t.Logf("cycle=%d close=%s owned=%+v", cycle, elapsed, Owned())
		if elapsed > 200*time.Millisecond {
			t.Errorf("Close exceeded the 200ms cancellation budget: %s", elapsed)
		}
		if e.Snapshot().State != contract.Closed || Owned() != (Resources{}) {
			t.Fatal("session or adapter ownership remains after Close")
		}
		if err := e.Close(); err != nil {
			t.Fatal("repeated Close:", err)
		}
	}
	time.Sleep(8 * time.Second)
	t.Log("survived delayed native cleanup")
}

func nativeCleanupAwait(t *testing.T, e contract.Engine, limit time.Duration, ready func(contract.Snapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for {
		s := e.Snapshot()
		if s.State == contract.Failed {
			t.Fatalf("native session failed: %s", s.Error)
		}
		if ready(s) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("native state deadline: %+v", s)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
