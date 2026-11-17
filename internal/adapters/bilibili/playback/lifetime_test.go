package playback

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
)

func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("owned media operation did not finish")
	}
	var zero T
	return zero
}
func gate() (<-chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}

func TestSourceRetirementAfterHTTPStillPreventsPublication(t *testing.T) {
	for _, kind := range []string{"caller", "catalog", "scope"} {
		f := newFixture(t)
		f.load(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f.source.now = func() time.Time {
			switch kind {
			case "caller":
				cancel()
			case "catalog":
				f.details.Close()
			case "scope":
				f.service.Close()
			}
			return time.Unix(1700000000, 0)
		}
		result, err := f.source.Resolve(ctx, app.ResolveRequest{Part: "101"})
		if err != catalog.Canceled || result.Input.URL != "" || len(result.Qualities) != 0 {
			t.Fatal("retired result survived post-response scope/context check")
		}
	}
}

func TestSourceCancellationReachesInflightMediaRequest(t *testing.T) {
	for _, scopeClose := range []bool{false, true} {
		f := newFixture(t)
		f.load(t)
		entered := make(chan struct{})
		f.serveMedia = func(req *http.Request) ([]byte, error) {
			close(entered)
			<-req.Context().Done()
			return nil, errors.New(privateMarker)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			result, err := f.source.Resolve(ctx, app.ResolveRequest{Part: "101"})
			if result.Input.URL != "" {
				t.Error("canceled source returned media")
			}
			done <- err
		}()
		await(t, entered)
		if scopeClose {
			f.service.Close()
		} else {
			cancel()
		}
		err := await(t, done)
		if err != catalog.Canceled && err != catalog.Closed {
			t.Fatal("inflight cancellation lost safe category")
		}
		assertSafe(t, err)
	}
}

func TestCanceledLateMediaBytesDoNotFallbackToPriorInput(t *testing.T) {
	f := newFixture(t)
	f.load(t)
	f.resolve(t, "")
	entered := make(chan struct{})
	release, open := gate()
	defer open()
	f.serveMedia = func(*http.Request) ([]byte, error) { close(entered); <-release; return envelope(f.media), nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := f.source.Resolve(ctx, app.ResolveRequest{Part: "101"})
		if result.Input.URL != "" {
			t.Error("canceled result reused old input")
		}
		done <- err
	}()
	await(t, entered)
	cancel()
	open()
	if await(t, done) != catalog.Canceled || f.count("/x/player/wbi/playurl") != 2 {
		t.Fatal("late bytes accepted or retried")
	}
}

func TestNilCanceledAndEarlierDeadlineRemainBounded(t *testing.T) {
	f := newFixture(t)
	if _, err := f.source.Detail(nil); err != catalog.Invalid {
		t.Fatal("nil detail context accepted")
	}
	if _, err := f.source.Resolve(nil, app.ResolveRequest{Part: "101"}); err != catalog.Invalid {
		t.Fatal("nil media context accepted")
	}
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	if _, err := f.source.Detail(expired); err != catalog.Timeout || len(f.requests) != 0 {
		t.Fatal("expired detail made a request")
	}
	f.load(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	f.serveMedia = func(req *http.Request) ([]byte, error) {
		end, ok := req.Context().Deadline()
		if !ok || end != deadline {
			t.Error("media transport extended caller deadline")
		}
		<-req.Context().Done()
		return nil, errors.New(privateMarker)
	}
	if result, err := f.source.Resolve(ctx, app.ResolveRequest{Part: "101"}); err != catalog.Timeout || result.Input.URL != "" {
		t.Fatal("media deadline lost or became success")
	}
}

func TestMediaTransportDenialsAndStructureCannotBecomeSuccess(t *testing.T) {
	for _, item := range []struct {
		body []byte
		want catalog.Code
	}{
		{[]byte(`{"code":-101}`), catalog.LoginRequired}, {[]byte(`{"code":-403}`), catalog.Restricted}, {[]byte(`{"code":-412}`), catalog.VerificationRequired},
		{[]byte(`{"code":0,"data":{"v_voucher":"PRIVATE_SYNTHETIC_MEDIA"}}`), catalog.VerificationRequired},
		{[]byte(`{"code":0,"code":0,"data":{}}`), catalog.InvalidResponse},
	} {
		f := newFixture(t)
		f.load(t)
		f.serveMedia = func(*http.Request) ([]byte, error) { return item.body, nil }
		result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"})
		if err != item.want || result.Input.URL != "" || f.count("/x/player/wbi/playurl") != 1 {
			t.Fatal("denied/malformed media became success or retry")
		}
		assertSafe(t, err)
	}
	f := newFixture(t)
	f.load(t)
	f.resolve(t, "")
	f.serveMedia = func(*http.Request) ([]byte, error) { return nil, errors.New(privateMarker) }
	if result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"}); err != catalog.Network || result.Input.URL != "" || f.count("/x/player/wbi/playurl") != 2 {
		t.Fatal("failed fresh resolution silently reused old locator")
	}
}

func TestMediaRefusalsCannotBeRescuedByPlayableDASH(t *testing.T) {
	// These are the documented ordinary-UGC refusal boundaries. An otherwise
	// playable DASH body never overrides HTTP, service-code or voucher refusal.
	for _, item := range []struct {
		name         string
		code, status int
		voucher      bool
		want         catalog.Code
	}{
		{name: "service denied", code: -403, want: catalog.Restricted},
		{name: "login required", code: -101, want: catalog.LoginRequired},
		{name: "voucher", voucher: true, want: catalog.VerificationRequired},
		{name: "http forbidden", status: http.StatusForbidden, want: catalog.Restricted},
	} {
		t.Run(item.name, func(t *testing.T) {
			f := newFixture(t)
			f.load(t)
			f.mediaStatus = item.status
			if item.voucher {
				f.media["v_voucher"] = privateMarker
			}
			data, err := json.Marshal(map[string]any{"code": item.code, "data": f.media})
			if err != nil {
				t.Fatal("synthetic envelope could not be encoded")
			}
			f.serveMedia = func(*http.Request) ([]byte, error) { return data, nil }
			result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"})
			if err != item.want || result.Input.URL != "" || result.Input.AudioURL != "" || len(result.Qualities) != 0 || f.count("/x/player/wbi/playurl") != 1 {
				t.Fatal("playable DASH bypassed an established refusal boundary")
			}
			assertSafe(t, err)
		})
	}
}
