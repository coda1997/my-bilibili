package serviceprobe

import "unicode/utf8"

const (
	entryLimit = 20000
	entryBytes = 4096
	textBytes  = 1024
)

type segmentCounts struct {
	total, supported, unsupported, skippedEmpty int
	unavailable                                 bool
}

// wireField reads one non-group protobuf field. The returned bytes alias the
// bounded response; no unknown field or entry content is retained or logged.
func wireField(b []byte) (field uint64, wire byte, number uint64, value, rest []byte, ok bool) {
	key, b, ok := varint(b)
	if !ok || key>>3 == 0 || key>>3 > (1<<29)-1 {
		return 0, 0, 0, nil, nil, false
	}
	field, wire = key>>3, byte(key&7)
	switch wire {
	case 0:
		number, b, ok = varint(b)
	case 1:
		if len(b) < 8 {
			return 0, 0, 0, nil, nil, false
		}
		b = b[8:]
	case 2:
		var n uint64
		n, b, ok = varint(b)
		if !ok || n > uint64(len(b)) {
			return 0, 0, 0, nil, nil, false
		}
		value, b = b[:int(n)], b[int(n):]
	case 5:
		if len(b) < 4 {
			return 0, 0, 0, nil, nil, false
		}
		b = b[4:]
	default:
		return 0, 0, 0, nil, nil, false
	}
	return field, wire, number, value, b, ok
}

func varint(b []byte) (uint64, []byte, bool) {
	var n uint64
	for i := 0; i < 10 && i < len(b); i++ {
		v := b[i]
		if i == 9 && v > 1 {
			return 0, nil, false
		}
		n |= uint64(v&127) << (7 * i)
		if v < 128 {
			return n, b[i+1:], true
		}
	}
	return 0, nil, false
}

func decodeSegment(b []byte) (segmentCounts, bool) {
	var counts segmentCounts
	for len(b) != 0 {
		field, wire, number, value, rest, ok := wireField(b)
		if !ok {
			return segmentCounts{}, false
		}
		b = rest
		switch field {
		case 1:
			if wire != 2 || len(value) > entryBytes || counts.total+counts.skippedEmpty == entryLimit {
				return segmentCounts{}, false
			}
			supported, empty, valid := decodeEntry(value)
			if !valid {
				return segmentCounts{}, false
			}
			if empty {
				counts.skippedEmpty++
				continue
			}
			counts.total++
			if supported {
				counts.supported++
			} else {
				counts.unsupported++
			}
		case 2:
			// Negative int32 values use a sign-extended ten-byte varint. The
			// round trip rejects values outside either valid signed range.
			state := int32(number)
			if wire != 0 || uint64(int64(state)) != number {
				return segmentCounts{}, false
			}
			counts.unavailable = state != 0
		}
	}
	return counts, true
}

func decodeEntry(b []byte) (supported, empty, valid bool) {
	var mode uint64
	var text []byte
	for len(b) != 0 {
		field, wire, number, value, rest, ok := wireField(b)
		if !ok {
			return false, false, false
		}
		b = rest
		switch field {
		case 2:
			// Omitted progress has protobuf scalar default zero.
			if wire != 0 || number > 360000 {
				return false, false, false
			}
		case 3:
			if wire != 0 || number > (1<<31)-1 {
				return false, false, false
			}
			mode = number
		case 7:
			if wire != 2 || len(value) > textBytes || !utf8.Valid(value) {
				return false, false, false
			}
			text = value
		}
	}
	if mode == 0 {
		return false, false, false
	}
	// Eligibility is decided only after every field and the mode are valid.
	if len(text) == 0 {
		return false, true, true
	}
	return mode == 1 || mode == 4 || mode == 5 || mode == 6, false, true
}
