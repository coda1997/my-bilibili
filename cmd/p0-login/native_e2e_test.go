//go:build e2e

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skip2/go-qrcode"
	"my-bilibili/internal/account"
)

type nativeLog struct {
	mu        sync.Mutex
	file      *os.File
	remaining int
	truncated bool
}

func (l *nativeLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := min(len(p), l.remaining)
	if count < len(p) {
		l.truncated = true
	}
	if count > 0 {
		if _, err := l.file.Write(p[:count]); err != nil {
			return 0, err
		}
		l.remaining -= count
	}
	return len(p), nil
}

type nativeRect struct{ X, Y, Width, Height float64 }
type nativeNode struct {
	ID     string
	Key    any
	Label  string
	Bounds nativeRect
	States []string
}
type nativeWindow struct {
	ID            int
	Title, Kind   string
	Visible       bool
	ContentBounds nativeRect
}
type nativeClient struct {
	conn       net.Conn
	scanner    *bufio.Scanner
	id, window int
	focus      string
}

func (c *nativeClient) call(ctx context.Context, method string, params, out any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	deadline := time.Now().Add(4 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if c.conn.SetDeadline(deadline) != nil {
		return errors.New("RPC deadline failed")
	}
	c.id++
	data, err := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{c.id, method, params})
	if err != nil {
		return errors.New("RPC encode failed")
	}
	if n, e := c.conn.Write(append(data, byte(10))); e != nil || n != len(data)+1 {
		return errors.New("RPC write failed")
	}
	if !c.scanner.Scan() {
		return errors.New("RPC read failed")
	}
	var response struct {
		ID     int
		Result json.RawMessage
		Error  json.RawMessage
	}
	if json.Unmarshal(c.scanner.Bytes(), &response) != nil || response.ID != c.id {
		return errors.New("RPC response invalid")
	}
	if len(response.Error) > 0 && string(response.Error) != "null" {
		return errors.New("RPC action rejected")
	}
	if out != nil && (len(response.Result) == 0 || json.Unmarshal(response.Result, out) != nil) {
		return errors.New("RPC result invalid")
	}
	return nil
}
func (c *nativeClient) tree(ctx context.Context) ([]nativeNode, error) {
	var r struct {
		Nodes []nativeNode
		Focus string
	}
	err := c.call(ctx, "tree", map[string]any{"window": c.window}, &r)
	if err != nil {
		return nil, err
	}
	if len(r.Nodes) == 0 || len(r.Nodes) > 1024 {
		return nil, errors.New("tree size invalid")
	}
	c.focus = r.Focus
	return r.Nodes, nil
}
func nativeFind(nodes []nativeNode, key string) (nativeNode, bool) {
	var found nativeNode
	count := 0
	for _, n := range nodes {
		if n.Key == key {
			found = n
			count++
		}
	}
	return found, count == 1
}
func nativeFocused(nodes []nativeNode, key, focus string) bool {
	id, err := strconv.ParseUint(focus, 10, 64)
	if err != nil || id == 0 {
		return false
	}
	node, ok := nativeFind(nodes, key)
	return ok && node.ID == focus
}

func (c *nativeClient) act(ctx context.Context, key string) error {
	return c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "press"}, nil)
}
func (c *nativeClient) input(ctx context.Context, key string) error {
	return c.call(ctx, "input", map[string]any{"window": c.window, "events": []map[string]string{{"type": "key", "key": key}}}, nil)
}
func nativeWait(ctx context.Context, predicate func() (bool, error)) error {
	for {
		ok, e := predicate()
		if e != nil {
			return e
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
}
func (c *nativeClient) phase(ctx context.Context, want account.Phase) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return nativeWait(ctx, func() (bool, error) {
		nodes, e := c.tree(ctx)
		if e != nil {
			return false, e
		}
		n, ok := nativeFind(nodes, "synthetic-state")
		if !ok {
			return false, nil
		}
		var r receipt
		if json.Unmarshal([]byte(n.Label), &r) != nil {
			return false, errors.New("synthetic state invalid")
		}
		return r.Phase == want && !r.Busy || r.Phase == want && (want == account.Waiting || want == account.Scanned), nil
	})
}
func (c *nativeClient) captureQR(ctx context.Context, outfile string) error {
	var windows struct{ Windows []nativeWindow }
	if err := c.call(ctx, "windows", nil, &windows); err != nil {
		return err
	}
	if len(windows.Windows) != 1 || windows.Windows[0].ID != c.window || windows.Windows[0].Title != syntheticTitle {
		return errors.New("synthetic window identity changed")
	}
	content := windows.Windows[0].ContentBounds
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	n, ok := nativeFind(nodes, "login-qr")
	if !ok {
		return errors.New("QR image node missing")
	}
	for _, node := range nodes {
		if strings.Contains(node.Label, syntheticPayload) || strings.Contains(node.Label, "synthetic-login-material") {
			return errors.New("private presentation exposed in tree")
		}
	}
	var capture struct{ PNG []byte }
	if err := c.call(ctx, "capture", map[string]any{"window": c.window}, &capture); err != nil {
		return err
	}
	if len(capture.PNG) == 0 || len(capture.PNG) > 8<<20 {
		return errors.New("native capture size invalid")
	}
	config, err := png.DecodeConfig(bytes.NewReader(capture.PNG))
	if err != nil || config.Width > 4096 || config.Height > 4096 || content.Width <= 0 || content.Height <= 0 {
		return errors.New("capture geometry invalid")
	}
	im, err := png.Decode(bytes.NewReader(capture.PNG))
	if err != nil {
		return errors.New("capture decode failed")
	}
	qr, err := qrcode.New(syntheticPayload, qrcode.Medium)
	if err != nil {
		return errors.New("synthetic reference failed")
	}
	matrix := qr.Bitmap()
	modules := len(matrix)
	unit := n.Bounds.Width / float64(modules)
	if n.Bounds.Width != n.Bounds.Height || unit < 1 || math.Abs(unit-math.Round(unit)) > .01 {
		return errors.New("QR module geometry distorted")
	}
	sx, sy := float64(config.Width)/content.Width, float64(config.Height)/content.Height
	if math.Abs(sx-sy) > .05 {
		return errors.New("capture scale differs")
	}
	for y, row := range matrix {
		for x, dark := range row {
			px := int((n.Bounds.X + (float64(x)+.5)*unit) * sx)
			py := int((n.Bounds.Y + (float64(y)+.5)*unit) * sy)
			if px < 0 || py < 0 || px >= config.Width || py >= config.Height {
				return errors.New("QR clipped outside capture")
			}
			r, g, b, a := im.At(px, py).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			a >>= 8
			if a < 250 || dark && (r > 25 || g > 25 || b > 25) || !dark && (r < 230 || g < 230 || b < 230) {
				return errors.New("QR pixels or quiet zone changed")
			}
		}
	}
	return os.WriteFile(outfile, capture.PNG, 0600)
}
func nativeEnvironment(socket string) []string {
	env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "MYGO_AUTOMATION=" + socket}
	for _, key := range []string{"HOME", "TMPDIR", "__CF_USER_TEXT_ENCODING"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
func TestNativeSyntheticLogin(t *testing.T) {
	binary := os.Getenv("P0_LOGIN_NATIVE_BINARY")
	if binary == "" {
		t.Skip("explicit synthetic native binary required")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("native login evidence requires macOS")
	}
	expected := os.Getenv("P0_LOGIN_NATIVE_SHA256")
	outdir := os.Getenv("P0_LOGIN_NATIVE_OUTPUT")
	if !filepath.IsAbs(binary) || !filepath.IsAbs(outdir) || len(expected) != 64 {
		t.Fatal("absolute artifact paths and exact digest required")
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal("native artifact unavailable")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		t.Fatal("native artifact changed")
	}
	if os.Mkdir(outdir, 0700) != nil {
		t.Fatal("native evidence directory must be fresh")
	}
	socketRoot, err := os.MkdirTemp("/tmp", "bili-login-")
	if err != nil {
		t.Fatal("socket directory unavailable")
	}
	defer os.RemoveAll(socketRoot)
	socket := filepath.Join(socketRoot, "rpc.sock")
	cmd := exec.Command(binary, "--synthetic")
	cmd.Env = nativeEnvironment(socket)
	logFile, err := os.OpenFile(filepath.Join(outdir, "synthetic-child.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal("synthetic child log unavailable")
	}
	defer logFile.Close()
	logWriter := &nativeLog{file: logFile, remaining: 64 << 10}
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.WaitDelay = time.Second
	if cmd.Start() != nil {
		t.Fatal("synthetic app failed to start")
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	reaped := false
	defer func() {
		if !reaped {
			select {
			case <-exited:
				reaped = true
			default:
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-exited:
					reaped = true
				case <-time.After(2 * time.Second):
					_ = cmd.Process.Kill()
					<-exited
					t.Errorf("synthetic app required forced cleanup")
				}

			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var conn net.Conn
	if err := nativeWait(ctx, func() (bool, error) {
		select {
		case <-exited:
			reaped = true
			return false, errors.New("synthetic app exited before RPC")
		default:
		}
		var e error
		conn, e = (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(ctx, "unix", socket)
		return e == nil, nil
	}); err != nil {
		t.Fatal("synthetic RPC unavailable")
	}
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	client := &nativeClient{conn: conn, scanner: scanner}
	if err := nativeWait(ctx, func() (bool, error) {
		var w struct{ Windows []nativeWindow }
		if e := client.call(ctx, "windows", nil, &w); e != nil {
			return false, e
		}
		if len(w.Windows) != 1 || !w.Windows[0].Visible {
			return false, nil
		}
		if w.Windows[0].Title != syntheticTitle || w.Windows[0].Kind != "content" {
			return false, errors.New("not the assigned synthetic window")
		}
		client.window = w.Windows[0].ID
		return client.window > 0, nil
	}); err != nil {
		t.Fatal("synthetic window not ready")
	}
	must := func(err error, step string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	must(client.phase(ctx, account.SignedOut), "signed-out startup")
	// Actual keyboard events reach the normal Begin button; only press Space once
	// focus was observed on that exact button, never assume initial focus.
	focused := false
	for i := 0; i < 8; i++ {
		nodes, e := client.tree(ctx)
		must(e, "keyboard tree")
		focused = nativeFocused(nodes, "begin-login", client.focus)
		if focused {
			break
		}
		must(client.input(ctx, "Tab"), "keyboard Tab")
		time.Sleep(50 * time.Millisecond)
	}
	if !focused {
		t.Fatal("Begin button not keyboard reachable")
	}
	must(client.input(ctx, "Space"), "keyboard Begin")
	must(client.phase(ctx, account.Waiting), "waiting after keyboard action")
	must(client.captureQR(ctx, filepath.Join(outdir, "light.png")), "light QR geometry")
	must(client.act(ctx, "test-dark"), "dark appearance")
	must(nativeWait(ctx, func() (bool, error) {
		nodes, e := client.tree(ctx)
		if e != nil {
			return false, e
		}
		n, ok := nativeFind(nodes, "synthetic-theme")
		return ok && n.Label == "dark", nil
	}), "dark frame applied")
	must(client.captureQR(ctx, filepath.Join(outdir, "dark.png")), "dark QR geometry")
	must(client.act(ctx, "test-small-window"), "minimum window")
	must(nativeWait(ctx, func() (bool, error) {
		var w struct{ Windows []nativeWindow }
		if e := client.call(ctx, "windows", nil, &w); e != nil {
			return false, e
		}
		return len(w.Windows) == 1 && math.Abs(w.Windows[0].ContentBounds.Width-400) < 1, nil
	}), "minimum window applied")
	must(client.captureQR(ctx, filepath.Join(outdir, "minimum.png")), "minimum QR geometry")
	must(client.act(ctx, "cancel-login"), "cancel attempt")
	must(client.phase(ctx, account.SignedOut), "canceled signed-out")
	nodes, e := client.tree(ctx)
	must(e, "canceled tree")
	if _, ok := nativeFind(nodes, "login-qr"); ok {
		t.Fatal("canceled QR remains")
	}
	must(client.act(ctx, "begin-login"), "new synthetic attempt")
	must(client.phase(ctx, account.Waiting), "new waiting")
	must(client.act(ctx, "test-expire"), "expire fixture")
	must(client.phase(ctx, account.Expired), "expiry")
	nodes, e = client.tree(ctx)
	must(e, "expired tree")
	if _, ok := nativeFind(nodes, "login-qr"); ok {
		t.Fatal("expired QR remains")
	}
	must(client.act(ctx, "begin-login"), "retry expired")
	must(client.phase(ctx, account.Waiting), "retry waiting")
	must(client.act(ctx, "test-scanned"), "scanned fixture")
	must(client.phase(ctx, account.Scanned), "scanned confirmation")
	must(client.act(ctx, "test-save-failure"), "save failure fixture")
	must(client.act(ctx, "test-verify-failure"), "verify failure fixture")
	must(client.act(ctx, "test-complete"), "completed fixture")
	must(client.phase(ctx, account.SaveFailed), "save failure retaining session")
	must(client.act(ctx, "retry-save"), "retry save without QR")
	must(client.phase(ctx, account.VerifyFailed), "saved network failure")
	nodes, e = client.tree(ctx)
	must(e, "saved tree")
	if _, ok := nativeFind(nodes, "login-qr"); ok {
		t.Fatal("completed QR remains")
	}
	must(client.act(ctx, "retry-verify"), "retry validation")
	must(client.phase(ctx, account.SignedIn), "saved synthetic login")
	must(client.act(ctx, "logout"), "logout confirmation")
	must(client.act(ctx, "cancel-logout"), "cancel logout")
	must(client.phase(ctx, account.SignedIn), "canceled logout retains session")
	must(client.act(ctx, "test-delete-failure"), "delete failure fixture")
	must(client.act(ctx, "logout"), "second logout confirmation")
	must(client.act(ctx, "confirm-logout"), "confirm synthetic logout")
	must(client.phase(ctx, account.DeleteFailed), "delete failure visible")
	must(client.act(ctx, "retry-logout"), "retry delete")
	must(client.phase(ctx, account.SignedOut), "synthetic logout complete")
	must(client.act(ctx, "begin-login"), "pending close attempt")
	must(client.phase(ctx, account.Waiting), "pending close waiting")
	must(client.act(ctx, "test-close"), "normal window close")
	select {
	case err := <-exited:
		reaped = true
		if err != nil {
			t.Fatal("synthetic process exit failed")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("synthetic process did not close gracefully")
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatal("synthetic RPC socket remains")
	}
	logWriter.mu.Lock()
	truncated := logWriter.truncated
	logWriter.mu.Unlock()
	if truncated {
		t.Fatal("synthetic child output exceeded bound")
	}
	report := map[string]any{"synthetic_only": true, "app_sha256": expected, "keyboard_begin": true, "qr_pixel_checks": 3, "cancel_expiry_retry": true, "save_verify_retry": true, "logout_confirm_failure_retry": true, "graceful_close": true, "native_platform": "darwin", "live_credentials_accessed": false}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	if os.WriteFile(filepath.Join(outdir, "report.json"), encoded, 0600) != nil {
		t.Fatal("synthetic report write failed")
	}
	t.Log("synthetic native login flow and QR rendering passed")
}

func TestNativeFocusUsesTreeFocusID(t *testing.T) {
	var tree struct {
		Focus string
		Nodes []nativeNode
	}
	if json.Unmarshal([]byte("{\"focus\":\"42\",\"nodes\":[{\"id\":\"42\",\"key\":\"begin-login\",\"states\":[\"focusable\"]}]}"), &tree) != nil {
		t.Fatal("synthetic focus fixture invalid")
	}
	if !nativeFocused(tree.Nodes, "begin-login", tree.Focus) {
		t.Fatal("authoritative tree focus ignored")
	}
	for _, focus := range []string{"", "0", "no-id", "43", "18446744073709551616"} {
		if nativeFocused(tree.Nodes, "begin-login", focus) {
			t.Fatal("unavailable/other focus accepted")
		}
	}
	tree.Nodes[0].ID = "0"
	if nativeFocused(tree.Nodes, "begin-login", "0") {
		t.Fatal("zero focus accepted")
	}
}
