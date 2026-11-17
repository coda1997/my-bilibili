package previewbundle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func validObservation() Observation {
	return Observation{State: app.State{DetailPhase: app.DetailReady, Video: catalog.Video{ID: "preview-video", Title: "Synthetic", Creator: "Local", Description: "Synthetic fixtures", CoverID: "preview-cover", Parts: []catalog.Part{{ID: "part-1", Title: "One", Duration: 30 * time.Second}, {ID: "part-2", Title: "Two", Duration: 30 * time.Second}}}, SelectedPart: "part-1", PlaybackPart: "part-1", Rate: 1, Volume: 1, Playback: playback.Snapshot{Generation: 1, State: playback.Closed, Rate: 1, Volume: 1}}}
}
func TestObservationRequiresEveryFieldExactlyOnce(t *testing.T) {
	data, _ := json.Marshal(validObservation())
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
