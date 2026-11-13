package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func TestChildEnvironmentAndFixedArguments(t *testing.T) {
	home, temp := filepath.Join(t.TempDir(), "home"), filepath.Join(t.TempDir(), "tmp")
	env := childEnvironment([]string{"HOME=" + home, "TMPDIR=" + temp, "PATH=/malicious", "MYGO_AUTOMATION=/foreign", "HTTP_PROXY=https://SECRET", "HTTPS_PROXY=https://SECRET", "ALL_PROXY=https://SECRET", "http_proxy=https://SECRET", "https_proxy=https://SECRET", "all_proxy=https://SECRET", "NO_PROXY=SECRET", "no_proxy=SECRET", "FTP_PROXY=SECRET", "DYLD_INSERT_LIBRARIES=SECRET", "DYLD_PRINT_LIBRARIES=1", "LD_PRELOAD=SECRET", "MPV_HOME=SECRET", "SSL_CERT_FILE=SECRET", "SSH_AUTH_SOCK=SECRET", "AWS_SECRET_ACCESS_KEY=SECRET", "GODEBUG=SECRET", "__CF_USER_TEXT_ENCODING=0x1:0:0", "HOME=" + filepath.Join(t.TempDir(), "duplicate")}, "/owned/socket")
	expected := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + home, "TMPDIR=" + temp, "__CF_USER_TEXT_ENCODING=0x1:0:0", "MYGO_AUTOMATION=/owned/socket"}
	if !reflect.DeepEqual(env, expected) {
		t.Fatalf("environment policy mismatch: %v", env)
	}
	cmd := ownedCommand("/owned/app", "/owned/library", "/owned/socket")
	if !reflect.DeepEqual(cmd.Args, []string{"/owned/app", "-library", "/owned/library"}) || cmd.SysProcAttr != nil {
		t.Fatal("argv or process group changed")
	}
	for _, entry := range childEnvironment([]string{"HOME=/bad\nvalue", "TMPDIR=relative", "__CF_USER_TEXT_ENCODING=bad\x00value"}, "/owned/socket") {
		if strings.Contains(entry, "bad") || strings.Contains(entry, "relative") {
			t.Fatal("unclean environment")
		}
	}
}

// A re-executed Go test binary is the only process used by these tests. It does
// not load libmpv, use the Internet, or consume target media. The only transport
// is an owned local Unix socket implementing the tiny automation protocol.
func TestSyntheticOwnedChild(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "target-helper-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "target-helper-")
	b, _ := io.ReadAll(os.Stdin)
	fmt.Fprintln(os.Stdout, "SECRET_RAW_NATIVE_OUTPUT", string(b))
	fmt.Fprintln(os.Stderr, "SECRET_RAW_NATIVE_OUTPUT", string(b))
	if mode == "exit" {
		os.Exit(0)
	}
	socket := os.Getenv("MYGO_AUTOMATION")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		os.Exit(4)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			os.Exit(5)
		}
		scan := bufio.NewScanner(conn)
		for scan.Scan() {
			var req fakeRequest
			if json.Unmarshal(scan.Bytes(), &req) != nil {
				os.Exit(6)
			}
			if req.Method == "windows" {
				_, _ = fmt.Fprintf(conn, "{\"id\":%d,\"result\":{\"windows\":[{\"id\":1,\"title\":\"Authorized Target Playback Probe\"}]}}\n", req.ID)
				continue
			}
			if req.Method == "act" && req.Params.Key == "quit" {
				if mode == "hang" {
					// A timer keeps this synthetic child alive even when no goroutine can
					// otherwise make progress; the supervisor must kill and reap it.
					for {
						time.Sleep(time.Hour)
					}
				}
				_, _ = fmt.Fprintf(conn, "{\"id\":%d,\"result\":{}}\n", req.ID)
				conn.Close()
				listener.Close()
				os.Remove(socket)
				os.Exit(0)
			}
		}
		conn.Close()
	}
}
func helperProcess(t *testing.T, mode string) (*ownedProcess, string) {
	t.Helper()
	// macOS has a short Unix socket path limit, so use a short owned temp directory.
	dir, err := os.MkdirTemp("", "ts-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "a.sock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSyntheticOwnedChild$", "--", "target-helper-"+mode)
	cmd.Env = childEnvironment(os.Environ(), socket)
	p, err := startCommand(cmd, []byte("SECRET_TRANSIENT_DESCRIPTOR"))
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Stdout != io.Discard || cmd.Stderr != io.Discard {
		t.Fatal("native output is not discarded")
	}
	t.Cleanup(func() {
		if !p.reaped {
			p.cmd.Process.Kill()
			if !p.await(3 * time.Second) {
				t.Error("test process unreaped")
			}
		}
	})
	return p, socket
}
func TestOwnedProcessGracefulCleanupAfterInterruption(t *testing.T) {
	p, socket := helperProcess(t, "quit")
	r := new(report)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	c, err := connect(ctx, socket, p, r)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.identify(ctx); err != nil {
		t.Fatal(err)
	}
	c.conn.Close()
	cancel() // Session context is no longer usable for cleanup.
	if err = shutdown(p, socket, r, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if !p.reaped || r.ForcedCleanup || p.exit != nil {
		t.Fatal("not a graceful reap")
	}
	if r.SessionFailure != nil || r.CleanupFailure != nil {
		t.Fatal("graceful cleanup reported a failure")
	}
	if strings.Contains(mustJSON(r), "SECRET") {
		t.Fatal("raw child output leaked")
	}
}
func TestOwnedProcessForcedCleanupIsFailure(t *testing.T) {
	p, socket := helperProcess(t, "hang")
	r := new(report)
	err := shutdown(p, socket, r, 20*time.Millisecond)
	if err == nil || !r.ForcedCleanup || !p.reaped {
		t.Fatalf("forced=%v reaped=%v err=%v", r.ForcedCleanup, p.reaped, err)
	}
	if strings.Contains(mustJSON(r), "SECRET") {
		t.Fatal("raw child output leaked")
	}
}
func TestOwnedProcessEarlyExitAndSpawnFailure(t *testing.T) {
	p, socket := helperProcess(t, "exit")
	if !p.await(3 * time.Second) {
		t.Fatal("helper exit")
	}
	r := new(report)
	if shutdown(p, socket, r, time.Second) == nil || r.ForcedCleanup {
		t.Fatal("early exit treated as supervised success")
	}
	if _, err := startOwned(filepath.Join(t.TempDir(), "absent"), "/library", "/socket", nil); !errors.Is(err, contract.Media) {
		t.Fatal(err)
	}
}
func TestConnectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := connect(ctx, filepath.Join(t.TempDir(), "none"), nil, new(report))
	if !errors.Is(err, context.Canceled) || time.Since(start) > 200*time.Millisecond {
		t.Fatal(err)
	}
}
func TestInvalidDescriptorStopsBeforeSpawnAndRedacts(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")
	for _, data := range []string{`{"video_url":"SECRET_RAW_LOCATOR"}`, strings.Repeat("SECRET_RAW_LOCATOR", 3000)} {
		out := filepath.Join(t.TempDir(), "result")
		var stdout, stderr bytes.Buffer
		code := command(context.Background(), []string{"-app", absent, "-runtime", absent, "-out", out}, io.NopCloser(strings.NewReader(data)), &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code=%d", code)
		}
		b, err := os.ReadFile(filepath.Join(out, "result.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{stdout.String(), stderr.String(), string(b)} {
			if strings.Contains(s, "SECRET") || strings.Contains(s, "video_url") {
				t.Fatal("private descriptor leaked")
			}
		}
		var r report
		if json.Unmarshal(b, &r) != nil || r.AppSHA256 != "" || r.RuntimeSHA256 != "" || len(r.Automation) != 0 {
			t.Fatal("invalid descriptor reached launch")
		}
		if r.SessionFailure == nil || r.SessionFailure.Phase != phaseDescriptor || r.SessionFailure.LastSnapshot != nil || r.SessionFailure.LastResources != nil || r.CleanupFailure != nil {
			t.Fatal("descriptor failure fabricated native state or lost its phase")
		}
	}
	var output, diagnostic bytes.Buffer
	if command(context.Background(), []string{"--SECRET_RAW_LOCATOR"}, io.NopCloser(strings.NewReader("")), &output, &diagnostic) != 2 || strings.Contains(diagnostic.String(), "SECRET") {
		t.Fatal("flag error leaked")
	}
}
func TestSafeErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want contract.ErrorCode
	}{{context.DeadlineExceeded, contract.Timeout}, {context.Canceled, contract.Canceled}, {contract.Unsupported, contract.Unsupported}, {contract.Invalid, contract.Invalid}, {errors.New("SECRET_RAW_NATIVE_OUTPUT"), contract.Media}, {contract.ErrorCode("SECRET"), contract.Media}} {
		if got := safeCode(tc.err); got != tc.want {
			t.Fatalf("want %s got %s", tc.want, got)
		}
	}
}
func TestArtifactVerification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	data := []byte("synthetic artifact")
	if os.WriteFile(path, data, 0600) != nil {
		t.Fatal("write")
	}
	if err := verifyArtifact(artifact{path, digest(data), int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if verifyArtifact(artifact{path, digest(data), 1}) == nil {
		t.Fatal("size mismatch")
	}
	if verifyArtifact(artifact{path, strings.Repeat("0", 64), int64(len(data))}) == nil {
		t.Fatal("hash mismatch")
	}
	link := path + "-link"
	if os.Symlink(path, link) != nil {
		t.Fatal("link")
	}
	if verifyArtifact(artifact{link, digest(data), int64(len(data))}) == nil {
		t.Fatal("symlink accepted")
	}
	if _, _, err := verifyRuntime(path); err == nil {
		t.Fatal("unpinned runtime accepted")
	}
}

type unreadableInput struct{ called bool }

func (r *unreadableInput) Read([]byte) (int, error) {
	r.called = true
	return 0, errors.New("must not read")
}
func (r *unreadableInput) Close() error { return nil }
func TestCLIRejectsRelativeAndExistingOutputBeforeInput(t *testing.T) {
	owned := t.TempDir()
	for _, relative := range []string{"app", "runtime", "out"} {
		args := []string{"-app", filepath.Join(owned, "app"), "-runtime", filepath.Join(owned, "runtime"), "-out", filepath.Join(t.TempDir(), "new")}
		for i := 0; i < len(args); i += 2 {
			if args[i] == "-"+relative {
				args[i+1] = "relative"
			}
		}
		input := new(unreadableInput)
		var out, err bytes.Buffer
		if code := command(context.Background(), args, input, &out, &err); code != 2 || input.called {
			t.Fatalf("relative %s code=%d input=%v", relative, code, input.called)
		}
	}
	dir := t.TempDir()
	input := new(unreadableInput)
	var out, err bytes.Buffer
	if code := command(context.Background(), []string{"-app", filepath.Join(owned, "app"), "-runtime", filepath.Join(owned, "runtime"), "-out", dir}, input, &out, &err); code != 1 || input.called {
		t.Fatalf("existing result code=%d input=%v", code, input.called)
	}
}
