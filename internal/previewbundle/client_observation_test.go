package previewbundle

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func clientObservationForTest() ClientObservation {
	return ClientObservation{
		AccountPhase: account.SignedOut, ScopeRevision: 1, Navigation: 1, Visit: 1,
		SearchPhase: catalog.SearchReady, SearchGeneration: 1, Results: 2, Page: 1,
		DetailPhase: app.DetailReady, Parts: 2, SelectedPart: 1, PlaybackPart: 1,
		Qualities: 2, SelectedQuality: 2, PlaybackQuality: 2,
		Playback:   playback.Snapshot{Generation: 9, State: playback.Paused, Position: 5 * time.Second, Duration: 30 * time.Second, Rate: 1, Volume: .5},
		VideoTrack: true, AudioTrack: true, PicturePresented: true,
		SourceCalls: 2, SourcePeak: 1, SearchCalls: 2, SearchPeak: 1,
		APIRequests: 4, APIPeak: 2, CoverRequests: 1, CoverRetained: 1,
		DanmakuPhase: danmaku.LoadReady, DanmakuGeneration: 9, DanmakuRevision: 1,
		Segment: 1, CachedSegments: 1, CachedEntries: 1, CachedTextBytes: 7,
		VisibleComments: 1, TextLayouts: 1,
		HistoryPhase: history.Ready, HistoryCount: 1, HistoryRevision: 2, HistorySavedRevision: 2,
		Resources: mpv.Resources{Cores: 1, Renderers: 1, Workers: 2},
	}
}

func closedClientObservationForTest() ClientObservation {
	o := clientObservationForTest()
	o.AccountPhase, o.SearchPhase = account.Done, catalog.SearchClosed
	o.DanmakuPhase, o.HistoryPhase = danmaku.LoadClosed, history.Done
	o.Closing, o.Closed = true, true
	o.Playback.State = playback.Closed
	o.VideoTrack, o.AudioTrack, o.PicturePresented = false, false, false
	o.Resources = mpv.Resources{}
	o.CoverRetained, o.Segment, o.CachedSegments, o.CachedEntries, o.CachedTextBytes = 0, 0, 0, 0, 0
	o.VisibleComments, o.TextLayouts = 0, 0
	return o
}

func requireClientParse(t *testing.T, o ClientObservation, valid bool) {
	t.Helper()
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseClientObservation(data)
	if (err == nil) != valid || valid && got != o {
		t.Fatalf("client observation valid=%v: %v", valid, err)
	}
}

func TestClientObservationsKeepPendingAcceptedAndClosedTruth(t *testing.T) {
	initial := ClientObservation{AccountPhase: account.Initial, SearchPhase: catalog.SearchInitial,
		DetailPhase: app.DetailInitial, Playback: playback.Snapshot{State: playback.Idle, Rate: 1, Volume: 1},
		DanmakuPhase: danmaku.LoadInitial, HistoryPhase: history.Initial}
	ready := clientObservationForTest()
	quality := ready
	quality.SelectedQuality, quality.Resolving = 1, true
	quality.Playback.State, quality.DesiredPlaying = playback.Playing, true
	closing := ready
	closing.Closing, closing.PendingOwners, closing.HistoryBusy = true, 9, true
	closing.SourceActive, closing.SearchActive, closing.APIActive, closing.APIPeak, closing.CoverActive, closing.DanmakuActiveRequests = 1, 1, 4, 4, 1, 1
	noPicture := ready
	noPicture.PicturePresented = false
	minimized := ready
	minimized.Minimized = true // The per-engine bit records earlier paint, not current visibility.
	empty := initial
	empty.SearchPhase, empty.Page = catalog.SearchReady, 1
	for _, o := range []ClientObservation{initial, ready, quality, closing, noPicture, minimized, empty, closedClientObservationForTest()} {
		requireClientParse(t, o, true)
	}
}

func TestClientObservationAcceptsExactBoundsWithoutTruncation(t *testing.T) {
	o := clientObservationForTest()
	o.Results, o.Page, o.Parts, o.SelectedPart, o.PlaybackPart = 1000, 50, 1000, 1000, 1000
	o.Qualities, o.SelectedQuality, o.PlaybackQuality = 16, 16, 16
	o.Playback.Position, o.Playback.Duration, o.Playback.Rate, o.Playback.Volume = 7*24*time.Hour, 7*24*time.Hour, 3, 1
	o.SourceCalls, o.SearchCalls, o.APIRequests, o.CoverRequests = math.MaxInt32, math.MaxInt32, math.MaxInt32, math.MaxInt32
	o.SourceActive, o.SourcePeak, o.SearchActive, o.SearchPeak, o.APIActive, o.APIPeak, o.CoverActive, o.CoverRetained = 1, 1, 1, 1, 4, 4, 1, 12
	o.Segment, o.CachedSegments, o.CachedEntries, o.CachedTextBytes = 1680, 2, 40000, 8<<20
	o.DanmakuActiveRequests, o.VisibleComments, o.TextLayouts, o.HistoryCount = 1, 16, 16, 500
	o.HistoryRevision, o.HistorySavedRevision = ^uint64(0), ^uint64(0)
	o.Closing, o.PendingOwners = true, 9
	requireClientParse(t, o, true)
}

func strictClientFields(t *testing.T, data []byte, parse func([]byte) error) {
	t.Helper()
	if err := parse(data); err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}
	var walk func(map[string]any)
	walk = func(object map[string]any) {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		for _, key := range keys {
			value := object[key]
			delete(object, key)
			missing, _ := json.Marshal(tree)
			if parse(missing) == nil {
				t.Errorf("accepted missing %s", key)
			}
			object[key] = nil
			null, _ := json.Marshal(tree)
			if parse(null) == nil {
				t.Errorf("accepted null %s", key)
			}
			object[key] = value
			alias := strings.ToLower(key)
			if alias == key {
				alias = strings.ToUpper(key)
			}
			quoted, _ := json.Marshal(key)
			aliasKey, _ := json.Marshal(alias)
			needle := string(quoted) + ":"
			if parse([]byte(strings.Replace(string(data), needle, string(aliasKey)+":", 1))) == nil {
				t.Errorf("accepted case alias %s", key)
			}
			encoded, _ := json.Marshal(value)
			if parse([]byte(strings.Replace(string(data), needle, needle+string(encoded)+","+needle, 1))) == nil {
				t.Errorf("accepted duplicate %s", key)
			}
			if child, ok := value.(map[string]any); ok {
				walk(child)
			}
		}
	}
	walk(tree)
	for _, bad := range [][]byte{nil, []byte("null"), []byte("{}"), append(append([]byte{}, data...), []byte(" {}")...), data[:len(data)-1], []byte(strings.Repeat(" ", 65537-len(data)) + string(data)), []byte(strings.Replace(string(data), "{", "{\"Unexpected\":0,", 1))} {
		if parse(bad) == nil {
			t.Error("accepted malformed/bounded client evidence")
		}
	}
}

func TestClientObservationRequiresAllExactFields(t *testing.T) {
	data, _ := json.Marshal(clientObservationForTest())
	strictClientFields(t, data, func(b []byte) error { _, err := ParseClientObservation(b); return err })
	for _, pair := range [][2]string{{"\"Parts\":2", "\"Parts\":2.0"}, {"\"Parts\":2", "\"Parts\":\"2\""}, {"\"Closed\":false", "\"Closed\":0"}, {"\"rate\":1", "\"rate\":1e9999"}} {
		if _, err := ParseClientObservation([]byte(strings.Replace(string(data), pair[0], pair[1], 1))); err == nil {
			t.Fatal("accepted wrong scalar evidence type")
		}
	}
}

func TestClientObservationRejectsBoundsAndUnacceptedIndexes(t *testing.T) {
	changes := map[string]func(*ClientObservation){
		"result-cap":                func(o *ClientObservation) { o.Results = 1001 },
		"page-cap":                  func(o *ClientObservation) { o.Page = 51 },
		"negative-page":             func(o *ClientObservation) { o.Page = -1 },
		"part-cap":                  func(o *ClientObservation) { o.Parts = 1001 },
		"selected-part":             func(o *ClientObservation) { o.SelectedPart = 3 },
		"accepted-part":             func(o *ClientObservation) { o.PlaybackPart = 3 },
		"generation-zero":           func(o *ClientObservation) { o.Playback.Generation, o.DanmakuGeneration = 0, 0 },
		"quality-cap":               func(o *ClientObservation) { o.Qualities = 17 },
		"selected-quality":          func(o *ClientObservation) { o.SelectedQuality = 3 },
		"accepted-quality":          func(o *ClientObservation) { o.PlaybackQuality = 3 },
		"missing-accepted-quality":  func(o *ClientObservation) { o.PlaybackQuality = 0 },
		"qualities-before-part":     func(o *ClientObservation) { o.PlaybackPart, o.VisibleComments, o.TextLayouts = 0, 0, 0 },
		"auth-without-epoch":        func(o *ClientObservation) { o.Authenticated = true },
		"epoch-without-auth":        func(o *ClientObservation) { o.AccountEpoch = 1 },
		"source-count-cap":          func(o *ClientObservation) { o.SourceCalls = math.MaxInt32 + 1 },
		"source-active":             func(o *ClientObservation) { o.SourceActive, o.SourcePeak = 2, 2 },
		"source-peak-cap":           func(o *ClientObservation) { o.SourcePeak = 2 },
		"source-negative":           func(o *ClientObservation) { o.SourceActive = -1 },
		"source-peak-without-calls": func(o *ClientObservation) { o.SourceCalls = 0 },
		"search-active":             func(o *ClientObservation) { o.SearchActive, o.SearchPeak = 2, 2 },
		"search-peak-cap":           func(o *ClientObservation) { o.SearchPeak = 2 },
		"search-peak":               func(o *ClientObservation) { o.SearchActive, o.SearchPeak = 1, 0 },
		"api-active":                func(o *ClientObservation) { o.APIRequests, o.APIActive, o.APIPeak = 5, 5, 5 },
		"api-peak-cap":              func(o *ClientObservation) { o.APIRequests, o.APIPeak = 5, 5 },
		"api-negative":              func(o *ClientObservation) { o.APIRequests = -1 },
		"cover-active":              func(o *ClientObservation) { o.CoverRequests, o.CoverActive = 2, 2 },
		"cover-without-call":        func(o *ClientObservation) { o.CoverRequests, o.CoverActive = 0, 1 },
		"cover-retention":           func(o *ClientObservation) { o.CoverRetained = 13 },
		"segment-index":             func(o *ClientObservation) { o.Segment = 1681 },
		"segment-missing":           func(o *ClientObservation) { o.Segment = 0 },
		"segment-slots":             func(o *ClientObservation) { o.CachedSegments = 3 },
		"segment-entry-cap":         func(o *ClientObservation) { o.CachedEntries, o.CachedTextBytes = 20001, 20001 },
		"segment-byte-cap":          func(o *ClientObservation) { o.CachedTextBytes = 4<<20 + 1 },
		"segment-empty-cache":       func(o *ClientObservation) { o.CachedSegments = 0 },
		"segment-text-count":        func(o *ClientObservation) { o.CachedTextBytes = 0 },
		"segment-active":            func(o *ClientObservation) { o.DanmakuActiveRequests = 2 },
		"visible-cap":               func(o *ClientObservation) { o.VisibleComments = 17 },
		"layout-cap":                func(o *ClientObservation) { o.TextLayouts = 17 },
		"stale-comment-generation":  func(o *ClientObservation) { o.DanmakuGeneration-- },
		"history-cap":               func(o *ClientObservation) { o.HistoryCount = 501 },
		"history-revision":          func(o *ClientObservation) { o.HistorySavedRevision++ },
		"owner-cap":                 func(o *ClientObservation) { o.Closing, o.PendingOwners = true, 10 },
		"owners-before-close":       func(o *ClientObservation) { o.PendingOwners = 1 },
		"native-core-cap":           func(o *ClientObservation) { o.Resources.Cores = 2 },
		"native-renderer-cap":       func(o *ClientObservation) { o.Resources.Renderers = 2 },
		"native-worker-cap":         func(o *ClientObservation) { o.Resources.Workers = 3 },
		"native-negative":           func(o *ClientObservation) { o.Resources.Cores = -1 },
		"duration-cap":              func(o *ClientObservation) { o.Playback.Duration = 7*24*time.Hour + 1 },
		"position-cap":              func(o *ClientObservation) { o.Playback.Position = 7*24*time.Hour + 1 },
		"position-negative":         func(o *ClientObservation) { o.Playback.Position = -1 },
		"rate-cap":                  func(o *ClientObservation) { o.Playback.Rate = 4 },
		"volume-cap":                func(o *ClientObservation) { o.Playback.Volume = 2 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) { o := clientObservationForTest(); change(&o); requireClientParse(t, o, false) })
	}
}

func TestClientObservationRejectsWrongEnumDomainsAndNonfiniteSettings(t *testing.T) {
	for name, change := range map[string]func(*ClientObservation){
		"account-phase":   func(o *ClientObservation) { o.AccountPhase = "private" },
		"account-error":   func(o *ClientObservation) { o.AccountError = account.Code(catalog.Restricted) },
		"search-phase":    func(o *ClientObservation) { o.SearchPhase = "private" },
		"search-error":    func(o *ClientObservation) { o.SearchError = catalog.Code(account.Storage) },
		"detail-phase":    func(o *ClientObservation) { o.DetailPhase = "private" },
		"detail-error":    func(o *ClientObservation) { o.DetailError = catalog.Code(playback.Media) },
		"selection-error": func(o *ClientObservation) { o.SelectionError = "private" },
		"retry-error":     func(o *ClientObservation) { o.PlaybackRetryError = "private" },
		"resume-error":    func(o *ClientObservation) { o.ResumeError = "private" },
		"native-state":    func(o *ClientObservation) { o.Playback.State = "private" },
		"native-error":    func(o *ClientObservation) { o.Playback.Error = playback.ErrorCode(catalog.Unavailable) },
		"danmaku-phase":   func(o *ClientObservation) { o.DanmakuPhase = "private" },
		"danmaku-error":   func(o *ClientObservation) { o.DanmakuError = "private" },
		"history-phase":   func(o *ClientObservation) { o.HistoryPhase = "private" },
		"history-error":   func(o *ClientObservation) { o.HistoryError = history.Code(account.Storage) },
	} {
		t.Run(name, func(t *testing.T) { o := clientObservationForTest(); change(&o); requireClientParse(t, o, false) })
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		o := clientObservationForTest()
		o.Playback.Rate = value
		if validClientObservation(o) {
			t.Fatal("nonfinite rate passed semantic check")
		}
		o = clientObservationForTest()
		o.Playback.Volume = value
		if validClientObservation(o) {
			t.Fatal("nonfinite volume passed semantic check")
		}
	}
}

func TestClosedClientRequiresEveryActualOwnerAndReferenceToEnd(t *testing.T) {
	for name, change := range map[string]func(*ClientObservation){
		"pending":       func(o *ClientObservation) { o.PendingOwners = 1 },
		"core":          func(o *ClientObservation) { o.Resources.Cores = 1 },
		"renderer":      func(o *ClientObservation) { o.Resources.Renderers = 1 },
		"worker":        func(o *ClientObservation) { o.Resources.Workers = 1 },
		"video":         func(o *ClientObservation) { o.VideoTrack = true },
		"audio":         func(o *ClientObservation) { o.AudioTrack = true },
		"picture":       func(o *ClientObservation) { o.PicturePresented = true },
		"native-state":  func(o *ClientObservation) { o.Playback.State = playback.Paused },
		"auth":          func(o *ClientObservation) { o.Authenticated, o.AccountEpoch = true, 1 },
		"source":        func(o *ClientObservation) { o.SourceActive = 1 },
		"search":        func(o *ClientObservation) { o.SearchActive = 1 },
		"api":           func(o *ClientObservation) { o.APIActive = 1 },
		"cover-work":    func(o *ClientObservation) { o.CoverActive = 1 },
		"cover-cache":   func(o *ClientObservation) { o.CoverRetained = 1 },
		"danmaku-work":  func(o *ClientObservation) { o.DanmakuActiveRequests = 1 },
		"visible":       func(o *ClientObservation) { o.VisibleComments = 1 },
		"layout":        func(o *ClientObservation) { o.TextLayouts = 1 },
		"segment-cache": func(o *ClientObservation) { o.Segment, o.CachedSegments = 1, 1 },
		"resolve":       func(o *ClientObservation) { o.Resolving = true },
		"resume":        func(o *ClientObservation) { o.Resuming = true },
		"play-intent":   func(o *ClientObservation) { o.DesiredPlaying = true },
		"history-work":  func(o *ClientObservation) { o.HistoryBusy = true },
		"history-owner": func(o *ClientObservation) { o.HistoryPhase = history.Closing },
		"account-owner": func(o *ClientObservation) { o.AccountPhase = account.Closing },
		"search-owner":  func(o *ClientObservation) { o.SearchPhase = catalog.SearchReady },
		"danmaku-owner": func(o *ClientObservation) { o.DanmakuPhase = danmaku.LoadReady },
	} {
		t.Run(name, func(t *testing.T) { o := closedClientObservationForTest(); change(&o); requireClientParse(t, o, false) })
	}
}
