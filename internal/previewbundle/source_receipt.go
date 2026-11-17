package previewbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const SourceWindowMinimized = "source-window-minimized"
const SourceWindowClosed = "source-window-closed"

// SourceReceipt is synthetic-only evidence for a window which cannot repaint.
// Its nested observation has exactly the same strict schema as a visible tree.
type SourceReceipt struct {
	Event       string
	Observation Observation
}

func ParseSourceReceipt(data []byte) (SourceReceipt, error) {
	invalid := errors.New("invalid source receipt")
	if len(data) == 0 || len(data) > 65536 {
		return SourceReceipt{}, invalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return SourceReceipt{}, invalid
	}
	var event string
	var observation json.RawMessage
	fields := 0
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return SourceReceipt{}, invalid
		}
		switch key {
		case "Event":
			if fields&1 != 0 || d.Decode(&event) != nil {
				return SourceReceipt{}, invalid
			}
			fields |= 1
		case "Observation":
			if fields&2 != 0 || d.Decode(&observation) != nil {
				return SourceReceipt{}, invalid
			}
			fields |= 2
		default:
			return SourceReceipt{}, invalid
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || fields != 3 {
		return SourceReceipt{}, invalid
	}
	if _, err := d.Token(); err != io.EOF || event != SourceWindowMinimized && event != SourceWindowClosed {
		return SourceReceipt{}, invalid
	}
	o, err := ParseObservation(observation)
	if err != nil {
		return SourceReceipt{}, invalid
	}
	return SourceReceipt{Event: event, Observation: o}, nil
}
