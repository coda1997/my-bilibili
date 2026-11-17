package main

import (
	"bytes"
	"context"
	"errors"
	"my-bilibili/experiments/p0/serviceprobe"
	"strings"
	"testing"
	"time"
)

func success() serviceprobe.Report {
	r := serviceprobe.Report{}
	for range 5 {
		r.Observations = append(r.Observations, serviceprobe.Observation{Step: "synthetic", Status: "pass", Category: "expected-response"})
	}
	return r
}

func TestExplicitLiveGateAndPrivateArgumentErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--live=false"}, {"--cookie=PRIVATE_SENTINEL"}, {"--live", "PRIVATE_SENTINEL"}, {"--live=PRIVATE_SENTINEL"}, {"--help"}, {"--live", "--endpoint=PRIVATE_SENTINEL"}} {
		var out, diagnostics bytes.Buffer
		calls := 0
		code := run(args, &out, &diagnostics, func(context.Context) serviceprobe.Report { calls++; return success() })
		if code != 2 || calls != 0 || out.Len() != 0 || strings.Contains(diagnostics.String(), "PRIVATE_SENTINEL") || diagnostics.String() != "usage: service-probe --live\n" {
			t.Fatal("invalid invocation reached network or exposed input")
		}
	}
}

func TestLiveRunDeadlineAndExitStatus(t *testing.T) {
	for _, pass := range []bool{true, false} {
		var out, diagnostics bytes.Buffer
		var child context.Context
		calls := 0
		code := run([]string{"--live"}, &out, &diagnostics, func(ctx context.Context) serviceprobe.Report {
			child = ctx
			calls++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 60*time.Second || time.Until(deadline) < 59*time.Second {
				t.Fatal("missing whole-invocation limit")
			}
			r := success()
			if !pass {
				r.Observations[2].Status = "fail"
			}
			return r
		})
		want := 1
		if pass {
			want = 0
		}
		if code != want || calls != 1 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "observations") || child.Err() != context.Canceled {
			t.Fatal("wrong output, status or context cleanup")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("PRIVATE_SENTINEL") }
func TestOutputFailureIsSanitized(t *testing.T) {
	var diagnostics bytes.Buffer
	code := run([]string{"--live"}, failingWriter{}, &diagnostics, func(context.Context) serviceprobe.Report { return success() })
	if code != 1 || diagnostics.String() != "could not write sanitized probe report\n" {
		t.Fatal("write failure hidden or exposed")
	}
}
