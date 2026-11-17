package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"my-bilibili/internal/previewbundle"
)

func sourceReceiptBytes(t *testing.T, event string) []byte {
	t.Helper()
	b, err := json.Marshal(previewbundle.SourceReceipt{Event: event, Observation: completeObservation(t)})
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func TestSourceReceiptsHandleChunkingAndDiscardUnstructuredOutput(t *testing.T) {
	s := newSourceReceipts()
	data := append([]byte("unstructured-private-sentinel\n"), sourceReceiptBytes(t, previewbundle.SourceWindowClosed)...)
	for _, b := range data {
		if n, err := s.Write([]byte{b}); n != 1 || err != nil {
			t.Fatal(n, err)
		}
	}
	got, err := s.get(previewbundle.SourceWindowClosed)
	if err != nil || got.State.Playback != completeObservation(t).State.Playback {
		t.Fatal(got, err)
	}
	got.State.Video.Parts[0].Title = "mutated"
	again, _ := s.get(previewbundle.SourceWindowClosed)
	if again.State.Video.Parts[0].Title == "mutated" {
		t.Fatal("receipt ownership escaped")
	}
	if _, err := s.get(previewbundle.SourceWindowMinimized); err == nil {
		t.Fatal("missing event became evidence")
	}
}

func TestSourceReceiptStreamFailsClosed(t *testing.T) {
	valid := sourceReceiptBytes(t, previewbundle.SourceWindowClosed)
	for name, data := range map[string][]byte{
		"duplicate":  append(bytes.Clone(valid), valid...),
		"incomplete": bytes.TrimSuffix(valid, []byte{'\n'}),
		"malformed":  []byte("{\"Event\":\"private-sentinel\"}\n"),
		"bounded":    append(append(bytes.Clone(valid), bytes.Repeat([]byte{'x'}, 65537)...), '\n'),
	} {
		t.Run(name, func(t *testing.T) {
			s := newSourceReceipts()
			n, err := s.Write(data)
			if n != len(data) || err != nil {
				t.Fatal("collector must keep draining the owned process")
			}
			if _, err := s.get(previewbundle.SourceWindowClosed); err == nil || strings.Contains(err.Error(), "private-sentinel") {
				t.Fatal("invalid stream became evidence or disclosed raw output")
			}
		})
	}
}
