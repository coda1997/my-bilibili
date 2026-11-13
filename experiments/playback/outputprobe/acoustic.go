package outputprobe

import (
	"errors"
	"math"
	"math/cmplx"
	"sort"
)

// SpectralSupport is a conservatively localized contiguous interval. One
// analysis block has already been removed at each edge; the uncertainty records
// that discarded localization margin. Sample positions are absolute in the
// supplied PCM after DecodeAcoustic returns.
type SpectralSupport struct {
	FirstSample                    int `json:"first_sample"`
	EndSample                      int `json:"end_sample_exclusive"`
	LocalizationUncertaintySamples int `json:"localization_uncertainty_samples"`
}

type SpectralPeak struct {
	FrequencyHz           float64           `json:"frequency_hz"`
	UncertaintyHz         float64           `json:"uncertainty_hz"`
	Power                 float64           `json:"power"`
	ObservedBaselinePower float64           `json:"observed_baseline_power"`
	DetectionFloorPower   float64           `json:"detection_floor_power"`
	BaselinePower         float64           `json:"effective_baseline_power"`
	SustainedSeconds      float64           `json:"sustained_seconds"`
	Support               []SpectralSupport `json:"support"`
}

type AcousticEvidence struct {
	ID                   int               `json:"id"`
	FirstSample          int               `json:"first_sample"`
	EndSample            int               `json:"end_sample_exclusive"`
	BaselineStartSample  int               `json:"baseline_start_sample"`
	BaselineEndSample    int               `json:"baseline_end_sample_exclusive"`
	Peaks                []SpectralPeak    `json:"qualified_peaks"`
	UnresolvedPeaks      []SpectralPeak    `json:"unresolved_peaks,omitempty"`
	CandidateIDs         []int             `json:"candidate_ids"`
	RejectedAlternatives []string          `json:"rejected_alternatives,omitempty"`
	Reason               string            `json:"reason,omitempty"`
	SharedSupport        []SpectralSupport `json:"shared_support,omitempty"`
	QualifiedForPhysical bool              `json:"qualified_for_physical"`
}

// DecodeAcoustic identifies pulses without visual times or expected event order.
// A spectral label alone is deliberately never physical identification approval.
// The digital power floor is one signed-16-bit sample quantum squared. This
// conservative floor prevents spectral window/numerical residue from becoming
// an apparent component against a mathematically zero synthetic baseline.
func DecodeAcoustic(samples []float64, rate int) ([]AcousticEvidence, error) {
	if err := validateSamples(samples, rate); err != nil {
		return nil, err
	}
	block := rate / 1000
	threshold := 0.002
	active := make([]bool, (len(samples)+block-1)/block)
	for i := range active {
		end := min((i+1)*block, len(samples))
		var power float64
		for _, v := range samples[i*block : end] {
			power += v * v
		}
		active[i] = math.Sqrt(power/float64(end-i*block)) > threshold
	}
	// Fill only sub-3ms gaps inside one acoustic window; never infer missing IDs.
	for i := 1; i < len(active)-2; i++ {
		if active[i-1] && !active[i] {
			j := i
			for j < len(active) && !active[j] && j-i < 3 {
				j++
			}
			if j < len(active) && active[j] && j-i < 3 {
				for k := i; k < j; k++ {
					active[k] = true
				}
			}
		}
	}
	var result []AcousticEvidence
	for i := 0; i < len(active); {
		if !active[i] {
			i++
			continue
		}
		start := i
		for i < len(active) && active[i] {
			i++
		}
		end := i
		ev := AcousticEvidence{FirstSample: start * block, EndSample: min(end*block, len(samples))}
		// Use 30ms of adjacent silence, separated by 10ms to exclude AAC edge ringing.
		ev.BaselineEndSample = ev.FirstSample - rate/100
		ev.BaselineStartSample = ev.BaselineEndSample - rate*3/100
		if ev.BaselineStartSample < 0 {
			ev.Reason = "no adjacent silent baseline"
			result = append(result, ev)
			continue
		}
		baseline := samples[ev.BaselineStartSample:ev.BaselineEndSample]
		silent := true
		for _, v := range baseline {
			if math.Abs(v) > threshold {
				silent = false
				break
			}
		}
		if !silent {
			ev.Reason = "adjacent baseline is not silent"
			result = append(result, ev)
			continue
		}
		pulse := samples[ev.FirstSample:ev.EndSample]
		if len(pulse) < (rate+49)/50 {
			ev.Reason = "pulse shorter than 20ms"
			result = append(result, ev)
			continue
		}
		ev.Peaks, ev.UnresolvedPeaks = qualifiedPeaks(pulse, baseline, rate)
		for k := range ev.Peaks {
			for j := range ev.Peaks[k].Support {
				ev.Peaks[k].Support[j].FirstSample += ev.FirstSample
				ev.Peaks[k].Support[j].EndSample += ev.FirstSample
			}
		}
		if len(ev.UnresolvedPeaks) > 0 {
			for k := range ev.UnresolvedPeaks {
				for j := range ev.UnresolvedPeaks[k].Support {
					ev.UnresolvedPeaks[k].Support[j].FirstSample += ev.FirstSample
					ev.UnresolvedPeaks[k].Support[j].EndSample += ev.FirstSample
				}
			}
			ev.Reason = "component duration overlaps 20ms but cannot be resolved conservatively"
			result = append(result, ev)
			continue
		}
		if len(ev.Peaks) == 0 {
			ev.Reason = "no sustained qualified components"
			result = append(result, ev)
			continue
		}
		ref := ev.Peaks[0]
		if ref.FrequencyHz+ref.UncertaintyHz < 220 || ref.FrequencyHz-ref.UncertaintyHz > 880 {
			ev.Reason = "lowest sustained component outside reference range"
			result = append(result, ev)
			continue
		}
		ids := map[int]bool{}
		for _, p := range ev.Peaks[1:] {
			shared := commonSupport(ref.Support, p.Support, (rate+49)/50)
			if len(shared) == 0 {
				ev.RejectedAlternatives = append(ev.RejectedAlternatives, "components lack 20ms of simultaneous sustained support")
				continue
			}
			// Keep the full frequency uncertainty in matching. A reference at an
			// inclusive supported scale boundary may overlap 220 or 880 Hz, but
			// uncertainty is never clamped away to force an identifier ratio.
			refLow, refHigh := ref.FrequencyHz-ref.UncertaintyHz, ref.FrequencyHz+ref.UncertaintyHz
			ratioLow := (p.FrequencyHz - p.UncertaintyHz) / refHigh
			ratioHigh := (p.FrequencyHz + p.UncertaintyHz) / refLow
			matched := false
			for id, hz := range identifierHz {
				expected := hz / 440
				if refLow > 0 && ratioLow >= expected-0.02 && ratioHigh <= expected+0.02 {
					ids[id+1] = true
					matched = true
					ev.SharedSupport = append(ev.SharedSupport, shared...)
				}
			}
			if !matched {
				ev.RejectedAlternatives = append(ev.RejectedAlternatives, "component ratio is unknown or ambiguous within frequency uncertainty")
			}
		}
		for id := 1; id <= 6; id++ {
			if ids[id] {
				ev.CandidateIDs = append(ev.CandidateIDs, id)
			}
		}
		if len(ev.CandidateIDs) == 1 {
			ev.ID = ev.CandidateIDs[0]
		} else if len(ev.CandidateIDs) == 0 {
			ev.Reason = "no identifier ratio for lowest component"
		} else {
			ev.Reason = "ambiguous acoustic identifier"
		}
		result = append(result, ev)
	}
	if len(result) == 0 {
		return result, errors.New("no acoustic pulses detected")
	}
	return result, nil
}
func qualifiedPeaks(pulse, baseline []float64, rate int) ([]SpectralPeak, []SpectralPeak) {
	// Bound FFT storage even for a long non-silent recording. The longest
	// supported 0.5x pulse (200ms) fits this cap at every accepted sample rate.
	if len(pulse) > (1<<20)/8 {
		return nil, nil
	}
	candidates := spectrumCandidates(pulse, baseline, rate)
	// Whole-pulse tapers suppress components near the pulse edges. Discover
	// candidates in overlapping local windows as well, after removing the
	// already resolved components jointly. Local discovery uses local power;
	// it must never be gated by the attenuated whole-pulse projection.
	local := rate * 3 / 100
	hop := rate / 200
	if len(candidates) > 0 && len(candidates) <= 16 && len(pulse) >= local {
		for {
			model := newToneModel(candidates, local, rate)
			if model == nil {
				return nil, nil
			}
			found := false
			for offset := 0; offset+local <= len(pulse) && !found; offset += hop {
				residual := model.residual(pulse[offset : offset+local])
				for _, c := range spectrumCandidates(residual, baseline, rate) {
					// Amplitude ramps can leave the same frequency in the
					// residual. Overlapping localization intervals are not
					// evidence of an additional independently resolved tone.
					known := false
					for _, p := range candidates {
						if math.Abs(p.FrequencyHz-c.FrequencyHz) <= p.UncertaintyHz+c.UncertaintyHz {
							known = true
							break
						}
					}
					if known {
						continue
					}
					trial := append(append([]SpectralPeak(nil), candidates...), c)
					// Ramps and transitions also create residual spectral peaks.
					// A local discovery must meet the SAME sustained-support
					// rule before it can join the joint component model.
					supported := supportedPeaksMinimum(trial, pulse, baseline, rate, (rate+49)/50-4*hop)
					for _, p := range supported {
						if p.FrequencyHz == c.FrequencyHz {
							if p.SustainedSeconds < .02 {
								// Its conservative core is short, but adding the
								// discarded localization margin reaches 20ms. It
								// cannot safely be erased when choosing an anchor.
								return supportedPeaks(candidates, pulse, baseline, rate), []SpectralPeak{unresolvedDuration(p)}
							}
							candidates = trial
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				break
			}
			if len(candidates) > 16 {
				return nil, nil
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].FrequencyHz < candidates[j].FrequencyHz })
	var qualified, unresolved []SpectralPeak
	// The same duration ambiguity applies to whole-pulse discoveries. In a
	// short pulse, the lower tone may already be visible globally, yet its
	// support interval still straddles the 20ms qualification boundary.
	for _, p := range supportedPeaksMinimum(candidates, pulse, baseline, rate, (rate+49)/50-4*hop) {
		if p.SustainedSeconds >= .02 {
			qualified = append(qualified, p)
		} else {
			unresolved = append(unresolved, unresolvedDuration(p))
		}
	}
	return qualified, unresolved
}

// A qualified core discards one complete block at each edge. A rejected
// partial block just outside that run can contain almost another full block
// of tone. For an unresolved duration, retain both margins when bounding how
// long the component could have persisted; otherwise a 21ms unaligned tone
// could disappear behind the block grid. This never qualifies a short core.
func unresolvedDuration(p SpectralPeak) SpectralPeak {
	for i := range p.Support {
		p.Support[i].LocalizationUncertaintySamples *= 2
	}
	return p
}

func supportedPeaks(candidates []SpectralPeak, pulse, baseline []float64, rate int) []SpectralPeak {
	return supportedPeaksMinimum(candidates, pulse, baseline, rate, (rate+49)/50)
}

func supportedPeaksMinimum(candidates []SpectralPeak, pulse, baseline []float64, rate, minimumCore int) []SpectralPeak {
	// An unresolved crowded/ill-conditioned spectrum cannot establish a lowest
	// sustained fundamental; reject it rather than selecting convenient tones.
	if len(candidates) == 0 || len(candidates) > 16 {
		return nil
	}
	block := rate / 200 // nonoverlapping 5ms amplitude-estimation blocks
	model := newToneModel(candidates, block, rate)
	if model == nil {
		return nil
	}
	for offset := 0; offset+block <= len(baseline); offset += block {
		_, upper := model.powers(baseline[offset : offset+block])
		for k := range candidates {
			candidates[k].BaselinePower = max(candidates[k].BaselinePower, upper[k])
		}
	}
	qualified := make([][]bool, len(candidates))
	for offset := 0; offset+block <= len(pulse); offset += block {
		lower, _ := model.powers(pulse[offset : offset+block])
		for k := range candidates {
			qualified[k] = append(qualified[k], lower[k] >= 100*candidates[k].BaselinePower)
		}
	}
	var result []SpectralPeak
	for k, c := range candidates {
		// A partial edge block cannot establish a full block of support. Trim
		// both edges of every run. Six qualified 5ms blocks are therefore needed
		// for a 20ms lower-bound core; a 19ms burst can never gain duration from
		// straddling a block boundary. Runs are not joined across failed blocks.
		for first := 0; first < len(qualified[k]); {
			if !qualified[k][first] {
				first++
				continue
			}
			end := first
			for end < len(qualified[k]) && qualified[k][end] {
				end++
			}
			low, high := (first+1)*block, (end-1)*block
			if high-low >= minimumCore {
				c.Support = append(c.Support, SpectralSupport{low, high, block})
				c.SustainedSeconds = max(c.SustainedSeconds, float64(high-low)/float64(rate))
			}
			first = end
		}
		if len(c.Support) > 0 {
			result = append(result, c)
		}
	}
	return result
}

// spectrumCandidates supplies frequencies for the independent local support
// check. It uses only the supplied analysis window and the adjacent baseline.
func spectrumCandidates(samples, baseline []float64, rate int) []SpectralPeak {
	n := 1
	for n < len(samples)*8 {
		n *= 2
	}
	p := spectrum(samples, n)
	step := float64(rate) / float64(n)
	floor := 1.0 / (32768 * 32768)
	var candidates []SpectralPeak
	for i := 1; i < len(p)/2-1; i++ {
		if p[i] <= p[i-1] || p[i] < p[i+1] || p[i] < 100*floor {
			continue
		}
		delta := 0.0
		den := math.Log(max(p[i-1], 1e-300)) - 2*math.Log(max(p[i], 1e-300)) + math.Log(max(p[i+1], 1e-300))
		if den != 0 {
			delta = 0.5 * (math.Log(max(p[i-1], 1e-300)) - math.Log(max(p[i+1], 1e-300))) / den
		}
		freq := (float64(i) + delta) * step
		power, observed := tonePower(samples, freq, rate), tonePower(baseline, freq, rate)
		base := max(observed, floor)
		if power >= 100*base {
			candidates = append(candidates, SpectralPeak{FrequencyHz: freq, UncertaintyHz: step / 2, Power: power, ObservedBaselinePower: observed, DetectionFloorPower: floor, BaselinePower: base})
		}
	}
	return candidates
}

func commonSupport(a, b []SpectralSupport, minSamples int) []SpectralSupport {
	var result []SpectralSupport
	for _, x := range a {
		for _, y := range b {
			lo, hi := max(x.FirstSample, y.FirstSample), min(x.EndSample, y.EndSample)
			if hi-lo >= minSamples {
				result = append(result, SpectralSupport{lo, hi, max(x.LocalizationUncertaintySamples, y.LocalizationUncertaintySamples)})
			}
		}
	}
	return result
}

// toneModel jointly estimates all candidate sin/cos amplitudes plus DC. A
// single-tone projection would interpret leakage from another tone as sustained
// support. Short blocks provide temporal localization; the joint fit and its
// residual-based coefficient bounds conservatively reject unresolved blocks.
type toneModel struct {
	basis, pseudo [][]float64
	components    int
}

func newToneModel(peaks []SpectralPeak, n, rate int) *toneModel {
	m := 2*len(peaks) + 1
	if n <= m {
		return nil
	}
	basis := make([][]float64, n)
	gram := make([][]float64, m)
	for k := range gram {
		gram[k] = make([]float64, 2*m)
		gram[k][m+k] = 1
	}
	for i := range basis {
		basis[i] = make([]float64, m)
		basis[i][0] = 1
		for k, p := range peaks {
			angle := 2 * math.Pi * p.FrequencyHz * float64(i) / float64(rate)
			basis[i][2*k+1] = math.Sin(angle)
			basis[i][2*k+2] = math.Cos(angle)
		}
		for j := 0; j < m; j++ {
			for k := 0; k < m; k++ {
				gram[j][k] += basis[i][j] * basis[i][k]
			}
		}
	}
	// Pivoted inversion of the fixed Gram matrix. A small pivot means component
	// amplitudes cannot be separated reliably in this temporal window.
	for j := 0; j < m; j++ {
		pivot := j
		for k := j + 1; k < m; k++ {
			if math.Abs(gram[k][j]) > math.Abs(gram[pivot][j]) {
				pivot = k
			}
		}
		if math.Abs(gram[pivot][j]) < float64(n)*1e-10 {
			return nil
		}
		gram[j], gram[pivot] = gram[pivot], gram[j]
		v := gram[j][j]
		for k := 0; k < 2*m; k++ {
			gram[j][k] /= v
		}
		for row := 0; row < m; row++ {
			if row != j {
				v = gram[row][j]
				for k := 0; k < 2*m; k++ {
					gram[row][k] -= v * gram[j][k]
				}
			}
		}
	}
	pseudo := make([][]float64, m)
	for j := range pseudo {
		pseudo[j] = make([]float64, n)
		for i := range basis {
			for k := 0; k < m; k++ {
				pseudo[j][i] += gram[j][m+k] * basis[i][k]
			}
		}
	}
	return &toneModel{basis, pseudo, len(peaks)}
}

func (m *toneModel) residual(samples []float64) []float64 {
	coeff := make([]float64, len(m.pseudo))
	for k := range coeff {
		for i, v := range samples {
			coeff[k] += m.pseudo[k][i] * v
		}
	}
	residual := append([]float64(nil), samples...)
	for i := range residual {
		for k, c := range coeff {
			residual[i] -= c * m.basis[i][k]
		}
	}
	return residual
}

func (m *toneModel) powers(samples []float64) (lower, upper []float64) {
	coeff := make([]float64, len(m.pseudo))
	for k := range coeff {
		for i, v := range samples {
			coeff[k] += m.pseudo[k][i] * v
		}
	}
	residual := 0.0
	for i, v := range samples {
		prediction := 0.0
		for k, c := range coeff {
			prediction += c * m.basis[i][k]
		}
		residual = max(residual, math.Abs(v-prediction))
	}
	bounds := make([]float64, len(coeff))
	for k := range coeff {
		for _, v := range m.pseudo[k] {
			bounds[k] += math.Abs(v) * residual
		}
	}
	lower, upper = make([]float64, m.components), make([]float64, m.components)
	for k := 0; k < m.components; k++ {
		amp := math.Hypot(coeff[2*k+1], coeff[2*k+2])
		errorBound := math.Hypot(bounds[2*k+1], bounds[2*k+2])
		lower[k] = math.Pow(max(0, amp-errorBound), 2)
		upper[k] = math.Pow(amp+errorBound, 2)
	}
	return lower, upper
}

func window(i, n int) float64 {
	if n < 2 {
		return 1
	}
	x := 2 * math.Pi * float64(i) / float64(n-1)
	return 0.35875 - 0.48829*math.Cos(x) + 0.14128*math.Cos(2*x) - 0.01168*math.Cos(3*x)
}
func tonePower(samples []float64, freq float64, rate int) float64 {
	var real, imag, norm float64
	for i, v := range samples {
		w := window(i, len(samples))
		angle := 2 * math.Pi * freq * float64(i) / float64(rate)
		real += v * w * math.Cos(angle)
		imag -= v * w * math.Sin(angle)
		norm += w
	}
	if norm == 0 {
		return 0
	}
	return 4 * (real*real + imag*imag) / (norm * norm)
}
func spectrum(samples []float64, n int) []float64 {
	a := make([]complex128, n)
	var norm float64
	for i, v := range samples {
		w := window(i, len(samples))
		a[i] = complex(v*w, 0)
		norm += w
	}
	fft(a)
	p := make([]float64, n)
	for i, v := range a {
		p[i] = 4 * cmplx.Abs(v) * cmplx.Abs(v) / (norm * norm)
	}
	return p
}
func fft(a []complex128) {
	n := len(a)
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		theta := -2 * math.Pi / float64(length)
		root := cmplx.Rect(1, theta)
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for j := 0; j < length/2; j++ {
				u, v := a[i+j], a[i+j+length/2]*w
				a[i+j], a[i+j+length/2] = u+v, u-v
				w *= root
			}
		}
	}
}
