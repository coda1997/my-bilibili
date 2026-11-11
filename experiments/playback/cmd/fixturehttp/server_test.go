package fixturehttp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var m struct {
		Fixtures []struct {
			Name, SHA256 string
			Bytes        int
		}
	}
	for _, name := range []string{"combined.mp4", "video.mp4", "audio.m4a"} {
		b := []byte("0123456789-fixture-only")
		h := sha256.Sum256(b)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		m.Fixtures = append(m.Fixtures, struct {
			Name, SHA256 string
			Bytes        int
		}{name, hex.EncodeToString(h[:]), len(b)})
	}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRoutesRangeAndProtection(t *testing.T) {
	s, err := Start(context.Background(), fixtureDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := &http.Client{Timeout: time.Second}
	defer c.CloseIdleConnections()
	for _, tc := range []struct {
		path, header, value, method string
		status                      int
		body                        string
	}{
		{"/plain/combined.mp4", "Range", "bytes=2-5", "GET", 206, "2345"},
		{"/plain/combined.mp4", "Range", "bytes=999-", "GET", 416, ""},
		{"/plain/combined.mp4", "", "", "HEAD", 200, ""},
		{"/protected/combined.mp4", "", "", "GET", 403, ""},
		{"/protected/combined.mp4", Header, Sentinel, "GET", 200, "0123456789-fixture-only"},
		{"/failure/combined.mp4", "", "", "GET", 503, ""},
		{"/plain/manifest.json", "", "", "GET", 404, ""},
		{"/plain/combined.mp4?x=y", "", "", "GET", 404, ""},
		{"/other/combined.mp4", "", "", "GET", 404, ""},
		{"/plain/combined.mp4", "", "", "POST", 405, ""},
	} {
		req, _ := http.NewRequest(tc.method, s.BaseURL+tc.path, nil)
		if tc.header != "" {
			req.Header.Set(tc.header, tc.value)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.status || tc.body != "" && string(b) != tc.body {
			t.Fatalf("route %s: status=%d body=%q", tc.path, resp.StatusCode, b)
		}
	}
	stats := s.Stats()
	if stats.ProtectedDenied != 1 || stats.ProtectedAllowed != 1 || stats.Range != 2 || stats.Failed != 1 {
		t.Fatalf("wrong counts: %+v", stats)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Get(s.BaseURL + "/plain/combined.mp4"); err == nil {
		t.Fatal("listener remained open")
	}
}

func TestConcurrencyCancellationAndShutdown(t *testing.T) {
	s, err := Start(context.Background(), fixtureDir(t), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := &http.Client{Timeout: 4 * time.Second}
	defer c.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			req, _ := http.NewRequestWithContext(ctx, "GET", s.BaseURL+"/delayed/combined.mp4", nil)
			resp, err := c.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		})
	}
	end := time.Now().Add(time.Second)
	for s.Stats().Active != 8 && time.Now().Before(end) {
		time.Sleep(time.Millisecond)
	}
	if s.Stats().Active != 8 {
		t.Fatal("delayed handlers not active")
	}
	resp, err := c.Get(s.BaseURL + "/plain/combined.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 503 || s.Stats().Peak != 8 || s.Stats().Rejected != 1 {
		t.Fatal("concurrency bound not enforced")
	}
	cancel()
	wg.Wait()
	start := time.Now()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second || s.Stats().Active != 0 {
		t.Fatal("cancellation/shutdown leaked handlers")
	}
}

func TestFixtureValidation(t *testing.T) {
	for _, which := range []string{"tampered", "symlink", "large", "manifest"} {
		t.Run(which, func(t *testing.T) {
			dir := fixtureDir(t)
			p := filepath.Join(dir, "combined.mp4")
			switch which {
			case "tampered":
				_ = os.WriteFile(p, []byte("changed"), 0600)
			case "symlink":
				_ = os.Remove(p)
				if err := os.Symlink(filepath.Join(dir, "video.mp4"), p); err != nil {
					t.Skip("symlinks unavailable")
				}
			case "large":
				_ = os.WriteFile(p, make([]byte, (2<<20)+1), 0600)
			case "manifest":
				_ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{}`), 0600)
			}
			if s, err := Start(context.Background(), dir, 0); err == nil {
				_ = s.Close()
				t.Fatal("invalid fixture accepted")
			}
		})
	}
}
