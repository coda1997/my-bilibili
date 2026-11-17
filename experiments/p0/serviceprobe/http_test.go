package serviceprobe

import (
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDecodedBodyLimitWithProductionTransport(t *testing.T) {
	var encoded bytes.Buffer
	z := gzip.NewWriter(&encoded)
	if _, err := z.Write(bytes.Repeat([]byte(" "), jsonLimit+1)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected credential header")
		}
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()
	tr := productionTransport()
	defer tr.CloseIdleConnections()
	o := Observation{}
	_, category := fetch(context.Background(), &http.Client{Transport: tr}, server.URL, jsonLimit, &o)
	if category != "body-limit" || o.Bytes != jsonLimit+1 || calls.Load() != 1 {
		t.Fatalf("decoded cap not enforced: %s bytes=%d calls=%d", category, o.Bytes, calls.Load())
	}
}

func TestProductionTransportDoesNotReuseOrRetry(t *testing.T) {
	var calls, connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error("hijack failed")
				return
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write(successBodies[0])
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()
	tr := productionTransport()
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr}
	if _, category := fetch(context.Background(), client, server.URL, jsonLimit, &Observation{}); category != "" {
		t.Fatal("first local request failed")
	}
	if _, category := fetch(context.Background(), client, server.URL, jsonLimit, &Observation{}); category != "transport" {
		t.Fatal("new-connection failure retried or hidden")
	}
	if calls.Load() != 2 || connections.Load() != 2 {
		t.Fatalf("unexpected connection reuse/retry: calls=%d connections=%d", calls.Load(), connections.Load())
	}
}

func TestRequestConstructionFailureIsSanitized(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, nil })}
	_, category := fetch(context.Background(), client, "https://invalid/%zz?"+sentinel, jsonLimit, &Observation{})
	if category != "invalid-request" || calls != 0 {
		t.Fatal("invalid request not stopped at construction")
	}
}
