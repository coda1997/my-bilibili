package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ownedProcess struct {
	cmd            *exec.Cmd
	done           chan error
	reaped         bool
	exit           error
	closeRequested bool
	sourceReceipts *sourceReceipts
}

func childEnvironment(in []string, socket, scenario string) []string {
	env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "MYGO_AUTOMATION=" + socket, "MY_BILIBILI_E2E_SCENARIO=" + scenario}
	seen := map[string]bool{}
	for _, entry := range in {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || seen[key] || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
			continue
		}
		switch key {
		case "HOME", "TMPDIR":
			if !filepath.IsAbs(value) {
				continue
			}
		case "__CF_USER_TEXT_ENCODING":
		default:
			continue
		}
		seen[key] = true
		env = append(env, key+"="+value)
	}
	return env
}
func startOwned(app, cwd, socket, scenario string) (*ownedProcess, error) {
	cmd := exec.Command(app)
	cmd.Dir, cmd.Env = cwd, childEnvironment(os.Environ(), socket, scenario)
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = io.Discard, io.Discard, time.Second
	p := &ownedProcess{cmd: cmd, done: make(chan error, 1)}
	if strings.HasPrefix(scenario, "source-") {
		p.sourceReceipts = newSourceReceipts()
		cmd.Stdout = p.sourceReceipts
	}
	if cmd.Start() != nil {
		return nil, errors.New("owned preview process did not start")
	}
	go func() { p.done <- cmd.Wait() }()
	return p, nil
}
func (p *ownedProcess) exited() bool {
	if p.reaped {
		return true
	}
	select {
	case p.exit = <-p.done:
		p.reaped = true
		return true
	default:
		return false
	}
}
func (p *ownedProcess) await(limit time.Duration) bool {
	if p.exited() {
		return true
	}
	t := time.NewTimer(limit)
	defer t.Stop()
	select {
	case p.exit = <-p.done:
		p.reaped = true
		return true
	case <-t.C:
		return false
	}
}
func connect(ctx context.Context, socket string, p *ownedProcess) (*client, error) {
	for {
		if p.exited() {
			return nil, errors.New("owned preview exited before connection")
		}
		conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", socket)
		if err == nil {
			return newClient(conn), nil
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return nil, err
		}
	}
}
func closeOwned(p *ownedProcess, socket string, r *report) error {
	var err error
	if !p.closeRequested {
		if p.exited() {
			return errors.New("owned preview exited before actual window close")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var c *client
		c, err = connect(ctx, socket, p)
		if err == nil {
			c.report, c.phase = r, "cleanup"
			_, err = c.identify(ctx)
			if err == nil {
				err = c.act(ctx, "e2e-close-window")
			}
			_ = c.conn.Close()
		}
	}
	if !p.await(5 * time.Second) {
		r.ForcedCleanup = true
		_ = p.cmd.Process.Kill()
		if !p.await(3 * time.Second) {
			return errors.New("owned preview did not reap after forced cleanup")
		}
		return errors.New("owned preview required forced cleanup")
	}
	if err != nil {
		return err
	}
	if p.exit != nil {
		return errors.New("owned preview exited unsuccessfully")
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		return errors.New("owned preview automation socket remained")
	}
	return nil
}

func invocation(ctx context.Context, app, name, seam, dir string, r *report, exercise func(context.Context, *client, *ownedProcess) error) (err error) {
	scratch, err := os.MkdirTemp("", "m2-preview-")
	if err != nil {
		return errors.New("preview scratch unavailable")
	}
	defer os.RemoveAll(scratch)
	socket := filepath.Join(scratch, "a.sock")
	p, err := startOwned(app, scratch, socket, seam)
	if err != nil {
		return err
	}
	r.Scenario = name
	r.Processes = append(r.Processes, processRecord{Scenario: name, PID: p.cmd.Process.Pid})
	defer func() {
		if err != nil {
			r.PrimaryFailure = name + ": " + err.Error()
		}
		cleanupErr := closeOwned(p, socket, r)
		if cleanupErr == nil && p.sourceReceipts != nil {
			var closed observed
			closed, cleanupErr = sourceClosed(p)
			if cleanupErr == nil {
				r.pass(name+"-source-ownership-ended", "closed receipt has no active source call or native ownership; source peak concurrency at most one", &closed)
			}
		}
		if cleanupErr != nil {
			r.CleanupFailure = name + ": " + cleanupErr.Error()
			if err == nil {
				err = cleanupErr
			}
		} else {
			r.pass(name+"-actual-window-close", "zero exit and socket removed after an explicit window-close request", nil)
		}
	}()
	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c, err := connect(startup, socket, p)
	if err != nil {
		return err
	}
	defer c.conn.Close()
	c.report, c.phase = r, name
	// The automation socket starts before App.WhenReady. The first windows RPC
	// may wait for native initialization, so it has the startup budget, not the
	// ordinary RPC limit. Cancellation checks below retain their strict 200ms.
	c.callLimit = 20 * time.Second
	for {
		if _, err = c.identify(startup); err == nil {
			break
		}
		if p.exited() {
			return errors.New("preview exited during startup")
		}
		if err = wait(startup, 50*time.Millisecond); err != nil {
			return err
		}
	}
	c.callLimit = 3 * time.Second
	return exercise(ctx, c, p)
}
