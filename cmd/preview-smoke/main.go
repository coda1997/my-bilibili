package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"my-bilibili/internal/fixture"
	"my-bilibili/internal/previewbundle"
)

type check struct {
	Scenario, Name, Result, Detail string
	Observation                    *previewbundle.Observation       `json:",omitempty"`
	ClientObservation              *previewbundle.ClientObservation `json:",omitempty"`
}
type processRecord struct {
	Scenario string
	PID      int
}
type report struct {
	AppSHA256, DriverSHA256, RuntimeSHA256, AssetManifestSHA256 string
	Platform, StartedUTC, Scenario                              string
	Checks                                                      []check
	Processes                                                   []processRecord
	PrimaryFailure, CleanupFailure                              string
	ForcedCleanup                                               bool
	Stage                                                       string
	Diagnostics                                                 []rpcTrace
	traceStart                                                  time.Time
}

func (r *report) pass(name, detail string, o *previewbundle.Observation) {
	if o != nil {
		snapshot := *o
		snapshot.State.Video.Parts = slices.Clone(o.State.Video.Parts)
		snapshot.Danmaku.Items = slices.Clone(o.Danmaku.Items)
		o = &snapshot
	}
	r.Checks = append(r.Checks, check{Scenario: r.Scenario, Name: name, Result: "pass", Detail: detail, Observation: o})
}
func (r *report) passed() int {
	count := 0
	for _, check := range r.Checks {
		if check.Result == "pass" {
			count++
		}
	}
	return count
}

func (r *report) passClient(name, detail string, o previewbundle.ClientObservation) {
	r.Checks = append(r.Checks, check{Scenario: r.Scenario, Name: name, Result: "pass", Detail: detail, ClientObservation: &o})
}
func main() {
	if err := command(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && len(s) == 64
}
func command(args []string, out, stderr io.Writer) error {
	f := flag.NewFlagSet("preview-smoke", flag.ContinueOnError)
	f.SetOutput(stderr)
	app := f.String("app", "", "absolute e2e bundle executable")
	appHash := f.String("app-sha256", "", "assigned exact app SHA256")
	driverHash := f.String("driver-sha256", "", "assigned exact driver SHA256")
	dir := f.String("output", "", "fresh absolute evidence directory")
	stage := f.String("stage", "full", "full acceptance, fullscreen diagnostic prefix, synthetic source, danmaku, or client scenarios")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || !filepath.IsAbs(*app) || !filepath.IsAbs(*dir) || !validDigest(*appHash) || !validDigest(*driverHash) {
		return errors.New("require absolute -app/-output and exact -app-sha256/-driver-sha256")
	}
	if *stage != "full" && *stage != "fullscreen" && *stage != "source" && *stage != "danmaku" && *stage != "client" {
		return errors.New("stage must be full, fullscreen, source, danmaku or client")
	}
	if runtime.GOOS != "darwin" {
		return errors.New("native preview smoke requires macOS")
	}
	if err := previewbundle.VerifyRuntime(); err != nil {
		return err
	}
	if err := verifyAssignedApp(*app, *appHash); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return errors.New("driver identity unavailable")
	}
	h, err := previewbundle.Hash(executable)
	if err != nil || h != *driverHash {
		return errors.New("assigned preview driver identity mismatch")
	}
	resources, err := previewbundle.Resources(*app)
	if err != nil {
		return err
	}
	if _, err = fixture.New(resources); err != nil {
		return errors.New("packaged fixture integrity failed")
	}
	assetsHash, err := previewbundle.Hash(filepath.Join(resources, "manifest.json"))
	if err != nil {
		return err
	}
	if err = os.Mkdir(*dir, 0700); err != nil {
		return errors.New("evidence output must be fresh with an existing parent")
	}
	r := &report{AppSHA256: *appHash, DriverSHA256: *driverHash, RuntimeSHA256: previewbundle.RuntimeSHA256, AssetManifestSHA256: assetsHash, Platform: runtime.GOOS + "/" + runtime.GOARCH, StartedUTC: time.Now().UTC().Format(time.RFC3339)}
	r.Stage, r.traceStart = *stage, time.Now()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	err = run(ctx, *app, *dir, r)
	if err != nil && r.PrimaryFailure == "" {
		r.PrimaryFailure = err.Error()
	}
	r.Checks = append(r.Checks, check{Name: "physical-audibility-calibrated-sync-Windows-CI", Result: "not-tested", Detail: "selected audio tracks are not physical audibility; this invocation is macOS synthetic fixture evidence only"})
	data, marshalErr := json.MarshalIndent(r, "", "  ")
	if marshalErr != nil || os.WriteFile(filepath.Join(*dir, "report.json"), append(data, '\n'), 0600) != nil {
		return errors.New("preview evidence persistence failed")
	}
	if err != nil {
		return fmt.Errorf("preview smoke failed: %w", err)
	}
	_, err = fmt.Fprintf(out, "preview smoke stage %s passed %d recorded checks; consult report for required independent visual inspection and not-tested scopes\n", r.Stage, r.passed())
	return err
}
