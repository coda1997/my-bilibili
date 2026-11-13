// Package fixturehttp serves only the three hash-verified synthetic fixtures.
// It is not a forwarding proxy and never accesses a Bilibili service.
package fixturehttp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const Header = "X-M1-Fixture"
const Sentinel = "synthetic-only"

// Probe values contain commas to exercise typed list encoding. Only match
// counters are recorded; request metadata never enters logs or observations.
const ProbeSentinel = "synthetic-only,comma"
const ProbeReferer = "http://127.0.0.1/synthetic,referrer"

type StreamStats struct{ Requests, HeaderMatches, RefererMatches, Allowed, Denied int64 }
type Stats struct {
	Plain, ProtectedDenied, ProtectedAllowed, Delayed, Failed, Range, Rejected, Active, Peak int64
	Streams                                                                                  map[string]StreamStats `json:"streams,omitempty"`
}
type Server struct {
	BaseURL  string
	server   *http.Server
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
	closeErr error
	stats    [9]atomic.Int64
	probe    bool
	streams  [3][5]atomic.Int64
}

type fixture struct {
	data       []byte
	mime, etag string
}

func readRegular(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, errors.New("invalid fixture file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("fixture unavailable")
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("fixture changed during open")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("fixture exceeds read limit")
	}
	return b, nil
}

func load(dir string) (map[string]fixture, error) {
	info, err := os.Lstat(filepath.Join(dir, "manifest.json"))
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<10 {
		return nil, errors.New("invalid fixture manifest")
	}
	b, err := readRegular(filepath.Join(dir, "manifest.json"), 32<<10)
	if err != nil {
		return nil, errors.New("fixture manifest unavailable")
	}
	var m struct {
		Fixtures []struct {
			Name, SHA256 string
			Bytes        int
		}
	}
	if json.Unmarshal(b, &m) != nil || len(m.Fixtures) != 3 {
		return nil, errors.New("invalid fixture manifest")
	}
	allowed := map[string]string{"combined.mp4": "video/mp4", "video.mp4": "video/mp4", "audio.m4a": "audio/mp4"}
	result := map[string]fixture{}
	for _, f := range m.Fixtures {
		mime, ok := allowed[f.Name]
		if !ok || result[f.Name].data != nil {
			return nil, errors.New("unexpected fixture name")
		}
		p := filepath.Join(dir, f.Name)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 || info.Size() <= 0 {
			return nil, errors.New("invalid fixture file")
		}
		b, err := readRegular(p, 2<<20)
		if err != nil {
			return nil, errors.New("fixture unavailable")
		}
		h := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(h[:]) != f.SHA256 {
			return nil, errors.New("fixture checksum mismatch")
		}
		result[f.Name] = fixture{b, mime, "\"" + f.SHA256 + "\""}
	}
	return result, nil
}

func Start(ctx context.Context, dir string, delay time.Duration) (*Server, error) {
	return start(ctx, dir, delay, false)
}

// StartProbe opts into strict custom-header + Referer protection and sanitized
// per-file observations. Start preserves all legacy routing/header semantics.
func StartProbe(ctx context.Context, dir string, delay time.Duration) (*Server, error) {
	return start(ctx, dir, delay, true)
}

func start(ctx context.Context, dir string, delay time.Duration, probe bool) (*Server, error) {
	if ctx == nil || delay < 0 || delay > 3*time.Second {
		return nil, errors.New("invalid server options")
	}
	files, err := load(dir)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("loopback listener unavailable")
	}
	root, cancel := context.WithCancel(ctx)
	s := &Server{BaseURL: "http://" + listener.Addr().String(), cancel: cancel, done: make(chan struct{}), probe: probe}
	sem := make(chan struct{}, 8)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			s.stats[6].Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		active := s.stats[7].Add(1)
		defer s.stats[7].Add(-1)
		for peak := s.stats[8].Load(); active > peak; peak = s.stats[8].Load() {
			if s.stats[8].CompareAndSwap(peak, active) {
				break
			}
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 3 || parts[0] != "" || r.URL.RawQuery != "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f, ok := files[parts[2]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch parts[1] {
		case "plain":
			s.stats[0].Add(1)
		case "protected":
			w.Header().Set("Cache-Control", "no-store")
			matched := r.Header.Get(Header) == Sentinel
			if probe {
				index := map[string]int{"combined.mp4": 0, "video.mp4": 1, "audio.m4a": 2}[parts[2]]
				counts := &s.streams[index]
				counts[0].Add(1)
				header, referer := r.Header.Get(Header) == ProbeSentinel, r.Header.Get("Referer") == ProbeReferer
				if header {
					counts[1].Add(1)
				}
				if referer {
					counts[2].Add(1)
				}
				matched = header && referer
				if matched {
					counts[3].Add(1)
				} else {
					counts[4].Add(1)
				}
			}
			if !matched {
				s.stats[1].Add(1)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			s.stats[2].Add(1)
		case "delayed":
			s.stats[3].Add(1)
			t := time.NewTimer(delay)
			defer t.Stop()
			select {
			case <-r.Context().Done():
				return
			case <-t.C:
			}
		case "failure":
			s.stats[4].Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Range") != "" {
			s.stats[5].Add(1)
		}
		w.Header().Set("Content-Type", f.mime)
		w.Header().Set("ETag", f.etag)
		http.ServeContent(w, r, parts[2], time.Time{}, bytes.NewReader(f.data))
	})
	s.server = &http.Server{Handler: handler, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192, BaseContext: func(net.Listener) context.Context { return root }}
	go func() { defer close(s.done); _ = s.server.Serve(listener) }()
	return s, nil
}

func (s *Server) Stats() Stats {
	result := Stats{Plain: s.stats[0].Load(), ProtectedDenied: s.stats[1].Load(), ProtectedAllowed: s.stats[2].Load(), Delayed: s.stats[3].Load(), Failed: s.stats[4].Load(), Range: s.stats[5].Load(), Rejected: s.stats[6].Load(), Active: s.stats[7].Load(), Peak: s.stats[8].Load()}
	if s.probe {
		result.Streams = make(map[string]StreamStats, 3)
		for i, name := range []string{"combined.mp4", "video.mp4", "audio.m4a"} {
			c := &s.streams[i]
			result.Streams[name] = StreamStats{c[0].Load(), c[1].Load(), c[2].Load(), c[3].Load(), c[4].Load()}
		}
	}
	return result
}

func (s *Server) Close() error {
	s.once.Do(func() {
		s.cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if s.server.Shutdown(ctx) != nil {
			_ = s.server.Close()
			s.closeErr = errors.New("fixture server shutdown deadline")
		}
		select {
		case <-s.done:
		case <-ctx.Done():
			s.closeErr = errors.New("fixture server cleanup deadline")
		}
	})
	return s.closeErr
}
