// Target-smoke supervises one explicitly authorized transient playback input.
// It never resolves media, retries, captures frames, or retains native output.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	"my-bilibili/experiments/playback/targetprobe"
)

type check struct {
	Name     string        `json:"name"`
	Result   string        `json:"result"`
	Detail   string        `json:"detail"`
	Snapshot *observed     `json:"snapshot,omitempty"`
	Elapsed  time.Duration `json:"elapsed_ns,omitempty"`
}
type report struct {
	Policy         string                `json:"policy"`
	Platform       string                `json:"platform"`
	Go             string                `json:"go"`
	Metadata       *targetprobe.Metadata `json:"source,omitempty"`
	AppSHA256      string                `json:"app_sha256,omitempty"`
	RuntimeSHA256  string                `json:"runtime_manifest_sha256,omitempty"`
	Checks         []check               `json:"checks"`
	ForcedCleanup  bool                  `json:"forced_cleanup"`
	Automation     []automationCall      `json:"automation"`
	DroppedCalls   int                   `json:"dropped_calls"`
	SessionFailure *failureEvidence      `json:"session_failure,omitempty"`
	CleanupFailure *failureEvidence      `json:"cleanup_failure,omitempty"`
	started        time.Time
	phase          driverPhase
	lastSnapshot   *sample[observed]
	lastResources  *sample[mpvprobe.Resources]
}

func safeCode(err error) contract.ErrorCode {
	if err == nil {
		return contract.NoError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return contract.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return contract.Canceled
	}
	var code contract.ErrorCode
	if errors.As(err, &code) && validCode(code) && code != contract.NoError {
		return code
	}
	return contract.Media
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(command(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func command(ctx context.Context, args []string, input io.ReadCloser, output, diagnostic io.Writer) int {
	fs := flag.NewFlagSet("target-smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Flag errors can otherwise echo private caller input.
	app := fs.String("app", "", "owned e2e target app")
	manifest := fs.String("runtime", "", "pinned runtime manifest")
	out := fs.String("out", "", "fresh private evidence directory")
	if fs.Parse(args) != nil || fs.NArg() != 0 || !filepath.IsAbs(*app) || !filepath.IsAbs(*manifest) || !filepath.IsAbs(*out) {
		fmt.Fprintln(diagnostic, "target-smoke requires absolute -app, -runtime and fresh -out; descriptor is stdin only")
		return 2
	}
	// Reserve a fresh result directory before any child/native side effect.
	if os.Mkdir(*out, 0700) != nil {
		fmt.Fprintln(diagnostic, "fresh safe result directory unavailable")
		return 1
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	r := report{Policy: targetprobe.PolicyID, Platform: runtime.GOOS + "/" + runtime.GOARCH, Go: runtime.Version()}
	r.setPhase(phaseDescriptor)
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	data, err := targetprobe.ReadDescriptor(readCtx, input)
	readCancel()
	if err == nil {
		var p *targetprobe.Policy
		p, err = targetprobe.Parse(data, time.Now)
		if err == nil {
			metadata := p.Metadata()
			r.Metadata = &metadata
			childCtx, stop := context.WithDeadline(ctx, p.Deadline())
			err = run(childCtx, *app, *manifest, data, &r)
			stop()
		}
	}
	clear(data) // Best effort only; no claim of Go heap zeroization.
	if err != nil {
		// A cleanup-only failure must not be relabeled as a primary session error.
		if r.CleanupFailure == nil {
			r.sessionFailed(err)
		}
		r.Checks = append(r.Checks, check{Name: "target-session", Result: "fail", Detail: string(safeCode(err))})
	}
	b, marshalErr := json.MarshalIndent(r, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(diagnostic, "safe result encoding failed")
		return 1
	}
	// Retain only the normalized report. Never create a descriptor or native log file.
	if os.WriteFile(filepath.Join(*out, "result.json"), append(b, '\n'), 0600) != nil {
		fmt.Fprintln(diagnostic, "safe result write failed")
		return 1
	}
	_, writeErr := fmt.Fprintln(output, string(b))
	if err != nil || writeErr != nil {
		return 1
	}
	return 0
}

func record(r *report, name, detail string, s observed, elapsed time.Duration) {
	r.Checks = append(r.Checks, check{Name: name, Result: "pass", Detail: detail, Snapshot: &s, Elapsed: elapsed})
}
