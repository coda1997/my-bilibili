package outputprobe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestToolChild(t *testing.T) {
	if len(os.Args) < 2 {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if stream, count, ok := strings.Cut(mode, ":"); ok && (stream == "output-probe-stdout" || stream == "output-probe-stderr") {
		n, err := strconv.Atoi(count)
		if err != nil || n < 0 || n > 1<<20 {
			os.Exit(8)
		}
		out := os.Stdout
		if stream == "output-probe-stderr" {
			out = os.Stderr
		}
		if _, err := out.Write([]byte(strings.Repeat("x", n))); err != nil {
			os.Exit(9)
		}
		os.Exit(0)
	}
	switch mode {
	case "output-probe-child-block":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "output-probe-child-flood":
		for i := 0; i < 10000; i++ {
			fmt.Println(strings.Repeat("x", 1024))
		}
		os.Exit(0)
	case "output-probe-child-fail":
		fmt.Fprintln(os.Stderr, "controlled tool failure")
		os.Exit(7)
	}
}
func TestToolCancellationOutputAndFailure(t *testing.T) {
	for _, mode := range []string{"block", "flood", "fail"} {
		t.Run(mode, func(t *testing.T) {
			deadline := 5 * time.Second
			if mode == "block" {
				deadline = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			start := time.Now()
			stdout, stderr, err := runTool(ctx, os.Args[0], []string{"-test.run=^TestToolChild$", "--", "output-probe-child-" + mode}, 4096)
			switch mode {
			case "block":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("caller deadline lost: %v", err)
				}
			case "flood":
				if err == nil || err.Error() != "tool output limit exceeded" || len(stdout) > 4096 || len(stderr) > 64<<10 {
					t.Fatalf("output limit not enforced: stdout=%d stderr=%d err=%v", len(stdout), len(stderr), err)
				}
			case "fail":
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 7 || stderr != "controlled tool failure\n" {
					t.Fatalf("tool failure lost: stderr=%q err=%v", stderr, err)
				}
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("child not reaped within bounded deadline")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := runTool(ctx, os.Args[0], []string{"-test.run=^TestToolChild$", "--", "output-probe-child-block"}, 4096); !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation lost: %v", err)
	}
	if _, _, err := runTool(context.Background(), filepath.Join(t.TempDir(), "absent"), nil, 100); err == nil {
		t.Fatal("missing tool accepted")
	}
}

func TestToolOutputBoundaries(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		limit := 4096
		if stream == "stderr" {
			limit = 64 << 10
		}
		for _, count := range []int{limit - 1, limit, limit + 1} {
			t.Run(fmt.Sprintf("%s/%d", stream, count), func(t *testing.T) {
				// Separate the output bound from a caller timeout, including the
				// race runtime's normal child-exit delay.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				stdout, stderr, err := runTool(ctx, os.Args[0], []string{"-test.run=^TestToolChild$", "--", fmt.Sprintf("output-probe-%s:%d", stream, count)}, 4096)
				captured, other := stdout, []byte(stderr)
				if stream == "stderr" {
					captured, other = other, captured
				}
				if len(captured) > limit || len(other) != 0 {
					t.Fatalf("capture exceeded limit or crossed streams: captured=%d limit=%d other=%d err=%v", len(captured), limit, len(other), err)
				}
				if count > limit {
					if err == nil || err.Error() != "tool output limit exceeded" {
						t.Fatalf("overflow not reported: captured=%d err=%v", len(captured), err)
					}
				} else if err != nil || string(captured) != strings.Repeat("x", count) {
					t.Fatalf("bounded output changed: captured=%d want=%d err=%v", len(captured), count, err)
				}
			})
		}
	}
}
func TestDestinationProtection(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "existing")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(existing, link); err != nil {
		t.Fatal(err)
	}
	for _, dest := range []string{"", existing, link, filepath.Join(parent, "absent", "child"), "../testdata/never-write"} {
		if _, err := validateDestination(dest); err == nil {
			t.Fatalf("destination accepted: %s", dest)
		}
	}
	dest := filepath.Join(parent, "new")
	if _, err := validateDestination(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(context.Background(), GenerateOptions{Destination: dest, Timeout: 601 * time.Second}); err == nil {
		t.Fatal("unbounded timeout accepted")
	}
	if _, err := Generate(context.Background(), GenerateOptions{Destination: dest, FFmpeg: filepath.Join(parent, "no-tool")}); err == nil {
		t.Fatal("missing generator tool accepted")
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatal("failed preparation published destination")
	}
}

func TestMain(m *testing.M) {
	if mode := os.Getenv("OUTPUTPROBE_TEST_TOOL"); mode != "" {
		for _, arg := range os.Args[1:] {
			if arg == "-version" {
				fmt.Println("controlled synthetic tool v1")
				os.Exit(0)
			}
		}
		if mode == "encode-failure" {
			fmt.Fprintln(os.Stderr, "controlled encoding failure")
			os.Exit(9)
		}
		for _, arg := range os.Args[1:] {
			if arg == "-show_entries" {
				fmt.Println(`{"streams":[],"format":{"duration":"8.000000"}}`)
				os.Exit(0)
			}
		}
		if err := os.WriteFile(os.Args[len(os.Args)-1], []byte("invalid controlled media"), 0600); err != nil {
			os.Exit(8)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestGenerationFailurePreservesStageWithoutPublishing(t *testing.T) {
	for _, mode := range []string{"encode-failure", "invalid-decode"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("OUTPUTPROBE_TEST_TOOL", mode)
			parent := t.TempDir()
			dest := filepath.Join(parent, "published")
			m, err := Generate(context.Background(), GenerateOptions{Destination: dest, FFmpeg: os.Args[0], FFprobe: os.Args[0]})
			if err == nil || m == nil || m.Status != "incomplete" {
				t.Fatalf("failed tool/decoder accepted: %+v %v", m, err)
			}
			if _, err := os.Lstat(dest); !os.IsNotExist(err) {
				t.Fatal("failed fixture published")
			}
			stages, err := filepath.Glob(filepath.Join(parent, ".output-fixtures-stage-*"))
			if err != nil || len(stages) != 1 {
				t.Fatalf("missing failure stage: %v %v", stages, err)
			}
			if _, err := os.Stat(filepath.Join(stages[0], "failed-timing.json")); err != nil {
				t.Fatal("failed command/audit evidence discarded")
			}
		})
	}
}
