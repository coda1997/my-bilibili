package danmaku

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	model "my-bilibili/internal/danmaku"
)

const entryBytes = 4096

func decodeMetadata(data []byte) (model.Metadata, error) {
	var seen uint32
	var state uint64
	for len(data) != 0 {
		f, rest, err := next(data)
		if err != nil {
			return model.Metadata{}, err
		}
		data = rest
		switch f.number {
		case 1, 7, 8:
			if singular(&seen, f, 0) != nil {
				return model.Metadata{}, catalog.InvalidResponse
			}
			if f.number == 1 {
				if !signed32(f.value) {
					return model.Metadata{}, catalog.InvalidResponse
				}
				state = f.value
			}
		case 2, 3, 4, 5, 10:
			if singular(&seen, f, 2) != nil || (f.number == 2 || f.number == 3) && !utf8.Valid(f.bytes) {
				return model.Metadata{}, catalog.InvalidResponse
			}
		case 6, 9:
			if f.kind != 2 || f.number == 6 && !utf8.Valid(f.bytes) {
				return model.Metadata{}, catalog.InvalidResponse
			}
		}
	}
	switch state {
	case 0:
		return model.Metadata{Known: true, Available: true}, nil
	case 1:
		return model.Metadata{Known: true}, nil
	default:
		return model.Metadata{}, catalog.Unsupported
	}
}

type decodedEntry struct {
	entry       model.Entry
	id          uint64
	fingerprint [32]byte
	empty       bool
	unsupported bool
}

// Scope.Protobuf already limits the body. This independent converter bounds
// each entry and all retained output; it never decodes special packages.
func decodeSegment(data []byte, index int, duration time.Duration) (model.Segment, error) {
	segment := model.Segment{Index: index, Entries: []model.SegmentEntry{}}
	var seen uint32
	var state uint64
	identities := make(map[uint64][32]byte)
	ordinals := make(map[[32]byte]int)
	textBytes := 0
	for len(data) != 0 {
		f, rest, err := next(data)
		if err != nil {
			return model.Segment{}, err
		}
		data = rest
		switch f.number {
		case 1:
			if f.kind != 2 || len(f.bytes) > entryBytes || segment.Encoded == model.MaxSegmentEntries {
				return model.Segment{}, catalog.InvalidResponse
			}
			segment.Encoded++
			entry, err := decodeEntry(f.bytes, index, duration)
			if err != nil {
				return model.Segment{}, err
			}
			duplicate := false
			if entry.id != 0 {
				previous, exists := identities[entry.id]
				if exists && previous != entry.fingerprint {
					return model.Segment{}, catalog.InvalidResponse
				}
				identities[entry.id], duplicate = entry.fingerprint, exists
			}
			if entry.empty {
				segment.SkippedEmpty++
				continue
			}
			if entry.unsupported {
				segment.Unsupported++
				continue
			}
			if duplicate {
				continue
			}
			id := "i:" + strconv.FormatUint(entry.id, 10)
			if entry.id == 0 {
				ordinal := ordinals[entry.fingerprint]
				ordinals[entry.fingerprint] = ordinal + 1
				id = "h:" + hex.EncodeToString(entry.fingerprint[:]) + ":" + strconv.Itoa(ordinal)
			}
			textBytes += len(entry.entry.Text)
			if textBytes > model.MaxSegmentTextBytes {
				return model.Segment{}, catalog.InvalidResponse
			}
			segment.Entries = append(segment.Entries, model.SegmentEntry{ID: id, Entry: entry.entry})
		case 2:
			if singular(&seen, f, 0) != nil || !signed32(f.value) || f.value > math.MaxInt32 {
				return model.Segment{}, catalog.InvalidResponse
			}
			state = f.value
		case 3:
			if singular(&seen, f, 2) != nil {
				return model.Segment{}, catalog.InvalidResponse
			}
		case 5:
			if f.kind != 2 {
				return model.Segment{}, catalog.InvalidResponse
			}
		}
	}
	if state == 1 {
		return model.Segment{}, catalog.Unavailable
	}
	if state != 0 {
		return model.Segment{}, catalog.Unsupported
	}
	return segment, nil
}

func entryWire(number uint32) (byte, bool) {
	switch number {
	case 1, 2, 3, 4, 5, 8, 9, 11, 13, 24:
		return 0, true
	case 6, 7, 10, 12, 22:
		return 2, true
	default:
		return 0, false
	}
}

func decodeEntry(data []byte, index int, duration time.Duration) (decodedEntry, error) {
	var result decodedEntry
	var seen uint32
	var progress, mode, color, idString uint64
	var content []byte
	for len(data) != 0 {
		f, rest, err := next(data)
		if err != nil {
			return result, err
		}
		data = rest
		kind, known := entryWire(f.number)
		if !known {
			continue
		}
		if singular(&seen, f, kind) != nil || kind == 2 && !utf8.Valid(f.bytes) {
			return result, catalog.InvalidResponse
		}
		switch f.number {
		case 1:
			if f.value == 0 || f.value > math.MaxInt64 {
				return result, catalog.InvalidResponse
			}
			result.id = f.value
		case 2, 3:
			if f.value > math.MaxInt32 {
				return result, catalog.InvalidResponse
			}
			if f.number == 2 {
				progress = f.value
			} else {
				mode = f.value
			}
		case 4, 9, 11, 13, 24:
			if !signed32(f.value) {
				return result, catalog.InvalidResponse
			}
		case 5:
			if f.value > 0xffffff {
				return result, catalog.InvalidResponse
			}
			color = f.value
		case 7:
			if len(f.bytes) > model.MaxTextBytes {
				return result, catalog.InvalidResponse
			}
			content = f.bytes
		case 12:
			idString, err = strconv.ParseUint(string(f.bytes), 10, 63)
			if err != nil || idString == 0 {
				return result, catalog.InvalidResponse
			}
		}
	}
	at := time.Duration(progress) * time.Millisecond
	start, end := time.Duration(index-1)*model.SegmentDuration, min(time.Duration(index)*model.SegmentDuration, duration)
	if mode == 0 || at < start || at > end || result.id != 0 && idString != 0 && result.id != idString {
		return result, catalog.InvalidResponse
	}
	if result.id == 0 {
		result.id = idString
	}
	for _, r := range string(content) {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return result, catalog.InvalidResponse
		}
	}
	result.empty = len(content) == 0
	normalizedMode := mode
	switch mode {
	case 1, 2, 3:
		normalizedMode = uint64(model.ScrollRTL)
	case 4, 5, 6:
	default:
		result.unsupported = true
	}
	text := strings.Join(strings.Fields(string(content)), " ")
	// Hash only canonical presentation. Sender, source segment and raw response
	// bytes are deliberately absent, including for fallback boundary identities.
	key := binary.LittleEndian.AppendUint64(nil, uint64(at))
	key = binary.LittleEndian.AppendUint64(key, normalizedMode)
	key = binary.LittleEndian.AppendUint32(key, uint32(color))
	key = append(key, text...)
	result.fingerprint = sha256.Sum256(key)
	if !result.unsupported {
		result.entry = model.Entry{At: at, Mode: model.Mode(normalizedMode), Text: text, Color: uint32(color)}
	}
	return result, nil
}
