package previewbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"

	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/playback"
)

// Observation contains only safe application/native state, never media inputs.
// Normal builds do not publish it. Audio track selection does not prove audibility.
type Observation struct {
	State         app.State
	Fullscreen    bool
	Minimized     bool
	Resources     mpv.Resources
	Tracks        mpv.Tracks
	DisposalError playback.ErrorCode
	Delay         DelayObservation
	Source        SourceObservation
	Danmaku       DanmakuObservation
}
type DelayObservation struct{ Requests, Active int64 }
type SourceObservation struct{ DetailRequests, ResolveRequests, Active, PeakActive int64 }

func ParseObservation(data []byte) (Observation, error) {
	var o Observation
	invalid := errors.New("invalid preview observation")
	if len(data) == 0 || len(data) > 65536 {
		return o, invalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := requiredValue(d, reflect.TypeFor[Observation]()); err != nil {
		return o, invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return o, invalid
	}
	if json.Unmarshal(data, &o) != nil {
		return Observation{}, invalid
	}
	s := o.State.Playback
	switch s.State {
	case playback.Idle, playback.Loading, playback.Ready, playback.Paused, playback.Playing, playback.Ended, playback.Failed, playback.Closed:
	default:
		return Observation{}, invalid
	}
	for _, code := range []playback.ErrorCode{s.Error, o.State.DetailError, o.State.SelectionError, o.State.PlaybackRetryError, o.DisposalError} {
		switch code {
		case playback.NoError, playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
		default:
			return Observation{}, invalid
		}
	}
	if o.Resources.Cores < 0 || o.Resources.Renderers < 0 || o.Resources.Workers < 0 || s.Position < 0 || s.Duration < 0 || s.Rate < 0 || s.Rate > 3 || s.Volume < 0 || s.Volume > 1 || o.State.Rate < .25 || o.State.Rate > 3 || o.State.Volume < 0 || o.State.Volume > 1 {
		return Observation{}, invalid
	}
	if !validSourceState(o.State) {
		return Observation{}, invalid
	}
	if o.State.Closing && (s.State != playback.Closed || o.State.ResolvingPart != "" || o.State.DesiredPlaying) {
		return Observation{}, invalid
	}
	if o.Delay.Requests < 0 || o.Delay.Active < 0 || o.Delay.Active > o.Delay.Requests {
		return Observation{}, invalid
	}
	source := o.Source
	if source.DetailRequests < 0 || source.ResolveRequests < 0 || source.Active < 0 || source.PeakActive < source.Active || source.PeakActive > source.DetailRequests+source.ResolveRequests {
		return Observation{}, invalid
	}
	if !validDanmakuObservation(o.Danmaku, s) {
		return Observation{}, invalid
	}
	return o, nil
}

func validSourceState(state app.State) bool {
	v, native := state.Video, state.Playback
	part := func(id string) bool { return id == "part-1" || id == "part-2" }
	if state.PlaybackPart == "" {
		if native.Generation != 0 || native.State != playback.Idle && native.State != playback.Closed {
			return false
		}
	} else if !part(string(state.PlaybackPart)) || native.Generation == 0 {
		return false
	}
	switch state.DetailPhase {
	case app.DetailInitial, app.DetailLoading, app.DetailFailed:
		if v.ID != "" || v.Title != "" || v.Creator != "" || v.Description != "" || v.CoverID != "" || len(v.Parts) != 0 || state.SelectedPart != "" || state.PlaybackPart != "" || state.ResolvingPart != "" || state.DesiredPlaying || state.SelectionError != playback.NoError || state.PlaybackRetryError != playback.NoError {
			return false
		}
		return (state.DetailPhase == app.DetailFailed) == (state.DetailError != playback.NoError)
	case app.DetailReady:
		if state.DetailError != playback.NoError || v.ID != "preview-video" || v.CoverID != "preview-cover" || v.Title == "" || v.Creator == "" || v.Description == "" || len(v.Parts) != 2 || v.Parts[0].ID != "part-1" || v.Parts[1].ID != "part-2" || !part(string(state.SelectedPart)) {
			return false
		}
		for _, p := range v.Parts {
			if p.Title == "" || p.Duration <= 0 {
				return false
			}
		}
		return (state.ResolvingPart == "" || part(string(state.ResolvingPart))) && (state.PlaybackRetryError == playback.NoError || state.PlaybackPart != "")
	default:
		return false
	}
}

// Walk the fixed observation type before unmarshalling: every declared field is
// required exactly once with its exact case and non-null type. encoding/json's
// zero values, case aliases and duplicate-key overwrite cannot become evidence.
func requiredValue(d *json.Decoder, typ reflect.Type) error {
	bad := errors.New("invalid required field")
	switch typ.Kind() {
	case reflect.Struct:
		if t, err := d.Token(); err != nil || t != json.Delim('{') {
			return bad
		}
		fields := make(map[string]reflect.Type, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" {
				name = f.Name
			}
			fields[name] = f.Type
		}
		for d.More() {
			t, err := d.Token()
			name, ok := t.(string)
			if err != nil || !ok {
				return bad
			}
			field, ok := fields[name]
			if !ok {
				return bad
			}
			delete(fields, name)
			if err := requiredValue(d, field); err != nil {
				return err
			}
		}
		if t, err := d.Token(); err != nil || t != json.Delim('}') || len(fields) != 0 {
			return bad
		}
	case reflect.Slice:
		if t, err := d.Token(); err != nil || t != json.Delim('[') {
			return bad
		}
		count := 0
		for d.More() {
			count++
			if count > 16 {
				return bad
			}
			if err := requiredValue(d, typ.Elem()); err != nil {
				return err
			}
		}
		if t, err := d.Token(); err != nil || t != json.Delim(']') {
			return bad
		}
	default:
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(raw, []byte("null")) {
			return bad
		}
		if json.Unmarshal(raw, reflect.New(typ).Interface()) != nil {
			return bad
		}
	}
	return nil
}
