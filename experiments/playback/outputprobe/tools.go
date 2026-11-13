package outputprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ToolIdentity struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Version string `json:"version"`
}
type ToolCommand struct {
	Tool        string   `json:"tool"`
	Arguments   []string `json:"arguments"`
	StdoutBytes int      `json:"stdout_bytes"`
	Stderr      string   `json:"stderr,omitempty"`
	Error       string   `json:"error,omitempty"`
}
type boundedBuffer struct {
	bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.overflow = true
		b.cancel()
		return 0, errors.New("tool output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func runTool(ctx context.Context, tool string, args []string, limit int) ([]byte, string, error) {
	child, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	stdout := &boundedBuffer{limit: limit, cancel: cancel}
	stderr := &boundedBuffer{limit: 64 << 10, cancel: cancel}
	cmd := exec.CommandContext(child, tool, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if stdout.overflow || stderr.overflow {
		return stdout.Bytes(), stderr.String(), errors.New("tool output limit exceeded")
	}
	if child.Err() != nil {
		return stdout.Bytes(), stderr.String(), child.Err()
	}
	return stdout.Bytes(), stderr.String(), err
}
func identifyTool(ctx context.Context, name string) (ToolIdentity, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return ToolIdentity{}, errors.New("required media tool unavailable")
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return ToolIdentity{}, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return ToolIdentity{}, err
	}
	h, _, err := hashRegular(path, 256<<20)
	if err != nil {
		return ToolIdentity{}, err
	}
	b, _, err := runTool(ctx, path, []string{"-version"}, 64<<10)
	if err != nil {
		return ToolIdentity{}, errors.New("media tool version failed")
	}
	return ToolIdentity{path, h, strings.Split(string(b), "\n")[0]}, nil
}
func hashRegular(path string, maxBytes int64) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return "", 0, errors.New("invalid bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(info, after) {
		return "", 0, errors.New("file changed during open")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxBytes+1))
	if err != nil || n != info.Size() || n > maxBytes {
		return "", 0, errors.New("file size changed")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func writeJSON(path string, value any) error { return writeJSONFile(path, value) }
func toolError(tool string, err error) error {
	return fmt.Errorf("%s command failed: %w", filepath.Base(tool), err)
}
