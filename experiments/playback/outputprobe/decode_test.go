package outputprobe

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestDecodeRejectsMetadataBeforeReadingMedia(t *testing.T) {
	video := StreamIdentity{Codec: "h264", Type: "video", Width: Width, Height: Height}
	audio := StreamIdentity{Codec: "aac", Type: "audio", SampleRate: "48000", Channels: 1}
	for _, tc := range []struct {
		name, duration string
		streams        []StreamIdentity
	}{
		{"extra-stream", "8", []StreamIdentity{video, audio, {Codec: "text", Type: "subtitle"}}},
		{"duplicate-audio", "8", []StreamIdentity{video, audio, audio}},
		{"missing-audio", "8", []StreamIdentity{video}},
		{"wrong-duration", "9", []StreamIdentity{video, audio}},
		{"wrong-codec", "8", []StreamIdentity{{Codec: "hevc", Type: "video", Width: Width, Height: Height}, audio}},
		{"wrong-sample-rate", "8", []StreamIdentity{video, {Codec: "aac", Type: "audio", SampleRate: "44100", Channels: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			run := func(tool string, args []string, limit int) ([]byte, error) {
				calls++
				if tool != "probe" {
					t.Fatal("bad metadata proceeded to decode")
				}
				return json.Marshal(map[string]any{"streams": tc.streams, "format": map[string]string{"duration": tc.duration}})
			}
			if _, err := decodeMedia(t.TempDir(), "combined.mp4", "decoder", "probe", run); err == nil {
				t.Fatal("invalid fixture metadata accepted")
			}
			if calls != 1 {
				t.Fatal("unexpected tool invocation")
			}
		})
	}
	if _, err := decodeMedia(t.TempDir(), "combined.mp4", "decoder", "probe", func(string, []string, int) ([]byte, error) { return nil, errors.New("decode failure") }); err == nil {
		t.Fatal("tool failure hidden")
	}
}
func TestTimestampParsingAndAudioMapping(t *testing.T) {
	for _, s := range []string{"NaN", "Inf", "-Inf", "garbage", ""} {
		if _, err := number(s); err == nil {
			t.Fatalf("invalid time %s", s)
		}
	}
	d := DecodedMedia{AudioFrames: []AudioFrame{{PTS: 0.021333, FirstSample: 1024, Samples: 1024}}}
	got, err := audioPTS(d, 1048)
	if err != nil || math.Abs(got-.021833) > 1e-9 {
		t.Fatalf("did not use presentation timestamp: %g %v", got, err)
	}
	if _, err := audioPTS(d, 1000); err == nil {
		t.Fatal("missing frame map accepted")
	}
}

func TestRetainedAuditRejectsUnresolvedComponent(t *testing.T) {
	audio, err := DecodeAcoustic(SourcePCM(), SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	combined := DecodedMedia{
		Name: "combined.mp4", Audio: audio,
		DecodedVideoSHA256: "unit-video", DecodedAudioSHA256: "unit-audio",
		AudioFrames: []AudioFrame{{PTS: 0, FirstSample: 0, Samples: SampleRate * 8}},
	}
	for _, e := range DefaultRecipe().Events {
		combined.Video = append(combined.Video, VisualEvent{ID: e.ID, FirstFrame: e.FirstFrame, EndFrame: e.EndFrame, PTS: float64(e.FirstFrame) / FrameRate})
	}
	video, separateAudio := combined, combined
	video.Name, separateAudio.Name = "video.mp4", "audio.m4a"
	m := TimingManifest{Decoded: []DecodedMedia{combined, video, separateAudio}}
	if err := validateDecoded(&m); err != nil {
		t.Fatalf("valid unit audit: %v", err)
	}
	// Keep combined/separate evidence equal, but leave a conflicting unresolved
	// component beside a previously valid ID. The retained audit must reject it.
	m.Decoded[0].Audio[0].UnresolvedPeaks = []SpectralPeak{{FrequencyHz: 220, SustainedSeconds: .015}}
	if err := validateDecoded(&m); err == nil {
		t.Fatal("unresolved component accepted in retained decoded audit")
	}
}
