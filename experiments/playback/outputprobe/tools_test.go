package outputprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolChild(t *testing.T) {
	if len(os.Args) < 2 {
		return
	}
	mode := os.Args[len(os.Args)-1]
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
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, _, err := runTool(ctx, os.Args[0], []string{"-test.run=^TestToolChild$", "--", "output-probe-child-" + mode}, 4096)
			if err == nil {
				t.Fatal("failure hidden")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("child not reaped within bounded deadline")
			}
		})
	}
	if _, _, err := runTool(context.Background(), filepath.Join(t.TempDir(), "absent"), nil, 100); err == nil {
		t.Fatal("missing tool accepted")
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
