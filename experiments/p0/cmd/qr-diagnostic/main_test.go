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

func observed() serviceprobe.QRDiagnostic {
	return serviceprobe.QRDiagnostic{Observation: serviceprobe.Observation{Step: "qr-format-diagnostic", Status: "pass", Category: "observed-format"}, Facts: &serviceprobe.QRDiagnosticFacts{DataKind: "object", KeyKind: "absent", URLKind: "absent", PriorValidatorAccepts: false}}
}

func TestQRExplicitLiveGateAndPrivateArgumentErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--live=false"}, {"--cookie=PRIVATE_SENTINEL"}, {"--live", "PRIVATE_SENTINEL"}, {"--live=PRIVATE_SENTINEL"}, {"--help"}, {"--live", "--endpoint=PRIVATE_SENTINEL"}} {
		var out, diagnostics bytes.Buffer
		calls := 0
		code := run(args, &out, &diagnostics, func(context.Context) serviceprobe.QRDiagnostic { calls++; return observed() })
		if code != 2 || calls != 0 || out.Len() != 0 || strings.Contains(diagnostics.String(), "PRIVATE_SENTINEL") || diagnostics.String() != "usage: qr-diagnostic --live\n" {
			t.Fatal("invalid invocation reached network or exposed input")
		}
	}
}

func TestQRLiveDeadlineAndDiagnosticExitStatus(t *testing.T) {
	for _, pass := range []bool{true, false} {
		var out, diagnostics bytes.Buffer
		var child context.Context
		calls := 0
		code := run([]string{"--live"}, &out, &diagnostics, func(ctx context.Context) serviceprobe.QRDiagnostic {
			child = ctx
			calls++
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 15*time.Second || time.Until(deadline) < 14*time.Second {
				t.Fatal("missing 15-second invocation limit")
			}
			r := observed()
			if !pass {
				r.Observation.Status = "fail"
				r.Facts = nil
			}
			return r
		})
		want := 1
		if pass {
			want = 0
		}
		if code != want || calls != 1 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "observation") || child.Err() != context.Canceled {
			t.Fatal("wrong diagnostic status or context cleanup")
		}
		if pass && !strings.Contains(out.String(), `"prior_validator_accepts":false`) {
			t.Fatal("observed-format incorrectly requires prior acceptance")
		}
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, errors.New("PRIVATE_SENTINEL") }
func TestQRWriteFailureIsSanitized(t *testing.T) {
	var diagnostics bytes.Buffer
	code := run([]string{"--live"}, failedOutput{}, &diagnostics, func(context.Context) serviceprobe.QRDiagnostic { return observed() })
	if code != 1 || diagnostics.String() != "could not write sanitized QR diagnostic\n" {
		t.Fatal("output failure hidden or exposed")
	}
}
