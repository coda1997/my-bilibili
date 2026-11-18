package catalog

import (
	"bytes"
	"encoding/json"
	"html"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	domain "my-bilibili/internal/catalog"
)

func object(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var value map[string]json.RawMessage
	err := json.Unmarshal(raw, &value)
	return value, err == nil && value != nil
}
func text(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err == nil
}
func number(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, false
	}
	var value int64
	err := json.Unmarshal(raw, &value)
	return value, err == nil
}
func boolean(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, false
	}
	var value bool
	err := json.Unmarshal(raw, &value)
	return value, err == nil
}
func array(raw json.RawMessage) ([]json.RawMessage, bool) {
	var values []json.RawMessage
	err := json.Unmarshal(raw, &values)
	return values, err == nil && values != nil
}

// Scope.JSON establishes full-envelope UTF-8, duplicate and nesting invariants.
// This adapter selects and validates feature fields without another JSON policy.
func dataObject(body []byte) (map[string]json.RawMessage, bool) {
	root, ok := object(body)
	if !ok {
		return nil, false
	}
	return object(root["data"])
}

func plain(raw json.RawMessage, minimum, maximum int, multiline, highlight bool) (string, bool) {
	value, ok := text(raw)
	if !ok {
		return "", false
	}
	if highlight {
		value = stripHighlights(value)
	}
	value = html.UnescapeString(value)
	if multiline {
		// The description is rendered as plain text. Preserve its words while
		// converting ordinary line formatting into spaces; other controls fail.
		value = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(value)
	}
	if !utf8.ValidString(value) || len(value) < minimum || len(value) > maximum ||
		minimum > 0 && strings.TrimSpace(value) == "" || strings.ContainsFunc(value, unicode.IsControl) {
		return "", false
	}
	return value, true
}

func stripHighlights(value string) string {
	const opening = "<em class=\"keyword\">"
	const closing = "</em>"
	var out strings.Builder
	for {
		start := strings.Index(value, opening)
		if start < 0 {
			out.WriteString(value)
			return out.String()
		}
		end := strings.Index(value[start+len(opening):], closing)
		if end < 0 {
			out.WriteString(value)
			return out.String()
		}
		end += start + len(opening)
		out.WriteString(value[:start])
		out.WriteString(value[start+len(opening) : end])
		value = value[end+len(closing):]
	}
}

func validBV(id string) bool {
	entry, err := domain.ParseEntry(id)
	return err == nil && entry.Kind == domain.BVEntry && entry.ID == id && entry.Page == 0
}

func durationText(raw json.RawMessage) (time.Duration, bool) {
	value, ok := text(raw)
	if !ok {
		return 0, false
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var seconds int64
	for i, part := range parts {
		if part == "" {
			return 0, false
		}
		for _, c := range []byte(part) {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil || v > 7*24*3600 || i > 0 && v >= 60 {
			return 0, false
		}
		seconds = seconds*60 + v
		if seconds > 7*24*3600 {
			return 0, false
		}
	}
	return time.Duration(seconds) * time.Second, seconds > 0
}

func decodeSearch(body []byte, expected int) (domain.Page, []string, error) {
	invalid := func() (domain.Page, []string, error) { return domain.Page{}, nil, domain.InvalidResponse }
	data, ok := dataObject(body)
	if !ok {
		return invalid()
	}
	page, pOK := number(data["page"])
	pages, pagesOK := number(data["numPages"])
	total, totalOK := number(data["numResults"])
	rows, rowsOK := array(data["result"])
	if !pOK || page != int64(expected) || !pagesOK || pages < 0 || pages > math.MaxInt32 || !totalOK || total < 0 || total > math.MaxInt32 || !rowsOK {
		return invalid()
	}
	result := domain.Page{Number: expected, TotalPages: int(pages), TotalResults: int(total), Videos: make([]domain.Summary, 0, len(rows))}
	locations := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, ok := object(raw)
		if !ok {
			return invalid()
		}
		kind, kOK := text(row["type"])
		id, idOK := text(row["bvid"])
		title, tOK := plain(row["title"], 1, 512, false, true)
		creator, cOK := plain(row["author"], 1, 256, false, false)
		duration, dOK := durationText(row["duration"])
		if !kOK || kind != "video" || !idOK || !validBV(id) || !tOK || !cOK || !dOK {
			return invalid()
		}
		location, _ := text(row["pic"])
		result.Videos = append(result.Videos, domain.Summary{ID: domain.VideoID(id), Title: title, Creator: creator, Duration: duration})
		locations = append(locations, location)
	}
	return result, locations, nil
}

func supported(data map[string]json.RawMessage) error {
	if raw, exists := data["redirect_url"]; exists {
		value, ok := text(raw)
		if !ok {
			return domain.InvalidResponse
		}
		if value != "" {
			return domain.Unsupported
		}
	}
	rights, ok := object(data["rights"])
	if !ok {
		return domain.InvalidResponse
	}
	for _, name := range []string{"movie", "pay", "ugc_pay", "is_stein_gate", "ugc_pay_preview", "arc_pay"} {
		value, ok := number(rights[name])
		if !ok || value < 0 {
			return domain.InvalidResponse
		}
		if value > 0 {
			return domain.Unsupported
		}
	}
	for _, name := range []string{"is_chargeable_season", "is_upower_exclusive", "is_upower_preview"} {
		value, ok := boolean(data[name])
		if !ok {
			return domain.InvalidResponse
		}
		if value {
			return domain.Unsupported
		}
	}
	return nil
}

func decodeDetail(body []byte, requested domain.Entry) (domain.Video, string, error) {
	invalid := func() (domain.Video, string, error) { return domain.Video{}, "", domain.InvalidResponse }
	data, ok := dataObject(body)
	if !ok {
		return invalid()
	}
	id, idOK := text(data["bvid"])
	aid, aidOK := number(data["aid"])
	if !idOK || !validBV(id) || !aidOK || aid <= 0 ||
		requested.Kind == domain.BVEntry && requested.ID != id || requested.Kind == domain.AVEntry && requested.ID != strconv.FormatInt(aid, 10) {
		return invalid()
	}
	if err := supported(data); err != nil {
		return domain.Video{}, "", err
	}
	title, tOK := plain(data["title"], 1, 512, false, false)
	description, dOK := plain(data["desc"], 0, 65536, true, false)
	owner, oOK := object(data["owner"])
	if !oOK {
		return invalid()
	}
	creator, cOK := plain(owner["name"], 1, 256, false, false)
	pages, pOK := array(data["pages"])
	if !tOK || !dOK || !cOK || !pOK {
		return invalid()
	}
	video := domain.Video{ID: domain.VideoID(id), Title: title, Creator: creator, Description: description, Parts: make([]domain.Part, 0, len(pages))}
	for i, raw := range pages {
		page, ok := object(raw)
		if !ok {
			return invalid()
		}
		cid, cOK := number(page["cid"])
		order, oOK := number(page["page"])
		duration, dOK := number(page["duration"])
		title, tOK := plain(page["part"], 1, 512, false, false)
		if !cOK || cid <= 0 || !oOK || order != int64(i+1) || !dOK || duration <= 0 || duration > 7*24*3600 || !tOK {
			return invalid()
		}
		video.Parts = append(video.Parts, domain.Part{ID: domain.PartID(strconv.FormatInt(cid, 10)), Title: title, Duration: time.Duration(duration) * time.Second})
	}
	if domain.ValidateVideo(video) != nil {
		return invalid()
	}
	if raw, exists := data["cid"]; exists {
		cid, ok := number(raw)
		if !ok || strconv.FormatInt(cid, 10) != string(video.Parts[0].ID) {
			return invalid()
		}
	}
	location, _ := text(data["pic"])
	return video, location, nil
}
