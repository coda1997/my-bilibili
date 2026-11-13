package outputprobe

import (
	"encoding/hex"
	"errors"
	"math"
	"math/big"
)

// Interval is a closed uncertainty bound in seconds (or in the stated rate
// units). Bounds are propagated without assuming independent errors.
type Interval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// IdentityEvidence references retained evidence, not an assertion that the
// measurement was independently reviewed. A decoder label from a physical
// capture also needs the same-mode signal-qualification evidence required by
// the measurement contract. Independent identification records its procedure.
type IdentityEvidence struct {
	Method              string `json:"method"` // decoder or independent
	EvidenceSHA256      string `json:"evidence_sha256"`
	QualificationSHA256 string `json:"qualification_sha256,omitempty"`
	Procedure           string `json:"procedure,omitempty"`
}

type EventMeasurement struct {
	SegmentID       string           `json:"segment_id"`
	VideoID         int              `json:"video_id"`
	AudioID         int              `json:"audio_id"`
	DecodedVideoPTS Interval         `json:"decoded_video_pts_seconds"`
	DecodedAudioPTS Interval         `json:"decoded_audio_pts_seconds"`
	CapturedVideo   Interval         `json:"captured_video_seconds"`
	CapturedAudio   Interval         `json:"captured_audio_seconds"`
	VideoIdentity   IdentityEvidence `json:"video_identity"`
	AudioIdentity   IdentityEvidence `json:"audio_identity"`
}

// CalibrationPoint is capture-path audio-minus-video skew, not player skew.
// A positive correction means the capture path makes audio appear late.
type CalibrationPoint struct {
	EventID int      `json:"event_id"`
	Skew    Interval `json:"capture_skew_seconds"`
}

type CaptureCalibration struct {
	EvidenceSHA256 string             `json:"evidence_sha256"`
	Independent    bool               `json:"independent_of_player"`
	Device         string             `json:"device"`
	Mode           string             `json:"mode_and_cadence"`
	Reference      string             `json:"reference_method"`
	Placement      string             `json:"acoustic_placement"`
	Points         []CalibrationPoint `json:"points"`
}

// Segment describes one explicitly delimited continuous observation. A transient
// or discontinuous segment is preserved but cannot yield a constant-slope map.
// SelectedRate is retained configuration only; it is never used in a fit.
type Segment struct {
	ID                     string              `json:"id"`
	Kind                   string              `json:"kind"` // steady, post-seek, transient
	BoundaryEvidenceSHA256 string              `json:"boundary_evidence_sha256"`
	CaptureStart           float64             `json:"capture_start_seconds"`
	CaptureEnd             float64             `json:"capture_end_seconds"`
	SelectedRate           float64             `json:"selected_rate_configuration"`
	SeekTarget             *float64            `json:"seek_target_media_seconds,omitempty"`
	ContainsDiscontinuity  bool                `json:"contains_discontinuity"`
	Calibration            *CaptureCalibration `json:"calibration,omitempty"`
	Events                 []EventMeasurement  `json:"events"`
}

// AffineFit is a feasible positive local slope interval, not a statistical
// confidence interval. Representative parameters and residual bounds retain
// the fit's actual data relationship; an audio intercept is never used to
// remove skew from the calibrated or player-residual outputs.
type AffineFit struct {
	Slope                   Interval   `json:"capture_seconds_per_media_second"`
	Rate                    Interval   `json:"media_seconds_per_capture_second"`
	RepresentativeSlope     float64    `json:"representative_slope"`
	RepresentativeIntercept float64    `json:"representative_intercept_seconds"`
	FitResiduals            []Interval `json:"fit_residual_seconds"`
}

type AnalyzedEvent struct {
	ID                   int       `json:"id"`
	RawOffset            Interval  `json:"raw_offset_seconds"`
	DecodedFixtureOffset Interval  `json:"decoded_fixture_offset_media_seconds"`
	CaptureCorrection    *Interval `json:"capture_correction_seconds,omitempty"`
	PhysicalOutputOffset *Interval `json:"physical_output_offset_seconds,omitempty"`
	MappedFixtureOffset  *Interval `json:"mapped_fixture_offset_capture_seconds,omitempty"`
	PlayerResidual       *Interval `json:"player_residual_seconds,omitempty"`
}

// SegmentAnalysis never declares a physical PASS or invents an acceptance
// tolerance. Even a measured result needs an independently assessed method and
// owner-approved acceptance criteria. Input retains rejected/transient points.
type SegmentAnalysis struct {
	Input     Segment         `json:"input"`
	Status    string          `json:"status"` // rejected, inconclusive, uncalibrated, measured
	Reasons   []string        `json:"reasons,omitempty"`
	VideoFit  *AffineFit      `json:"video_fit,omitempty"`
	AudioFit  *AffineFit      `json:"audio_fit,omitempty"`
	RateDelta *Interval       `json:"audio_minus_video_rate,omitempty"`
	Events    []AnalyzedEvent `json:"events,omitempty"`
}

func finiteAnalysis(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validInterval(v Interval) bool {
	return finiteAnalysis(v.Low) && finiteAnalysis(v.High) && v.Low <= v.High
}
func subtractInterval(a, b Interval) Interval {
	return Interval{math.Nextafter(a.Low-b.High, math.Inf(-1)), math.Nextafter(a.High-b.Low, math.Inf(1))}
}
func multiplyInterval(a, b Interval) Interval {
	v := [4]float64{a.Low * b.Low, a.Low * b.High, a.High * b.Low, a.High * b.High}
	lo, hi := v[0], v[0]
	for _, n := range v[1:] {
		lo, hi = math.Min(lo, n), math.Max(hi, n)
	}
	return Interval{math.Nextafter(lo, math.Inf(-1)), math.Nextafter(hi, math.Inf(1))}
}
func validSHA(v string) bool  { b, e := hex.DecodeString(v); return e == nil && len(b) == 32 }
func shortText(v string) bool { return len(v) > 0 && len(v) <= 512 }
func validIdentity(v IdentityEvidence) bool {
	if !validSHA(v.EvidenceSHA256) {
		return false
	}
	switch v.Method {
	case "decoder":
		return validSHA(v.QualificationSHA256)
	case "independent":
		return shortText(v.Procedure)
	default:
		return false
	}
}

// AnalyzeSegment derives offsets using only identified measured events. The
// returned result retains the input even on rejection, so callers can retain
// the failed observation rather than silently selecting better-fitting points.
func AnalyzeSegment(s Segment) (SegmentAnalysis, error) {
	r := SegmentAnalysis{Input: cloneSegment(s), Status: "rejected"}
	reject := func(reason string) (SegmentAnalysis, error) {
		r.Reasons = append(r.Reasons, reason)
		return r, errors.New(reason)
	}
	if !shortText(s.ID) || !validSHA(s.BoundaryEvidenceSHA256) ||
		!finiteAnalysis(s.CaptureStart) || !finiteAnalysis(s.CaptureEnd) || s.CaptureStart < 0 || s.CaptureEnd <= s.CaptureStart ||
		!finiteAnalysis(s.SelectedRate) || s.SelectedRate <= 0 || s.SelectedRate > 3 {
		return reject("invalid segment boundary or configuration")
	}
	if s.Kind != "steady" && s.Kind != "post-seek" && s.Kind != "transient" {
		return reject("invalid segment kind")
	}
	if s.SeekTarget != nil && (!finiteAnalysis(*s.SeekTarget) || *s.SeekTarget < 0) {
		return reject("invalid seek target")
	}
	if s.Kind == "post-seek" && s.SeekTarget == nil {
		return reject("post-seek segment lacks seek target")
	}
	if len(s.Events) < 4 || len(s.Events) > 6 {
		return reject("segment needs four to six identified event pairs")
	}
	for i, e := range s.Events {
		if e.SegmentID != s.ID || e.VideoID < 1 || e.VideoID > 6 || e.VideoID != e.AudioID || !validIdentity(e.VideoIdentity) || !validIdentity(e.AudioIdentity) {
			return reject("mismatched or unqualified event identity")
		}
		for _, v := range []Interval{e.DecodedVideoPTS, e.DecodedAudioPTS, e.CapturedVideo, e.CapturedAudio} {
			if !validInterval(v) || v.Low < 0 {
				return reject("invalid event timestamp interval")
			}
		}
		if e.CapturedVideo.Low < s.CaptureStart || e.CapturedAudio.Low < s.CaptureStart || e.CapturedVideo.High > s.CaptureEnd || e.CapturedAudio.High > s.CaptureEnd {
			return reject("event lies outside recorded segment")
		}
		if i > 0 {
			p := s.Events[i-1]
			if e.VideoID != p.VideoID+1 {
				return reject("missing duplicate or nonmonotonic event ID")
			}
			if e.DecodedVideoPTS.Low <= p.DecodedVideoPTS.High || e.DecodedAudioPTS.Low <= p.DecodedAudioPTS.High || e.CapturedVideo.Low <= p.CapturedVideo.High || e.CapturedAudio.Low <= p.CapturedAudio.High {
				return reject("nonmonotonic or unresolved event order")
			}
		}
		raw, decoded := subtractInterval(e.CapturedAudio, e.CapturedVideo), subtractInterval(e.DecodedAudioPTS, e.DecodedVideoPTS)
		if !validInterval(raw) || !validInterval(decoded) {
			return reject("analysis arithmetic exceeds finite range")
		}
		r.Events = append(r.Events, AnalyzedEvent{ID: e.VideoID, RawOffset: raw, DecodedFixtureOffset: decoded})
	}
	if s.Kind == "transient" || s.ContainsDiscontinuity {
		r.Reasons = append(r.Reasons, "transient or discontinuous segment has no constant-slope mapping")
	} else {
		vx, vy, ax, ay := make([]Interval, len(s.Events)), make([]Interval, len(s.Events)), make([]Interval, len(s.Events)), make([]Interval, len(s.Events))
		for i, e := range s.Events {
			vx[i], vy[i], ax[i], ay[i] = e.DecodedVideoPTS, e.CapturedVideo, e.DecodedAudioPTS, e.CapturedAudio
		}
		r.VideoFit = fitAffineIntervals(vx, vy)
		r.AudioFit = fitAffineIntervals(ax, ay)
		if r.VideoFit == nil {
			r.Reasons = append(r.Reasons, "visual mapping unidentified within declared uncertainty")
		}
		if r.AudioFit == nil {
			r.Reasons = append(r.Reasons, "audio rate unidentified within declared uncertainty")
		}
		if r.VideoFit != nil && r.AudioFit != nil {
			d := subtractInterval(r.AudioFit.Rate, r.VideoFit.Rate)
			r.RateDelta = &d
		}
	}
	corrections, calibrationReason := calibrationFor(s)
	if calibrationReason != "" {
		r.Reasons = append(r.Reasons, calibrationReason)
	}
	for i := range r.Events {
		e := &r.Events[i]
		if corrections != nil {
			c := corrections[i]
			physical := subtractInterval(e.RawOffset, c)
			if !validInterval(physical) {
				return reject("analysis arithmetic exceeds finite range")
			}
			e.CaptureCorrection, e.PhysicalOutputOffset = &c, &physical
		}
		if r.VideoFit != nil {
			f := multiplyInterval(r.VideoFit.Slope, e.DecodedFixtureOffset)
			if !validInterval(f) {
				return reject("analysis arithmetic exceeds finite range")
			}
			e.MappedFixtureOffset = &f
			if e.PhysicalOutputOffset != nil {
				residual := subtractInterval(*e.PhysicalOutputOffset, f)
				if !validInterval(residual) {
					return reject("analysis arithmetic exceeds finite range")
				}
				e.PlayerResidual = &residual
			}
		}
	}
	switch {
	case r.VideoFit == nil || r.AudioFit == nil:
		r.Status = "inconclusive"
	case corrections == nil:
		r.Status = "uncalibrated"
	default:
		r.Status = "measured"
	}
	return r, nil
}

func calibrationFor(s Segment) ([]Interval, string) {
	c := s.Calibration
	if c == nil {
		return nil, "capture calibration unavailable"
	}
	if !c.Independent || !validSHA(c.EvidenceSHA256) || !shortText(c.Device) || !shortText(c.Mode) || !shortText(c.Reference) || !shortText(c.Placement) || len(c.Points) != len(s.Events) {
		return nil, "capture calibration provenance or event coverage invalid"
	}
	v := make([]Interval, len(c.Points))
	for i, p := range c.Points {
		if p.EventID != s.Events[i].VideoID || !validInterval(p.Skew) {
			return nil, "capture calibration correction invalid"
		}
		v[i] = p.Skew
	}
	return v, ""
}

// fitAffineIntervals solves the one-dimensional feasible-slope constraints.
// For positive b each datum admits an intercept in [y.low-b*x.high,
// y.high-b*x.low]. A common intercept exists exactly when every lower bound
// is <= every upper bound. These pair inequalities bound b. No residual
// tolerance is introduced and no point is removed. Independent uncertain x/y
// rectangles enlarge the feasible set conservatively.
func fitAffineIntervals(x, y []Interval) *AffineFit {
	if len(x) < 4 || len(x) != len(y) {
		return nil
	}
	for i := range x {
		if !validInterval(x[i]) || !validInterval(y[i]) {
			return nil
		}
	}
	// Use exact rationals for binary floating-point inputs so roundoff cannot
	// turn one feasible boundary into a contradiction or narrow an uncertainty
	// interval. Six-event records make this bounded calculation inexpensive.
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	lo := new(big.Rat)
	var hi *big.Rat
	for i := range x {
		for j := range x {
			dx := new(big.Rat).Sub(q(x[j].Low), q(x[i].High))
			dy := new(big.Rat).Sub(q(y[j].High), q(y[i].Low))
			if dx.Sign() == 0 {
				if dy.Sign() < 0 {
					return nil
				}
				continue
			}
			b := new(big.Rat).Quo(dy, dx)
			if dx.Sign() > 0 {
				if hi == nil || b.Cmp(hi) < 0 {
					hi = b
				}
			} else if b.Cmp(lo) > 0 {
				lo = b
			}
		}
	}
	if lo.Sign() <= 0 || hi == nil || hi.Cmp(lo) < 0 {
		return nil
	}
	b := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
	var al, ah *big.Rat
	for i := range x {
		l := new(big.Rat).Sub(q(y[i].Low), new(big.Rat).Mul(b, q(x[i].High)))
		h := new(big.Rat).Sub(q(y[i].High), new(big.Rat).Mul(b, q(x[i].Low)))
		if al == nil || l.Cmp(al) > 0 {
			al = l
		}
		if ah == nil || h.Cmp(ah) < 0 {
			ah = h
		}
	}
	if ah.Cmp(al) < 0 {
		return nil
	}
	a := new(big.Rat).Quo(new(big.Rat).Add(al, ah), big.NewRat(2, 1))
	bf, _ := b.Float64()
	af, _ := a.Float64()
	bounds := func(l, h *big.Rat) Interval {
		lf, lexact := l.Float64()
		hf, hexact := h.Float64()
		if !lexact {
			lf = math.Nextafter(lf, math.Inf(-1))
		}
		if !hexact {
			hf = math.Nextafter(hf, math.Inf(1))
		}
		return Interval{lf, hf}
	}
	f := &AffineFit{Slope: bounds(lo, hi), Rate: bounds(new(big.Rat).Inv(hi), new(big.Rat).Inv(lo)), RepresentativeSlope: bf, RepresentativeIntercept: af}
	if !validInterval(f.Slope) || f.Slope.Low <= 0 || !validInterval(f.Rate) || !finiteAnalysis(af) || !finiteAnalysis(bf) {
		return nil
	}
	for i := range x {
		p := multiplyInterval(Interval{bf, bf}, x[i])
		predicted := Interval{math.Nextafter(af+p.Low, math.Inf(-1)), math.Nextafter(af+p.High, math.Inf(1))}
		residual := subtractInterval(y[i], predicted)
		if !validInterval(residual) {
			return nil
		}
		f.FitResiduals = append(f.FitResiduals, residual)
	}
	return f
}

func cloneSegment(s Segment) Segment {
	s.Events = append([]EventMeasurement(nil), s.Events...)
	if s.SeekTarget != nil {
		value := *s.SeekTarget
		s.SeekTarget = &value
	}
	if s.Calibration != nil {
		c := *s.Calibration
		c.Points = append([]CalibrationPoint(nil), c.Points...)
		s.Calibration = &c
	}
	return s
}
