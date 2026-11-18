package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"my-bilibili/internal/catalog"
)

type metricsRoundTripper func(*http.Request) (*http.Response, error)

func (f metricsRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type metricsBody struct {
	io.Reader
	closes int
}

func (b *metricsBody) Close() error { b.closes++; return catalog.Network }

func TestClientRequestActivityKeepsBodyOwnedUntilOneActualClose(t *testing.T) {
	activity := &callActivity{}
	body := &metricsBody{Reader: strings.NewReader("original synthetic response")}
	transport := countedTransport{activity: activity, delegate: metricsRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: body}, nil
	})}
	request, _ := http.NewRequest(http.MethodGet, "https://example.test/", nil)
	response, err := transport.RoundTrip(request)
	if err != nil || activity.snapshot() != (activitySnapshot{calls: 1, active: 1, peak: 1}) {
		t.Fatal("headers prematurely ended request ownership")
	}
	if _, err = io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	if activity.snapshot().active != 1 {
		t.Fatal("EOF ended ownership before body close")
	}
	for i := 0; i < 2; i++ {
		if !errors.Is(response.Body.Close(), catalog.Network) {
			t.Fatal("close error identity lost")
		}
	}
	if body.closes != 1 || activity.snapshot() != (activitySnapshot{calls: 1, active: 0, peak: 1}) {
		t.Fatal("body close was duplicated or ownership remained")
	}
	transport.delegate = metricsRoundTripper(func(*http.Request) (*http.Response, error) { return nil, catalog.Timeout })
	if _, err = transport.RoundTrip(request); err != catalog.Timeout || activity.snapshot() != (activitySnapshot{calls: 2, active: 0, peak: 1}) {
		t.Fatal("failed submission lost error or ownership")
	}
}
