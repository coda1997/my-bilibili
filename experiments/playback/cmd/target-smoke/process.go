package main

import (
	"bytes"
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

	"my-bilibili/experiments/playback/contract"
)

// A whitelist avoids inherited proxy credentials, loader injections, mpv options,
// automation endpoints, debugging, and credential helpers (including future names).
func childEnvironment(in []string, socket string) []string {
	env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	seen := map[string]bool{}
	for _, entry := range in {
		k, v, ok := strings.Cut(entry, "=")
		if !ok || seen[k] || !utf8.ValidString(v) || strings.ContainsFunc(v, unicode.IsControl) {
			continue
		}
		switch k {
		case "HOME", "TMPDIR":
			if !filepath.IsAbs(v) {
				continue
			}
		case "__CF_USER_TEXT_ENCODING":
		default:
			continue
		}
		seen[k] = true
		env = append(env, k+"="+v)
	}
	return append(env, "MYGO_AUTOMATION="+socket)
}

type ownedProcess struct {
	cmd    *exec.Cmd
	done   chan error
	reaped bool
	exit   error
}

func startCommand(cmd *exec.Cmd, descriptor []byte) (*ownedProcess, error) {
	cmd.Stdin = bytes.NewReader(descriptor)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if cmd.Start() != nil {
		return nil, contract.Media
	}
	p := &ownedProcess{cmd: cmd, done: make(chan error, 1)}
	go func() { p.done <- cmd.Wait() }()
	return p, nil
}
func ownedCommand(app, library, socket string) *exec.Cmd {
	cmd := exec.Command(app, "-library", library)
	cmd.Env = childEnvironment(os.Environ(), socket)
	return cmd
}
func startOwned(app, library, socket string, descriptor []byte) (*ownedProcess, error) {
	return startCommand(ownedCommand(app, library, socket), descriptor)
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
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case p.exit = <-p.done:
		p.reaped = true
		return true
	case <-timer.C:
		return false
	}
}
func connect(ctx context.Context, socket string, p *ownedProcess, r *report) (*client, error) {
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if p != nil && p.exited() {
			return nil, contract.Media
		}
		conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", socket)
		if err == nil {
			return newClient(conn, r), nil
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

// Shutdown has a fresh bounded context even when a session signal/expiry has
// canceled a blocked RPC. A new connection avoids reusing its failed scanner.
func shutdown(p *ownedProcess, socket string, r *report, grace time.Duration) (err error) {
	r.setPhase(phaseCleanupExit)
	defer func() {
		if err != nil && r.CleanupFailure == nil {
			r.CleanupFailure = r.failure(err)
		}
	}()
	if p.exited() {
		r.Checks = append(r.Checks, check{Name: "owned-process-exit", Result: "fail", Detail: "unexpected exit before supervised quit"})
		return contract.Media
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	r.setPhase(phaseCleanupConnect)
	c, err := connect(ctx, socket, p, r)
	if err == nil {
		err = c.identify(ctx)
		if err == nil {
			r.setPhase(phaseCleanupQuit)
			err = c.act(ctx, "quit")
		}
		_ = c.conn.Close()
	}
	cancel()
	rpcPhase := r.phase
	r.setPhase(phaseCleanupExit)
	// Even a lost quit acknowledgement can be followed by actual process exit;
	// it is still a failed RPC and cannot be relabeled as passing cleanup.
	if !p.await(grace) {
		r.ForcedCleanup = true
		_ = p.cmd.Process.Kill()
		if !p.await(3 * time.Second) {
			return contract.Timeout
		}
		r.Checks = append(r.Checks, check{Name: "owned-process-exit", Result: "fail", Detail: "forced kill and reap"})
		return contract.Timeout
	}
	if err != nil || p.exit != nil {
		r.Checks = append(r.Checks, check{Name: "owned-process-exit", Result: "fail", Detail: "quit or exit failed"})
		if err != nil {
			r.setPhase(rpcPhase)
			return safeCode(err)
		}
		return contract.Media
	}
	r.setPhase(phaseCleanupSocket)
	if _, statErr := os.Lstat(socket); !errors.Is(statErr, os.ErrNotExist) {
		return contract.Media
	}
	r.Checks = append(r.Checks, check{Name: "owned-process-exit", Result: "pass", Detail: "quit acknowledged; zero exit; owned automation socket removed"})
	return nil
}

func run(ctx context.Context, app, manifest string, descriptor []byte, r *report) (err error) {
	r.setPhase(phasePreflight)
	// command has already strictly validated the private descriptor. Validate again
	// at this internal boundary so tests/future callers cannot launch by bypassing it.
	if err = validateDescriptor(ctx, descriptor); err != nil {
		return err
	}
	m, h, err := verifyRuntime(manifest)
	if err != nil {
		return contract.Invalid
	}
	r.RuntimeSHA256 = h
	app, err = filepath.Abs(app)
	if err != nil {
		return contract.Invalid
	}
	r.AppSHA256, _, err = artifactHash(app, 128<<20)
	if err != nil {
		return contract.Invalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	socketDir, err := os.MkdirTemp("", "tgt-")
	if err != nil {
		return contract.Media
	}
	defer os.RemoveAll(socketDir)
	socket := filepath.Join(socketDir, "a.sock")
	r.setPhase(phaseStart)
	p, err := startOwned(app, m.Library.Path, socket, descriptor)
	if err != nil {
		return err
	}
	defer func() {
		// Freeze primary evidence before cleanup can change phase or report state.
		r.sessionFailed(err)
		cleanupErr := shutdown(p, socket, r, 5*time.Second)
		if err == nil {
			err = cleanupErr
		}
	}()
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	r.setPhase(phaseConnect)
	c, err := connect(startup, socket, p, r)
	if err == nil {
		err = c.identify(startup)
	}
	stop()
	if c != nil {
		defer c.conn.Close()
	}
	if err != nil {
		return err
	}
	if err = c.capabilities(ctx, r); err != nil {
		return err
	}
	return nil
}
