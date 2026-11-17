package previewbundle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

func validObservation() Observation {
	return Observation{State: app.State{DetailPhase: app.DetailReady, Video: catalog.Video{ID: "preview-video", Title: "Synthetic", Creator: "Local", Description: "Synthetic fixtures", CoverID: "preview-cover", Parts: []catalog.Part{{ID: "part-1", Title: "One", Duration: 30 * time.Second}, {ID: "part-2", Title: "Two", Duration: 30 * time.Second}}}, SelectedPart: "part-1", PlaybackPart: "part-1", SelectedQuality: "fixture", PlaybackQuality: "fixture", Qualities: []app.Quality{{ID: "fixture", Label: "Fixture", Width: 640, Height: 360}}, Rate: 1, Volume: 1, Playback: playback.Snapshot{Generation: 1, State: playback.Closed, Rate: 1, Volume: 1}}, Danmaku: DanmakuObservation{Settings: danmaku.DefaultSettings(), Items: []DanmakuItemObservation{}}}
}
func testRequiredObservationFields(t *testing.T, data []byte) {
	t.Helper()
	if _, err := ParseObservation(data); err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}
	var walk func(map[string]any, string)
	walk = func(object map[string]any, path string) {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		for _, key := range keys {
			value := object[key]
			delete(object, key)
			missing, _ := json.Marshal(tree)
			if _, err := ParseObservation(missing); err == nil {
				t.Errorf("accepted missing %s%s", path, key)
			}
			object[key] = nil
			null, _ := json.Marshal(tree)
			if _, err := ParseObservation(null); err == nil {
				t.Errorf("accepted null %s%s", path, key)
			}
			object[key] = value
			switch child := value.(type) {
			case map[string]any:
				walk(child, path+key+".")
			case []any:
				for _, item := range child {
					if nested, ok := item.(map[string]any); ok {
						walk(nested, path+key+"[].")
					}
				}
			}
		}
	}
	walk(tree, "")
}

func TestObservationRequiresEveryFieldExactlyOnce(t *testing.T) {
	data, _ := json.Marshal(validObservation())
	testRequiredObservationFields(t, data)
	text := string(data)
	for _, bad := range []string{
		"null", "{}", text + " {}", text[:len(text)-1],
		strings.Replace(text, `"Cores":0`, `"Cores":0,"Cores":0`, 1),
		strings.Replace(text, `"Cores":0`, `"cores":0`, 1),
		strings.Replace(text, `"Cores":0`, `"Cores":-1`, 1),
		strings.Replace(text, `"Cores":0`, `"Cores":0.0`, 1),
		strings.Replace(text, `"Cores":0`, `"Cores":"0"`, 1),
		strings.Replace(text, `"Fullscreen":false`, `"Fullscreen":"false"`, 1),
		strings.Replace(text, `"state":"closed"`, `"state":"unknown"`, 1),
		strings.Replace(text, `"error":""`, `"error":"private-hostile-input"`, 1),
		strings.Replace(text, `"generation":1`, `"generation":0`, 1),
		strings.Replace(text, `"Active":0`, `"Active":1`, 1),
	} {
		if _, err := ParseObservation([]byte(bad)); err == nil {
			t.Errorf("accepted malformed observation: %.100s", bad)
		}
	}
}
func TestCompletePositiveOwnershipRemainsPositive(t *testing.T) {
	o := validObservation()
	o.Resources.Cores = 1
	o.Resources.Workers = 2
	o.Resources.Renderers = 1
	b, _ := json.Marshal(o)
	got, err := ParseObservation(b)
	if err != nil || got.Resources != o.Resources {
		t.Fatal(got, err)
	}
}

func TestObservationContentCodesStayDistinctFromNativeCodes(t *testing.T) {
	for _, code := range []catalog.Code{catalog.None, catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network,
		catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired,
		catalog.VerificationRequired, catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed} {
		t.Run(string(code), func(t *testing.T) {
			o := validObservation()
			o.State.SelectionError, o.State.PlaybackRetryError, o.State.ResumeError = code, code, code
			o.State.Playback.Error = playback.Media
			data, err := json.Marshal(o)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseObservation(data)
			if err != nil || got.State.SelectionError != code || got.State.PlaybackRetryError != code || got.State.ResumeError != code || got.State.Playback.Error != playback.Media {
				t.Fatal("content/native error domains were conflated", err)
			}
		})
	}
	for _, change := range []func(*Observation){
		func(o *Observation) { o.State.SelectionError = catalog.Code(playback.Media) },
		func(o *Observation) { o.State.PlaybackRetryError = catalog.Code(playback.Media) },
		func(o *Observation) { o.State.ResumeError = "private-sentinel" },
		func(o *Observation) { o.State.Playback.Error = playback.ErrorCode(catalog.Unavailable) },
		func(o *Observation) { o.DisposalError = playback.ErrorCode(catalog.Network) },
	} {
		o := validObservation()
		change(&o)
		data, _ := json.Marshal(o)
		if _, err := ParseObservation(data); err == nil {
			t.Fatal("an unknown or wrong-domain error became evidence")
		}
	}
}

func TestObservationRequiresActualFixtureChoices(t *testing.T) {
	for name, change := range map[string]func(*Observation){
		"null-choices":               func(o *Observation) { o.State.Qualities = nil },
		"missing-choices":            func(o *Observation) { o.State.Qualities = []app.Quality{} },
		"missing-accepted-choice":    func(o *Observation) { o.State.PlaybackQuality = "" },
		"invented-selected-choice":   func(o *Observation) { o.State.SelectedQuality = "64" },
		"invented-choice-label":      func(o *Observation) { o.State.Qualities[0].Label = "Service quality" },
		"invented-choice-dimensions": func(o *Observation) { o.State.Qualities[0].Width = 1920 },
		"duplicate-choice":           func(o *Observation) { o.State.Qualities = append(o.State.Qualities, o.State.Qualities[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			o := validObservation()
			change(&o)
			data, _ := json.Marshal(o)
			if _, err := ParseObservation(data); err == nil {
				t.Fatal("invented fixture choice became evidence")
			}
		})
	}
	for _, change := range []func(*Observation){
		func(o *Observation) { o.State.SelectedQuality = "fixture" },
		func(o *Observation) { o.State.PlaybackQuality = "fixture" },
		func(o *Observation) {
			o.State.Qualities = []app.Quality{{ID: "fixture", Label: "Fixture", Width: 640, Height: 360}}
		},
	} {
		o := pendingDetailObservation()
		change(&o)
		data, _ := json.Marshal(o)
		if _, err := ParseObservation(data); err == nil {
			t.Fatal("unaccepted choices became evidence")
		}
	}
}
