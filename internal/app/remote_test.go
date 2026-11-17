package app

import (
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type inputPolicyFunc func(playback.Input) error

func (f inputPolicyFunc) Validate(in playback.Input) error { return f(in) }

func TestSessionAndControllerRequireExplicitHTTPSPolicies(t *testing.T) {
	for _, pair := range []struct {
		session, controller playback.InputPolicy
		pass                bool
	}{
		{nil, nil, false}, {playback.HTTPSPolicy{}, nil, false}, {nil, playback.HTTPSPolicy{}, false},
		{playback.HTTPSPolicy{}, playback.HTTPSPolicy{}, true},
	} {
		h := newHarness(t)
		h.source.inputs["p1"] = playback.Input{URL: "https://media.example.test/video", AudioURL: "https://other.example.test/audio", Headers: map[string]string{"Referer": playback.PublicReferer, "User-Agent": playback.PublicUserAgent}}
		var err error
		h.s, err = NewWithOptions(h.source, playback.NewWithPolicy(h.opener, func() time.Time { return h.now }, pair.controller), Options{InputPolicy: pair.session})
		if err != nil {
			t.Fatal(err)
		}
		h.s.now = func() time.Time { return h.now }
		h.start(t)
		if pair.pass {
			if h.s.State().SelectionError != catalog.None || len(h.opener.inputs) != 1 {
				t.Fatal("explicit remote composition rejected")
			}
		} else if h.s.State().SelectionError != catalog.Invalid || len(h.opener.inputs) != 0 {
			t.Fatal("remote crossed an unconfigured boundary", h.s.State())
		}
	}
}

func TestSessionPolicyCannotMutateAcceptedHeaders(t *testing.T) {
	h := newHarness(t)
	h.source.inputs["p1"] = playback.Input{URL: "https://media.example.test/video", Headers: map[string]string{"Referer": playback.PublicReferer, "User-Agent": playback.PublicUserAgent}}
	policy := inputPolicyFunc(func(in playback.Input) error {
		err := (playback.HTTPSPolicy{}).Validate(in)
		in.Headers["Referer"] = "PRIVATE_SYNTHETIC"
		return err
	})
	var err error
	h.s, err = NewWithOptions(h.source, playback.NewWithPolicy(h.opener, func() time.Time { return h.now }, playback.HTTPSPolicy{}), Options{InputPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	h.s.now = func() time.Time { return h.now }
	h.start(t)
	if len(h.opener.inputs) != 1 || h.opener.inputs[0].Headers["Referer"] != playback.PublicReferer {
		t.Fatal("policy mutated cached input")
	}
}
