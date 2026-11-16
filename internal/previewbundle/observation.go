package previewbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	"my-bilibili/internal/app"
)

// Observation contains only safe application/native state, never media inputs.
// Normal builds do not publish it. Audio track selection does not prove audibility.
type Observation struct {
	State         app.State
	Fullscreen    bool
	Resources     mpvprobe.Resources
	Tracks        mpvprobe.Tracks
	DisposalError contract.ErrorCode
	Delay         DelayObservation
}
type DelayObservation struct{ Requests, Active int64 }

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
	case contract.Idle, contract.Loading, contract.Ready, contract.Paused, contract.Playing, contract.Ended, contract.Failed, contract.Closed:
	default:
		return Observation{}, invalid
	}
	for _, code := range []contract.ErrorCode{s.Error, o.State.SelectionError, o.DisposalError} {
		switch code {
		case contract.NoError, contract.Invalid, contract.Unsupported, contract.Canceled, contract.Timeout, contract.Media, contract.IsClosed:
		default:
			return Observation{}, invalid
		}
	}
	if o.Resources.Cores < 0 || o.Resources.Renderers < 0 || o.Resources.Workers < 0 || s.Position < 0 || s.Duration < 0 || s.Rate < 0 || s.Rate > 3 || s.Volume < 0 || s.Volume > 1 || o.State.Rate < .25 || o.State.Rate > 3 || o.State.Volume < 0 || o.State.Volume > 1 {
		return Observation{}, invalid
	}
	v := o.State.Video
	if v.ID != "preview-video" || v.CoverID != "preview-cover" || v.Title == "" || v.Creator == "" || v.Description == "" || len(v.Parts) != 2 || v.Parts[0].ID != "part-1" || v.Parts[1].ID != "part-2" {
		return Observation{}, invalid
	}
	for _, p := range v.Parts {
		if p.Title == "" || p.Duration <= 0 {
			return Observation{}, invalid
		}
	}
	for _, id := range []string{string(o.State.SelectedPart), string(o.State.PlaybackPart)} {
		if id != "part-1" && id != "part-2" {
			return Observation{}, invalid
		}
	}
	if s.Generation == 0 {
		return Observation{}, invalid
	}
	if o.Delay.Requests < 0 || o.Delay.Active < 0 || o.Delay.Active > o.Delay.Requests {
		return Observation{}, invalid
	}
	return o, nil
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
