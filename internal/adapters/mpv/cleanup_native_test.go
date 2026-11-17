//go:build darwin && arm64 && nativecleanup

package mpv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"my-bilibili/internal/playback"
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
	for _, scenario := range []string{"paused-close", "playing-close", "playing-cancel", "pause-resume", "idle-expiry", "reopen"} {
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
	fixture, err := filepath.Abs("../../../experiments/playback/testdata/combined.mp4")
	if err != nil {
		t.Fatal(err)
	}
	cycles := 1
	if scenario == "reopen" || scenario == "playing-close" {
		cycles = 3
	}
	for cycle := 0; cycle < cycles; cycle++ {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		e, err := (Opener{Library: library}).Open(ctx, playback.Input{FilePath: fixture})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = e.Close() })
		nativeCleanupAwait(t, e, 10*time.Second, func(s playback.Snapshot) bool { return s.State == playback.Ready })
		play := func() {
			before := e.Snapshot().Position
			if err := e.Command(playback.Command{Kind: playback.Play}); err != nil {
				t.Fatal(err)
			}
			nativeCleanupAwait(t, e, 2*time.Second, func(s playback.Snapshot) bool {
				return s.State == playback.Playing && s.Position >= before+300*time.Millisecond
			})
		}
		switch scenario {
		case "paused-close", "reopen":
		case "playing-close", "playing-cancel":
			play()
		case "pause-resume":
			for n := 0; n < 4; n++ {
				play()
				if err := e.Command(playback.Command{Kind: playback.Pause}); err != nil {
					t.Fatal(err)
				}
				nativeCleanupAwait(t, e, 2*time.Second, func(s playback.Snapshot) bool { return s.State == playback.Paused })
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
		if scenario == "playing-cancel" {
			cancel()
			<-e.(*engine).done // Include the complete cancellation/join in timing.
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(started)
		if e.(*engine).failure != playback.NoError {
			t.Error("shutdown used its failure path:", e.(*engine).failure)
		}
		t.Logf("cycle=%d close=%s owned=%+v", cycle, elapsed, Owned())
		if elapsed > 200*time.Millisecond {
			t.Errorf("Close exceeded the 200ms cancellation budget: %s", elapsed)
		}
		if e.Snapshot().State != playback.Closed || Owned() != (Resources{}) {
			t.Fatal("session or adapter ownership remains after Close")
		}
		if err := e.Close(); err != nil {
			t.Fatal("repeated Close:", err)
		}
	}
	time.Sleep(8 * time.Second)
	t.Log("survived delayed native cleanup")
}

func nativeCleanupAwait(t *testing.T, e playback.Engine, limit time.Duration, ready func(playback.Snapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for {
		s := e.Snapshot()
		if s.State == playback.Failed {
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
