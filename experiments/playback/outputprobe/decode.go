package outputprobe

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

type StreamIdentity struct {
	Codec      string `json:"codec_name"`
	Type       string `json:"codec_type"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	SampleRate string `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}
type AudioFrame struct {
	PTS         float64 `json:"pts_seconds"`
	FirstSample int     `json:"first_sample"`
	Samples     int     `json:"samples"`
}
type VisualEvent struct {
	ID         int     `json:"id"`
	FirstFrame int     `json:"first_frame"`
	EndFrame   int     `json:"end_frame_exclusive"`
	PTS        float64 `json:"pts_seconds"`
}
type toolCall func(string, []string, int) ([]byte, error)

func sumBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func number(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || !finite(v) {
		return 0, errors.New("invalid decoded timestamp")
	}
	return v, nil
}
func decodeMedia(dir, name, ffmpeg, ffprobe string, run toolCall) (DecodedMedia, error) {
	d := DecodedMedia{Name: name}
	path := filepath.Join(dir, name)
	b, err := run(ffprobe, []string{"-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height,sample_rate,channels:format=duration", "-of", "json", path}, 64<<10)
	if err != nil {
		return d, err
	}
	var metadata struct {
		Streams []StreamIdentity `json:"streams"`
		Format  struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err = json.Unmarshal(b, &metadata); err != nil {
		return d, err
	}
	d.Streams = metadata.Streams
	d.Duration, err = number(metadata.Format.Duration)
	if err != nil || math.Abs(d.Duration-Seconds) > 1e-6 {
		return d, errors.New("fixture duration is not eight seconds")
	}
	video, audio := 0, 0
	for _, s := range d.Streams {
		switch s.Type {
		case "video":
			video++
			if s.Codec != "h264" || s.Width != Width || s.Height != Height {
				return d, errors.New("unexpected video stream")
			}
		case "audio":
			audio++
			if s.Codec != "aac" || s.SampleRate != "48000" || s.Channels != 1 {
				return d, errors.New("unexpected audio stream")
			}
		default:
			return d, errors.New("unexpected extra stream")
		}
	}
	wantVideo, wantAudio := 1, 1
	if name == "video.mp4" {
		wantAudio = 0
	}
	if name == "audio.m4a" {
		wantVideo = 0
	}
	if video != wantVideo || audio != wantAudio {
		return d, errors.New("stream identity mismatch")
	}
	if video == 1 {
		b, err = run(ffprobe, []string{"-v", "error", "-select_streams", "v:0", "-show_frames", "-show_entries", "frame=best_effort_timestamp_time", "-of", "json", path}, 512<<10)
		if err != nil {
			return d, err
		}
		var frames struct {
			Frames []struct {
				PTS string `json:"best_effort_timestamp_time"`
			} `json:"frames"`
		}
		if err = json.Unmarshal(b, &frames); err != nil {
			return d, err
		}
		for _, f := range frames.Frames {
			p, e := number(f.PTS)
			if e != nil {
				return d, e
			}
			d.VideoPTS = append(d.VideoPTS, p)
		}
		raw, e := run(ffmpeg, []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", path, "-map", "0:v:0", "-fps_mode", "passthrough", "-pix_fmt", "gray", "-f", "rawvideo", "pipe:1"}, Width*Height*FrameRate*(Seconds+1))
		if e != nil {
			return d, e
		}
		if len(raw) != len(d.VideoPTS)*Width*Height || len(d.VideoPTS) != Seconds*FrameRate {
			return d, errors.New("decoded video frame count mismatch")
		}
		d.DecodedVideoSHA256 = sumBytes(raw)
		d.Video, err = detectVisual(raw, d.VideoPTS)
		if err != nil {
			return d, err
		}
	}
	if audio == 1 {
		b, err = run(ffprobe, []string{"-v", "error", "-select_streams", "a:0", "-show_frames", "-show_entries", "frame=best_effort_timestamp_time,nb_samples", "-of", "json", path}, 512<<10)
		if err != nil {
			return d, err
		}
		var frames struct {
			Frames []struct {
				PTS     string `json:"best_effort_timestamp_time"`
				Samples int    `json:"nb_samples"`
			} `json:"frames"`
		}
		if err = json.Unmarshal(b, &frames); err != nil {
			return d, err
		}
		first := 0
		for _, f := range frames.Frames {
			p, e := number(f.PTS)
			if e != nil || f.Samples <= 0 || f.Samples > 8192 {
				return d, errors.New("invalid decoded audio frame")
			}
			if math.Abs(p-float64(first)/SampleRate) > 1.1e-6 {
				return d, errors.New("audio timestamp discontinuity or nonzero origin")
			}
			d.AudioFrames = append(d.AudioFrames, AudioFrame{p, first, f.Samples})
			first += f.Samples
		}
		raw, e := run(ffmpeg, []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", path, "-map", "0:a:0", "-c:a", "pcm_f32le", "-f", "f32le", "pipe:1"}, SampleRate*(Seconds+1)*4)
		if e != nil {
			return d, e
		}
		if len(raw)%4 != 0 || len(raw)/4 != first || first < Seconds*SampleRate || first > SampleRate*(Seconds+1) {
			return d, errors.New("decoded PCM/timestamp count mismatch")
		}
		d.DecodedAudioSHA256 = sumBytes(raw)
		samples := make([]float64, first)
		for i := range samples {
			samples[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
		}
		d.Audio, err = DecodeAcoustic(samples, SampleRate)
		if err != nil {
			return d, err
		}
	}
	return d, nil
}
func detectVisual(raw []byte, pts []float64) ([]VisualEvent, error) {
	var events []VisualEvent
	r := DefaultRecipe()
	lastID := 0
	for i, p := range pts {
		if !finite(p) || math.Abs(p-float64(i)/FrameRate) > 1.1e-6 {
			return nil, errors.New("unexpected video frame presentation time")
		}
		frame := raw[i*Width*Height : (i+1)*Width*Height]
		on := frame[100*Width+320] > 180
		id := 0
		if on {
			for bit, rect := range r.Bits {
				if frame[(rect.Y+rect.Height/2)*Width+rect.X+rect.Width/2] > 180 {
					id |= 1 << bit
				}
			}
			if id < 1 || id > 6 {
				return nil, errors.New("unrecognized visual marker")
			}
		}
		if id != 0 && id != lastID {
			events = append(events, VisualEvent{id, i, i + 1, p})
		} else if id != 0 {
			events[len(events)-1].EndFrame = i + 1
		}
		lastID = id
	}
	return events, nil
}
func audioPTS(d DecodedMedia, index int) (float64, error) {
	for _, f := range d.AudioFrames {
		if index >= f.FirstSample && index < f.FirstSample+f.Samples {
			return f.PTS + float64(index-f.FirstSample)/SampleRate, nil
		}
	}
	return 0, errors.New("audio event outside decoded timestamp map")
}
func validateDecoded(m *TimingManifest) error {
	if len(m.Decoded) != 3 {
		return errors.New("three decoded media audits required")
	}
	combined, video, audio := m.Decoded[0], m.Decoded[1], m.Decoded[2]
	if combined.Name != "combined.mp4" || video.Name != "video.mp4" || audio.Name != "audio.m4a" {
		return errors.New("decoded audit identities differ")
	}
	if combined.DecodedVideoSHA256 == "" || combined.DecodedVideoSHA256 != video.DecodedVideoSHA256 || combined.DecodedAudioSHA256 == "" || combined.DecodedAudioSHA256 != audio.DecodedAudioSHA256 || !reflect.DeepEqual(combined.Video, video.Video) || !reflect.DeepEqual(combined.VideoPTS, video.VideoPTS) || !reflect.DeepEqual(combined.Audio, audio.Audio) || !reflect.DeepEqual(combined.AudioFrames, audio.AudioFrames) {
		return errors.New("separate streams are not decoded-equivalent to combined media")
	}
	if len(combined.Video) != 6 || len(combined.Audio) != 6 {
		return errors.New("missing or extra decoded events")
	}
	var events []DecodedEvent
	for i, source := range DefaultRecipe().Events {
		v, a := combined.Video[i], combined.Audio[i]
		if v.ID != source.ID || a.ID != source.ID || a.Reason != "" || len(a.UnresolvedPeaks) != 0 || len(a.CandidateIDs) != 1 || a.CandidateIDs[0] != source.ID || len(a.Peaks) < 2 {
			return errors.New("decoded acoustic or visual ID is missing, ambiguous or out of order")
		}
		if a.QualifiedForPhysical || len(a.SharedSupport) == 0 || a.BaselineStartSample < 0 || a.BaselineEndSample <= a.BaselineStartSample || a.BaselineEndSample > a.FirstSample {
			return errors.New("missing or contradictory acoustic support/baseline evidence")
		}
		for _, support := range a.SharedSupport {
			if support.FirstSample < a.FirstSample || support.EndSample > a.EndSample || support.EndSample-support.FirstSample < SampleRate/50 || support.LocalizationUncertaintySamples <= 0 {
				return errors.New("invalid simultaneous acoustic support")
			}
		}
		if v.FirstFrame != source.FirstFrame || v.EndFrame != source.EndFrame {
			return errors.New("decoded visual pulse differs from recipe")
		}
		if a.EndSample <= a.FirstSample || math.Abs(float64(a.FirstSample-source.FirstSample)/SampleRate) > 1.0/FrameRate || math.Abs(float64(a.EndSample-source.EndSample)/SampleRate) > 1.0/FrameRate {
			return errors.New("decoded acoustic window differs from recipe")
		}
		// Establish intact fundamentals against the recipe; spectral coincidence
		// alone is insufficient to qualify even the generated fixture.
		ref, identifier := false, false
		for _, p := range a.Peaks {
			if math.Abs(p.FrequencyHz-source.ReferenceHz) <= 1 {
				ref = true
			}
			if math.Abs(p.FrequencyHz-source.IdentifierHz) <= 1 {
				identifier = true
			}
			if !finite(p.Power) || !finite(p.ObservedBaselinePower) || p.ObservedBaselinePower < 0 || !finite(p.DetectionFloorPower) || p.DetectionFloorPower <= 0 || p.Power < 100*p.BaselinePower || p.BaselinePower <= 0 || p.SustainedSeconds < 0.02 || len(p.Support) == 0 {
				return errors.New("invalid retained acoustic detector evidence")
			}
		}
		if !ref || !identifier {
			return errors.New("decoded fundamentals do not match source recipe")
		}
		ap, err := audioPTS(combined, a.FirstSample)
		if err != nil {
			return err
		}
		offset := ap - v.PTS
		if math.Abs(offset) > 1.0/FrameRate {
			return errors.New("decoded A/V offset exceeds one frame")
		}
		events = append(events, DecodedEvent{source.ID, v.PTS, ap, offset, 1e-6, 0.001 + 1.0/SampleRate})
	}
	m.Events = events
	return nil
}

// VerifyDirectory checks the two manifests and bound media identities without
// running a native player or media tool. Decoded audit provenance is hash-bound
// to the media; callers must retain the trusted generator/candidate identity.
func VerifyDirectory(dir string) (*TimingManifest, error) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid fixture directory")
	}
	load := func(name string, maxBytes int64, target any) error {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
			return errors.New("invalid bounded manifest")
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		after, err := f.Stat()
		if err != nil || !os.SameFile(info, after) {
			return errors.New("manifest changed during open")
		}
		b, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
		if err != nil {
			return err
		}
		if int64(len(b)) != info.Size() || int64(len(b)) > maxBytes {
			return errors.New("manifest size changed")
		}
		return json.Unmarshal(b, target)
	}
	var m TimingManifest
	if err = load(TimingManifestName, 2<<20, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion != 1 || m.Status != "validated-fixture-only" || !reflect.DeepEqual(m.Recipe, DefaultRecipe()) || len(m.Media) != 3 || len(m.Tools) != 2 {
		return nil, errors.New("invalid timing manifest")
	}
	var server ServerManifest
	if err = load("manifest.json", 32<<10, &server); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(m.Media, server.Fixtures) {
		return nil, errors.New("manifests disagree")
	}
	for i, name := range []string{"combined.mp4", "video.mp4", "audio.m4a"} {
		item := m.Media[i]
		if item.Name != name {
			return nil, errors.New("unexpected fixture name")
		}
		h, n, err := hashRegular(filepath.Join(dir, name), MaxMediaBytes)
		if err != nil || h != item.SHA256 || n != item.Bytes {
			return nil, errors.New("fixture identity mismatch")
		}
	}
	prior := append([]DecodedEvent(nil), m.Events...)
	if err = validateDecoded(&m); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(prior, m.Events) {
		return nil, errors.New("recorded event summary differs from retained decoded audit")
	}
	return &m, nil
}
func (m TimingManifest) Summary() string {
	return fmt.Sprintf("%s: %d media files, %d decoded event pairs; physical output not-tested", m.Status, len(m.Media), len(m.Events))
}
