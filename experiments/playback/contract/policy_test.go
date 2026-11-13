package contract

import (
	"context"
	"errors"
	"testing"
	"time"
)

type policyFunc func(Input) error

func (f policyFunc) Validate(in Input) error { return f(in) }

func TestPolicyIsExplicitAndCopiesAcrossBoundaries(t *testing.T) {
	in := Input{URL: "https://public.example.test/video", AudioURL: "https://public.example.test/audio", Headers: map[string]string{"Referer": "public"}}
	calls := 0
	var captured Input
	e := &fakeEngine{s: Snapshot{State: Ready}}
	opener := openFunc(func(_ context.Context, got Input) (Engine, error) { calls++; captured = got; return e, nil })
	if New(opener, nil).Open(context.Background(), in) != Invalid || calls != 0 {
		t.Fatal("default policy expanded")
	}
	var validated Input
	c := NewWithPolicy(opener, nil, policyFunc(func(got Input) error { validated = got; return nil }))
	if err := c.Open(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Headers["Referer"] = "caller mutation"
	validated.Headers["Referer"] = "validator mutation"
	if captured.Headers["Referer"] != "public" {
		t.Fatal("input alias escaped")
	}
	_ = c.Close()
	if e.closed != 1 {
		t.Fatal("engine retained")
	}
}

func TestOptInPolicyPreservesDeadlineCategory(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	c := NewWithPolicy(nil, nil, policyFunc(func(Input) error { return nil }))
	if c.Open(ctx, Input{}) != Timeout || c.Snapshot().Error != Timeout {
		t.Fatal("target deadline was not classified as timeout")
	}
}

func TestPolicyRejectionPreservesExistingSessionAndSafeCodes(t *testing.T) {
	e := &fakeEngine{s: Snapshot{State: Ready}}
	rejected := error(nil)
	calls := 0
	c := NewWithPolicy(openFunc(func(context.Context, Input) (Engine, error) { calls++; return e, nil }), nil, policyFunc(func(Input) error { return rejected }))
	in := Input{URL: "https://public.example.test/video"}
	if err := c.Open(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	for _, code := range []ErrorCode{Invalid, Unsupported, Timeout, Canceled} {
		rejected = code
		if c.Open(context.Background(), in) != code || e.closed != 0 || calls != 1 {
			t.Fatal("rejected replacement disturbed ownership")
		}
	}
	rejected = errors.New("PRIVATE_SYNTHETIC")
	if c.Open(context.Background(), in) != Media {
		t.Fatal("unsafe policy error")
	}
	_ = c.Close()
}
