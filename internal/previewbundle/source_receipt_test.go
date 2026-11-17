package previewbundle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceReceiptRetainsStrictObservationBoundary(t *testing.T) {
	want := SourceReceipt{Event: SourceWindowClosed, Observation: validObservation()}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSourceReceipt(data)
	if err != nil || got.Event != want.Event || got.Observation.State.Playback != want.Observation.State.Playback {
		t.Fatal(got, err)
	}
	text := string(data)
	for _, bad := range []string{
		"", "null", "{}", text + " {}", text[:len(text)-1], text + strings.Repeat(" ", 65537),
		strings.Replace(text, `"Event":"source-window-closed",`, ``, 1),
		strings.Replace(text, `"Event":`, `"event":`, 1),
		strings.Replace(text, `"Event":`, `"Event":"source-window-closed","Event":`, 1),
		strings.Replace(text, `"source-window-closed"`, `"private-sentinel"`, 1),
		strings.Replace(text, `"source-window-closed"`, `null`, 1),
		strings.Replace(text, `"source-window-closed"`, `0`, 1),
		strings.Replace(text, `"Observation":`, `"Observation":null,"Observation":`, 1),
		strings.Replace(text, `"Observation":`, `"Unknown":0,"Observation":`, 1),
		strings.Replace(text, `"DetailPhase":"ready"`, `"DetailPhase":null`, 1),
		strings.Replace(text, `"generation":1`, `"generation":0`, 1),
	} {
		if _, err := ParseSourceReceipt([]byte(bad)); err == nil || strings.Contains(err.Error(), "private-sentinel") {
			t.Fatal("invalid receipt was accepted or its content disclosed")
		}
	}
}
