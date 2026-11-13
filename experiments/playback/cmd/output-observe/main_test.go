package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"my-bilibili/experiments/playback/cmd/fixturehttp"
)

func TestControlledChild(t *testing.T) {
	if seek := os.Getenv("OUTPUT_OBSERVE_INVALID_SEEK"); seek != "" {
		os.Args = []string{"output-observe", "-" + os.Getenv("OUTPUT_OBSERVE_INVALID_FLAG"), seek, "-out", os.Getenv("OUTPUT_OBSERVE_INVALID_OUT")}
		flag.CommandLine = flag.NewFlagSet("output-observe", flag.ExitOnError)
		main()
		os.Exit(0)
	}
	mode := os.Getenv("OUTPUT_OBSERVE_TEST_CHILD")
	if mode == "" {
		return
	}
	if mode == "exit" {
		os.Exit(0)
	}
	if mode == "fail" {
		os.Exit(7)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	if err := os.WriteFile(os.Getenv("OUTPUT_OBSERVE_TEST_READY"), []byte("ready"), 0600); err != nil {
		os.Exit(8)
	}
	if mode == "interrupt" {
		<-signals
		os.Exit(0)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestNonfiniteCLIRejectedBeforeSession(t *testing.T) {
	for _, option := range []string{"seek", "rate"} {
		for _, value := range []string{"NaN", "+Inf", "-Inf"} {
			t.Run(option+"_"+value, func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "session")
				cmd := exec.Command(os.Args[0], "-test.run=^TestControlledChild$")
				cmd.Env = append(os.Environ(), "OUTPUT_OBSERVE_INVALID_SEEK="+value, "OUTPUT_OBSERVE_INVALID_FLAG="+option, "OUTPUT_OBSERVE_INVALID_OUT="+out)
				log, err := cmd.CombinedOutput()
				if err == nil || cmd.ProcessState.ExitCode() != 2 {
					t.Fatalf("expected rejected CLI input before session, got %v: %s", err, log)
				}
				if _, err := os.Stat(out); !os.IsNotExist(err) {
					t.Fatal("invalid option created session directory")
				}
			})
		}
	}
}

func child(mode, ready string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestControlledChild$")
	cmd.Env = append(os.Environ(), "OUTPUT_OBSERVE_TEST_CHILD="+mode, "OUTPUT_OBSERVE_TEST_READY="+ready)
	return cmd
}

func server(t *testing.T) *fixturehttp.Server {
	t.Helper()
	// Legacy files exercise the unchanged server lifecycle only. They are never
	// accepted by prepare as paired-event fixtures or physical evidence.
	srv, err := fixturehttp.StartProbe(context.Background(), "../../testdata", 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func assertClosed(t *testing.T, srv *fixturehttp.Server, r *result) {
	t.Helper()
	if !r.ServerClosed || r.ServerStats.Active != 0 {
		t.Fatalf("server not cleaned: %+v", r)
	}
	c := &http.Client{Timeout: 200 * time.Millisecond}
	response, err := c.Get(srv.BaseURL + "/plain/combined.mp4")
	if err == nil {
		response.Body.Close()
		t.Fatal("owned listener still accepts requests")
	}
	for key, cell := range r.Observations {
		if cell.Status != "not-tested" || cell.Value != nil {
			t.Fatalf("invented %s observation: %+v", key, cell)
		}
	}
}

func TestNormalFailedSpawnAndExpiredCleanup(t *testing.T) {
	for _, scenario := range []string{"exit", "fail", "spawn", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			srv := server(t)
			r := freshResult(options{})
			ctx := context.Background()
			cmd := child(scenario, "")
			if scenario == "spawn" {
				cmd = exec.Command(filepath.Join(t.TempDir(), "missing-app"))
			}
			if scenario == "expired" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
			}
			err := supervise(ctx, cmd, srv, r, 3*time.Second)
			if (err == nil) != (scenario == "exit") {
				t.Fatalf("unexpected result %v", err)
			}
			if r.ForcedReap {
				t.Fatal("unexpected forced reap")
			}
			if scenario == "exit" && (!r.Reaped || r.ExitCode == nil || *r.ExitCode != 0) {
				t.Fatal("normal exit not retained")
			}
			if (scenario == "expired" || scenario == "spawn") && r.AppStarted {
				t.Fatal("invalid child started")
			}
			assertClosed(t, srv, r)
		})
	}
}

func TestInterruptedDeadlineAndForcedCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt child signaling is unavailable on Windows; optional native platform")
	}
	for _, scenario := range []string{"interrupt", "deadline", "forced"} {
		t.Run(scenario, func(t *testing.T) {
			srv := server(t)
			r := freshResult(options{})
			ready := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "deadline" {
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithTimeout(ctx, 2*time.Second)
				defer deadlineCancel()
			}
			mode := "interrupt"
			if scenario == "forced" {
				mode = "ignore"
			}
			done := make(chan error, 1)
			start := time.Now()
			go func() { done <- supervise(ctx, child(mode, ready), srv, r, 3*time.Second) }()
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if time.Since(start) > 5*time.Second {
					cancel()
					<-done
					t.Fatal("child never ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if scenario != "deadline" {
				cancel()
			}
			if err := <-done; err == nil {
				t.Fatal("interrupted session passed")
			}
			if r.ForcedReap != (scenario == "forced") || !r.Reaped {
				t.Fatalf("cleanup outcome lost: %+v", r)
			}
			if scenario == "deadline" && r.StopReason != "deadline" {
				t.Fatal("deadline not distinguished")
			}
			assertClosed(t, srv, r)
		})
	}
}

func TestBoundedLogAndIdentities(t *testing.T) {
	var out bytes.Buffer
	b := &boundedLog{w: &out, remaining: 4}
	if n, err := b.Write([]byte("123456")); n != 6 || err != nil || out.String() != "1234" || !b.truncated {
		t.Fatal("log bound failed")
	}
	path := filepath.Join(t.TempDir(), "app")
	data := []byte("owned reviewed app")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	digest := hex.EncodeToString(h[:])
	if _, err := identify(path, digest, 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := identify(path, digest, 2); err == nil {
		t.Fatal("oversized identity accepted")
	}
	if err := os.WriteFile(path, []byte("different"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := identify(path, digest, 1024); err == nil {
		t.Fatal("changed binary accepted")
	}
}
