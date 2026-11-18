package playback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	player "my-bilibili/internal/playback"
)

const videoLimit = 64
const audioLimit = 32
const metadataLimit = 16
const bandwidthLimit = math.MaxInt32

var qualities = [...]struct {
	code  int64
	label string
}{
	{116, "1080P 60帧"}, {112, "1080P+"}, {80, "1080P"},
	{74, "720P 60帧"}, {64, "720P"}, {32, "480P"}, {16, "360P"},
}

func qualityCode(id app.QualityID) (int64, bool) {
	for _, quality := range qualities {
		if string(id) == strconv.FormatInt(quality.code, 10) {
			return quality.code, true
		}
	}
	return 0, false
}
func qualityLabel(code int64) (string, bool) {
	for _, quality := range qualities {
		if code == quality.code {
			return quality.label, true
		}
	}
	return "", false
}

type stream struct {
	fields                map[string]json.RawMessage
	id, bandwidth         int64
	codec, location, mime string
}
type rendition struct {
	quality   app.Quality
	location  string
	bandwidth int64
}

func array(raw json.RawMessage, limit int, nullable bool) ([]json.RawMessage, bool) {
	if nullable && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		return nil, false
	}
	var result []json.RawMessage
	for d.More() {
		if len(result) >= limit {
			return nil, false
		}
		var item json.RawMessage
		if d.Decode(&item) != nil {
			return nil, false
		}
		result = append(result, item)
	}
	if end, err := d.Token(); err != nil || end != json.Delim(']') {
		return nil, false
	}
	_, err = d.Token()
	return result, err == io.EOF
}
func text(raw json.RawMessage) (string, bool) {
	var value *string
	err := json.Unmarshal(raw, &value)
	if err != nil || value == nil {
		return "", false
	}
	return *value, true
}
func number(raw json.RawMessage) (int64, bool) {
	var value *int64
	err := json.Unmarshal(raw, &value)
	if err != nil || value == nil {
		return 0, false
	}
	return *value, true
}
func object(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	err := json.Unmarshal(raw, &fields)
	return fields, err == nil && fields != nil
}
func alias(first, second json.RawMessage) (string, bool) {
	var a, b string
	var ok bool
	if len(first) == 0 && len(second) == 0 {
		return "", false
	}
	if len(first) != 0 {
		if a, ok = text(first); !ok {
			return "", false
		}
	}
	if len(second) != 0 {
		if b, ok = text(second); !ok {
			return "", false
		}
	}
	if len(first) != 0 && len(second) != 0 && a != b {
		return "", false
	}
	if len(first) != 0 {
		return a, a != ""
	}
	return b, b != ""
}
func bounded(value string, limit int) bool {
	return value != "" && len(value) <= limit && !strings.ContainsFunc(value, unicode.IsControl)
}
func avc(codec string) bool {
	if len(codec) != 11 || !strings.HasPrefix(codec, "avc1.") {
		return false
	}
	for _, c := range []byte(codec[5:]) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func decodeStream(raw json.RawMessage) (stream, error) {
	fields, ok := object(raw)
	if !ok {
		return stream{}, catalog.InvalidResponse
	}
	id, idOK := number(fields["id"])
	bandwidth, bandwidthOK := number(fields["bandwidth"])
	codec, codecOK := text(fields["codecs"])
	location, locationOK := alias(fields["baseUrl"], fields["base_url"])
	mime, mimeOK := alias(fields["mimeType"], fields["mime_type"])
	if !idOK || id <= 0 || id > math.MaxInt32 || !bandwidthOK || bandwidth <= 0 || bandwidth > bandwidthLimit ||
		!codecOK || !bounded(codec, 128) || !locationOK || !mimeOK || !bounded(mime, 128) {
		return stream{}, catalog.InvalidResponse
	}
	return stream{fields: fields, id: id, bandwidth: bandwidth, codec: codec, location: location, mime: mime}, nil
}

// Scope.JSON has already established complete-envelope UTF-8/structure and
// safe success semantics. This function owns selected DASH fields and choices.
func decodeMedia(body []byte, requested app.QualityID) (app.Resolution, error) {
	root, ok := object(body)
	if !ok {
		return app.Resolution{}, catalog.InvalidResponse
	}
	data, ok := object(root["data"])
	if !ok {
		return app.Resolution{}, catalog.InvalidResponse
	}
	dash, ok := object(data["dash"])
	if !ok {
		return app.Resolution{}, catalog.InvalidResponse
	}
	if err := metadata(data["quality"], data["accept_quality"], data["accept_description"], data["accept_format"], data["support_formats"]); err != nil {
		return app.Resolution{}, err
	}
	videos, ok := array(dash["video"], videoLimit, false)
	if !ok {
		return app.Resolution{}, catalog.InvalidResponse
	}
	audios, ok := array(dash["audio"], audioLimit, true)
	if !ok {
		return app.Resolution{}, catalog.InvalidResponse
	}
	byID := make(map[int64]rendition)
	for _, raw := range videos {
		value, err := decodeStream(raw)
		if err != nil {
			return app.Resolution{}, err
		}
		codecid, codeOK := number(value.fields["codecid"])
		width, widthOK := number(value.fields["width"])
		height, heightOK := number(value.fields["height"])
		if !codeOK || codecid < 0 || codecid > math.MaxInt32 || !widthOK || !heightOK || width <= 0 || height <= 0 || width > math.MaxInt32 || height > math.MaxInt32 {
			return app.Resolution{}, catalog.InvalidResponse
		}
		label, supported := qualityLabel(value.id)
		if !supported || codecid != 7 || !avc(value.codec) || value.mime != "video/mp4" || width > 1920 || height > 1080 {
			continue
		}
		candidate := rendition{quality: app.Quality{ID: app.QualityID(strconv.FormatInt(value.id, 10)), Label: fmt.Sprintf("%s（%d×%d）", label, width, height), Width: int(width), Height: int(height)}, location: value.location, bandwidth: value.bandwidth}
		if prior, found := byID[value.id]; !found || candidate.bandwidth > prior.bandwidth {
			byID[value.id] = candidate
		}
	}
	available := make([]app.Quality, 0, len(byID))
	var chosen rendition
	for _, quality := range qualities {
		if candidate, found := byID[quality.code]; found {
			available = append(available, candidate.quality)
			if requested == candidate.quality.ID || requested == "" && chosen.quality.ID == "" {
				chosen = candidate
			}
		}
	}
	if chosen.quality.ID == "" {
		if requested != "" {
			return app.Resolution{}, catalog.QualityUnavailable
		}
		return app.Resolution{}, catalog.Unsupported
	}
	var audioURL string
	var audioBandwidth int64
	for _, raw := range audios {
		value, err := decodeStream(raw)
		if err != nil {
			return app.Resolution{}, err
		}
		id := value.id
		if (id == 30216 || id == 30232 || id == 30280) && value.codec == "mp4a.40.2" && value.mime == "audio/mp4" && value.bandwidth > audioBandwidth {
			audioURL, audioBandwidth = value.location, value.bandwidth
		}
	}
	if len(audios) > 0 && audioURL == "" {
		return app.Resolution{}, catalog.Unsupported
	}
	input := player.Input{URL: chosen.location, AudioURL: audioURL, Headers: map[string]string{"Referer": player.PublicReferer, "User-Agent": player.PublicUserAgent}}
	for _, candidate := range byID {
		checked := input
		checked.URL = candidate.location
		if (player.HTTPSPolicy{}).Validate(checked) != nil {
			return app.Resolution{}, catalog.InvalidResponse
		}
	}
	return app.Resolution{Input: input, Quality: chosen.quality, Qualities: available}, nil
}
