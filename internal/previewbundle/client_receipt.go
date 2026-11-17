package previewbundle

import "errors"

const ClientWindowMinimized = "client-window-minimized"
const ClientWindowClosed = "client-window-closed"

// ClientReceipt is synthetic client evidence for windows that cannot repaint.
// Live verification has a different exact schema and never publishes this type.
type ClientReceipt struct {
	Event       string
	Observation ClientObservation
}

func ParseClientReceipt(data []byte) (ClientReceipt, error) {
	var r ClientReceipt
	if !decodeClientValue(data, &r) || !validClientObservation(r.Observation) {
		return ClientReceipt{}, errors.New("invalid client receipt")
	}
	switch r.Event {
	case ClientWindowMinimized:
		if r.Observation.Minimized && !r.Observation.Closed {
			return r, nil
		}
	case ClientWindowClosed:
		if r.Observation.Closed {
			return r, nil
		}
	}
	return ClientReceipt{}, errors.New("invalid client receipt")
}
