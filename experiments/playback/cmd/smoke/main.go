// Smoke is an explicit native experiment. Ordinary go test never launches UI.
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
)

type check struct {
	Name     string    `json:"name"`
	Result   string    `json:"result"`
	Detail   string    `json:"detail"`
	Snapshot *observed `json:"snapshot,omitempty"`
}
type report struct {
	Suite          string             `json:"suite"`
	Platform       string             `json:"platform"`
	Go             string             `json:"go"`
	AppSHA256      string             `json:"app_sha256"`
	Checks         []check            `json:"checks"`
	ServerStats    *fixturehttp.Stats `json:"server_stats,omitempty"`
	RuntimeBlocked bool               `json:"runtime_blocked"`
}

var errOccluded = errors.New("native rendering/control acceptance blocked by hidden/occluded runtime window")

func main() {
	app := flag.String("app", "", "path to the owned e2e experiment binary")
	out := flag.String("out", "", "new private evidence directory")
	fixtures := flag.String("fixtures", "experiments/playback/testdata", "generated fixture directory")
	suite := suiteFlag(flag.CommandLine)
	flag.Parse()
	selected, err := selectSuite(*suite)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *app == "" {
		fmt.Fprintln(os.Stderr, "-app is required")
		os.Exit(2)
	}
	dir := *out
	if dir == "" {
		dir, err = os.MkdirTemp(".agent", "native-smoke-")
	} else {
		err = os.Mkdir(dir, 0700)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create fresh evidence directory")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	r := report{Suite: selected, Platform: runtime.GOOS + "/" + runtime.GOARCH, Go: runtime.Version()}
	err = run(ctx, *app, dir, *fixtures, &r)
	cancel()
	if err != nil {
		result := "fail"
		if errors.Is(err, errOccluded) {
			result = "not-tested"
		}
		r.Checks = append(r.Checks, check{Name: "smoke", Result: result, Detail: err.Error()})
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	writeErr := os.WriteFile(filepath.Join(dir, "result.json"), append(b, '\n'), 0600)
	fmt.Println(string(b))
	fmt.Println("Evidence:", dir)
	if err != nil || writeErr != nil {
		os.Exit(1)
	}
}

func suiteFlag(flags *flag.FlagSet) *string {
	return flags.String("suite", "full", "native suite: full (strict M1 probes) or preview (bounded temporary-player controls only)")
}

func selectSuite(name string) (string, error) {
	switch name {
	case "full":
		return "full", nil
	case "preview":
		return "preview", nil
	default:
		return "", errors.New("unknown native suite: use full or preview")
	}
}

func run(ctx context.Context, app, dir, fixtures string, r *report) (err error) {
	if r.Suite, err = selectSuite(r.Suite); err != nil {
		return err
	}
	app, err = filepath.Abs(app)
	if err != nil {
		return errors.New("invalid app path")
	}
	b, err := os.ReadFile(app)
	if err != nil {
		return errors.New("cannot read app binary")
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
			if err == nil {
				err = errors.New("fixture server teardown failed")
			}
			return
		}
		r.Checks = append(r.Checks, check{Name: "owned-server-teardown", Result: "pass", Detail: "shutdown completed with zero active handlers"})
	}()
	// The UDS path stays short and private even in long worktree paths.
	socketDir, err := os.MkdirTemp("", "m1-")
	if err != nil {
		return errors.New("cannot create private socket directory")
	}
	defer os.RemoveAll(socketDir)
	socket := filepath.Join(socketDir, "a.sock")
	log, err := os.OpenFile(filepath.Join(dir, "native.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("cannot create native log")
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, app, "-fixtures", fixtures, "-http-base", srv.BaseURL)
	cmd.Env = append(os.Environ(), "MYGO_AUTOMATION="+socket)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.WaitDelay = 3 * time.Second
	if err = cmd.Start(); err != nil {
		return errors.New("cannot launch owned app")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if done == nil {
			return
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			err = errors.New("owned app cleanup deadline")
		}
	}()
	var conn net.Conn
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
		conn, err = (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", socket)
		if err == nil {
			break
		}
		if err = wait(ctx, 100*time.Millisecond); err != nil {
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
	if len(windows.Windows) != 1 || windows.Windows[0].Title != "M1 Playback Lab" {
		return errors.New("owned window not found")
	}
	c.window = windows.Windows[0].ID
	r.Checks = append(r.Checks, check{Name: "native-window", Result: "pass", Detail: "one owned native UI window"})
	if err = c.call(ctx, "act", map[string]any{"window": c.window, "key": "chinese", "action": "setValue", "text": "哔哩哔哩 测试"}, nil); err != nil {
		return err
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	inserted := false
	for _, n := range nodes {
		if n.Key == "chinese" && n.Value == "哔哩哔哩 测试" {
			inserted = true
		}
	}
	if !inserted {
		return errors.New("native Chinese insertion not observed")
	}
	r.Checks = append(r.Checks, check{Name: "injected-Chinese", Result: "pass", Detail: "automation setValue observed in native text field; real IME not-tested"})
	var capture struct {
		PNG []byte `json:"png"`
	}
	if err = c.call(ctx, "capture", map[string]any{"window": c.window}, &capture); err != nil {
		return err
	}
	if len(capture.PNG) == 0 || len(capture.PNG) > 4<<20 {
		return errors.New("invalid app-window capture")
	}
	if err = os.WriteFile(filepath.Join(dir, "ui.png"), capture.PNG, 0600); err != nil {
		return errors.New("cannot save app-window capture")
	}
	if r.Suite == "preview" {
		err = c.preview(ctx, dir, r)
	} else {
		err = c.media(ctx, dir, r, srv)
	}
	if err != nil {
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
		return errors.New("owned app exit deadline")
	}
	if _, err = os.Stat(socket); !os.IsNotExist(err) {
		return errors.New("owned automation socket remains")
	}
	r.Checks = append(r.Checks, check{Name: "owned-process-teardown", Result: "pass", Detail: "graceful exit and removed socket"})
	if r.RuntimeBlocked {
		return errOccluded
	}
	return nil
}
