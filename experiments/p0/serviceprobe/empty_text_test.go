package serviceprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestMixedEmptyTextPreservesValidDanmaku(t *testing.T) {
	for _, tt := range []struct {
		name  string
		empty []byte
	}{
		{"explicit", entry(0, 1, nil)},
		{"omitted", fieldNumber(3, 1)},
	} {
		segment := fieldBytes(1, entry(0, 1, []byte(sentinel)))
		segment = append(segment, fieldBytes(1, tt.empty)...)
		segment = append(segment, fieldBytes(1, entry(100, 4, []byte("synthetic")))...)
		t.Run(tt.name+"-decode", func(t *testing.T) {
			counts, valid := decodeSegment(segment)
			if !valid || counts.total != 2 || counts.supported != 2 || counts.unsupported != 0 || counts.skippedEmpty != 1 {
				t.Fatal("valid empty entry discarded nonempty danmaku")
			}
		})
		t.Run(tt.name+"-report", func(t *testing.T) {
			bodies := fixtures()
			bodies[4] = segment
			transport := &script{t: t, bodies: bodies}
			report := Run(context.Background(), transport)
			transport.verify()
			checkPrivate(t, report)
			if !report.Passed() || report.Observations[4].Entries != 2 || report.Observations[4].SkippedEmpty != 1 || transport.calls != 5 {
				t.Fatal("mixed valid/empty segment failed the bounded probe")
			}
		})
	}
}

func TestEmptyTextIsEligibilityNotMalformedData(t *testing.T) {
	// These exact missing/empty fixtures previously lived in malformed-input
	// coverage. They now assert valid skipped eligibility, not removal of coverage.
	for _, tt := range []struct {
		name                                   string
		wire                                   []byte
		total, supported, unsupported, skipped int
	}{
		{"missing-text", fieldBytes(1, fieldNumber(3, 1)), 0, 0, 0, 1},
		{"empty-text", fieldBytes(1, entry(0, 1, nil)), 0, 0, 0, 1},
		{"unsupported-mode-empty", fieldBytes(1, entry(0, 8, nil)), 0, 0, 0, 1},
		{"whitespace-is-text", fieldBytes(1, entry(0, 1, []byte(" \t"))), 1, 1, 0, 0},
		{"unsupported-nonempty", fieldBytes(1, entry(0, 8, []byte("synthetic"))), 1, 0, 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, valid := decodeSegment(tt.wire)
			if !valid || c.total != tt.total || c.supported != tt.supported || c.unsupported != tt.unsupported || c.skippedEmpty != tt.skipped {
				t.Fatal("wrong empty eligibility or accepted-entry counters")
			}
		})
	}
}

func TestSkippedEntriesConsumeSegmentLimit(t *testing.T) {
	empty := fieldBytes(1, fieldNumber(3, 1))
	normal := fieldBytes(1, entry(0, 1, []byte("synthetic")))
	for _, mixed := range []bool{false, true} {
		for _, n := range []int{entryLimit, entryLimit + 1} {
			name := "empty"
			if mixed {
				name = "mixed"
			}
			if n > entryLimit {
				name += "-over"
			} else {
				name += "-exact"
			}
			t.Run(name, func(t *testing.T) {
				count := n
				var segment []byte
				if mixed {
					segment = append(segment, normal...)
					count--
				}
				segment = append(segment, bytes.Repeat(empty, count)...)
				c, valid := decodeSegment(segment)
				if n > entryLimit {
					if valid {
						t.Fatal("skipped entries evaded segment count limit")
					}
					return
				}
				expectedTotal := 0
				if mixed {
					expectedTotal = 1
				}
				if !valid || c.total != expectedTotal || c.skippedEmpty != count || c.total+c.skippedEmpty != entryLimit {
					t.Fatal("exact count including skips rejected or miscounted")
				}
			})
		}
	}
}

func TestEmptyTextStillValidatesEntireEntry(t *testing.T) {
	prefix := entry(0, 1, nil)
	for _, tt := range []struct {
		name   string
		suffix []byte
	}{
		{"later-truncated-key", []byte{0x80}},
		{"later-invalid-wire", []byte{11}},
		{"later-zero-field", []byte{0}},
		{"later-progress-overflow", fieldNumber(2, 360001)},
		{"later-negative-progress", fieldNumber(2, ^uint64(0))},
		{"later-mode-zero", fieldNumber(3, 0)},
		{"later-mode-overflow", fieldNumber(3, 1<<31)},
		{"later-mode-wrong-type", fieldBytes(3, nil)},
		{"later-content-wrong-type", fieldNumber(7, 0)},
		{"later-invalid-utf8", fieldBytes(7, []byte{0xff})},
		{"later-oversized-text", fieldBytes(7, bytes.Repeat([]byte("x"), textBytes+1))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			content := append(append([]byte(nil), prefix...), tt.suffix...)
			if _, valid := decodeSegment(fieldBytes(1, content)); valid {
				t.Fatal("empty text bypassed later field validation")
			}
		})
	}
	for _, invalid := range [][]byte{nil, fieldBytes(7, nil), entry(0, 0, nil), append(fieldBytes(3, nil), fieldBytes(7, nil)...)} {
		if _, valid := decodeSegment(fieldBytes(1, invalid)); valid {
			t.Fatal("empty entry without valid positive mode accepted")
		}
	}
	for _, size := range []int{entryBytes, entryBytes + 1} {
		padding := size - len(prefix) - 4
		content := append(append([]byte(nil), prefix...), fieldBytes(99, bytes.Repeat([]byte("p"), padding))...)
		if len(content) != size {
			t.Fatal("synthetic empty entry size")
		}
		c, valid := decodeSegment(fieldBytes(1, content))
		if size == entryBytes {
			if !valid || c.skippedEmpty != 1 {
				t.Fatal("exact-size valid empty entry rejected")
			}
		} else if valid {
			t.Fatal("empty entry evaded byte limit")
		}
	}
}

func TestAllEmptySegmentsStayInconclusiveAndPrivate(t *testing.T) {
	for _, segment := range [][]byte{fieldBytes(1, fieldNumber(3, 1)), fieldBytes(1, entry(0, 1, nil)), bytes.Repeat(fieldBytes(1, entry(0, 8, nil)), entryLimit)} {
		bodies := fixtures()
		bodies[4] = segment
		transport := &script{t: t, bodies: bodies}
		report := Run(context.Background(), transport)
		transport.verify()
		failedAt(t, report, 4, "danmaku-unavailable", "inconclusive")
		if report.Observations[4].Entries != 0 || report.Observations[4].SkippedEmpty == 0 {
			t.Fatal("all-empty response gained displayable entries or lost skip count")
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(encoded, []byte(`"skipped_empty":`)) {
			t.Fatal("nonzero skip count missing from report")
		}
		for _, o := range report.Observations[:4] {
			if o.SkippedEmpty != 0 {
				t.Fatal("skip counter leaked into another step")
			}
		}
	}
}

func TestSkippedEmptyCounterOmittedForExistingReports(t *testing.T) {
	transport := &script{t: t, bodies: fixtures()}
	report := Run(context.Background(), transport)
	transport.verify()
	checkPrivate(t, report)
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed() || bytes.Contains(encoded, []byte("skipped_empty")) {
		t.Fatal("old no-skip report schema changed")
	}
	diagnostic := qrFixture(t, `{"code":0,"data":{}}`)
	encoded, err = json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("skipped_empty")) {
		t.Fatal("QR diagnostic gained an unrelated counter")
	}
}

func TestEmptySkipPreservesSignedStateAndLaterFailures(t *testing.T) {
	segment := fieldBytes(1, fieldNumber(3, 1))
	segment = append(segment, fieldBytes(1, entry(0, 1, []byte(sentinel)))...)
	for _, state := range []uint64{1, ^uint64(0), ^uint64((1 << 31) - 1)} {
		bodies := fixtures()
		bodies[4] = append(append([]byte(nil), segment...), fieldNumber(2, state)...)
		transport := &script{t: t, bodies: bodies}
		report := Run(context.Background(), transport)
		transport.verify()
		failedAt(t, report, 4, "danmaku-unavailable", "inconclusive")
		if report.Observations[4].Entries != 1 || report.Observations[4].SkippedEmpty != 1 {
			t.Fatal("signed-state observation lost valid/skip counts")
		}
	}
	for _, suffix := range [][]byte{fieldNumber(2, 1<<31), []byte{0x80}, fieldBytes(1, entry(0, 0, nil))} {
		if _, valid := decodeSegment(append(append([]byte(nil), segment...), suffix...)); valid {
			t.Fatal("earlier skip hid later malformed state/entry")
		}
	}
}
