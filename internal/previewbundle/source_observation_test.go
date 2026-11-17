package previewbundle

import (
	"encoding/json"
	"testing"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func pendingDetailObservation() Observation {
	o := validObservation()
	o.State.DetailPhase = app.DetailLoading
	o.State.Video = catalog.Video{Parts: []catalog.Part{}}
	o.State.SelectedPart, o.State.PlaybackPart = "", ""
	o.State.Playback = playback.Snapshot{State: playback.Idle, Rate: 1, Volume: 1}
	o.Source = SourceObservation{DetailRequests: 1, Active: 1, PeakActive: 1}
	return o
}

func TestCompleteSourceStartupAndClosingObservations(t *testing.T) {
	initial := pendingDetailObservation()
	initial.State.DetailPhase = app.DetailInitial
	initial.Source = SourceObservation{}
	failure := pendingDetailObservation()
	failure.State.DetailPhase, failure.State.DetailError = app.DetailFailed, playback.Timeout
	failure.Source.Active = 0
	closing := pendingDetailObservation()
	closing.State.Playback.State = playback.Closed
	closing.State.Closing, closing.Minimized = true, true
	closedBeforeStart := initial
	closedBeforeStart.State.Playback.State = playback.Closed
	resolving := validObservation()
	resolving.State.PlaybackPart, resolving.State.ResolvingPart = "", "part-1"
	resolving.State.Playback = playback.Snapshot{State: playback.Idle, Rate: 1, Volume: 1}
	resolving.State.DesiredPlaying = true
	refresh := validObservation()
	refresh.State.SelectedPart, refresh.State.SelectionError = "part-2", playback.Media
	refresh.State.PlaybackRetryError = playback.Timeout
	refresh.State.Playback.State = playback.Playing
	for name, observation := range map[string]Observation{
		"initial": initial, "detail-loading": pendingDetailObservation(),
		"detail-failed": failure, "closing-source": closing,
		"closed-before-start": closedBeforeStart, "first-resolution": resolving,
		"separate-playback-retry-error": refresh,
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseObservation(data)
			if err != nil || got.State.DetailPhase != observation.State.DetailPhase || got.State.Playback.Generation != observation.State.Playback.Generation || got.Source != observation.Source {
				t.Fatal("valid source state was lost", got, err)
			}
		})
	}
}

func TestSourceObservationRejectsContradictoryOrInventedState(t *testing.T) {
	mutations := map[string]func(*Observation){
		"unknown-phase":                  func(o *Observation) { o.State.DetailPhase = "unknown" },
		"metadata-before-detail":         func(o *Observation) { o.State.Video.Title = "not loaded" },
		"selection-before-detail":        func(o *Observation) { o.State.SelectedPart = "part-1" },
		"accepted-before-detail":         func(o *Observation) { o.State.PlaybackPart = "part-1" },
		"generation-without-acceptance":  func(o *Observation) { o.State.Playback.Generation = 1 },
		"playing-before-open":            func(o *Observation) { o.State.Playback.State = playback.Playing },
		"failed-without-error":           func(o *Observation) { o.State.DetailPhase = app.DetailFailed },
		"loading-with-failure":           func(o *Observation) { o.State.DetailError = playback.Media },
		"private-detail-error":           func(o *Observation) { o.State.DetailPhase = app.DetailFailed; o.State.DetailError = "private-sentinel" },
		"selection-error-before-detail":  func(o *Observation) { o.State.SelectionError = playback.Media },
		"refresh-error-without-playback": func(o *Observation) { o.State.PlaybackRetryError = playback.Media },
		"closing-but-not-closed":         func(o *Observation) { o.State.Closing = true },
		"negative-source-count":          func(o *Observation) { o.Source.DetailRequests = -1; o.Source.ResolveRequests = 2 },
		"negative-resolve-count":         func(o *Observation) { o.Source.ResolveRequests = -1; o.Source.DetailRequests = 2 },
		"negative-active":                func(o *Observation) { o.Source.Active = -1 },
		"missing-peak":                   func(o *Observation) { o.Source.PeakActive = 0 },
		"peak-without-calls":             func(o *Observation) { o.Source.PeakActive = 2 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			o := pendingDetailObservation()
			mutate(&o)
			data, err := json.Marshal(o)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseObservation(data); err == nil {
				t.Fatal("contradictory observation accepted")
			}
		})
	}
	for _, mutate := range []func(*Observation){
		func(o *Observation) { o.State.ResolvingPart = "unknown" },
		func(o *Observation) { o.State.PlaybackRetryError = "private-sentinel" },
		func(o *Observation) { o.State.Closing = true; o.State.ResolvingPart = "part-1" },
		func(o *Observation) { o.State.Closing = true; o.State.DesiredPlaying = true },
	} {
		o := validObservation()
		mutate(&o)
		data, _ := json.Marshal(o)
		if _, err := ParseObservation(data); err == nil {
			t.Fatal("invalid loaded source state accepted")
		}
	}
}

func TestPositiveSourceOwnershipIsNeverCoercedToZero(t *testing.T) {
	o := pendingDetailObservation()
	o.Source = SourceObservation{DetailRequests: 2, ResolveRequests: 1, Active: 2, PeakActive: 2}
	data, _ := json.Marshal(o)
	got, err := ParseObservation(data)
	if err != nil || got.Source != o.Source {
		t.Fatal("ownership observation lost rather than left for the scenario assertion", got, err)
	}
}
