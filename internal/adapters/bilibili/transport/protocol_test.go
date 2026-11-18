package transport

import (
	"context"
	"my-bilibili/internal/catalog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSafeProtocolAndHTTPFailuresNeverSucceedOrExpire(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		want       catalog.Code
	}{
		{"missing code", `{"data":{}}`, 200, catalog.InvalidResponse}, {"null code", `{"code":null}`, 200, catalog.InvalidResponse},
		{"string code", `{"code":"0","data":{}}`, 200, catalog.InvalidResponse}, {"duplicate code", `{"code":-101,"code":0,"data":{}}`, 200, catalog.InvalidResponse},
		{"null data", `{"code":0,"data":null}`, 200, catalog.InvalidResponse}, {"missing data", `{"code":0}`, 200, catalog.InvalidResponse},
		{"wrong data", `{"code":0,"data":[]}`, 200, catalog.InvalidResponse}, {"nested duplicate", `{"code":0,"data":{"bvid":"A","bvid":"B"}}`, 200, catalog.InvalidResponse},
		{"voucher type", `{"code":0,"data":{"v_voucher":false}}`, 200, catalog.InvalidResponse},
		{"trailing JSON", `{"code":0,"data":{}} {}`, 200, catalog.InvalidResponse}, {"unknown HTML", `<html>PRIVATE_SYNTHETIC_TRANSPORT</html>`, 200, catalog.InvalidResponse},
		{"too deep", `{"code":0,"data":{"unknown":` + strings.Repeat("[", 33) + "0" + strings.Repeat("]", 33) + `}}`, 200, catalog.InvalidResponse},
		{"redirect", `{"code":0,"data":{}}`, 302, catalog.InvalidResponse}, {"restricted", private, 403, catalog.Restricted}, {"unauthorized", private, 401, catalog.LoginRequired},
		{"rate limited", private, 429, catalog.RateLimited}, {"service unavailable", private, 503, catalog.ServiceUnavailable},
		{"non nav login required", `{"code":-101}`, 200, catalog.LoginRequired}, {"challenge code", `{"code":-412,"message":"PRIVATE_SYNTHETIC_TRANSPORT"}`, 200, catalog.VerificationRequired},
		{"top voucher", `{"code":0,"data":{},"v_voucher":"PRIVATE_SYNTHETIC_TRANSPORT"}`, 200, catalog.VerificationRequired},
		{"nested voucher", `{"code":0,"data":{"v_voucher":"PRIVATE_SYNTHETIC_TRANSPORT"}}`, 200, catalog.VerificationRequired},
		{"unknown code", `{"code":87654321,"message":"PRIVATE_SYNTHETIC_TRANSPORT"}`, 200, catalog.ServiceUnavailable},
		{"unavailable", `{"code":-404}`, 200, catalog.Unavailable}, {"service restricted", `{"code":-403}`, 200, catalog.Restricted},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var calls atomic.Int32
			var body *observedBody
			c := testClient(t, func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				result := response(item.body)
				result.StatusCode = item.status
				result.Header.Set("Location", "https://private.invalid/"+private)
				body = result.Body.(*observedBody)
				return result, nil
			})
			if c.Activate(1, testMaterial()) != nil {
				t.Fatal("activation failed")
			}
			s := bind(t, c)
			got, err := s.JSON(context.Background(), Detail, detailParams())
			if err != item.want || got != nil || calls.Load() != 1 || body.closed.Load() != 1 {
				t.Fatal("failure category, request count or body cleanup wrong")
			}
			safeError(t, err)
			if !c.State().Authenticated || c.State().ExpiredEpoch != 0 || s.Context().Err() != nil {
				t.Fatal("non-expiry failure revoked session")
			}
		})
	}
}

func TestCurrentNavOnlyExpiresUnequivocalAuthenticatedScope(t *testing.T) {
	for _, item := range []struct {
		body    string
		want    catalog.Code
		expires bool
	}{
		{`{"code":-101}`, catalog.SessionExpired, true},
		{`{"code":0,"data":{"isLogin":false}}`, catalog.SessionExpired, true},
		{`{"code":-412}`, catalog.VerificationRequired, false},
		{`{"code":0,"data":{"isLogin":null}}`, catalog.InvalidResponse, false},
		{`{"code":0,"data":{"isLogin":"false"}}`, catalog.InvalidResponse, false},
		{`{"code":0,"data":{}}`, catalog.InvalidResponse, false},
		{`{"code":-101,"data":{"isLogin":true}}`, catalog.InvalidResponse, false},
		{`{"code":-101,"data":[]}`, catalog.InvalidResponse, false},
		{`{"code":0,"data":{"isLogin":false,"v_voucher":"PRIVATE_SYNTHETIC_TRANSPORT"}}`, catalog.VerificationRequired, false},
	} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(item.body), nil })
		_ = c.Activate(5, testMaterial())
		s := bind(t, c)
		if _, err := s.JSON(context.Background(), Nav, nil); err != item.want {
			t.Fatal("nav classification wrong")
		}
		state := c.State()
		if item.expires {
			if state.Authenticated || state.AccountEpoch != 0 || state.ExpiredEpoch != 5 || s.Context().Err() == nil || !s.State().Closed || s.State().ExpiredEpoch != 5 {
				t.Fatal("current nav did not retire matching scope")
			}
			c.Revoke(5)
			if c.State() != state {
				t.Fatal("expiry feedback changed newer anonymous scope")
			}
		} else if !state.Authenticated || state.ExpiredEpoch != 0 || s.Context().Err() != nil {
			t.Fatal("uncertain nav expired a session")
		}
	}
}

func TestLateNavAndRevocationCannotExpireNewAccount(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	c := testClient(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		close(entered)
		<-release
		return response(`{"code":-101}`), nil
	})
	_ = c.Activate(10, testMaterial())
	a := bind(t, c)
	done := make(chan error, 1)
	go func() { _, err := a.JSON(context.Background(), Nav, nil); done <- err }()
	await(t, entered)
	_ = c.Activate(11, testMaterial())
	b := bind(t, c)
	c.Revoke(10)
	close(release)
	if await(t, done) != catalog.Canceled {
		t.Fatal("late nav was accepted")
	}
	if !c.State().Authenticated || c.State().AccountEpoch != 11 || c.State().ExpiredEpoch != 0 || b.Context().Err() != nil {
		t.Fatal("late A expired B")
	}
	if _, err := a.JSON(context.Background(), Detail, detailParams()); err != catalog.Canceled || calls.Load() != 1 {
		t.Fatal("retired scope acquired another request")
	}
}

func TestAnonymousNavAndProtocolErrorBodies(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return response(anonymousNav), nil })
	s := bind(t, c)
	if _, err := s.JSON(context.Background(), Nav, nil); err != nil || c.State().ExpiredEpoch != 0 || s.Context().Err() != nil {
		t.Fatal("anonymous nav became expiry")
	}
	for _, item := range []struct {
		body string
		want catalog.Code
	}{
		{`{"code":-101}`, catalog.LoginRequired}, {`{"code":-412}`, catalog.VerificationRequired}, {`{"code":0,"data":{}}`, catalog.InvalidResponse}, {`<html>denied</html>`, catalog.InvalidResponse}, {`[]`, catalog.InvalidResponse},
	} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(item.body), nil })
		s := bind(t, c)
		if _, err := s.Protobuf(context.Background(), DanmakuSegment, url.Values{"oid": {"123"}, "segment_index": {"1"}}); err != item.want {
			t.Fatal("non-protobuf response became segment success")
		}
	}
}

func TestJSONRejectsInvalidRawUTF8AndPreservesFullEnvelope(t *testing.T) {
	valid := `{"code":0,"data":{"title":"�中文","unknown":[true,null,1]},"extra":"retained"}`
	for _, body := range []string{valid, strings.Replace(valid, "中文", "\xff", 1), strings.Replace(valid, "unknown", "\xff", 1)} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(body), nil })
		got, err := bind(t, c).JSON(context.Background(), Detail, detailParams())
		if body == valid {
			if err != nil || string(got) != body {
				t.Fatal("successful full envelope bytes were changed")
			}
		} else if err != catalog.InvalidResponse || got != nil {
			t.Fatal("invalid raw UTF-8 escaped the JSON boundary")
		}
	}
}
