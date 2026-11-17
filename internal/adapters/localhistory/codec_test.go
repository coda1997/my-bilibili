package localhistory

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

const originalRecord = `{"video_id":"BV0123456789","part_id":"123","title":"Synthetic video","part_title":"Synthetic part","position_ms":1700,"duration_ms":120000,"updated_at":"2026-11-18T00:00:00Z","completed":false}`
const originalDocument = `{"version":1,"records":[` + originalRecord + `]}`
const historyFileBytes = 1048576

func original(part string) history.Record {
	return history.Record{
		Ref:   catalog.PartRef{Video: "BV0123456789", Part: catalog.PartID(part)},
		Title: "Synthetic video", PartTitle: "Synthetic part", Position: 1700 * time.Millisecond,
		Duration: 2 * time.Minute, UpdatedAt: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
	}
}

func TestStrictHistoryDocument(t *testing.T) {
	if records, err := decode([]byte(originalDocument)); err != nil || len(records) != 1 || records[0] != original("123") {
		t.Fatalf("original document: %+v %v", records, err)
	}
	cases := map[string]string{
		"missing version":           `{"records":[]}`,
		"missing records":           `{"version":1}`,
		"unknown root":              `{"version":1,"records":[],"extra":1}`,
		"duplicate root":            `{"version":1,"records":[],"version":1}`,
		"null root":                 `null`,
		"array root":                `[]`,
		"null version":              `{"version":null,"records":[]}`,
		"null records":              `{"version":1,"records":null}`,
		"object records":            `{"version":1,"records":{}}`,
		"fractional version":        `{"version":1.1,"records":[]}`,
		"string version":            `{"version":"1","records":[]}`,
		"null record":               `{"version":1,"records":[null]}`,
		"trailing value":            originalDocument + ` {}`,
		"unterminated":              originalDocument[:len(originalDocument)-1],
		"duplicate records":         `{"version":1,"records":[` + originalRecord + `,` + originalRecord + `]}`,
		"unknown record":            strings.Replace(originalDocument, `"completed":false`, `"completed":false,"url":"private-sentinel"`, 1),
		"unknown replaces required": strings.Replace(originalDocument, `"completed":false`, `"shadow":false`, 1),
		"invalid UTF8":              strings.Replace(originalDocument, "Synthetic video", string([]byte{0xff}), 1),
	}
	fields := []string{`"video_id":"BV0123456789"`, `"part_id":"123"`, `"title":"Synthetic video"`, `"part_title":"Synthetic part"`, `"position_ms":1700`, `"duration_ms":120000`, `"updated_at":"2026-11-18T00:00:00Z"`, `"completed":false`}
	for _, field := range fields {
		key := strings.SplitN(field, ":", 2)[0]
		cases["null "+key] = strings.Replace(originalDocument, field, key+`:null`, 1)
		cases["duplicate "+key] = strings.Replace(originalDocument, field, field+`,`+field, 1)
		if strings.Contains(originalDocument, field+`,`) {
			cases["missing "+key] = strings.Replace(originalDocument, field+`,`, "", 1)
		} else {
			cases["missing "+key] = strings.Replace(originalDocument, `,`+field, "", 1)
		}
	}
	replacements := []struct{ name, old, replacement string }{
		{"empty ID", `"part_id":"123"`, `"part_id":""`},
		{"location in ID", `"part_id":"123"`, `"part_id":"https://invalid.example/sentinel"`},
		{"oversized ID", `"part_id":"123"`, `"part_id":"` + strings.Repeat("1", 257) + `"`},
		{"empty title", `"title":"Synthetic video"`, `"title":""`},
		{"oversized title", `"title":"Synthetic video"`, `"title":"` + strings.Repeat("a", 257) + `"`},
		{"control title", `"title":"Synthetic video"`, `"title":"hello\nworld"`},
		{"negative position", `"position_ms":1700`, `"position_ms":-1`},
		{"past duration", `"position_ms":1700`, `"position_ms":120001`},
		{"fractional ms", `"position_ms":1700`, `"position_ms":0.1`},
		{"overflow ms", `"duration_ms":120000`, `"duration_ms":9223372036854775808`},
		{"zero duration", `"duration_ms":120000`, `"duration_ms":0`},
		{"duration bound", `"duration_ms":120000`, `"duration_ms":604800001`},
		{"invalid timestamp", `2026-11-18T00:00:00Z`, `2026-02-30T00:00:00Z`},
		{"non-Z timestamp", `2026-11-18T00:00:00Z`, `2026-11-18T00:00:00+00:00`},
		{"zero timestamp", `2026-11-18T00:00:00Z`, `0001-01-01T00:00:00Z`},
		{"excess precision", `2026-11-18T00:00:00Z`, `2026-11-18T00:00:00.1234567891Z`},
		{"completed type", `"completed":false`, `"completed":0`},
	}
	for _, replacement := range replacements {
		cases[replacement.name] = strings.Replace(originalDocument, replacement.old, replacement.replacement, 1)
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			records, err := decode([]byte(input))
			if !errors.Is(err, history.Corrupt) || records != nil {
				t.Fatalf("accepted invalid document: %v", err)
			}
		})
	}
	if _, err := decode([]byte(`{"version":2,"records":[]}`)); !errors.Is(err, history.UnsupportedVersion) {
		t.Fatal(err)
	}
}

func TestDocumentSizeAndWholeLoadBeforeRetention(t *testing.T) {
	input := []byte(`{"version":1,"records":[]}`)
	exact := append(bytes.Clone(input), bytes.Repeat([]byte(" "), historyFileBytes-len(input))...)
	if _, err := decode(exact); err != nil {
		t.Fatal("exact file bound rejected", err)
	}
	if _, err := decode(append(exact, ' ')); !errors.Is(err, history.Corrupt) {
		t.Fatal("oversized document accepted")
	}
	var records []string
	for n := range 501 {
		records = append(records, strings.Replace(originalRecord, `"part_id":"123"`, fmt.Sprintf(`"part_id":"%d"`, n+1), 1))
	}
	large := `{"version":1,"records":[` + strings.Join(records, ",") + `]}`
	if got, err := decode([]byte(large)); err != nil || len(got) != 501 {
		t.Fatal("bounded valid load was pruned before manager acceptance", err)
	}
	records[500] = strings.Replace(records[500], `"position_ms":1700`, `"position_ms":-1`, 1)
	invalid := `{"version":1,"records":[` + strings.Join(records, ",") + `]}`
	if got, err := decode([]byte(invalid)); !errors.Is(err, history.Corrupt) || got != nil {
		t.Fatal("invalid late record was silently evicted")
	}
}

func TestEncodingBoundAndExactFields(t *testing.T) {
	if data, err := encode(nil); err != nil || string(bytes.TrimSpace(data)) != `{"version":1,"records":[]}` {
		t.Fatalf("empty encoding %s %v", data, err)
	}
	record := original("123")
	record.Title = "中文 <原始>"
	record.UpdatedAt = record.UpdatedAt.In(time.FixedZone("synthetic", 3600))
	data, err := encode([]history.Record{record})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decode(data)
	if err != nil || len(got) != 1 || !got[0].UpdatedAt.Equal(record.UpdatedAt) || got[0].Title != record.Title || got[0].UpdatedAt.Location() != time.UTC {
		t.Fatalf("roundtrip %+v %v", got, err)
	}
	for _, records := range [][]history.Record{{record, record}, make([]history.Record, 501), {{}}, {func() history.Record { r := record; r.Position = -time.Nanosecond; return r }()}} {
		if _, err := encode(records); !errors.Is(err, history.Invalid) {
			t.Fatal("invalid save accepted", err)
		}
	}
}
