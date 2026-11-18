package previewbundle

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSourceReceiptPreservesDanmakuAndRejectsInvalidNestedState(t *testing.T) {
	want := SourceReceipt{Event: SourceWindowMinimized, Observation: activeDanmakuObservation()}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSourceReceipt(data)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("danmaku evidence lost in the source receipt", got, err)
	}
	for _, mutate := range []func(*Observation){
		func(o *Observation) { o.Danmaku.Layouts = 17 },
		func(o *Observation) { o.Danmaku.Settings.Visible = false },
		func(o *Observation) { o.Danmaku.Generation++ },
		func(o *Observation) { o.Danmaku.Error = "private-sentinel" },
	} {
		bad := want
		mutate(&bad.Observation)
		data, err := json.Marshal(bad)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseSourceReceipt(data)
		if err == nil || err.Error() != "invalid source receipt" || !reflect.DeepEqual(got, SourceReceipt{}) {
			t.Fatal("invalid danmaku accepted, partially returned or disclosed", got, err)
		}
	}
}

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
