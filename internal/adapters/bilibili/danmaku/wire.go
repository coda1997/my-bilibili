package danmaku

import (
	"encoding/binary"

	"my-bilibili/internal/catalog"
)

type field struct {
	number uint32
	kind   byte
	value  uint64
	bytes  []byte
}

// next consumes a single non-group protobuf field. Its byte slice is only a
// temporary view of the already body-bounded transport response.
func next(data []byte) (field, []byte, error) {
	key, size := binary.Uvarint(data)
	if size <= 0 || key>>3 == 0 || key>>3 >= 1<<29 {
		return field{}, nil, catalog.InvalidResponse
	}
	data = data[size:]
	f := field{number: uint32(key >> 3), kind: byte(key & 7)}
	switch f.kind {
	case 0:
		f.value, size = binary.Uvarint(data)
		if size <= 0 {
			return field{}, nil, catalog.InvalidResponse
		}
		data = data[size:]
	case 1, 5:
		size = 8
		if f.kind == 5 {
			size = 4
		}
		if len(data) < size {
			return field{}, nil, catalog.InvalidResponse
		}
		data = data[size:]
	case 2:
		length, used := binary.Uvarint(data)
		if used <= 0 || length > uint64(len(data)-used) {
			return field{}, nil, catalog.InvalidResponse
		}
		data = data[used:]
		f.bytes, data = data[:int(length)], data[int(length):]
	default:
		return field{}, nil, catalog.InvalidResponse
	}
	return f, data, nil
}

func signed32(value uint64) bool { return uint64(int64(int32(value))) == value }

func singular(seen *uint32, f field, kind byte) error {
	bit := uint32(1) << f.number
	if f.kind != kind || *seen&bit != 0 {
		return catalog.InvalidResponse
	}
	*seen |= bit
	return nil
}
