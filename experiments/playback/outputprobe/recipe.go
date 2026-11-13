// Package outputprobe prepares synthetic paired events. It does not record
// devices or claim that digital decoding measures physical playback output.
package outputprobe

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

const (
	Width              = 640
	Height             = 360
	FrameRate          = 30
	SampleRate         = 48000
	Seconds            = 8
	MaxMediaBytes      = 2 << 20
	TimingManifestName = "timing.json"
)

var identifierHz = [...]float64{770, 990, 1210, 1430, 1650, 1870}

type SourceEvent struct {
	ID           int     `json:"id"`
	FirstFrame   int     `json:"first_frame"`
	EndFrame     int     `json:"end_frame_exclusive"`
	FirstSample  int     `json:"first_sample"`
	EndSample    int     `json:"end_sample_exclusive"`
	ReferenceHz  float64 `json:"reference_hz"`
	IdentifierHz float64 `json:"identifier_hz"`
}
type Rectangle struct{ X, Y, Width, Height int }
type Recipe struct {
	Width, Height, FramesPerSecond, SamplesPerSecond, DurationSeconds int
	ComponentAmplitude                                                float64     `json:"component_amplitude"`
	RampSamples                                                       int         `json:"ramp_samples"`
	Pulse                                                             Rectangle   `json:"pulse"`
	Bits                                                              []Rectangle `json:"binary_marker_lsb_first"`
	Background, PulseLuma, OffBitLuma, OnBitLuma                      uint8
	AudioEncoding                                                     string        `json:"audio_encoding"`
	Events                                                            []SourceEvent `json:"events"`
}

func DefaultRecipe() Recipe {
	r := Recipe{Width: Width, Height: Height, FramesPerSecond: FrameRate, SamplesPerSecond: SampleRate, DurationSeconds: Seconds, ComponentAmplitude: 0.16, RampSamples: 240, Pulse: Rectangle{32, 32, 576, 180}, Bits: []Rectangle{{100, 250, 80, 60}, {230, 250, 80, 60}, {360, 250, 80, 60}}, Background: 16, PulseLuma: 230, OffBitLuma: 40, OnBitLuma: 230, AudioEncoding: "mono signed 16-bit little-endian; sum of two sinusoids; 5ms raised-cosine onset/offset ramps"}
	for i, hz := range identifierHz {
		second := i + 1
		r.Events = append(r.Events, SourceEvent{second, second * FrameRate, second*FrameRate + 3, second * SampleRate, second*SampleRate + SampleRate/10, 440, hz})
	}
	return r
}
func WriteVideo(w io.Writer) error {
	r := DefaultRecipe()
	frame := make([]byte, Width*Height)
	for n := 0; n < Seconds*FrameRate; n++ {
		for i := range frame {
			frame[i] = r.Background
		}
		for _, e := range r.Events {
			if n >= e.FirstFrame && n < e.EndFrame {
				fill(frame, r.Pulse, r.PulseLuma)
				for bit, rect := range r.Bits {
					v := r.OffBitLuma
					if e.ID&(1<<bit) != 0 {
						v = r.OnBitLuma
					}
					fill(frame, rect, v)
				}
			}
		}
		if _, err := w.Write(frame); err != nil {
			return err
		}
	}
	return nil
}
func fill(frame []byte, r Rectangle, v byte) {
	for y := r.Y; y < r.Y+r.Height; y++ {
		for x := r.X; x < r.X+r.Width; x++ {
			frame[y*Width+x] = v
		}
	}
}
func SourcePCM() []float64 {
	r := DefaultRecipe()
	samples := make([]float64, Seconds*SampleRate)
	for _, e := range r.Events {
		for n := e.FirstSample; n < e.EndSample; n++ {
			j := n - e.FirstSample
			remain := e.EndSample - 1 - n
			envelope := 1.0
			if j < r.RampSamples {
				envelope = 0.5 - 0.5*math.Cos(math.Pi*float64(j)/float64(r.RampSamples))
			}
			if remain < r.RampSamples {
				envelope = 0.5 - 0.5*math.Cos(math.Pi*float64(remain)/float64(r.RampSamples))
			}
			t := float64(j) / SampleRate
			samples[n] = r.ComponentAmplitude * envelope * (math.Sin(2*math.Pi*e.ReferenceHz*t) + math.Sin(2*math.Pi*e.IdentifierHz*t))
		}
	}
	return samples
}
func WriteAudio(w io.Writer) error {
	samples := SourcePCM()
	b := make([]byte, len(samples)*2)
	for i, v := range samples {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(int16(math.Round(v*32767))))
	}
	_, err := w.Write(b)
	return err
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validateSamples(samples []float64, rate int) error {
	if rate < 8000 || rate > 192000 || len(samples) == 0 || len(samples) > rate*600 {
		return errors.New("invalid bounded PCM input")
	}
	for _, v := range samples {
		if !finite(v) || math.Abs(v) > 8 {
			return errors.New("invalid PCM sample")
		}
	}
	return nil
}
