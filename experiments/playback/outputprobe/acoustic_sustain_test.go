package outputprobe

import (
	"fmt"
	"math"
	"testing"
)

func interruptedID(duration, start float64, sequential bool) []float64 {
	rate := SampleRate
	x := make([]float64, rate/2)
	for i := rate / 5; i < rate*3/10; i++ {
		t := float64(i-rate/5) / float64(rate)
		if !sequential || t < .05 {
			x[i] = .15 * math.Sin(2*math.Pi*440*t)
		}
		if sequential {
			if t >= .05 {
				x[i] += .15 * math.Sin(2*math.Pi*770*t)
			}
		} else if u := t - start; u >= 0 && u < duration {
			x[i] += .15 * (.5 - .5*math.Cos(2*math.Pi*u/duration)) * math.Sin(2*math.Pi*770*t)
		}
	}
	return x
}

func TestAcousticRequiresSimultaneousDuration(t *testing.T) {
	cases := []struct {
		name string
		pcm  []float64
	}{{"nonoverlapping-halves", interruptedID(0, 0, true)}}
	for _, duration := range []float64{.005, .010, .019} {
		for _, start := range []float64{.030, .031, .034, .037, .0405} {
			cases = append(cases, struct {
				name string
				pcm  []float64
			}{fmt.Sprintf("short-%g-at-%g", duration, start), interruptedID(duration, start, false)})
		}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			events, err := DecodeAcoustic(c.pcm, SampleRate)
			if err != nil && len(events) == 0 {
				return
			}
			for _, e := range events {
				if e.ID != 0 {
					t.Fatalf("less than 20ms common support accepted as ID%d: %+v", e.ID, e)
				}
			}
		})
	}
}

func TestAcousticRetainsJointSupportAndBaseline(t *testing.T) {
	events, err := DecodeAcoustic(syntheticPulse(SampleRate, .1, 1, []float64{440, 770}, .1), SampleRate)
	if err != nil || len(events) != 1 || events[0].ID != 1 {
		t.Fatalf("%+v %v", events, err)
	}
	e := events[0]
	if len(e.SharedSupport) == 0 {
		t.Fatal("missing coincident support")
	}
	for _, shared := range e.SharedSupport {
		if shared.EndSample-shared.FirstSample < SampleRate/50 || shared.LocalizationUncertaintySamples != SampleRate/200 || shared.FirstSample < e.FirstSample || shared.EndSample > e.EndSample {
			t.Fatalf("invalid localized shared support %+v", shared)
		}
	}
	for _, p := range e.Peaks {
		if p.ObservedBaselinePower != 0 || p.DetectionFloorPower <= 0 || p.BaselinePower < p.DetectionFloorPower {
			t.Fatalf("baseline floor presented as measured power %+v", p)
		}
		if p.SustainedSeconds < .02 || len(p.Support) == 0 {
			t.Fatalf("missing actual component duration %+v", p)
		}
	}
}

func TestAcousticRatioUncertaintyCannotForceBoundaryMatch(t *testing.T) {
	for _, c := range []struct {
		name  string
		ratio float64
		id    int
	}{
		{"inside", 1.765, 1}, {"ambiguous-upper-tolerance", 1.7695, 0}, {"outside", 1.78, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := DecodeAcoustic(syntheticPulse(SampleRate, .1, 1, []float64{440, 440 * c.ratio}, .1), SampleRate)
			if err != nil || len(e) != 1 || e[0].ID != c.id {
				t.Fatalf("%+v %v", e, err)
			}
		})
	}
}

func TestAcousticSevenHarmonicsKeepLowestAnchor(t *testing.T) {
	e, err := DecodeAcoustic(syntheticPulse(SampleRate, .1, 1, []float64{220, 440, 660, 880, 1100, 1320, 1540}, .1), SampleRate)
	if err != nil || len(e) != 1 || e[0].ID != 0 || len(e[0].Peaks) == 0 {
		t.Fatalf("%+v %v", e, err)
	}
	if math.Abs(e[0].Peaks[0].FrequencyHz-220) > e[0].Peaks[0].UncertaintyHz {
		t.Fatalf("lost lowest harmonic anchor %+v", e[0].Peaks)
	}
}

func TestAcousticInterruptedComponentCannotAccumulateSupport(t *testing.T) {
	x := make([]float64, SampleRate/2)
	for i := SampleRate / 5; i < SampleRate*3/10; i++ {
		tm := float64(i-SampleRate/5) / SampleRate
		x[i] = .15 * math.Sin(2*math.Pi*440*tm)
		// Repeated 4ms bursts separated by 1ms gaps never sustain 20ms.
		if (i-SampleRate/5)%(SampleRate/200) < SampleRate/250 {
			x[i] += .15 * math.Sin(2*math.Pi*770*tm)
		}
	}
	e, err := DecodeAcoustic(x, SampleRate)
	if err != nil && len(e) == 0 {
		return
	}
	for _, v := range e {
		if v.ID != 0 {
			t.Fatalf("interrupted component accumulated support: %+v", v)
		}
	}
}

func TestAcousticAnalysisStorageBound(t *testing.T) {
	pulse := make([]float64, (1<<20)/8+1)
	if peaks, unresolved := qualifiedPeaks(pulse, make([]float64, 1440), SampleRate); len(peaks) != 0 || len(unresolved) != 0 {
		t.Fatal("oversized FFT input accepted")
	}
}

func localLowerPulse(sampleRate int, total, duration, start, amplitude, pitch float64) []float64 {
	x := make([]float64, sampleRate/2)
	for i := 0; i < int(float64(sampleRate)*total); i++ {
		tm := float64(i) / float64(sampleRate)
		x[sampleRate/10+i] = .12 * (math.Sin(2*math.Pi*440*pitch*tm) + math.Sin(2*math.Pi*770*pitch*tm))
		if tm >= start && tm < start+duration {
			x[sampleRate/10+i] += amplitude * math.Sin(2*math.Pi*220*pitch*tm)
		}
	}
	return x
}

func checkLocalLowerRejected(t *testing.T, samples []float64, rate int) {
	t.Helper()
	e, err := DecodeAcoustic(samples, rate)
	if err != nil || len(e) != 1 {
		t.Fatalf("expected one retained pulse: %+v %v", e, err)
	}
	if e[0].ID != 0 {
		t.Fatalf("local lower component erased before qualification: %+v", e[0])
	}
	if e[0].Reason == "" {
		t.Fatalf("missing rejection reason: %+v", e[0])
	}
	for _, p := range e[0].UnresolvedPeaks {
		if p.SustainedSeconds >= .02 || len(p.Support) == 0 {
			t.Fatalf("unresolved local duration reported as qualified: %+v", p)
		}
		for _, s := range p.Support {
			if s.FirstSample < e[0].FirstSample || s.EndSample > e[0].EndSample || s.EndSample-s.FirstSample+2*s.LocalizationUncertaintySamples < (rate+49)/50 {
				t.Fatalf("unresolved duration interval cannot include 20ms: %+v", s)
			}
		}
	}
}

func lowerPlacements(total, duration float64) map[string]float64 {
	return map[string]float64{
		"head": 0, "tail": total - duration, "interior": (total - duration) / 2,
		"unaligned-head": .0015, "unaligned-tail": max(0, total-duration-.0015),
	}
}

func TestAcousticLocalLowerComponentCannotDisappear(t *testing.T) {
	for _, sampleRate := range []int{8000, 48000, 96000} {
		for _, duration := range []float64{.021, .025, .03, .04, .05} {
			for placement, start := range lowerPlacements(.2, duration) {
				t.Run(fmt.Sprintf("%dHz/%g/%s", sampleRate, duration, placement), func(t *testing.T) {
					checkLocalLowerRejected(t, localLowerPulse(sampleRate, .2, duration, start, .03, 1), sampleRate)
				})
			}
		}
	}
}

func TestAcousticLocalLowerAcrossPlaybackModes(t *testing.T) {
	for _, speed := range []float64{.5, 1, 2} {
		for _, pitchScaled := range []bool{false, true} {
			pitch := 1.0
			if pitchScaled {
				pitch = speed
			}
			for _, c := range []struct{ duration, amplitude float64 }{{.021, .03}, {.025, .03}, {.03, .03}, {.04, .01}, {.05, .003}} {
				for placement, start := range lowerPlacements(.1/speed, c.duration) {
					t.Run(fmt.Sprintf("speed%g/pitch%t/%g/%s", speed, pitchScaled, c.duration, placement), func(t *testing.T) {
						checkLocalLowerRejected(t, localLowerPulse(SampleRate, .1/speed, c.duration, start, c.amplitude, pitch), SampleRate)
					})
				}
			}
		}
	}
}
