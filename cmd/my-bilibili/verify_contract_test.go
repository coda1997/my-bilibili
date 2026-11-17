package main

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func TestVerificationControllerNeedsEveryFixedActionWithoutInvokingThem(t *testing.T) {
	r, _ := verifyTestRun(t)
	m := newVerifyMachine(t, r)
	if m.opens != 0 || m.searches != 0 || m.plays != 0 || m.pauses != 0 || m.closes != 0 || r.budget.snapshot().MediaOpens != 0 {
		t.Fatal("controller construction invoked normal actions")
	}
	if controller, err := newVerificationController(nil, m.v.actions); err != catalog.Invalid || controller != nil {
		t.Fatal("missing run admitted")
	}
	if controller, err := newVerificationController(&verificationRun{}, m.v.actions); err != catalog.Invalid || controller != nil {
		t.Fatal("missing owned setup admitted")
	}
	typ := reflect.TypeOf(m.v.actions)
	for i := 0; i < typ.NumField(); i++ {
		actions := m.v.actions
		reflect.ValueOf(&actions).Elem().Field(i).SetZero()
		if controller, err := newVerificationController(r, actions); err != catalog.Invalid || controller != nil {
			t.Fatal("missing fixed action admitted")
		}
	}
	if _, err := r.HistoryStore(m.store); err != history.Invalid {
		t.Fatal("one run transferred the Store twice")
	}
	if _, err := (&verificationRun{}).HistoryStore(nil); err != history.Invalid {
		t.Fatal("nil Store transferred")
	}
}

func TestVerificationGuardsRejectMissingAndCanceledDependenciesBeforeDelegation(t *testing.T) {
	r := verifyTestBudget(t, verificationCounts{}, 1)
	if response, err := r.APITransport(nil).RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav")); err != catalog.Invalid || response != nil {
		t.Fatal("nil transport admitted")
	}
	r = verifyTestBudget(t, verificationCounts{}, 1)
	calls := 0
	rt := r.APITransport(verificationRTFunc(func(*http.Request) (*http.Response, error) { calls++; return verifyTestResponse(), nil }))
	if response, err := rt.RoundTrip(nil); err != catalog.Invalid || response != nil || calls != 0 {
		t.Fatal("nil request reached delegate")
	}
	r = verifyTestBudget(t, verificationCounts{}, 1)
	if engine, err := r.Opener(nil).Open(context.Background(), playback.Input{}); err != playback.Invalid || engine != nil {
		t.Fatal("nil Opener admitted")
	}
	r = verifyTestBudget(t, verificationCounts{}, 1)
	opener := r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) {
		calls++
		return &verificationFakeEngine{}, nil
	}))
	if engine, err := opener.Open(nil, playback.Input{}); err != playback.Invalid || engine != nil {
		t.Fatal("nil native context admitted")
	}
	r = verifyTestBudget(t, verificationCounts{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rt = r.APITransport(verificationRTFunc(func(*http.Request) (*http.Response, error) { calls++; return verifyTestResponse(), nil }))
	if _, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav").WithContext(ctx)); err != catalog.Canceled {
		t.Fatal("pre-canceled request admitted")
	}
	opener = r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) {
		calls++
		return &verificationFakeEngine{}, nil
	}))
	if _, err := opener.Open(ctx, playback.Input{}); err != playback.Canceled || calls != 0 || r.budget.snapshot().verificationCounts != (verificationCounts{}) {
		t.Fatal("pre-canceled dependency spent budget or delegated")
	}
}
