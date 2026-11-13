// output-observe supervises a manual synthetic-fixture session. It records no
// screen, microphone, system audio or camera data and makes no physical claims.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"my-bilibili/experiments/playback/cmd/fixturehttp"
	"my-bilibili/experiments/playback/outputprobe"
)

type identity struct {
	Path, SHA256 string
	Bytes        int64
}
type runtimeIdentity struct {
	SourceCommit string
	API          string
	Library      identity
	Dependencies []identity
}
type options struct {
	App, AppHash, Runtime, RuntimeHash, Fixtures, TimingHash, Out string
	Source, Segment                                               string
	Rate, Seek                                                    float64
	Duration                                                      time.Duration
}
type observation struct {
	Status   string  `json:"status"`
	Observer string  `json:"observer,omitempty"`
	At       string  `json:"at,omitempty"`
	Evidence string  `json:"evidence,omitempty"`
	Value    *string `json:"value"`
}
type result struct {
	SchemaVersion int                         `json:"schema_version"`
	Started       time.Time                   `json:"started"`
	Finished      time.Time                   `json:"finished"`
	Platform      string                      `json:"platform"`
	Status        string                      `json:"status"`
	Failure       string                      `json:"failure,omitempty"`
	Intent        options                     `json:"intended_configuration_only"`
	App           identity                    `json:"app"`
	Runtime       identity                    `json:"runtime_manifest"`
	Library       identity                    `json:"library"`
	Timing        identity                    `json:"timing_manifest"`
	Manifest      identity                    `json:"fixture_manifest"`
	Media         []outputprobe.MediaIdentity `json:"media"`
	AppStarted    bool                        `json:"app_started"`
	ExitCode      *int                        `json:"exit_code"`
	StopReason    string                      `json:"stop_reason"`
	ForcedReap    bool                        `json:"forced_reap"`
	Reaped        bool                        `json:"reaped"`
	ServerClosed  bool                        `json:"server_closed"`
	ServerStats   fixturehttp.Stats           `json:"server_stats"`
	LogTruncated  bool                        `json:"log_truncated"`
	Observations  map[string]observation      `json:"observations"`
}

func freshResult(o options) *result {
	r := &result{SchemaVersion: 1, Started: time.Now().UTC(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Status: "partial", Intent: o, Observations: map[string]observation{}}
	for _, key := range []string{"selected_source", "rate_readback", "seek_readback", "video_streams", "audio_streams", "human_audibility", "raw_physical_output", "calibrated_sync", "physical_video_rate", "physical_audio_rate", "adapter_200ms_cancellation", "native_resource_disposal"} {
		r.Observations[key] = observation{Status: "not-tested"}
	}
	return r
}

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && hex.EncodeToString(b) == s
}

func identify(path, expected string, limit int64) (identity, error) {
	a := identity{Path: path}
	if !filepath.IsAbs(path) || !validDigest(expected) {
		return a, errors.New("absolute path and explicit SHA256 required")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > limit {
		return a, errors.New("invalid bounded identity file")
	}
	f, err := os.Open(path)
	if err != nil {
		return a, errors.New("identity file unavailable")
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return a, errors.New("identity file changed during open")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil || n != before.Size() || n > limit || hex.EncodeToString(h.Sum(nil)) != expected {
		return a, errors.New("identity mismatch")
	}
	a.SHA256, a.Bytes = expected, n
	return a, nil
}

func manifestIdentity(path string, limit int64) (identity, error) {
	b, err := readBounded(path, limit)
	if err != nil {
		return identity{}, errors.New("manifest unavailable")
	}
	h := sha256.Sum256(b)
	return identify(path, hex.EncodeToString(h[:]), limit)
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("manifest unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("invalid bounded manifest")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("manifest read failed")
	}
	return b, nil
}

func validateOptions(o options) error {
	if math.IsNaN(o.Rate) || math.IsInf(o.Rate, 0) || math.IsNaN(o.Seek) || math.IsInf(o.Seek, 0) || o.Duration <= 0 || o.Duration > 600*time.Second || (o.Source != "combined" && o.Source != "separate") || (o.Segment != "steady" && o.Segment != "post-seek") || (o.Rate != .5 && o.Rate != 1 && o.Rate != 2) || o.Seek < 0 || o.Seek > 3 || (o.Segment == "post-seek" && o.Rate != 1) {
		return errors.New("invalid duration or intended observation segment; numeric options must be finite")
	}
	return nil
}

func prepare(o options, r *result) (runtimeIdentity, error) {
	var m runtimeIdentity
	if err := validateOptions(o); err != nil {
		return m, err
	}
	var err error
	if r.App, err = identify(o.App, o.AppHash, 256<<20); err != nil {
		return m, err
	}
	if r.Runtime, err = identify(o.Runtime, o.RuntimeHash, 4<<20); err != nil {
		return m, err
	}
	b, err := readBounded(o.Runtime, 4<<20)
	h := sha256.Sum256(b)
	if err != nil || hex.EncodeToString(h[:]) != o.RuntimeHash || json.Unmarshal(b, &m) != nil || m.SourceCommit != "b0841e8c4d156b8403dc1876c4cbd9011eb583f3" || m.API != "2.6" || len(m.Dependencies) == 0 || len(m.Dependencies) > 256 {
		return m, errors.New("incompatible runtime manifest")
	}
	for _, a := range append([]identity{m.Library}, m.Dependencies...) {
		verified, err := identify(a.Path, a.SHA256, 128<<20)
		if err != nil || verified.Bytes != a.Bytes {
			return m, errors.New("runtime artifact mismatch")
		}
	}
	r.Library = m.Library
	if r.Timing, err = identify(filepath.Join(o.Fixtures, outputprobe.TimingManifestName), o.TimingHash, 2<<20); err != nil {
		return m, err
	}
	fixtures, err := outputprobe.VerifyDirectory(o.Fixtures)
	if err != nil {
		return m, err
	}
	r.Media = fixtures.Media
	r.Manifest, err = manifestIdentity(filepath.Join(o.Fixtures, "manifest.json"), 32<<10)
	return m, err
}

// The two child streams share one bounded writer. Excess log data is discarded,
// never allowed to block a child while the supervisor is trying to reap it.
type boundedLog struct {
	mu        sync.Mutex
	w         io.Writer
	remaining int
	truncated bool
}

func (b *boundedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if len(p) > b.remaining {
		p = p[:b.remaining]
		b.truncated = true
	}
	written, err := b.w.Write(p)
	b.remaining -= written
	if err != nil {
		return written, err
	}
	return n, nil
}

// supervise is shared by the real launcher and controlled-process tests. Only
// normal manual app exit completes the session. Interrupt/deadline is partial;
// forced termination is failure. Process exit never proves native disposal.
func supervise(ctx context.Context, cmd *exec.Cmd, srv *fixturehttp.Server, r *result, cleanup time.Duration) (err error) {
	defer func() {
		closeErr := srv.Close()
		r.ServerStats = srv.Stats()
		r.ServerClosed = closeErr == nil && r.ServerStats.Active == 0
		if !r.ServerClosed {
			err = errors.Join(err, errors.New("server cleanup failed"))
		}
	}()
	if err = ctx.Err(); err != nil {
		r.StopReason = "canceled-before-spawn"
		return err
	}
	cmd.WaitDelay = cleanup / 3
	if err = cmd.Start(); err != nil {
		r.StopReason = "spawn-failed"
		return errors.New("owned app spawn failed")
	}
	r.AppStarted = true
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finish := func(waitErr error) error {
		r.Reaped = true
		code := cmd.ProcessState.ExitCode()
		r.ExitCode = &code
		if waitErr != nil {
			return errors.New("owned app exited unsuccessfully")
		}
		return nil
	}
	select {
	case waitErr := <-done:
		r.StopReason = "app-exit"
		return finish(waitErr)
	case <-ctx.Done():
		r.StopReason = "interrupted"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.StopReason = "deadline"
		}
	}
	_ = cmd.Process.Signal(os.Interrupt)
	grace := time.NewTimer(cleanup * 2 / 3)
	defer grace.Stop()
	select {
	case waitErr := <-done:
		return errors.Join(ctx.Err(), finish(waitErr))
	case <-grace.C:
	}
	r.ForcedReap = true
	_ = cmd.Process.Kill()
	reap := time.NewTimer(cleanup / 3)
	defer reap.Stop()
	select {
	case waitErr := <-done:
		return errors.Join(errors.New("forced app reap"), finish(waitErr))
	case <-reap.C:
		return errors.New("owned app reap deadline")
	}
}

func execute(ctx context.Context, o options, r *result) error {
	m, err := prepare(o, r)
	if err != nil {
		return err
	}
	srv, err := fixturehttp.StartProbe(context.Background(), o.Fixtures, 3*time.Second)
	if err != nil {
		return errors.New("fixture server preparation failed")
	}
	log, err := os.OpenFile(filepath.Join(o.Out, "app.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		_ = srv.Close()
		return errors.New("session log unavailable")
	}
	defer log.Close()
	bounded := &boundedLog{w: log, remaining: 64 << 10}
	cmd := exec.Command(o.App, "-fixtures", o.Fixtures, "-http-base", srv.BaseURL, "-library", m.Library.Path)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "MYGO_AUTOMATION=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Stdout, cmd.Stderr = bounded, bounded
	ctx, cancel := context.WithTimeout(ctx, o.Duration)
	defer cancel()
	err = supervise(ctx, cmd, srv, r, 3*time.Second)
	bounded.mu.Lock()
	r.LogTruncated = bounded.truncated
	bounded.mu.Unlock()
	return err
}

func main() {
	var o options
	flag.StringVar(&o.App, "app", "", "absolute path to reviewed replacement-native app")
	flag.StringVar(&o.AppHash, "app-sha256", "", "expected app SHA256 from candidate evidence")
	flag.StringVar(&o.Runtime, "runtime", "", "absolute runtime manifest path")
	flag.StringVar(&o.RuntimeHash, "runtime-sha256", "", "expected runtime manifest SHA256")
	flag.StringVar(&o.Fixtures, "fixtures", "", "absolute generated output fixture directory")
	flag.StringVar(&o.TimingHash, "timing-sha256", "", "expected verified timing manifest SHA256")
	flag.StringVar(&o.Out, "out", "", "new evidence directory (parent must exist)")
	flag.StringVar(&o.Source, "source", "combined", "intended manual source: combined or separate")
	flag.StringVar(&o.Segment, "segment", "steady", "intended segment: steady or post-seek")
	flag.Float64Var(&o.Rate, "rate", 1, "intended manual rate: 0.5, 1 or 2")
	flag.Float64Var(&o.Seek, "seek", 0, "intended seek seconds, at most 3 to retain four events")
	flag.DurationVar(&o.Duration, "duration", 180*time.Second, "session limit, at most 600s")
	flag.Parse()
	if flag.NArg() != 0 || o.Out == "" {
		fmt.Fprintln(os.Stderr, "explicit options and fresh -out required")
		os.Exit(2)
	}
	if err := validateOptions(o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.Mkdir(o.Out, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create new evidence directory")
		os.Exit(2)
	}
	r := freshResult(o)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	fmt.Println("Manual synthetic observation only. Select the intended source/rate in the app; close its window before the deadline. No recorder is started.")
	err := execute(ctx, o, r)
	r.Finished = time.Now().UTC()
	if err != nil {
		r.Failure = err.Error()
		if r.ForcedReap || !r.AppStarted || r.ExitCode != nil && *r.ExitCode != 0 || !r.ServerClosed {
			r.Status = "failed"
		}
	} else {
		r.Status = "session-completed; observations not-tested"
	}
	b, marshalErr := json.MarshalIndent(r, "", "  ")
	if marshalErr != nil || os.WriteFile(filepath.Join(o.Out, "session.json"), append(b, '\n'), 0600) != nil {
		fmt.Fprintln(os.Stderr, "session evidence could not be saved")
		os.Exit(1)
	}
	fmt.Println(filepath.Join(o.Out, "session.json"))
	if err != nil {
		os.Exit(1)
	}
}
