// Replacement-smoke is an explicit, externally supervised native probe.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"my-bilibili/experiments/playback/cmd/fixturehttp"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

type check struct {
	Name     string    `json:"name"`
	Result   string    `json:"result"`
	Detail   string    `json:"detail"`
	Snapshot *observed `json:"snapshot,omitempty"`
}
type report struct {
	Stage          string             `json:"stage"`
	Platform       string             `json:"platform"`
	Go             string             `json:"go"`
	AppSHA256      string             `json:"app_sha256"`
	RuntimeSHA256  string             `json:"runtime_manifest_sha256"`
	Checks         []check            `json:"checks"`
	ServerStats    *fixturehttp.Stats `json:"server_stats,omitempty"`
	ForcedCleanup  bool               `json:"forced_cleanup"`
	RuntimeBlocked bool               `json:"runtime_blocked"`
}
type artifact struct{ Path, SHA256 string }
type runtimeManifest struct {
	SourceCommit string
	API          string
	Library      artifact
	Dependencies []artifact
}

func stage(name string) error {
	if name != "surface" {
		return errors.New("only the surface stage is implemented; broader capabilities require gate-one review")
	}
	return nil
}

func verifyArtifact(a artifact) error {
	info, err := os.Lstat(a.Path)
	if !filepath.IsAbs(a.Path) || err != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 {
		return errors.New("invalid runtime artifact")
	}
	b, err := os.ReadFile(a.Path)
	if err != nil {
		return errors.New("runtime artifact unavailable")
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != a.SHA256 {
		return errors.New("runtime artifact changed")
	}
	return nil
}

func verifyRuntime(path string) (runtimeManifest, string, error) {
	var m runtimeManifest
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return m, "", errors.New("invalid runtime manifest")
	}
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &m) != nil || m.SourceCommit != "b0841e8c4d156b8403dc1876c4cbd9011eb583f3" || m.API != "2.6" || len(m.Dependencies) == 0 || len(m.Dependencies) > 256 {
		return m, "", errors.New("runtime identity is incomplete or incompatible")
	}
	if err = verifyArtifact(m.Library); err != nil {
		return m, "", err
	}
	for _, a := range m.Dependencies {
		if err = verifyArtifact(a); err != nil {
			return m, "", err
		}
	}
	h := sha256.Sum256(b)
	return m, hex.EncodeToString(h[:]), nil
}

func main() {
	selected := flag.String("stage", "surface", "explicit native stage: surface")
	app := flag.String("app", "", "owned e2e replacement app")
	out := flag.String("out", "", "fresh private evidence directory")
	fixtures := flag.String("fixtures", "experiments/playback/testdata", "generated fixture directory")
	manifest := flag.String("runtime", "", "provenance-verified local runtime manifest")
	flag.Parse()
	if err := stage(*selected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *app == "" || *manifest == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "-app, -runtime and -out are required")
		os.Exit(2)
	}
	if os.Mkdir(*out, 0700) != nil {
		fmt.Fprintln(os.Stderr, "cannot create fresh evidence directory")
		os.Exit(1)
	}
	r := report{Stage: *selected, Platform: runtime.GOOS + "/" + runtime.GOARCH, Go: runtime.Version()}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	err := run(ctx, *app, *out, *fixtures, *manifest, &r)
	cancel()
	if err != nil {
		result := "fail"
		if r.RuntimeBlocked {
			result = "not-tested"
		}
		r.Checks = append(r.Checks, check{Name: "surface-stage", Result: result, Detail: err.Error()})
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	writeErr := os.WriteFile(filepath.Join(*out, "result.json"), append(b, '\n'), 0600)
	fmt.Println(string(b))
	if err != nil || writeErr != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, app, dir, fixtures, manifest string, r *report) (err error) {
	if err = stage(r.Stage); err != nil {
		return err // Before app, runtime or fixture access.
	}
	m, digest, err := verifyRuntime(manifest)
	if err != nil {
		return err
	}
	r.RuntimeSHA256 = digest
	app, err = filepath.Abs(app)
	if err != nil {
		return errors.New("invalid app path")
	}
	b, err := os.ReadFile(app)
	if err != nil {
		return errors.New("app unavailable")
	}
	h := sha256.Sum256(b)
	r.AppSHA256 = hex.EncodeToString(h[:])
	srv, err := fixturehttp.Start(ctx, fixtures, 3*time.Second)
	if err != nil {
		return errors.New("fixture server preparation failed")
	}
	defer func() {
		closeErr := srv.Close()
		stats := srv.Stats()
		r.ServerStats = &stats
		if closeErr != nil || stats.Active != 0 {
			err = errors.New("fixture server teardown failed")
		} else {
			r.Checks = append(r.Checks, check{Name: "server-cleanup", Result: "pass", Detail: "zero active handlers after bounded shutdown"})
		}
	}()
	socketDir, err := os.MkdirTemp("", "rpl-")
	if err != nil {
		return errors.New("socket directory unavailable")
	}
	defer os.RemoveAll(socketDir)
	socket := filepath.Join(socketDir, "a.sock")
	log, err := os.OpenFile(filepath.Join(dir, "native.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("cannot create native log")
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, app, "-fixtures", fixtures, "-http-base", srv.BaseURL, "-library", m.Library.Path)
	cmd.Env = append(os.Environ(), "MYGO_AUTOMATION="+socket, "DYLD_PRINT_LIBRARIES=1")
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = log, log, 3*time.Second
	if cmd.Start() != nil {
		return errors.New("owned app launch failed")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if done != nil {
			r.ForcedCleanup = true
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				err = errors.New("forced app reap deadline")
			}
		}
	}()
	var conn net.Conn
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
		conn, err = (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", socket)
		if err == nil {
			break
		}
		if wait(ctx, 100*time.Millisecond) != nil {
			return errors.New("app startup deadline")
		}
	}
	if conn == nil {
		return errors.New("automation socket unavailable")
	}
	defer conn.Close()
	c := newClient(conn)
	var windows struct {
		Windows []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"windows"`
	}
	if err = c.call(ctx, "windows", nil, &windows); err != nil {
		return err
	}
	if len(windows.Windows) != 1 || windows.Windows[0].Title != "Replacement Playback Probe" {
		return errors.New("owned replacement window not found")
	}
	c.window = windows.Windows[0].ID
	if err = c.surface(ctx, dir, r, srv); err != nil {
		return err
	}
	if err = c.act(ctx, "quit"); err != nil {
		return err
	}
	select {
	case exitErr := <-done:
		done = nil
		if exitErr != nil {
			return errors.New("owned app exit failed")
		}
	case <-time.After(3 * time.Second):
		return errors.New("owned app graceful-exit deadline")
	}
	if _, err = os.Stat(socket); !os.IsNotExist(err) {
		return errors.New("owned automation socket remains")
	}
	r.Checks = append(r.Checks, check{Name: "loading-window-close", Result: "pass", Detail: "graceful exit; app checked native/worker counters zero; socket removed"})
	return nil
}

func (c *client) surface(ctx context.Context, dir string, r *report, srv *fixturehttp.Server) error {
	for _, key := range []string{"local", "http"} {
		s, err := c.ready(ctx, key)
		if err != nil {
			return err
		}
		if s.WindowHidden {
			r.RuntimeBlocked = true
			return errors.New("owned window occluded: picture/control acceptance not-tested")
		}
		if err = wait(ctx, 200*time.Millisecond); err != nil {
			return err
		}
		if err = c.picture(ctx, dir, key+".png"); err != nil {
			return err
		}
		record(r, key+"-picture", "pass", "synthetic green/red/blue pixels captured inside mygo", s)
	}
	old, err := c.loading(ctx, srv)
	if err != nil {
		return err
	}
	switchStart := time.Now()
	s, err := c.ready(ctx, "local")
	switchElapsed := time.Since(switchStart)
	if err != nil || s.Generation <= old.Generation || s.Position > 500*time.Millisecond {
		return errors.New("loading switch/reset failed")
	}
	current := s.Generation
	if err = wait(ctx, 3100*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil || s.Generation != current || !s.Owned || s.State != contract.Ready {
		return errors.New("obsolete delayed session restored state")
	}
	record(r, "loading-switch-generation", "pass", fmt.Sprintf("replacement ready in %s and remains current beyond original delayed response", switchElapsed), s)
	active, err := c.loading(ctx, srv)
	if err != nil {
		return err
	}
	start := time.Now()
	if err = c.act(ctx, "cancel"); err != nil {
		return err
	}
	s, err = c.until(ctx, 100*time.Millisecond, func(s observed) bool {
		return s.Generation == active.Generation && s.State == contract.Closed && s.Error == contract.Canceled && !s.Owned
	})
	elapsed := time.Since(start)
	if err != nil || elapsed > 100*time.Millisecond {
		record(r, "loading-cancel", "fail", fmt.Sprintf("external observation took %s (100ms bound)", elapsed), s)
		return errors.New("loading cancel observation exceeds 100ms")
	}
	resources, err := c.resources(ctx)
	if err != nil || resources != (mpvprobe.Resources{}) {
		return errors.New("cancel left native resources or workers owned")
	}
	record(r, "loading-cancel", "pass", fmt.Sprintf("closed and no owned engine externally observed in %s; native/worker counters zero", elapsed), s)
	_, err = c.loading(ctx, srv) // Quit with an actual delayed request still owned.
	return err
}

func (c *client) loading(ctx context.Context, srv *fixturehttp.Server) (observed, error) {
	before := srv.Stats().Delayed
	s, err := c.openUntil(ctx, "delayed", 500*time.Millisecond, func(s observed) bool { return s.State == contract.Loading && s.Owned })
	if err != nil {
		return s, fmt.Errorf("delayed loading not observable: %w", err)
	}
	for end := time.Now().Add(time.Second); srv.Stats().Delayed <= before; {
		if !time.Now().Before(end) || wait(ctx, 10*time.Millisecond) != nil {
			return s, errors.New("delayed native HTTP request not observed")
		}
	}
	return s, nil
}
