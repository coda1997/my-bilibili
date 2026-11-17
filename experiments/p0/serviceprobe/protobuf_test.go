package serviceprobe

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func fieldNumber(field, value uint64) []byte {
	b := binary.AppendUvarint(nil, field<<3)
	return binary.AppendUvarint(b, value)
}
func fieldBytes(field uint64, value []byte) []byte {
	b := binary.AppendUvarint(nil, field<<3|2)
	b = binary.AppendUvarint(b, uint64(len(value)))
	return append(b, value...)
}
func entry(progress, mode uint64, text []byte) []byte {
	b := fieldNumber(2, progress)
	b = append(b, fieldNumber(3, mode)...)
	return append(b, fieldBytes(7, text)...)
}

func TestProtobufCountsModesAndScalarDefaults(t *testing.T) {
	var segment []byte
	for _, mode := range []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9} {
		segment = append(segment, fieldBytes(1, entry(360000, mode, []byte("中文测试")))...)
	}
	// Omitted scalar progress means zero. Unknown fixed64/bytes/fixed32/varint
	// fields are skipped at both levels, with no text interpretation.
	omitted := append(fieldNumber(3, 1), fieldBytes(7, []byte("zero"))...)
	unknown := append(fieldNumber(40, 100), fieldBytes(41, []byte("unknown"))...)
	unknown = append(unknown, binary.AppendUvarint(nil, 42<<3|1)...)
	unknown = append(unknown, make([]byte, 8)...)
	unknown = append(unknown, binary.AppendUvarint(nil, 43<<3|5)...)
	unknown = append(unknown, make([]byte, 4)...)
	omitted = append(omitted, unknown...)
	segment = append(segment, fieldBytes(1, omitted)...)
	segment = append(segment, unknown...)
	c, ok := decodeSegment(segment)
	if !ok || c.total != 10 || c.supported != 5 || c.unsupported != 5 || c.unavailable {
		t.Fatalf("incorrect decoded counts: %+v %v", c, ok)
	}
}

func TestMalformedProtobufRejected(t *testing.T) {
	tests := map[string][]byte{
		"zero-field": {0}, "overflow-field": binary.AppendUvarint(nil, 1<<32),
		"truncated-key": {0x80}, "overflow-varint": bytes.Repeat([]byte{0xff}, 10),
		"overflow-tenth-byte": append(bytes.Repeat([]byte{0x80}, 9), 2),
		"truncated-length":    {10, 0x80}, "length-overrun": {10, 5, 1},
		"bad-entry-wire": fieldNumber(1, 1), "group": {11}, "end-group": {12}, "wire6": {14}, "wire7": {15},
		"truncated-fixed64": {0x19, 1}, "truncated-fixed32": {0x1d, 1},
		"state-wire": fieldBytes(2, []byte{1}), "state-overflow": fieldNumber(2, 1<<31),
		"empty-entry":       fieldBytes(1, nil),
		"missing-mode":      fieldBytes(1, fieldBytes(7, []byte("text"))),
		"zero-mode":         fieldBytes(1, entry(0, 0, []byte("text"))),
		"mode-overflow":     fieldBytes(1, entry(0, 1<<31, []byte("text"))),
		"out-of-segment":    fieldBytes(1, entry(360001, 1, []byte("text"))),
		"negative-progress": fieldBytes(1, entry(^uint64(0), 1, []byte("text"))),
		"invalid-utf8":      fieldBytes(1, entry(0, 1, []byte{0xff})),
		"progress-wire":     fieldBytes(1, append(fieldBytes(2, nil), entry(0, 1, []byte("text"))...)),
		"mode-wire":         fieldBytes(1, append(fieldBytes(3, nil), entry(0, 1, []byte("text"))...)),
		"text-wire":         fieldBytes(1, append(fieldNumber(7, 1), entry(0, 1, []byte("text"))...)),
		"entry-malformed":   fieldBytes(1, []byte{0xff}),
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) {
			if _, ok := decodeSegment(b); ok {
				t.Fatal("malformed protobuf accepted")
			}
		})
	}
}

func TestProtobufResourceBoundaries(t *testing.T) {
	for _, n := range []int{textBytes, textBytes + 1} {
		_, ok := decodeSegment(fieldBytes(1, entry(0, 1, bytes.Repeat([]byte("a"), n))))
		if ok != (n == textBytes) {
			t.Fatal("text bound incorrect")
		}
	}
	base := entry(0, 1, []byte("a"))
	for _, n := range []int{entryBytes, entryBytes + 1} {
		padding := n - len(base) - 4
		b := append(append([]byte(nil), base...), fieldBytes(99, bytes.Repeat([]byte("p"), padding))...)
		if len(b) != n {
			t.Fatal("test fixture entry size")
		}
		_, ok := decodeSegment(fieldBytes(1, b))
		if ok != (n == entryBytes) {
			t.Fatal("entry-byte bound incorrect")
		}
	}
	for _, n := range []int{entryLimit, entryLimit + 1} {
		_, ok := decodeSegment(bytes.Repeat(fieldBytes(1, base), n))
		if ok != (n == entryLimit) {
			t.Fatal("entry-count bound incorrect")
		}
	}
}
