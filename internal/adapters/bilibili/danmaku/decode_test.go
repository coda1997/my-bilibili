package danmaku

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	model "my-bilibili/internal/danmaku"
)

func num(tag, value uint64) []byte {
	return binary.AppendUvarint(binary.AppendUvarint(nil, tag<<3), value)
}

func blob(tag uint64, value []byte) []byte {
	data := binary.AppendUvarint(nil, tag<<3|2)
	data = binary.AppendUvarint(data, uint64(len(value)))
	return append(data, value...)
}

func message(fields ...[]byte) []byte {
	return bytes.Join(fields, nil)
}

func element(at, mode uint64, text string, extra ...[]byte) []byte {
	fields := [][]byte{num(2, at), num(3, mode), blob(7, []byte(text))}
	return message(append(fields, extra...)...)
}

func decodeOne(data []byte, index int) (model.Segment, error) {
	return decodeSegment(blob(1, data), index, 720*time.Second)
}

func TestAbsoluteSecondSegmentModesAndDefaults(t *testing.T) {
	var body []byte
	for mode := uint64(1); mode <= 9; mode++ {
		body = append(body, blob(1, element(361250, mode, "synthetic", num(1, mode)))...)
	}
	body = append(body, blob(1, element(361250, 77, "ignored"))...)
	body = append(body, blob(1, element(361250, 1, ""))...)
	body = append(body, blob(1, message(num(2, 361250), num(3, 8)))...)
	got, err := decodeSegment(body, 2, 720*time.Second)
	if err != nil || got.Index != 2 || got.Encoded != 12 || got.SkippedEmpty != 2 || got.Unsupported != 4 || len(got.Entries) != 6 {
		t.Fatal("ordinary/unsupported/empty counters", got.Encoded, got.SkippedEmpty, got.Unsupported, len(got.Entries), err)
	}
	for i, entry := range got.Entries {
		want := model.ScrollRTL
		if i >= 3 {
			want = model.Mode(i + 1)
		}
		if entry.Entry.At != 361250*time.Millisecond || entry.Entry.Mode != want || entry.Entry.Color != 0 {
			t.Fatal("absolute progress, mode normalization or scalar color default changed", entry.Entry.At, entry.Entry.Mode, entry.Entry.Color)
		}
	}
	zero := message(num(3, 1), blob(7, []byte("zero")), num(4, 36))
	first, err := decodeOne(zero, 1)
	if err != nil || len(first.Entries) != 1 || first.Entries[0].Entry.At != 0 {
		t.Fatal("omitted progress default lost", err)
	}
	for _, empty := range []bool{false, true} {
		data := zero
		if empty {
			data = num(3, 1)
		}
		if _, err := decodeOne(data, 2); err != catalog.InvalidResponse {
			t.Fatal("second-segment zero/default progress accepted before eligibility", err)
		}
	}
	for _, at := range []uint64{359999, 720001} {
		if _, err := decodeOne(element(at, 1, "outside"), 2); err != catalog.InvalidResponse {
			t.Fatal("entry outside requested inclusive interval accepted", err)
		}
	}
	if _, err := decodeSegment(blob(1, element(721001, 1, "past duration")), 3, 721*time.Second); err != catalog.InvalidResponse {
		t.Fatal("last interval exceeded part duration", err)
	}
}

func TestEmptyEligibilityAfterCompleteValidation(t *testing.T) {
	for _, data := range [][]byte{num(3, 1), message(num(3, 1), blob(7, nil)), num(3, 8)} {
		got, err := decodeOne(data, 1)
		if err != nil || got.Encoded != 1 || got.SkippedEmpty != 1 || got.Unsupported != 0 || len(got.Entries) != 0 {
			t.Fatal("valid empty entry rejected or counted as content", err)
		}
	}
	base := message(num(3, 1), blob(7, nil))
	for name, extra := range map[string][]byte{
		"late-key": {0x80}, "late-group": {11}, "late-zero-field": {0},
		"late-progress": num(2, 360001), "negative-progress": num(2, math.MaxUint64),
		"late-color": num(5, 0x1000000), "late-id-zero": num(1, 0),
		"late-id-conflict": message(num(1, 1), blob(12, []byte("2"))),
		"late-mode":        num(3, 0), "late-bad-sender-wire": num(6, 7),
		"late-utf8": blob(6, []byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeOne(message(base, extra), 1); err != catalog.InvalidResponse {
				t.Fatal("empty text bypassed a later malformed field", err)
			}
		})
	}
	for _, data := range [][]byte{nil, blob(7, nil), element(0, 0, ""), element(0, 1, "\x00"), element(0, 1, string([]byte{0xff}))} {
		if _, err := decodeOne(data, 1); err != catalog.InvalidResponse {
			t.Fatal("invalid mode/text became a valid skip", err)
		}
	}
	got, err := decodeOne(element(0, 1, " \t\n"), 1)
	if err != nil || got.Encoded != 1 || got.SkippedEmpty != 0 || len(got.Entries) != 1 || got.Entries[0].Entry.Text != "" {
		t.Fatal("wire-empty accounting was conflated with timeline whitespace normalization", err)
	}
}

func TestIdentityAgreementDedupeAndSenderIndependence(t *testing.T) {
	first := element(360000, 1, " a  b ", num(1, 101), blob(12, []byte("000101")))
	second := element(360000, 2, "a\tb", num(1, 101))
	got, err := decodeSegment(message(blob(1, first), blob(1, second)), 1, 720*time.Second)
	if err != nil || got.Encoded != 2 || len(got.Entries) != 1 || got.Entries[0].ID != "i:101" || got.Entries[0].Entry.Text != "a b" {
		t.Fatal("canonical presentation duplicate did not collapse", err)
	}
	for _, other := range [][]byte{
		element(359999, 1, "a b", num(1, 101)),
		element(360000, 4, "a b", num(1, 101)),
		element(360000, 1, "different", num(1, 101)),
		element(360000, 1, "a b", num(1, 101), num(5, 1)),
		element(360000, 1, "", num(1, 101)),
	} {
		if _, err := decodeSegment(message(blob(1, first), blob(1, other)), 1, 720*time.Second); err != catalog.InvalidResponse {
			t.Fatal("contradictory real identity was silently overwritten", err)
		}
	}
	for _, id := range []uint64{0, 1 << 63, math.MaxUint64} {
		if _, err := decodeOne(element(0, 1, "x", num(1, id)), 1); err != catalog.InvalidResponse {
			t.Fatal("nonpositive real identity accepted", err)
		}
	}
	for _, id := range []string{"", "0", "-1", "+1", "x", "9223372036854775808"} {
		if _, err := decodeOne(element(0, 1, "x", blob(12, []byte(id))), 1); err != catalog.InvalidResponse {
			t.Fatal("invalid string identity triggered fallback", err)
		}
	}
	if _, err := decodeOne(element(0, 1, "x", num(1, 1), blob(12, []byte("2"))), 1); err != catalog.InvalidResponse {
		t.Fatal("disagreeing id/idStr became a fallback", err)
	}
	var previous []model.SegmentEntry
	for _, index := range []int{1, 2} {
		for _, sender := range []string{"synthetic sender A", "synthetic sender B"} {
			entry := element(360000, 1, "boundary", blob(6, []byte(sender)), blob(10, []byte("ignored action")), blob(22, []byte("ignored animation")))
			result, err := decodeSegment(message(blob(1, entry), blob(1, entry)), index, 720*time.Second)
			if err != nil || len(result.Entries) != 2 || result.Entries[0].ID == result.Entries[1].ID || !strings.HasSuffix(result.Entries[0].ID, ":0") || !strings.HasSuffix(result.Entries[1].ID, ":1") {
				t.Fatal("fallback ordinal identity", err)
			}
			if previous != nil && !reflect.DeepEqual(previous, result.Entries) {
				t.Fatal("sender or segment index changed canonical fallback data")
			}
			previous = result.Entries
		}
	}
}

func TestWireAndRecognizedSingularFields(t *testing.T) {
	if _, err := decodeOne(element(0, 1<<31, "overflow mode"), 1); err != catalog.InvalidResponse {
		t.Fatal("out-of-width mode was counted as supported protocol", err)
	}
	for _, tag := range []uint64{4, 9, 11, 13, 24} {
		if _, err := decodeOne(element(0, 1, "x", num(tag, 1<<31)), 1); err != catalog.InvalidResponse {
			t.Fatal("ignored int32 field escaped structural width validation", tag, err)
		}
		if _, err := decodeOne(element(0, 1, "x", num(tag, math.MaxUint64)), 1); err != nil {
			t.Fatal("valid signed ignored field was assigned invented positive semantics", tag, err)
		}
	}
	for name, data := range map[string][]byte{
		"zero": {0, 0}, "field-overflow": binary.AppendUvarint(binary.AppendUvarint(nil, 1<<32), 0),
		"unterminated-key": {0x80}, "overflow-varint": bytes.Repeat([]byte{0xff}, 10),
		"tenth-byte":     append(bytes.Repeat([]byte{0x80}, 9), 2),
		"length-overrun": {10, 4, 1}, "length-varint": {10, 0x80},
		"entry-wire": num(1, 1), "group": {11}, "end-group": {12},
		"wire6": {14}, "wire7": {15}, "short64": {0x19, 1}, "short32": {0x1d, 1},
		"state-wire": blob(2, nil), "negative-state": num(2, math.MaxUint64),
		"state-width": num(2, 1<<31), "duplicate-state": message(num(2, 0), num(2, 0)),
		"duplicate-ai": message(blob(3, nil), blob(3, nil)), "colorful-wire": num(5, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeSegment(data, 1, time.Hour); err != catalog.InvalidResponse {
				t.Fatal("malformed wrapper accepted", err)
			}
		})
	}
	fields := map[uint64][]byte{
		1: num(1, 5), 2: num(2, 0), 3: num(3, 1), 4: num(4, 25), 5: num(5, 0),
		6: blob(6, []byte("ignored")), 7: blob(7, []byte("x")), 8: num(8, 0),
		9: num(9, 0), 10: blob(10, nil), 11: num(11, 0), 12: blob(12, []byte("5")),
		13: num(13, 0), 22: blob(22, nil), 24: num(24, 0),
	}
	for tag, value := range fields {
		prefix := num(3, 1)
		if tag == 3 {
			prefix = nil
		}
		if _, err := decodeOne(message(prefix, value, value), 1); err != catalog.InvalidResponse {
			t.Fatal("recognized singular field accepted twice", tag, err)
		}
	}
	unknown := message(num(40, 1), blob(41, []byte{0xff}), binary.AppendUvarint(nil, 42<<3|1), make([]byte, 8), binary.AppendUvarint(nil, 43<<3|5), make([]byte, 4))
	if _, err := decodeSegment(message(blob(1, message(element(0, 1, "x"), unknown)), unknown), 1, time.Hour); err != nil {
		t.Fatal("well-formed unknown fields were interpreted instead of skipped", err)
	}
	for _, state := range []uint64{1, 2} {
		want := catalog.Unavailable
		if state == 2 {
			want = catalog.Unsupported
		}
		if _, err := decodeSegment(message(blob(1, element(0, 1, "x")), num(2, state)), 1, time.Hour); err != want {
			t.Fatal("explicit segment denial became success", err)
		}
	}
}

func TestDecoderEntryTextCountAndOwnedTextBounds(t *testing.T) {
	for _, length := range []int{1024, 1025} {
		_, err := decodeOne(element(0, 1, strings.Repeat("x", length)), 1)
		if (err == nil) != (length == 1024) {
			t.Fatal("text-byte bound", length, err)
		}
	}
	base := element(0, 1, "x")
	for _, length := range []int{4096, 4097} {
		data := message(base, blob(99, bytes.Repeat([]byte{'p'}, length-len(base)-4)))
		if len(data) != length {
			t.Fatal("incorrect synthetic entry size")
		}
		_, err := decodeOne(data, 1)
		if (err == nil) != (length == 4096) {
			t.Fatal("entry-byte bound", length, err)
		}
	}
	empty := blob(1, num(3, 1))
	for _, count := range []int{20000, 20001} {
		got, err := decodeSegment(bytes.Repeat(empty, count), 1, time.Hour)
		if (err == nil) != (count == 20000) || err == nil && (got.Encoded != count || got.SkippedEmpty != count) {
			t.Fatal("skipped entries escaped encoded count", count, err)
		}
	}
	// Direct decoder input isolates the owned-text limit from the separately
	// tested smaller wire-body limit enforced by Scope.Protobuf.
	full := blob(1, element(0, 1, strings.Repeat("x", 1024)))
	for _, count := range []int{4096, 4097} {
		got, err := decodeSegment(bytes.Repeat(full, count), 1, time.Hour)
		if (err == nil) != (count == 4096) || err == nil && len(got.Entries) != count {
			t.Fatal("owned text bound", count, err)
		}
	}
}

func TestMetadataDefaultsDenialsAndWire(t *testing.T) {
	for _, test := range []struct {
		body []byte
		want model.Metadata
		err  error
	}{
		{nil, model.Metadata{Known: true, Available: true}, nil},
		{num(1, 0), model.Metadata{Known: true, Available: true}, nil},
		{num(1, 1), model.Metadata{Known: true}, nil},
		{num(1, 2), model.Metadata{}, catalog.Unsupported},
		{num(1, math.MaxUint64), model.Metadata{}, catalog.Unsupported},
		{num(1, 1<<31), model.Metadata{}, catalog.InvalidResponse},
		{blob(1, nil), model.Metadata{}, catalog.InvalidResponse},
		{message(num(1, 0), num(1, 0)), model.Metadata{}, catalog.InvalidResponse},
		{message(blob(4, nil), blob(4, nil)), model.Metadata{}, catalog.InvalidResponse},
		{num(6, 1), model.Metadata{}, catalog.InvalidResponse},
		{blob(2, []byte{0xff}), model.Metadata{}, catalog.InvalidResponse},
		{[]byte{0x80}, model.Metadata{}, catalog.InvalidResponse},
		{message(blob(4, num(2, 0)), num(8, 0), blob(6, []byte("https://ignored.invalid/special")), blob(6, []byte("https://ignored.invalid/other")), blob(9, []byte("ignored commands"))), model.Metadata{Known: true, Available: true}, nil},
	} {
		got, err := decodeMetadata(test.body)
		if got != test.want || err != test.err {
			t.Fatal("metadata interpretation", got, err)
		}
	}
}
