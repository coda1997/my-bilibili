package previewbundle

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func liveTestReceipt(stage string) LiveReceipt {
	r := LiveReceipt{Version: 1, Event: "stage", Stage: stage, Result: "pass", AccountRestored: true, Stored: true, SearchCount: 1, Parts: 2, Qualities: 2, VideoTrack: true, AudioTrack: true, PicturePresented: true, NativeGeneration: 1, PositionMS: 1200, DurationMS: 30000, DanmakuLoaded: true, VisibleComments: 1, HistorySaved: true, APIRequests: 8, BootstrapRequests: 1, CoverRequests: 2, MediaOpens: 1, NativeCores: 1, NativeRenderers: 1, NativeWorkers: 2}
	if stage == "resume" {
		r.HistoryRestored, r.SamePart, r.Resumed = true, true, true
	}
	if stage == "close" {
		r.Event = "closed"
		r.VideoTrack, r.AudioTrack, r.PicturePresented = false, false, false
		r.NativeCores, r.NativeRenderers, r.NativeWorkers, r.VisibleComments = 0, 0, 0, 0
	}
	return r
}
func liveTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal("synthetic receipt encoding failed")
	}
	return data
}
func liveTestReject(t *testing.T, data []byte) {
	t.Helper()
	got, err := ParseLiveReceipt(data)
	if err == nil || got != (LiveReceipt{}) || err.Error() != "invalid live receipt" || strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
		t.Fatal("malformed receipt accepted or raw error exposed")
	}
}

func TestLiveReceiptExactSchemaAndSafeTypes(t *testing.T) {
	want := liveTestReceipt("play")
	data := liveTestJSON(t, want)
	got, err := ParseLiveReceipt(data)
	if err != nil || got != want {
		t.Fatal("complete receipt did not round trip")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		t.Fatal("synthetic field map failed")
	}
	for name, raw := range fields {
		t.Run(name, func(t *testing.T) {
			copyFields := func() map[string]json.RawMessage {
				result := make(map[string]json.RawMessage, len(fields))
				for k, v := range fields {
					result[k] = v
				}
				return result
			}
			missing := copyFields()
			delete(missing, name)
			liveTestReject(t, liveTestJSON(t, missing))
			null := copyFields()
			null[name] = json.RawMessage("null")
			liveTestReject(t, liveTestJSON(t, null))
			alias := copyFields()
			delete(alias, name)
			alias[strings.ToUpper(name)] = raw
			liveTestReject(t, liveTestJSON(t, alias))
			duplicate := append(append([]byte(nil), data[:len(data)-1]...), []byte(",\""+name+"\":"+string(raw)+"}")...)
			liveTestReject(t, duplicate)
			wrong := copyFields()
			wrong[name] = json.RawMessage(`{"PRIVATE_SENTINEL":true}`)
			liveTestReject(t, liveTestJSON(t, wrong))
		})
	}
	for _, extra := range []string{"query", "video_id", "url", "message", "cookie"} {
		bad := append(append([]byte(nil), data[:len(data)-1]...), []byte(",\""+extra+"\":\"PRIVATE_SENTINEL\"}")...)
		liveTestReject(t, bad)
	}
	liveTestReject(t, append(append([]byte(nil), data...), data...))
	liveTestReject(t, append([]byte{0xff}, data...))
	liveTestReject(t, append(bytes.Repeat([]byte{' '}, 4097), data...))
	if bytes.Contains(data, []byte("video_id")) || bytes.Contains(data, []byte("query")) {
		t.Fatal("receipt schema exposed input identity")
	}
}

func TestLiveReceiptSemanticBoundsAndPictureGate(t *testing.T) {
	for _, mutate := range []func(*LiveReceipt){
		func(r *LiveReceipt) { r.Version = 2 }, func(r *LiveReceipt) { r.Event = "other" }, func(r *LiveReceipt) { r.Stage = "PRIVATE_SENTINEL" }, func(r *LiveReceipt) { r.Result = "ok" }, func(r *LiveReceipt) { r.Error = "PRIVATE_SENTINEL" },
		func(r *LiveReceipt) { r.AccountRestored = false }, func(r *LiveReceipt) { r.Stored = false }, func(r *LiveReceipt) { r.VideoTrack = false }, func(r *LiveReceipt) { r.PicturePresented = false }, func(r *LiveReceipt) { r.NativeGeneration = 0 }, func(r *LiveReceipt) { r.PositionMS = 0 }, func(r *LiveReceipt) { r.PositionMS = r.DurationMS + 1 }, func(r *LiveReceipt) { r.DurationMS = livePositionLimit + 1 }, func(r *LiveReceipt) { r.Qualities = 0 }, func(r *LiveReceipt) { r.NativeCores = 0 }, func(r *LiveReceipt) { r.NativeRenderers = 0 }, func(r *LiveReceipt) { r.NativeWorkers = 0 }, func(r *LiveReceipt) { r.ForcedCleanup = true }, func(r *LiveReceipt) { r.Result = "fail" },
	} {
		r := liveTestReceipt("play")
		mutate(&r)
		liveTestReject(t, liveTestJSON(t, r))
	}
	for _, bound := range []struct {
		name    string
		maximum int64
	}{{"SearchCount", 1000}, {"Parts", 1000}, {"Qualities", 16}, {"VisibleComments", 16}, {"APIRequests", 24}, {"BootstrapRequests", 2}, {"CoverRequests", 4}, {"MediaOpens", 4}, {"PendingOwners", 9}, {"NativeCores", 1}, {"NativeRenderers", 1}, {"NativeWorkers", 2}, {"PositionMS", livePositionLimit}, {"DurationMS", livePositionLimit}} {
		for _, number := range []int64{-1, bound.maximum + 1} {
			r := liveTestReceipt("play")
			reflect.ValueOf(&r).Elem().FieldByName(bound.name).SetInt(number)
			liveTestReject(t, liveTestJSON(t, r))
		}
	}
	for _, stage := range []string{"restore", "search", "detail", "play", "danmaku", "history", "resume", "close"} {
		r := liveTestReceipt(stage)
		if _, err := ParseLiveReceipt(liveTestJSON(t, r)); err != nil {
			t.Fatal("valid fixed stage rejected")
		}
	}
	for _, mutate := range []func(*LiveReceipt){func(r *LiveReceipt) { r.HistoryRestored = false }, func(r *LiveReceipt) { r.SamePart = false }, func(r *LiveReceipt) { r.Resumed = false }, func(r *LiveReceipt) { r.PicturePresented = false }} {
		r := liveTestReceipt("resume")
		mutate(&r)
		liveTestReject(t, liveTestJSON(t, r))
	}
	for _, mutate := range []func(*LiveReceipt){func(r *LiveReceipt) { r.PendingOwners = 1 }, func(r *LiveReceipt) { r.NativeCores = 1 }, func(r *LiveReceipt) { r.NativeRenderers = 1 }, func(r *LiveReceipt) { r.NativeWorkers = 1 }, func(r *LiveReceipt) { r.VideoTrack = true }, func(r *LiveReceipt) { r.AudioTrack = true }, func(r *LiveReceipt) { r.PicturePresented = true }, func(r *LiveReceipt) { r.VisibleComments = 1 }, func(r *LiveReceipt) { r.HistorySaved = false }} {
		r := liveTestReceipt("close")
		mutate(&r)
		liveTestReject(t, liveTestJSON(t, r))
	}
	r := liveTestReceipt("play")
	r.AudioTrack = false
	if _, err := ParseLiveReceipt(liveTestJSON(t, r)); err != nil {
		t.Fatal("truthful silent video rejected")
	}
	for _, code := range []string{"restricted", "storage", "history-storage", "history-in-use", "history-lock-unsupported", "history-durability-uncertain", "media-failure", "canceled", "timeout"} {
		r := liveTestReceipt("close")
		r.Result, r.Error, r.ForcedCleanup, r.PendingOwners = "fail", code, true, 1
		if _, err := ParseLiveReceipt(liveTestJSON(t, r)); err != nil {
			t.Fatal("truthful failure/positive cleanup ownership rejected")
		}
	}
}

func TestLiveReceiptStreamIsBoundedCompleteAndPreservesFailure(t *testing.T) {
	line := func(r LiveReceipt) []byte { return append(liveTestJSON(t, r), '\n') }
	play, closed := liveTestReceipt("play"), liveTestReceipt("close")
	valid := append(line(play), line(closed)...)
	got, err := ParseLiveReceipts(valid)
	if err != nil || len(got) != 2 || got[1].Event != "closed" {
		t.Fatal("complete stream rejected")
	}
	failed := play
	failed.Result, failed.Error = "fail", "restricted"
	failedClose := closed
	failedClose.Result, failedClose.Error = "fail", "restricted"
	got, err = ParseLiveReceipts(append(line(failed), line(failedClose)...))
	if err != nil || got[0].Result != "fail" || got[1].Result != "fail" {
		t.Fatal("failed stage became passing output")
	}
	regressed := closed
	regressed.APIRequests = play.APIRequests - 1
	for _, bad := range [][]byte{nil, valid[:len(valid)-1], line(play), append(line(closed), line(play)...), append(line(failed), line(closed)...), append(line(play), line(regressed)...), append(valid, '\n'), bytes.Repeat(line(play), 17), bytes.Repeat([]byte{' '}, 65537)} {
		if rows, err := ParseLiveReceipts(bad); err == nil || rows != nil {
			t.Fatal("incomplete/malformed/nonmonotonic stream accepted")
		}
	}
	var maximum []byte
	for range 15 {
		maximum = append(maximum, line(play)...)
	}
	maximum = append(maximum, line(closed)...)
	if _, err := ParseLiveReceipts(maximum); err != nil {
		t.Fatal("exact stream count rejected")
	}
	tooMany := append(line(play), maximum...)
	if _, err := ParseLiveReceipts(tooMany); err == nil {
		t.Fatal("seventeen otherwise valid complete receipts exceeded the bound")
	}
	var maximumBytes []byte
	for i := 0; i < 16; i++ {
		r := play
		if i == 15 {
			r = closed
		}
		raw := liveTestJSON(t, r)
		maximumBytes = append(maximumBytes, raw...)
		maximumBytes = append(maximumBytes, bytes.Repeat([]byte{' '}, 4095-len(raw))...)
		maximumBytes = append(maximumBytes, '\n')
	}
	if _, err := ParseLiveReceipts(maximumBytes); err != nil {
		t.Fatal("exact complete stream byte bound rejected")
	}
	tooLarge := append([]byte{' '}, maximumBytes...)
	if _, err := ParseLiveReceipts(tooLarge); err == nil {
		t.Fatal("otherwise valid complete stream exceeded the byte bound")
	}
}
