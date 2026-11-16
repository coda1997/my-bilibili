package main

import (
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func TestReportPassOwnsSampledObservation(t *testing.T) {
	s := completeObservation(t)
	s.State.Playback.State = contract.Playing
	s.State.Playback.Position = 300 * time.Millisecond
	title := s.State.Video.Parts[0].Title
	r := &report{Scenario: "product"}
	r.pass("progress", "sampled while playing", &s)

	s.State.Playback.State = contract.Paused
	s.State.Playback.Position = 458333333 * time.Nanosecond
	s.State.Video.Parts[0].Title = "later title"
	r.pass("pause", "sampled while paused", &s)
	s = completeObservation(t)

	first := r.Checks[0].Observation
	if first.State.Playback.State != contract.Playing || first.State.Playback.Position != 300*time.Millisecond {
		t.Errorf("recorded playing sample changed: %+v", first.State.Playback)
	}
	if first.State.Video.Parts[0].Title != title {
		t.Errorf("recorded nested part changed: %q", first.State.Video.Parts[0].Title)
	}
	second := r.Checks[1].Observation
	if second.State.Playback.State != contract.Paused || second.State.Playback.Position != 458333333*time.Nanosecond {
		t.Errorf("recorded paused sample changed: %+v", second.State.Playback)
	}
	if second.State.Video.Parts[0].Title != "later title" {
		t.Errorf("recorded second nested part changed: %q", second.State.Video.Parts[0].Title)
	}
}

func TestReportPassPreservesNil(t *testing.T) {
	r := &report{}
	r.pass("without observation", "", nil)
	if r.Checks[0].Observation != nil {
		t.Fatal("nil observation became a fabricated snapshot")
	}
	s := completeObservation(t)
	s.State.Video.Parts = nil
	r.pass("without parts", "", &s)
	if r.Checks[1].Observation == nil || r.Checks[1].Observation.State.Video.Parts != nil {
		t.Fatal("nil parts were not preserved")
	}
}
