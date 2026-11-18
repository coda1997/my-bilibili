// Package catalog owns the framework-free video detail model.
package catalog

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type VideoID string
type PartID string

// Part identifies one playable section. Resource locations belong to adapters.
type Part struct {
	ID       PartID
	Title    string
	Duration time.Duration
}

// Video contains display data, with an opaque cover identity for presentation.
type Video struct {
	ID                          VideoID
	Title, Creator, Description string
	CoverID                     string
	Parts                       []Part
}

// ValidateVideo establishes the shared plain catalog invariants. Adapters also
// validate service-specific identity and part membership before conversion.
func ValidateVideo(video Video) error {
	if !nonemptyIdentity(string(video.ID)) || !boundedText(video.Title, 1, 512) ||
		!boundedText(video.Creator, 1, 256) || !boundedText(video.Description, 0, 65536) ||
		!plainText(video.CoverID) || len(video.Parts) == 0 || len(video.Parts) > 1000 {
		return Invalid
	}
	seen := make(map[PartID]bool, len(video.Parts))
	for _, part := range video.Parts {
		if !nonemptyIdentity(string(part.ID)) || seen[part.ID] || !boundedText(part.Title, 1, 512) ||
			part.Duration <= 0 || part.Duration > 7*24*time.Hour {
			return Invalid
		}
		seen[part.ID] = true
	}
	return nil
}

func nonemptyIdentity(text string) bool {
	return strings.TrimSpace(text) != "" && plainText(text)
}

func boundedText(text string, minimum, maximum int) bool {
	return len(text) >= minimum && len(text) <= maximum &&
		(minimum == 0 || strings.TrimSpace(text) != "") && plainText(text)
}

func plainText(text string) bool {
	return utf8.ValidString(text) && strings.IndexFunc(text, unicode.IsControl) == -1
}
