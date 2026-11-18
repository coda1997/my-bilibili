package previewbundle

import (
	"encoding/json"
	"testing"
)

func TestClientReceiptHasExactNestedFieldsAndLifecycle(t *testing.T) {
	closed := ClientReceipt{Event: ClientWindowClosed, Observation: closedClientObservationForTest()}
	data, _ := json.Marshal(closed)
	strictClientFields(t, data, func(b []byte) error { _, err := ParseClientReceipt(b); return err })
	minimized := ClientReceipt{Event: ClientWindowMinimized, Observation: clientObservationForTest()}
	minimized.Observation.Minimized = true
	data, _ = json.Marshal(minimized)
	if got, err := ParseClientReceipt(data); err != nil || got != minimized {
		t.Fatal("complete minimized state lost", err)
	}
	for _, receipt := range []ClientReceipt{
		{Event: "unknown", Observation: closed.Observation},
		{Event: ClientWindowMinimized, Observation: clientObservationForTest()},
		{Event: ClientWindowClosed, Observation: minimized.Observation},
		{Event: ClientWindowMinimized, Observation: closed.Observation},
	} {
		data, _ := json.Marshal(receipt)
		if _, err := ParseClientReceipt(data); err == nil {
			t.Fatal("invented receipt lifecycle accepted")
		}
	}
}
