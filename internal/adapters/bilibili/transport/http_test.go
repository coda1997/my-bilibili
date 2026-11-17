package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"my-bilibili/internal/catalog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New(private) }

type waitingReader struct {
	ctx     context.Context
	entered chan struct{}
}

func (r *waitingReader) Read([]byte) (int, error) {
	close(r.entered)
	<-r.ctx.Done()
	return 0, errors.New(private)
}
func gzipBytes(t *testing.T, value []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(value); err != nil {
		t.Fatal("fixture compression failed")
	}
	if w.Close() != nil {
		t.Fatal("fixture compression failed")
	}
	return b.Bytes()
}

func TestDecompressedBodyBoundsAndEveryCleanupFailure(t *testing.T) {
	valid := `{"code":0,"data":{}}`
	cases := []struct {
		name              string
		reader            io.Reader
		encoding          string
		closeErr, tripErr error
		status            int
		want              catalog.Code
	}{
		{"exact JSON bound", strings.NewReader(valid + strings.Repeat(" ", contractJSONLimit-len(valid))), "", nil, nil, 200, catalog.None},
		{"oversize JSON", strings.NewReader(valid + strings.Repeat(" ", contractJSONLimit+1-len(valid))), "", nil, nil, 200, catalog.InvalidResponse},
		{"oversize decompressed", bytes.NewReader(gzipBytes(t, []byte(valid+strings.Repeat(" ", contractJSONLimit+1-len(valid))))), "gzip", nil, nil, 200, catalog.InvalidResponse},
		{"valid gzip", bytes.NewReader(gzipBytes(t, []byte(valid))), "gzip", nil, nil, 200, catalog.None},
		{"invalid gzip", strings.NewReader(private), "gzip", nil, nil, 200, catalog.InvalidResponse},
		{"unknown encoding", strings.NewReader(private), "unsupported", nil, nil, 200, catalog.InvalidResponse},
		{"read failure", failingReader{}, "", nil, nil, 200, catalog.Network},
		{"close failure", strings.NewReader(valid), "", errors.New(private), nil, 200, catalog.Network},
		{"status close failure", strings.NewReader(private), "", errors.New(private), nil, 429, catalog.Network},
		{"response with RT failure", strings.NewReader(private), "", nil, errors.New(private), 200, catalog.Network},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			body := &observedBody{Reader: item.reader, closeErr: item.closeErr}
			c := testClient(t, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: item.status, Body: body, Header: http.Header{"Content-Encoding": {item.encoding}}}, item.tripErr
			})
			data, err := bind(t, c).JSON(context.Background(), Detail, detailParams())
			if item.want == catalog.None {
				if err != nil || len(data) == 0 {
					t.Fatal("valid bounded response rejected")
				}
			} else if err != item.want || data != nil {
				t.Fatal("body failure category or success wrong")
			}
			if body.closed.Load() != 1 {
				t.Fatal("response body was not closed exactly once")
			}
			safeError(t, err)
		})
	}
	for _, missingBody := range []bool{false, true} {
		c := testClient(t, func(*http.Request) (*http.Response, error) {
			if missingBody {
				return &http.Response{StatusCode: 200}, nil
			}
			return nil, nil
		})
		if _, err := bind(t, c).JSON(context.Background(), Detail, detailParams()); err != catalog.InvalidResponse {
			t.Fatal("absent response/body accepted")
		}
	}
}

func TestProtobufBodyLimitsAreRouteSpecific(t *testing.T) {
	for _, item := range []struct {
		route  Route
		length int
		want   catalog.Code
	}{{DanmakuView, contractJSONLimit, catalog.None}, {DanmakuView, contractJSONLimit + 1, catalog.InvalidResponse}, {DanmakuSegment, contractSegmentLimit, catalog.None}, {DanmakuSegment, contractSegmentLimit + 1, catalog.InvalidResponse}} {
		body := &observedBody{Reader: bytes.NewReader(bytes.Repeat([]byte{0x08}, item.length))}
		c := testClient(t, func(*http.Request) (*http.Response, error) { return &http.Response{StatusCode: 200, Body: body}, nil })
		_ = c.Activate(1, testMaterial())
		params := url.Values{"oid": {"123"}}
		if item.route == DanmakuSegment {
			params.Set("segment_index", "1")
		}
		data, err := bind(t, c).Protobuf(context.Background(), item.route, params)
		if item.want == catalog.None {
			if err != nil || len(data) != item.length {
				t.Fatal("exact protobuf byte bound rejected")
			}
		} else if err != item.want || data != nil {
			t.Fatal("protobuf route byte bound bypassed")
		}
		if body.closed.Load() != 1 {
			t.Fatal("protobuf body leaked")
		}
	}
}

func TestScopeAndCallerCancellationReachBodyRead(t *testing.T) {
	for _, kind := range []string{"scope", "caller", "deadline"} {
		entered := make(chan struct{})
		var body *observedBody
		c := testClient(t, func(req *http.Request) (*http.Response, error) {
			body = &observedBody{Reader: &waitingReader{ctx: req.Context(), entered: entered}}
			return &http.Response{StatusCode: 200, Body: body}, nil
		})
		_ = c.Activate(1, testMaterial())
		s := bind(t, c)
		ctx, cancel := context.WithCancel(context.Background())
		if kind == "deadline" {
			cancel()
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
		}
		defer cancel()
		result := make(chan error, 1)
		go func() { _, err := s.JSON(ctx, Detail, detailParams()); result <- err }()
		await(t, entered)
		switch kind {
		case "scope":
			c.Revoke(1)
		case "caller":
			cancel()
		}
		want := catalog.Canceled
		if kind == "deadline" {
			want = catalog.Timeout
		}
		if await(t, result) != want || body.closed.Load() != 1 {
			t.Fatal("cancellation did not reach/close owned body")
		}
	}
}

func TestFourRequestQueueIsCancelableAndCannotExtendDeadline(t *testing.T) {
	release, open := latch()
	defer open()
	entered := make(chan struct{}, contractConcurrency+1)
	var calls, active, maximum atomic.Int32
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		current := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); current > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, current) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-release:
			return response(`{"code":0,"data":{}}`), nil
		case <-req.Context().Done():
			return nil, errors.New(private)
		}
	})
	s := bind(t, c)
	results := make(chan error, contractConcurrency)
	for range contractConcurrency {
		go func() { _, err := s.JSON(context.Background(), Detail, detailParams()); results <- err }()
	}
	for range contractConcurrency {
		await(t, entered)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	_, err := s.JSON(ctx, Detail, detailParams())
	cancel()
	if err != catalog.Timeout || calls.Load() != contractConcurrency || maximum.Load() != contractConcurrency {
		t.Fatal("queue cap or canceled-wait behavior failed")
	}
	open()
	for range contractConcurrency {
		if await(t, results) != nil {
			t.Fatal("accepted request failed after queue release")
		}
	}
	if active.Load() != 0 || len(c.slots) != 0 {
		t.Fatal("request permit leaked")
	}
	deadline := time.Now().Add(time.Second)
	parent, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	c.roundTripper = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		got, ok := req.Context().Deadline()
		if !ok || got != deadline {
			t.Error("raw request extended caller deadline")
		}
		return response(`{"code":0,"data":{}}`), nil
	})
	if _, err := s.JSON(parent, Detail, detailParams()); err != nil {
		t.Fatal("deadline check request failed")
	}
}

func TestCanceledSubmissionAndLateSuccessCannotStartOrPublish(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	c := testClient(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		cancel()
		return response(`{"code":0,"data":{}}`), nil
	})
	s := bind(t, c)
	if data, err := s.JSON(ctx, Detail, detailParams()); err != catalog.Canceled || data != nil {
		t.Fatal("late canceled success was published")
	}
	if _, err := s.JSON(ctx, Detail, detailParams()); err != catalog.Canceled || calls.Load() != 1 {
		t.Fatal("already canceled submission made a request")
	}
}
