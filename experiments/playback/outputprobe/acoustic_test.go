package outputprobe

import (
	"fmt"
	"math"
	"testing"
)

func syntheticPulse(rate int, duration, scale float64, frequencies []float64, offset float64) []float64 {
	samples := make([]float64, int((offset+duration+0.15)*float64(rate)))
	start, n := int(offset*float64(rate)), int(duration*float64(rate))
	ramp := min(n/8, rate/200)
	for i := 0; i < n; i++ {
		window := 1.0
		if i < ramp {
			window = 0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(ramp))
		}
		if n-1-i < ramp {
			window = 0.5 - 0.5*math.Cos(math.Pi*float64(n-1-i)/float64(ramp))
		}
		for _, hz := range frequencies {
			samples[start+i] += 0.12 * window * math.Sin(2*math.Pi*hz*scale*float64(i)/float64(rate))
		}
	}
	return samples
}
func TestAcousticIDsAcrossRatesAndPitch(t *testing.T) {
	for _, rate := range []float64{0.5, 1, 2} {
		for _, pitchScaled := range []bool{false, true} {
			for id, hz := range identifierHz {
				t.Run(fmt.Sprintf("rate%g/pitch%t/id%d", rate, pitchScaled, id+1), func(t *testing.T) {
					scale := 1.0
					if pitchScaled {
						scale = rate
					}
					samples := syntheticPulse(SampleRate, 0.1/rate, scale, []float64{440, hz}, 0.1)
					events, err := DecodeAcoustic(samples, SampleRate)
					if err != nil || len(events) != 1 || events[0].ID != id+1 {
						t.Fatalf("decoded=%+v err=%v", events, err)
					}
					if events[0].QualifiedForPhysical {
						t.Fatal("synthetic label claimed physical qualification")
					}
					if len(events[0].Peaks) != 2 {
						t.Fatalf("unexpected components: %+v", events[0].Peaks)
					}
				})
			}
		}
	}
}
func TestAcousticRejectsUnsupportedSignals(t *testing.T) {
	cases := []struct {
		name             string
		freq             []float64
		offset, duration float64
	}{
		{"reference-only", []float64{440}, .1, .1}, {"identifier-only", []float64{770}, .1, .1}, {"unknown-ratio", []float64{440, 870}, .1, .1}, {"ambiguous-IDs", []float64{440, 770, 990}, .1, .1}, {"lower-out-of-range-anchor", []float64{100, 440, 770}, .1, .1}, {"seven-harmonics-regression", []float64{220, 440, 660, 880, 1100, 1320, 1540}, .1, .1}, {"no-silent-baseline", []float64{440, 770}, 0, .1}, {"too-short", []float64{440, 770}, .1, .01},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			events, err := DecodeAcoustic(syntheticPulse(SampleRate, c.duration, 1, c.freq, c.offset), SampleRate)
			if err != nil && len(events) == 0 {
				return
			}
			for _, ev := range events {
				if ev.ID != 0 || ev.Reason == "" {
					t.Fatalf("unsupported signal accepted: %+v", ev)
				}
			}
		})
	}
}
func TestAcousticMissingFundamentalsAreNotUniversalRejection(t *testing.T) {
	ev, err := DecodeAcoustic(syntheticPulse(SampleRate, .1, 1, []float64{880, 1540}, .1), SampleRate)
	if err != nil || len(ev) != 1 || ev[0].ID != 1 || ev[0].QualifiedForPhysical {
		t.Fatalf("indistinguishable fourth/seventh harmonic case: %+v %v", ev, err)
	}
	// This is a plausible digital ID only; AnalyzeSegment separately requires
	// retained decoder evidence plus actual same-mode setup qualification.
}
func TestAcousticCropsMissingAndDuplicatePulsesPreserveActualIDs(t *testing.T) {
	for _, ids := range [][]int{{3, 4, 5, 6}, {1, 3, 4, 6}, {2, 2, 3, 4}} {
		// More than one full event-period offset; no visual timestamps are supplied.
		samples := make([]float64, SampleRate*9)
		for i, id := range ids {
			pulse := syntheticPulse(SampleRate, .1, 1, []float64{440, identifierHz[id-1]}, 2.3+float64(i))
			for j, v := range pulse {
				samples[j] += v
			}
		}
		events, err := DecodeAcoustic(samples, SampleRate)
		if err != nil || len(events) != len(ids) {
			t.Fatalf("%v: %+v %v", ids, events, err)
		}
		for i, id := range ids {
			if events[i].ID != id {
				t.Fatalf("invented matching for %v: %+v", ids, events)
			}
		}
	}
}
func TestAcousticMalformed(t *testing.T) {
	for _, samples := range [][]float64{nil, {math.NaN()}, {math.Inf(1)}, {9}} {
		if _, err := DecodeAcoustic(samples, SampleRate); err == nil {
			t.Fatal("invalid PCM accepted")
		}
	}
	if _, err := DecodeAcoustic([]float64{0}, 1); err == nil {
		t.Fatal("invalid sample rate")
	}
}
