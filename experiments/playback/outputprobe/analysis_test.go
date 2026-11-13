package outputprobe

import (
	"math"
	"strings"
	"testing"
)

func point(v float64) Interval        { return Interval{v, v} }
func uncertain(v, u float64) Interval { return Interval{v - u, v + u} }
func evidence() IdentityEvidence {
	return IdentityEvidence{Method: "independent", EvidenceSHA256: strings.Repeat("a", 64), Procedure: "synthetic known-ID control record"}
}
func measurementSegment(rate, fixtureOffset, captureSkew, residual float64) Segment {
	s := Segment{ID: "segment-1", Kind: "steady", BoundaryEvidenceSHA256: strings.Repeat("b", 64), CaptureStart: 0, CaptureEnd: 30, SelectedRate: rate}
	s.Calibration = &CaptureCalibration{EvidenceSHA256: strings.Repeat("c", 64), Independent: true, Device: "synthetic", Mode: "known rational timestamps", Reference: "independent synthetic reference", Placement: "known synthetic zero distance"}
	for id := 1; id <= 6; id++ {
		media := float64(id)
		// Tiny explicit declared uncertainty avoids claiming exact real-number
		// equality from floating point construction of decimal timestamps.
		e := EventMeasurement{SegmentID: s.ID, VideoID: id, AudioID: id, DecodedVideoPTS: uncertain(media, 1e-10), DecodedAudioPTS: uncertain(media+fixtureOffset, 1e-10), CapturedVideo: uncertain(2+media/rate, 1e-10), CapturedAudio: uncertain(2+(media+fixtureOffset)/rate+captureSkew+residual, 1e-10), VideoIdentity: evidence(), AudioIdentity: evidence()}
		s.Events = append(s.Events, e)
		s.Calibration.Points = append(s.Calibration.Points, CalibrationPoint{EventID: id, Skew: point(captureSkew)})
	}
	return s
}
func contains(t *testing.T, got Interval, want float64) {
	t.Helper()
	if got.Low > want || got.High < want {
		t.Fatalf("%+v does not contain %.12g", got, want)
	}
}
func nearInterval(t *testing.T, got Interval, want, tolerance float64) {
	t.Helper()
	contains(t, got, want)
	if got.High-got.Low > tolerance {
		t.Fatalf("unexpected wide bound %+v", got)
	}
}

func TestAnalysisSignedTimeDomains(t *testing.T) {
	for _, rate := range []float64{.5, 1, 2} {
		for _, fixture := range []float64{-.02, .02} {
			for _, capture := range []float64{-.03, .03} {
				s := measurementSegment(rate, fixture, capture, .015)
				r, err := AnalyzeSegment(s)
				if err != nil || r.Status != "measured" {
					t.Fatalf("rate %v fixture %v capture %v: %s %v %+v", rate, fixture, capture, r.Status, err, r.Reasons)
				}
				nearInterval(t, r.VideoFit.Rate, rate, 1e-7)
				nearInterval(t, r.AudioFit.Rate, rate, 1e-7)
				for _, e := range r.Events {
					nearInterval(t, e.RawOffset, fixture/rate+capture+.015, 1e-7)
					nearInterval(t, *e.PhysicalOutputOffset, fixture/rate+.015, 1e-7)
					nearInterval(t, *e.MappedFixtureOffset, fixture/rate, 1e-7)
					nearInterval(t, *e.PlayerResidual, .015, 1e-7)
				}
			}
		}
	}
}

func TestAnalysisNonUnitPerfectPlayerCounterexamples(t *testing.T) {
	for _, rate := range []float64{.5, 2} {
		r, err := AnalyzeSegment(measurementSegment(rate, .02, 0, 0))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range r.Events {
			nearInterval(t, e.RawOffset, .02/rate, 1e-7)
			nearInterval(t, *e.PlayerResidual, 0, 1e-7)
		}
	}
}

func TestAnalysisRateDriftAndConfigurationIsNotMapping(t *testing.T) {
	s := measurementSegment(1, .02, 0, 0)
	s.SelectedRate = 2 // deliberately false configuration must not influence fit.
	for i := range s.Events {
		m := float64(i + 1)
		s.Events[i].CapturedAudio = uncertain(2+(m+.02)/.9, 1e-10)
	}
	r, err := AnalyzeSegment(s)
	if err != nil || r.Status != "measured" {
		t.Fatalf("%v %+v", err, r)
	}
	nearInterval(t, r.VideoFit.Rate, 1, 1e-7)
	nearInterval(t, r.AudioFit.Rate, .9, 1e-7)
	nearInterval(t, *r.RateDelta, -.1, 1e-7)
	// Independent audio intercept must not erase the accumulating skew.
	if r.Events[5].PlayerResidual.Low <= r.Events[0].PlayerResidual.High {
		t.Fatal("audio drift fitted away")
	}
}

func TestAnalysisConservativeUncertainty(t *testing.T) {
	s := measurementSegment(.5, .02, .03, .015)
	for i := range s.Events {
		e := &s.Events[i]
		e.DecodedVideoPTS = uncertain(float64(i+1), .001)
		e.DecodedAudioPTS = uncertain(float64(i+1)+.02, .002)
		e.CapturedVideo = uncertain(2+float64(i+1)*2, .003)
		e.CapturedAudio = uncertain(2+(float64(i+1)+.02)*2+.03+.015, .004)
		s.Calibration.Points[i].Skew = uncertain(.03, .005)
	}
	r, err := AnalyzeSegment(s)
	if err != nil || r.Status != "measured" {
		t.Fatalf("%v %s", err, r.Status)
	}
	contains(t, r.VideoFit.Slope, 2)
	for _, e := range r.Events {
		contains(t, e.RawOffset, .085)
		contains(t, *e.PhysicalOutputOffset, .055)
		contains(t, *e.PlayerResidual, .015)
		if e.PhysicalOutputOffset.High-e.PhysicalOutputOffset.Low < .024-1e-12 {
			t.Fatal("uncertainty was reduced as if errors independent")
		}
		if e.PlayerResidual.High-e.PlayerResidual.Low < .036-1e-12 {
			t.Fatal("fixture/slope uncertainty omitted")
		}
	}
}

func TestAnalysisKeepsUncalibratedRawData(t *testing.T) {
	for _, name := range []string{"missing", "dependent", "missing-reference", "missing-device", "bad-skew", "missing-event", "duplicate-event", "missing-evidence"} {
		t.Run(name, func(t *testing.T) {
			s := measurementSegment(1, .02, .03, 0)
			switch name {
			case "missing":
				s.Calibration = nil
			case "dependent":
				s.Calibration.Independent = false
			case "missing-reference":
				s.Calibration.Reference = ""
			case "missing-device":
				s.Calibration.Device = ""
			case "bad-skew":
				s.Calibration.Points[0].Skew = Interval{1, -1}
			case "missing-event":
				s.Calibration.Points = s.Calibration.Points[:5]
			case "duplicate-event":
				s.Calibration.Points[1].EventID = 1
			case "missing-evidence":
				s.Calibration.EvidenceSHA256 = ""
			}
			r, err := AnalyzeSegment(s)
			if err != nil || r.Status != "uncalibrated" {
				t.Fatalf("%v %s", err, r.Status)
			}
			if len(r.Input.Events) != 6 || len(r.Events) != 6 || r.VideoFit == nil {
				t.Fatal("raw data or valid mapping discarded")
			}
			for _, e := range r.Events {
				if e.PhysicalOutputOffset != nil || e.PlayerResidual != nil || e.CaptureCorrection != nil {
					t.Fatal("fabricated calibration")
				}
				contains(t, e.RawOffset, .05)
			}
		})
	}
}

func TestAnalysisRejectsMalformedAndUnqualifiedPairs(t *testing.T) {
	for _, name := range []string{"insufficient", "duplicate", "missing-interior", "mismatch", "unknown", "nonmonotonic", "nan", "infinity", "reversed", "segment", "boundary", "out-of-window", "identity", "decoder-unqualified", "post-seek-no-target"} {
		t.Run(name, func(t *testing.T) {
			s := measurementSegment(1, 0, 0, 0)
			switch name {
			case "insufficient":
				s.Events = s.Events[:3]
			case "duplicate":
				s.Events[1].VideoID = 1
				s.Events[1].AudioID = 1
			case "missing-interior":
				s.Events = append(s.Events[:2], s.Events[3:]...)
			case "mismatch":
				s.Events[1].AudioID = 3
			case "unknown":
				s.Events[1].AudioID = 0
			case "nonmonotonic":
				s.Events[1].CapturedAudio = s.Events[0].CapturedAudio
			case "nan":
				s.Events[1].CapturedVideo.Low = math.NaN()
			case "infinity":
				s.Events[1].DecodedAudioPTS.High = math.Inf(1)
			case "reversed":
				s.Events[1].CapturedVideo = Interval{5, 4}
			case "segment":
				s.Events[1].SegmentID = "other-seek"
			case "boundary":
				s.BoundaryEvidenceSHA256 = ""
			case "out-of-window":
				s.CaptureEnd = 3
			case "identity":
				s.Events[0].AudioIdentity = IdentityEvidence{}
			case "decoder-unqualified":
				s.Events[0].AudioIdentity = IdentityEvidence{Method: "decoder", EvidenceSHA256: strings.Repeat("a", 64)}
			case "post-seek-no-target":
				s.Kind = "post-seek"
			}
			r, err := AnalyzeSegment(s)
			if err == nil || r.Status != "rejected" || len(r.Reasons) == 0 {
				t.Fatalf("accepted %s", name)
			}
			if len(r.Input.Events) != len(s.Events) {
				t.Fatal("rejected input discarded")
			}
		})
	}
}

func TestAnalysisPostSeekCropAndQualifiedDecoder(t *testing.T) {
	s := measurementSegment(2, 0, 0, 0)
	s.Kind = "post-seek"
	target := 2.0
	s.SeekTarget = &target
	s.Events = s.Events[1:5]
	s.Calibration.Points = s.Calibration.Points[1:5]
	for i := range s.Events {
		s.Events[i].AudioIdentity = IdentityEvidence{Method: "decoder", EvidenceSHA256: strings.Repeat("d", 64), QualificationSHA256: strings.Repeat("e", 64)}
	}
	r, err := AnalyzeSegment(s)
	if err != nil || r.Status != "measured" {
		t.Fatalf("%v %s %+v", err, r.Status, r.Reasons)
	}
	if r.Events[0].ID != 2 || r.Events[3].ID != 5 {
		t.Fatal("cropped IDs relabeled")
	}
}

func TestAnalysisUnidentifiedMappingsKeepOffsets(t *testing.T) {
	for _, name := range []string{"nonlinear", "transient", "discontinuity", "rate-change"} {
		t.Run(name, func(t *testing.T) {
			s := measurementSegment(1, .02, 0, 0)
			switch name {
			case "nonlinear":
				s.Events[3].CapturedVideo = uncertain(6.1, 1e-10)
			case "transient":
				s.Kind = "transient"
			case "discontinuity":
				s.ContainsDiscontinuity = true
			case "rate-change":
				for i := 3; i < len(s.Events); i++ {
					s.Events[i].CapturedVideo = uncertain(5+float64(i-2)*.5, 1e-10)
				}
			}
			r, err := AnalyzeSegment(s)
			if err != nil || r.Status != "inconclusive" || r.VideoFit != nil {
				t.Fatalf("%v %s %+v", err, r.Status, r.VideoFit)
			}
			if len(r.Events) != 6 || len(r.Input.Events) != 6 {
				t.Fatal("transient points discarded")
			}
			for _, e := range r.Events {
				if e.MappedFixtureOffset != nil || e.PlayerResidual != nil {
					t.Fatal("invented mapping")
				}
				if e.PhysicalOutputOffset == nil {
					t.Fatal("calibrated raw output unnecessarily discarded")
				}
			}
		})
	}
}

func TestAnalysisExactRationalAndIntervalFit(t *testing.T) {
	x := []Interval{point(1), point(2), point(3), point(4)}
	y := []Interval{point(3), point(5), point(7), point(9)}
	f := fitAffineIntervals(x, y)
	if f == nil {
		t.Fatal("exact rational mapping unavailable")
	}
	nearInterval(t, f.Slope, 2, 1e-12)
	nearInterval(t, f.Rate, .5, 1e-12)
	for _, r := range f.FitResiduals {
		contains(t, r, 0)
	}
	y[2] = point(7.5)
	if fitAffineIntervals(x, y) != nil {
		t.Fatal("nonlinear exact records fitted")
	}
}

func TestAnalysisLargeOffsetDoesNotRematchEvents(t *testing.T) {
	s := measurementSegment(1, .02, 0, 2.25)
	r, err := AnalyzeSegment(s)
	if err != nil || r.Status != "measured" {
		t.Fatalf("%v %s", err, r.Status)
	}
	for _, e := range r.Events {
		nearInterval(t, *e.PlayerResidual, 2.25, 1e-7)
	}
}

func TestAnalysisRetainsIndependentInputSnapshot(t *testing.T) {
	s := measurementSegment(1, .02, .03, 0)
	r, err := AnalyzeSegment(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Events[0].VideoID = 6
	s.Calibration.Points[0].Skew = point(99)
	if r.Input.Events[0].VideoID != 1 || r.Input.Calibration.Points[0].Skew.Low != .03 {
		t.Fatal("retained evidence aliases caller memory")
	}
}

func TestAnalysisArithmeticBoundsDoNotBecomeNonfiniteResults(t *testing.T) {
	s := measurementSegment(1, .02, .03, 0)
	for i := range s.Events {
		s.Events[i].CapturedAudio = point(math.MaxFloat64/2 + float64(i)*1e300)
	}
	s.CaptureEnd = math.MaxFloat64
	for i := range s.Calibration.Points {
		s.Calibration.Points[i].Skew = point(-math.MaxFloat64)
	}
	r, err := AnalyzeSegment(s)
	if err == nil || r.Status != "rejected" {
		t.Fatal("overflow accepted")
	}
}
