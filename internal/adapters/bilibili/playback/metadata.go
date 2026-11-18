package playback

import (
	"encoding/json"
	"math"

	"my-bilibili/internal/catalog"
)

func metadataNumber(raw json.RawMessage) bool {
	value, ok := number(raw)
	return ok && value > 0 && value <= math.MaxInt32
}
func metadataText(raw json.RawMessage, limit int) bool {
	value, ok := text(raw)
	return ok && (value == "" || bounded(value, limit))
}

// Advertised quality data is validated and bounded, never used as evidence that
// a corresponding stream was returned or copied into presentation labels.
func metadata(quality, ids, labels, formats, support json.RawMessage) error {
	if len(quality) != 0 && !metadataNumber(quality) {
		return catalog.InvalidResponse
	}
	if len(formats) != 0 && !metadataText(formats, 4096) {
		return catalog.InvalidResponse
	}
	for _, field := range []struct {
		raw     json.RawMessage
		numeric bool
	}{{ids, true}, {labels, false}} {
		if len(field.raw) == 0 {
			continue
		}
		values, ok := array(field.raw, metadataLimit, true)
		if !ok {
			return catalog.InvalidResponse
		}
		for _, value := range values {
			if field.numeric && !metadataNumber(value) || !field.numeric && !metadataText(value, 256) {
				return catalog.InvalidResponse
			}
		}
	}
	if len(support) == 0 {
		return nil
	}
	rows, ok := array(support, metadataLimit, true)
	if !ok {
		return catalog.InvalidResponse
	}
	for _, raw := range rows {
		row, ok := object(raw)
		if !ok || !metadataNumber(row["quality"]) {
			return catalog.InvalidResponse
		}
		for _, label := range []json.RawMessage{row["format"], row["new_description"], row["display_desc"], row["superscript"]} {
			if len(label) != 0 && !metadataText(label, 256) {
				return catalog.InvalidResponse
			}
		}
		if len(row["codecs"]) != 0 {
			codecs, ok := array(row["codecs"], metadataLimit, true)
			if !ok {
				return catalog.InvalidResponse
			}
			for _, codec := range codecs {
				if !metadataText(codec, 128) {
					return catalog.InvalidResponse
				}
			}
		}
	}
	return nil
}
