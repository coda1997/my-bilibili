package previewbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"unicode/utf8"

	"my-bilibili/internal/danmaku"
)

// LiveReceipt is the entire public output of the fixed, normal-binary readonly
// verification. It contains no identity, query, title, URL, text or free error.
type LiveReceipt struct {
	Version           int    `json:"version"`
	Event             string `json:"event"`
	Stage             string `json:"stage"`
	Result            string `json:"result"`
	Error             string `json:"error"`
	AccountRestored   bool   `json:"account_restored"`
	Stored            bool   `json:"stored"`
	SearchCount       int    `json:"search_count"`
	Parts             int    `json:"parts"`
	Qualities         int    `json:"qualities"`
	VideoTrack        bool   `json:"video_track"`
	AudioTrack        bool   `json:"audio_track"`
	PicturePresented  bool   `json:"picture_presented"`
	NativeGeneration  uint64 `json:"native_generation"`
	PositionMS        int64  `json:"position_ms"`
	DurationMS        int64  `json:"duration_ms"`
	DanmakuLoaded     bool   `json:"danmaku_loaded"`
	VisibleComments   int    `json:"visible_comments"`
	HistorySaved      bool   `json:"history_saved"`
	HistoryRestored   bool   `json:"history_restored"`
	SamePart          bool   `json:"same_part"`
	Resumed           bool   `json:"resumed"`
	APIRequests       int    `json:"api_requests"`
	BootstrapRequests int    `json:"bootstrap_requests"`
	CoverRequests     int    `json:"cover_requests"`
	MediaOpens        int    `json:"media_opens"`
	PendingOwners     int    `json:"pending_owners"`
	NativeCores       int    `json:"native_cores"`
	NativeRenderers   int    `json:"native_renderers"`
	NativeWorkers     int    `json:"native_workers"`
	ForcedCleanup     bool   `json:"forced_cleanup"`
}

const liveReceiptLimit = 4096
const liveReceiptCount = 16
const livePositionLimit = int64(7 * 24 * 60 * 60 * 1000)

func liveError(code string) bool {
	switch code {
	case "", "invalid-input", "unsupported", "canceled", "timeout", "network",
		"invalid-response", "unavailable", "restricted", "login-required",
		"session-expired", "verification-required", "rate-limited",
		"service-unavailable", "quality-unavailable", "closed",
		"invalid", "service-rejected", "expired", "storage", "missing", "busy",
		"invalid-history", "history-storage", "history-in-use",
		"history-lock-unsupported", "history-corrupt", "history-version",
		"history-durability-uncertain", "history-closed", "media-failure":
		return true
	}
	return false
}

func validLiveReceipt(r LiveReceipt) bool {
	if r.Version != 1 || !liveError(r.Error) {
		return false
	}
	switch r.Event {
	case "stage":
	case "closed":
		if r.Stage != "close" {
			return false
		}
	default:
		return false
	}
	switch r.Stage {
	case "restore", "search", "detail", "play", "danmaku", "history", "resume", "close":
	default:
		return false
	}
	switch r.Result {
	case "pass", "not-tested":
		if r.Error != "" || r.ForcedCleanup {
			return false
		}
	case "fail":
		if r.Error == "" {
			return false
		}
	default:
		return false
	}
	for _, bound := range []struct{ value, maximum int }{
		{r.SearchCount, 1000}, {r.Parts, 1000}, {r.Qualities, 16},
		{r.VisibleComments, danmaku.MaxLanes}, {r.APIRequests, 24},
		{r.BootstrapRequests, 2}, {r.CoverRequests, 4}, {r.MediaOpens, 4},
		{r.PendingOwners, 9}, {r.NativeCores, 1}, {r.NativeRenderers, 1}, {r.NativeWorkers, 2},
	} {
		if bound.value < 0 || bound.value > bound.maximum {
			return false
		}
	}
	if r.PositionMS < 0 || r.DurationMS < 0 || r.DurationMS > livePositionLimit || r.PositionMS > r.DurationMS {
		return false
	}
	if (r.VideoTrack || r.AudioTrack || r.PicturePresented) && r.NativeGeneration == 0 {
		return false
	}
	if r.Resumed && (!r.HistoryRestored || !r.SamePart) {
		return false
	}
	if r.Event == "closed" && r.Result != "fail" && (r.PendingOwners != 0 || r.NativeCores != 0 || r.NativeRenderers != 0 || r.NativeWorkers != 0 || r.VideoTrack || r.AudioTrack || r.PicturePresented || r.VisibleComments != 0) {
		return false
	}
	if r.Result != "pass" {
		return true
	}
	if !r.AccountRestored || !r.Stored {
		return false
	}
	switch r.Stage {
	case "detail":
		return r.Parts > 0
	case "play", "resume":
		if !r.VideoTrack || !r.PicturePresented || r.NativeGeneration == 0 || r.PositionMS <= 0 || r.DurationMS <= 0 || r.Qualities == 0 || r.NativeCores != 1 || r.NativeRenderers != 1 || r.NativeWorkers == 0 {
			return false
		}
		return r.Stage != "resume" || r.HistoryRestored && r.SamePart && r.Resumed
	case "danmaku":
		return r.DanmakuLoaded && r.VisibleComments > 0
	case "history", "close":
		return r.HistorySaved
	}
	return true
}

func ParseLiveReceipt(data []byte) (LiveReceipt, error) {
	invalid := errors.New("invalid live receipt")
	if len(data) == 0 || len(data) > liveReceiptLimit || !utf8.Valid(data) {
		return LiveReceipt{}, invalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if requiredValue(d, reflect.TypeFor[LiveReceipt]()) != nil {
		return LiveReceipt{}, invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return LiveReceipt{}, invalid
	}
	var r LiveReceipt
	if json.Unmarshal(data, &r) != nil || !validLiveReceipt(r) {
		return LiveReceipt{}, invalid
	}
	return r, nil
}

// ParseLiveReceipts preserves fail/not-tested results. A complete stream is not
// itself a passing verification; the consumer must inspect every stage result.
func ParseLiveReceipts(data []byte) ([]LiveReceipt, error) {
	invalid := errors.New("invalid live receipt stream")
	if len(data) == 0 || len(data) > liveReceiptLimit*liveReceiptCount || data[len(data)-1] != '\n' {
		return nil, invalid
	}
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	if len(lines) == 0 || len(lines) > liveReceiptCount {
		return nil, invalid
	}
	result := make([]LiveReceipt, 0, len(lines))
	failed := false
	for i, line := range lines {
		r, err := ParseLiveReceipt(line)
		if err != nil || (r.Event == "closed") != (i == len(lines)-1) {
			return nil, invalid
		}
		if failed && r.Event == "closed" && r.Result == "pass" {
			return nil, invalid
		}
		failed = failed || r.Result == "fail"
		if len(result) != 0 {
			prior := result[len(result)-1]
			if r.APIRequests < prior.APIRequests || r.BootstrapRequests < prior.BootstrapRequests || r.CoverRequests < prior.CoverRequests || r.MediaOpens < prior.MediaOpens {
				return nil, invalid
			}
		}
		result = append(result, r)
	}
	return result, nil
}
