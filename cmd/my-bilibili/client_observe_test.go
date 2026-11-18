package main

import (
	"encoding/json"
	"strings"
	"testing"

	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func TestClientObservationHasOnlySafeValuesAndActualClosedOwnership(t *testing.T) {
	h := newCoreHarness(t, true)
	a := &clientApplication{core: h.core}
	startCore(t, h)
	openCore(t, h, "PRIVATE_ORIGINAL_TITLE")
	data, err := json.Marshal(a.clientObservation())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE_ORIGINAL_TITLE") || strings.Contains(string(data), "SYNTHETIC_CLIENT_ONLY") {
		t.Fatal("private product value in observation")
	}
	o, err := previewbundle.ParseClientObservation(data)
	if err != nil || o.SourceCalls < 2 || o.PlaybackPart != 1 || o.PicturePresented || o.PendingOwners != 0 {
		t.Fatal("active observation invalid or fake engine claimed picture", err)
	}
	h.core.RequestClose(false)
	if a.clientObservation().Closed {
		t.Fatal("close request became completed window")
	}
	waitCore(t, h.core, func() bool { return h.core.closed })
	a.closed = true
	data, err = json.Marshal(a.clientObservation())
	if err != nil {
		t.Fatal(err)
	}
	o, err = previewbundle.ParseClientObservation(data)
	if err != nil || !o.Closed || o.PendingOwners != 0 || o.Playback.State != playback.Closed {
		t.Fatal("completed ownership not represented truthfully", err)
	}
}

func TestClientCloseWithoutVideoStillHasTerminalObservation(t *testing.T) {
	h := newCoreHarness(t, true)
	a := &clientApplication{core: h.core}
	startCore(t, h)
	h.core.RequestClose(false)
	waitCore(t, h.core, func() bool { return h.core.closed })
	a.closed = true
	data, _ := json.Marshal(a.clientObservation())
	if _, err := previewbundle.ParseClientObservation(data); err != nil {
		t.Fatal(err)
	}
}
