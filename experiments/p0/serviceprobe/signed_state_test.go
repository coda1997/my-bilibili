package serviceprobe

import (
	"bytes"
	"context"
	"testing"
)

func TestSignedInt32SegmentStateClassification(t *testing.T) {
	for _, tt := range []struct {
		name             string
		wire             []byte
		status, category string
	}{
		{"absent-zero", nil, "pass", "expected-response"},
		{"explicit-zero", fieldNumber(2, 0), "pass", "expected-response"},
		{"positive-one", fieldNumber(2, 1), "inconclusive", "danmaku-unavailable"},
		{"positive-max", fieldNumber(2, (1<<31)-1), "inconclusive", "danmaku-unavailable"},
		{"negative-one", fieldNumber(2, ^uint64(0)), "inconclusive", "danmaku-unavailable"},
		{"negative-min", fieldNumber(2, ^uint64((1<<31)-1)), "inconclusive", "danmaku-unavailable"},
		{"positive-overflow", fieldNumber(2, 1<<31), "fail", "invalid-response"},
		{"negative-not-signextended", fieldNumber(2, (1<<32)-1), "fail", "invalid-response"},
		{"negative-underflow", fieldNumber(2, ^uint64(1<<31)), "fail", "invalid-response"},
		{"large-unsigned", fieldNumber(2, 1<<63), "fail", "invalid-response"},
		{"malformed-varint", []byte{16, 0x80}, "fail", "invalid-response"},
		{"overflow-varint", append([]byte{16}, bytes.Repeat([]byte{0xff}, 10)...), "fail", "invalid-response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bodies := fixtures()
			bodies[4] = append(append([]byte(nil), successBodies[4]...), tt.wire...)
			transport := &script{t: t, bodies: bodies}
			report := Run(context.Background(), transport)
			transport.verify()
			if transport.calls != 5 {
				t.Fatal("state classification changed fixed request count")
			}
			checkPrivate(t, report)
			if tt.status == "pass" {
				if !report.Passed() || report.Observations[4].Category != tt.category {
					t.Fatal("valid available state rejected")
				}
			} else {
				failedAt(t, report, 4, tt.category, tt.status)
			}
		})
	}
}
