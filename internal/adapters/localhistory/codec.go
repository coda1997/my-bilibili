package localhistory

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

type diskRecord struct {
	VideoID    string `json:"video_id"`
	PartID     string `json:"part_id"`
	Title      string `json:"title"`
	PartTitle  string `json:"part_title"`
	PositionMS int64  `json:"position_ms"`
	DurationMS int64  `json:"duration_ms"`
	UpdatedAt  string `json:"updated_at"`
	Completed  bool   `json:"completed"`
}

func identifier(text string) bool {
	if text == "" || len(text) > 256 {
		return false
	}
	for _, c := range text {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func title(text string) bool {
	return len(text) <= 256 && strings.TrimSpace(text) != "" && utf8.ValidString(text) && !strings.ContainsFunc(text, unicode.IsControl)
}

func (record diskRecord) record() (history.Record, error) {
	when, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	if err != nil || when.IsZero() || !strings.HasSuffix(record.UpdatedAt, "Z") || when.Format(time.RFC3339Nano) != record.UpdatedAt ||
		!identifier(record.VideoID) || !identifier(record.PartID) || !title(record.Title) || !title(record.PartTitle) ||
		record.DurationMS <= 0 || record.DurationMS > int64((7*24*time.Hour)/time.Millisecond) ||
		record.PositionMS < 0 || record.PositionMS > record.DurationMS {
		return history.Record{}, history.Corrupt
	}
	return history.Record{
		Ref:   catalog.PartRef{Video: catalog.VideoID(record.VideoID), Part: catalog.PartID(record.PartID)},
		Title: record.Title, PartTitle: record.PartTitle, Position: time.Duration(record.PositionMS) * time.Millisecond,
		Duration: time.Duration(record.DurationMS) * time.Millisecond, UpdatedAt: when, Completed: record.Completed,
	}, nil
}

func object(data []byte, fields ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, history.Corrupt
	}
	values := make(map[string]json.RawMessage, len(fields))
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
	}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || values[key] != nil {
			return nil, history.Corrupt
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, history.Corrupt
		}
		values[key] = value
	}
	end, err := decoder.Token()
	var extra json.RawMessage
	if err != nil || end != json.Delim('}') || decoder.Decode(&extra) != io.EOF || len(values) != len(fields) {
		return nil, history.Corrupt
	}
	return values, nil
}

func decode(data []byte) ([]history.Record, error) {
	if len(data) > fileLimit || !utf8.Valid(data) {
		return nil, history.Corrupt
	}
	root, err := object(data, "version", "records")
	if err != nil {
		return nil, err
	}
	var version int
	if json.Unmarshal(root["version"], &version) != nil {
		return nil, history.Corrupt
	}
	if version != 1 {
		return nil, history.UnsupportedVersion
	}
	decoder := json.NewDecoder(bytes.NewReader(root["records"]))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return nil, history.Corrupt
	}
	records := []history.Record{}
	seen := make(map[catalog.PartRef]bool)
	for decoder.More() {
		var data json.RawMessage
		if decoder.Decode(&data) != nil {
			return nil, history.Corrupt
		}
		if _, err := object(data, "video_id", "part_id", "title", "part_title", "position_ms", "duration_ms", "updated_at", "completed"); err != nil {
			return nil, err
		}
		var disk diskRecord
		if json.Unmarshal(data, &disk) != nil {
			return nil, history.Corrupt
		}
		record, err := disk.record()
		if err != nil || seen[record.Ref] {
			return nil, history.Corrupt
		}
		seen[record.Ref] = true
		records = append(records, record)
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') {
		return nil, history.Corrupt
	}
	return records, nil
}

func encode(records []history.Record) ([]byte, error) {
	if len(records) > 500 {
		return nil, history.Invalid
	}
	document := struct {
		Version int          `json:"version"`
		Records []diskRecord `json:"records"`
	}{Version: 1, Records: make([]diskRecord, 0, len(records))}
	seen := make(map[catalog.PartRef]bool, len(records))
	for _, record := range records {
		disk := diskRecord{
			VideoID: string(record.Ref.Video), PartID: string(record.Ref.Part), Title: record.Title, PartTitle: record.PartTitle,
			PositionMS: record.Position.Milliseconds(), DurationMS: record.Duration.Milliseconds(),
			UpdatedAt: record.UpdatedAt.UTC().Format(time.RFC3339Nano), Completed: record.Completed,
		}
		if _, err := disk.record(); err != nil || seen[record.Ref] || record.Position < 0 || record.Position > record.Duration {
			return nil, history.Invalid
		}
		seen[record.Ref] = true
		document.Records = append(document.Records, disk)
	}
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(document) != nil || data.Len() > fileLimit {
		return nil, history.Invalid
	}
	return data.Bytes(), nil
}
