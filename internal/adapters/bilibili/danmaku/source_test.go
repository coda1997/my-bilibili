package danmaku

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/catalog"
	model "my-bilibili/internal/danmaku"
)

type testRoundTrip func(*http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func testVideo() catalog.Video {
	return catalog.Video{ID: "BV0123456789", Title: "Synthetic", Creator: "Local",
		Parts: []catalog.Part{{ID: "101", Title: "One", Duration: 720 * time.Second}, {ID: "102", Title: "Two", Duration: 30 * time.Second}}}
}

func testRef() catalog.PartRef { return catalog.PartRef{Video: "BV0123456789", Part: "101"} }

func response(data []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}
}

func wait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("owned synthetic operation did not finish")
	}
	var zero T
	return zero
}

func clientSource(t *testing.T, authenticated bool, trip testRoundTrip) (*Source, *transport.Client) {
	t.Helper()
	client := transport.New(trip)
	t.Cleanup(func() { client.Close(); wait(t, client.Done()) })
	if authenticated {
		material := account.NewMaterial([]byte(`{"version":1,"scope":"bilibili.com/","cookies":[{"name":"SESSDATA","value":"SYNTHETIC_SEGMENT_ONLY","expires":0}]}`))
		if err := client.Activate(1, material); err != nil {
			t.Fatal("synthetic publisher setup", err)
		}
	}
	scope, err := client.Bind()
	if err != nil {
		t.Fatal(err)
	}
	source, err := New(scope, testVideo())
	if err != nil {
		t.Fatal(err)
	}
	return source, client
}

func TestConstructorMembershipAndAnonymousMetadata(t *testing.T) {
	var calls atomic.Int32
	source, client := clientSource(t, false, func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		if request.Method != http.MethodGet || request.URL.Host != "api.bilibili.com" || request.URL.Path != "/x/v2/dm/web/seg.so" || request.Header.Get("Cookie") != "" {
			t.Error("anonymous segment left the fixed cookie-free route")
		}
		query := request.URL.Query()
		if len(query) != 3 || query.Get("type") != "1" || query.Get("oid") != "101" || query.Get("segment_index") != "2" {
			t.Error("wrong segment parameters")
		}
		return response(blob(1, element(361250, 1, "owned"))), nil
	})
	if calls.Load() != 0 {
		t.Fatal("construction performed I/O")
	}
	metadata, err := source.Metadata(context.Background(), testRef())
	if err != nil || metadata != (model.Metadata{}) || calls.Load() != 0 {
		t.Fatal("anonymous metadata is not explicitly unknown without a request", metadata, err)
	}
	for _, ref := range []catalog.PartRef{{}, {Video: "other", Part: "101"}, {Video: testRef().Video, Part: "999"}} {
		if _, err := source.Metadata(context.Background(), ref); err != catalog.Invalid {
			t.Fatal("invalid metadata membership", err)
		}
		if _, err := source.Segment(context.Background(), ref, 1); err != catalog.Invalid {
			t.Fatal("invalid segment membership", err)
		}
	}
	for _, index := range []int{-1, 0, 3} {
		if _, err := source.Segment(context.Background(), testRef(), index); err != catalog.Invalid {
			t.Fatal("invalid segment index reached transport", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected membership/index made a request")
	}
	video := testVideo()
	scope, _ := client.Bind()
	copySource, err := New(scope, video)
	if err != nil {
		t.Fatal(err)
	}
	video.ID, video.Parts[0].ID, video.Parts[0].Duration = "changed", "bad", time.Second
	got, err := copySource.Segment(context.Background(), testRef(), 2)
	if err != nil || len(got.Entries) != 1 || got.Entries[0].Entry.At != 361250*time.Millisecond || calls.Load() != 1 {
		t.Fatal("copied membership or actual conversion changed", err)
	}
	for _, invalid := range []catalog.Video{
		{}, {ID: "v", Title: "T", Creator: "C", Parts: []catalog.Part{{ID: "01", Title: "P", Duration: time.Second}}},
		{ID: "v", Title: "T", Creator: "C", Parts: []catalog.Part{{ID: "0", Title: "P", Duration: time.Second}}},
	} {
		if _, err := New(scope, invalid); err != catalog.Invalid {
			t.Fatal("invalid copied video accepted", err)
		}
	}
	if _, err := New(nil, testVideo()); err != catalog.Invalid {
		t.Fatal("nil scope accepted")
	}
	if _, err := New(&transport.Scope{}, testVideo()); err != catalog.Invalid {
		t.Fatal("unbound scope accepted")
	}
}

func TestAuthenticatedMetadataHasNoFailureFallback(t *testing.T) {
	for _, test := range []struct {
		status int
		body   []byte
		err    error
	}{
		{200, num(1, 0), nil}, {200, num(1, 1), nil},
		{403, nil, catalog.Restricted}, {429, nil, catalog.RateLimited},
		{200, []byte(`{"code":-101}`), catalog.LoginRequired},
		{200, []byte("<html>not protocol</html>"), catalog.InvalidResponse},
		{200, []byte{0x80}, catalog.InvalidResponse},
	} {
		var calls atomic.Int32
		source, _ := clientSource(t, true, func(request *http.Request) (*http.Response, error) {
			calls.Add(1)
			query := request.URL.Query()
			if request.URL.Path != "/x/v2/dm/web/view" || len(query) != 2 || query.Get("type") != "1" || query.Get("oid") != "101" || len(request.Cookies()) != 1 {
				t.Error("metadata destination or scope cookie changed")
			}
			result := response(test.body)
			result.StatusCode = test.status
			return result, nil
		})
		got, err := source.Metadata(context.Background(), testRef())
		if err != test.err || calls.Load() != 1 || err == nil && !got.Known || err != nil && got != (model.Metadata{}) {
			t.Fatal("metadata failure became anonymous success or another request", got, err)
		}
	}
}

func TestScopeAndCallerCancellationNeverPublishOldData(t *testing.T) {
	var calls atomic.Int32
	source, client := clientSource(t, true, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(nil), nil
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, test := range []struct {
		ctx context.Context
		err error
	}{{nil, catalog.Invalid}, {canceled, catalog.Canceled}, {expired, catalog.Timeout}} {
		if _, err := source.Metadata(test.ctx, testRef()); err != test.err {
			t.Fatal("metadata context", err)
		}
		if _, err := source.Segment(test.ctx, testRef(), 1); err != test.err {
			t.Fatal("segment context", err)
		}
	}
	client.Revoke(1)
	if _, err := source.Metadata(context.Background(), testRef()); err != catalog.Canceled {
		t.Fatal("revoked metadata fell back to anonymous", err)
	}
	if _, err := source.Segment(context.Background(), testRef(), 1); err != catalog.Canceled || calls.Load() != 0 {
		t.Fatal("revoked source used a new scope", err)
	}
	if _, err := New(source.scope, testVideo()); err != catalog.Canceled {
		t.Fatal("retired scope constructor", err)
	}
	for _, metadata := range []bool{false, true} {
		started, release := make(chan struct{}), make(chan struct{})
		s, c := clientSource(t, true, func(*http.Request) (*http.Response, error) {
			close(started)
			<-release
			if metadata {
				return response(num(1, 0)), nil
			}
			return response(blob(1, element(0, 1, "retired text"))), nil
		})
		result := make(chan error, 1)
		go func() {
			if metadata {
				_, err := s.Metadata(context.Background(), testRef())
				result <- err
			} else {
				_, err := s.Segment(context.Background(), testRef(), 1)
				result <- err
			}
		}()
		wait(t, started)
		c.Revoke(1)
		close(release)
		if err := wait(t, result); err != catalog.Canceled {
			t.Fatal("late scoped response was accepted", err)
		}
	}
}

type closedBody struct {
	io.Reader
	closed *atomic.Int32
	err    error
}

func (b *closedBody) Close() error { b.closed.Add(1); return b.err }

func sizedUnknown(size int) []byte {
	padding := size
	for {
		data := blob(30, make([]byte, padding))
		if len(data) == size {
			return data
		}
		padding -= len(data) - size
	}
}

func TestScopedBodyBoundsAndCloseErrors(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		limit := 4 << 20
		if metadata {
			limit = 1 << 20
		}
		for _, size := range []int{limit, limit + 1} {
			var closed atomic.Int32
			source, _ := clientSource(t, metadata, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &closedBody{Reader: bytes.NewReader(sizedUnknown(size)), closed: &closed}}, nil
			})
			var err error
			if metadata {
				_, err = source.Metadata(context.Background(), testRef())
			} else {
				_, err = source.Segment(context.Background(), testRef(), 1)
			}
			if (err == nil) != (size == limit) || size > limit && err != catalog.InvalidResponse || closed.Load() != 1 {
				t.Fatal("protocol body bound or response ownership", size, closed.Load(), err)
			}
		}
	}
	var closed atomic.Int32
	source, _ := clientSource(t, false, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &closedBody{Reader: strings.NewReader(""), closed: &closed, err: errors.New("private body sentinel")}}, nil
	})
	got, err := source.Segment(context.Background(), testRef(), 1)
	if err != catalog.Network || !reflect.DeepEqual(got, model.Segment{}) || closed.Load() != 1 {
		t.Fatal("body close failure became empty success", err)
	}
}
