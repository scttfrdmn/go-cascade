package calibrate

import (
	"fmt"
	"math"
	"sort"
)

// Absorption modelling for §5.5(4): the direct test of the §2.9 cache-miss
// distribution shift.
//
// §2.9 says a warm cache absorbs the head of the query distribution, so the
// router downstream observes D | x not in H_t rather than D, and a certificate
// calibrated on the wrong one is void. Shadow sampling is the correction:
// route an epsilon fraction past the cache and calibrate on that stream.
//
// Testing it needs a cache that absorbs a non-trivial share of traffic. Experiment
// 20 established that MultiPL-E Go cannot supply one: retrieval candidacy is
// 464/488 (95.1%) but the *absorption* ceiling is 2/488 (0.4%), because arm zero
// re-executes every retrieved solution (invariant #5) and lexical similarity does
// not imply transferability. A 0.4% cache shifts no distribution, so the paid run
// would have reported a corpus artifact as a null.
//
// So absorption becomes a controlled dial instead of a property of the corpus. The
// dial is applied to *recorded* observations, which makes the whole sweep free: the
// n=409 records profile every tier on every problem, so any absorption pattern and
// any threshold vector can be replayed offline without querying a model.
//
// # This is a model, not a measurement
//
// Injecting absorption is not the same as observing it. What is measured here is
// the *sensitivity of the certificate to a shift of known shape and size* — which
// is precisely what §5.5(4) asks for, and precisely not a claim about how any real
// cache warms. Reading a number out of here as "the observed effect of cache
// warmth" would repeat the error experiment 20 caught.
//
// # Nothing here may touch a real certificate
//
// This file is offline analysis. It constructs synthetic record streams to compare
// certificates against each other; no path from here reaches Router acceptance, and
// invariant #8 is unaffected — `cmdCalibrate` still forces `CacheDir = ""`, and
// `Profile` still sets `Shadow: true`. Experiment 20's finding is a fact about
// independently-sampled benchmarks, not licence to calibrate behind a warm cache.

// AbsorptionPattern selects *which* queries a warm cache absorbs. The choice is
// the whole experiment, and getting it wrong makes the harness measure nothing.
//
// Uniform absorption does not shift the distribution: dropping a random subset of
// an exchangeable sample leaves an exchangeable sample. Measured on the n=409
// execution records, tier-0 accuracy in D is 0.7702 and under uniform absorption
// the residual reads 0.7706 / 0.7796 / 0.7500 / 0.7439 at rho = 0.2 / 0.4 / 0.6 / 0.8
// — noise, in both directions. So a harness that injects *exact duplicates uniformly
// at random* would report "no §2.9 effect" no matter how high the rate went, and the
// null would be an artifact of the injection design rather than evidence about §2.9.
//
// §2.9's premise is that the cache absorbs the *head* — the queries that recur, and
// recurring queries are the easy and popular ones. That is a selective filter, and
// it is what makes the residual harder than D. Same records, selective absorption:
// 0.7125 / 0.6163 / 0.4268 / 0.0000 at the same four rates. The effect §2.9 warns
// about is real and large, but only under a head-shaped filter.
//
// The uniform arm is therefore not redundant: its spread is the null envelope every
// selective row is read against (max |risk gap| 0.0267 at n=409), and by that yardstick
// rho = 0.2 does not clear it. Do not drop it as a no-op arm.
type AbsorptionPattern string

const (
	// AbsorbUniform drops queries independently of their difficulty. It is the
	// null control, included precisely because it produces no shift: it separates
	// "the harness detects a shift" from "the harness detects sample loss".
	AbsorbUniform AbsorptionPattern = "uniform"
	// AbsorbEasyFirst drops the queries the cheap tier already answers correctly,
	// which is the §2.9 head: the recurring, easy, high-volume queries a real cache
	// accumulates first. This is the pattern that actually tests the correction.
	AbsorbEasyFirst AbsorptionPattern = "easy-first"
	// AbsorbCheapAccept drops the queries the *policy* would have served from tier
	// 0, i.e. those whose tier-0 score clears the threshold. It is the closest
	// analogue to a cache warmed by a running cascade rather than by an oracle's
	// notion of easiness, and unlike AbsorbEasyFirst it depends on tau.
	AbsorbCheapAccept AbsorptionPattern = "cheap-accept"
)

// AbsorptionSweep is one row of the §5.5(4) result: what the certificate looks
// like when a cache absorbs a rho fraction of the stream under a given pattern,
// with and without shadow sampling.
type AbsorptionSweep struct {
	Pattern AbsorptionPattern `json:"pattern"`
	// Rho is the injected absorption rate: the fraction of the query stream the
	// cache serves. It is a dial, not an observation (see the file comment).
	Rho float64 `json:"rho"`
	// Epsilon is the shadow-sampling rate: the fraction routed *past* the cache,
	// which is the §2.9 correction under test. Zero means no correction.
	Epsilon float64 `json:"epsilon"`

	NResidual int `json:"n_residual"` // queries the router still sees
	NShadow   int `json:"n_shadow"`   // of those, ones drawn past the cache
	// NCalibration is how many records the calibrator was actually handed. On the
	// uncorrected arm that is the whole recorded sample; on the corrected arm it is
	// NShadow. It is the load-bearing column of the whole sweep: shadow sampling
	// buys distributional correctness by *spending sample size*, so a corrected row
	// can look worse than the uncorrected one purely because it calibrated on a
	// handful of records. A row must be read with this number in hand.
	NCalibration int `json:"n_calibration"`

	// Tier0Accuracy of D and of the residual stream. The gap between them IS the
	// §2.9 shift; if it is zero the pattern absorbed nothing selective and no
	// certificate difference below can be attributed to warmth.
	Tier0AccuracyD        float64 `json:"tier0_accuracy_d"`
	Tier0AccuracyResidual float64 `json:"tier0_accuracy_residual"`

	// CalibratedRisk is the empirical risk the certificate reports, computed on
	// whatever stream the calibrator was given. DeployedRisk is the risk those same
	// thresholds actually incur on the residual stream the router faces. Their
	// difference is the certificate's error, and it is the quantity §5.5(4) is
	// asking about: a certificate that calibrates on the wrong distribution
	// understates the risk it will deliver.
	CalibratedRisk float64 `json:"calibrated_risk"`
	DeployedRisk   float64 `json:"deployed_risk"`
	// RiskGap is DeployedRisk - CalibratedRisk. Positive means the certificate is
	// optimistic: it promised less risk than deployment delivers, which is the
	// §2.9 failure. Shadow sampling should drive this toward zero.
	RiskGap float64 `json:"risk_gap"`
	// Certified is whether the certificate was Valid at all. A shift can also
	// manifest as a refusal to certify rather than as an optimistic bound.
	Certified  bool      `json:"certified"`
	Thresholds []float64 `json:"thresholds"`
	// Underpowered marks a row whose calibration sample was too small for the bound
	// to mean anything, independent of any shift. Such a row's RiskGap must not be
	// read as evidence about the correction — see MinCalibrationSize.
	Underpowered bool `json:"underpowered,omitempty"`
	// Note carries anything that makes a row unsafe to read at face value.
	Note string `json:"note,omitempty"`
}

// AbsorptionOptions configures the sweep.
type AbsorptionOptions struct {
	Rhos     []float64 // absorption rates to sweep
	Epsilons []float64 // shadow rates to sweep, 0 included for the uncorrected arm
	Patterns []AbsorptionPattern
	Tiers    []string
	LTT      Options
	// Seed makes the uniform pattern and the shadow draw reproducible. The sweep
	// must be deterministic or two runs would differ for reasons unrelated to rho.
	Seed uint64
	// TauForCheapAccept is the threshold vector AbsorbCheapAccept treats as the
	// running policy's. Empty falls back to AbsorbEasyFirst's oracle-based notion,
	// which is noted on the row rather than done silently.
	TauForCheapAccept []float64
}

// SweepAbsorption replays recs under each (pattern, rho, epsilon) and reports how
// the certificate degrades. It never mutates recs.
//
// The two arms per row:
//
//   - Uncorrected (epsilon = 0): calibrate on the *full* recorded sample, as a
//     system would if it collected traffic behind an already-warm cache without
//     realising. Then deploy those thresholds on the residual stream.
//   - Corrected (epsilon > 0): calibrate only on the shadow draw — a Bernoulli(eps)
//     subsample of the *residual*, which §2.9 argues is distributed as D.
//
// The comparison is the experiment. A large positive RiskGap in the uncorrected arm
// that shrinks in the corrected one is §2.9's correction working; no gap in either
// under a selective pattern would be evidence against the mechanism mattering at
// this scale.
func SweepAbsorption(recs []Record, opts AbsorptionOptions) ([]AbsorptionSweep, error) {
	use := usableRecords(recs)
	if len(use) == 0 {
		return nil, fmt.Errorf("no usable records: all %d were excluded as contaminated or oracle-unsound", len(recs))
	}
	if len(opts.Tiers) == 0 {
		return nil, fmt.Errorf("no tier names given; the grid arity comes from them")
	}
	if len(opts.Rhos) == 0 {
		opts.Rhos = []float64{0, 0.2, 0.4, 0.6, 0.8}
	}
	if len(opts.Epsilons) == 0 {
		opts.Epsilons = []float64{0, 0.05}
	}
	if len(opts.Patterns) == 0 {
		opts.Patterns = []AbsorptionPattern{AbsorbUniform, AbsorbEasyFirst}
	}

	baseline := tier0Accuracy(use)
	// The sample size below which a certificate cannot succeed even with zero
	// observed errors. Rows under it are flagged, not dropped — see the use below.
	floor := MinCalibrationSize(opts.LTT.Alpha, opts.LTT.Delta)
	var out []AbsorptionSweep
	for _, pat := range opts.Patterns {
		for _, rho := range opts.Rhos {
			residual, note, err := absorb(use, pat, rho, opts)
			if err != nil {
				return nil, err
			}
			for _, eps := range opts.Epsilons {
				row := AbsorptionSweep{
					Pattern: pat, Rho: rho, Epsilon: eps,
					NResidual:      len(residual),
					Tier0AccuracyD: baseline, Tier0AccuracyResidual: tier0Accuracy(residual),
					Note: note,
				}
				// Too little left to say anything. Report the row rather than dropping
				// it silently: a missing row reads as "not swept", which is worse.
				if len(residual) < 2 {
					row.Note = joinNote(row.Note, fmt.Sprintf("only %d residual records; no certificate attempted", len(residual)))
					out = append(out, row)
					continue
				}

				// The stream the calibrator is given. Uncorrected sees the whole
				// recorded sample (the mistake §2.9 describes); corrected sees only
				// the shadow draw off the residual.
				calStream := use
				if eps > 0 {
					calStream = shadowDraw(residual, eps, opts.Seed, pat, rho)
					row.NShadow = len(calStream)
				}
				row.NCalibration = len(calStream)
				// Shadow sampling trades sample size for distributional correctness, and
				// below some n the trade stops being informative: an LTT bound off a
				// dozen records is dominated by its own finite-sample penalty, so the
				// grid either certifies nothing or certifies something wild, and the
				// resulting RiskGap says nothing about §2.9 either way. Flag it on the
				// row instead of dropping it — a missing row reads as "not swept", and
				// an unflagged one reads as a finding. Both are worse than a caveat.
				//
				// Flagged BEFORE the too-few-to-calibrate bail below, not after: a row
				// that never reached Calibrate is the *most* underpowered kind, and a
				// consumer filtering on Underpowered must not see it as sound.
				if row.NCalibration < floor {
					row.Note = joinNote(row.Note, fmt.Sprintf(
						"calibrated on only %d records (epsilon=%.2f of a %d-record residual); "+
							"certifying alpha=%.3f at delta=%.3f needs >= %d even with zero observed errors, "+
							"so this row is sample-size noise, not evidence about the correction",
						row.NCalibration, eps, len(residual), opts.LTT.Alpha, opts.LTT.Delta, floor))
					row.Underpowered = true
				}
				if row.NCalibration < 2 {
					row.Note = joinNote(row.Note,
						fmt.Sprintf("only %d calibration records; no certificate attempted", row.NCalibration))
					out = append(out, row)
					continue
				}

				cert, err := Calibrate(stripShadowFlags(calStream), opts.Tiers, opts.LTT)
				if err != nil {
					row.Note = joinNote(row.Note, "calibration failed: "+err.Error())
					out = append(out, row)
					continue
				}
				row.Certified, row.Thresholds = cert.Valid, cert.Thresholds
				row.CalibratedRisk = cert.EmpiricalRisk
				// The risk those thresholds actually deliver on the stream the router
				// faces. Measured against execution ground truth, not the oracle's
				// labels, so an unsound oracle cannot hide the gap.
				row.DeployedRisk = RealizedRisk(residual, cert.Thresholds)
				row.RiskGap = row.DeployedRisk - row.CalibratedRisk
				out = append(out, row)
			}
		}
	}
	return out, nil
}

// absorb removes a rho fraction of recs according to pattern, returning the
// residual stream the router would still see.
func absorb(recs []Record, pat AbsorptionPattern, rho float64, opts AbsorptionOptions) ([]Record, string, error) {
	if rho < 0 || rho >= 1 {
		return nil, "", fmt.Errorf("absorption rate rho must be in [0,1), got %v", rho)
	}
	n := len(recs)
	drop := int(math.Round(rho * float64(n)))
	if drop >= n {
		drop = n - 1
	}
	switch pat {
	case AbsorbUniform:
		// Deterministic shuffle, then take the tail. A fixed permutation keyed on
		// (seed, rho) is reproducible; Go's map iteration or time-seeded rand
		// would make two sweeps differ for reasons unrelated to rho.
		idx := permutation(n, opts.Seed^math.Float64bits(rho))
		keep := make([]Record, 0, n-drop)
		for _, i := range idx[drop:] {
			keep = append(keep, recs[i])
		}
		return keep, "", nil

	case AbsorbEasyFirst:
		// The §2.9 head: absorb what the cheap tier already gets right. Ordered by
		// (tier-0 correct desc, tier-0 score desc) so the drop is the easiest
		// prefix and ties break deterministically rather than by slice order.
		ord := make([]Record, len(recs))
		copy(ord, recs)
		sort.SliceStable(ord, func(i, j int) bool {
			ci, cj := tier0Correct(ord[i]), tier0Correct(ord[j])
			if ci != cj {
				return ci // correct-at-tier-0 absorbed first
			}
			return tier0Score(ord[i]) > tier0Score(ord[j])
		})
		return ord[drop:], "", nil

	case AbsorbCheapAccept:
		tau := opts.TauForCheapAccept
		note := ""
		if len(tau) == 0 {
			note = "cheap-accept given no tau; fell back to ordering by tier-0 score alone, " +
				"which is easy-first without the oracle label"
		}
		ord := make([]Record, len(recs))
		copy(ord, recs)
		sort.SliceStable(ord, func(i, j int) bool {
			ai, aj := servedAtTier0(ord[i], tau), servedAtTier0(ord[j], tau)
			if ai != aj {
				return ai
			}
			return tier0Score(ord[i]) > tier0Score(ord[j])
		})
		return ord[drop:], note, nil
	}
	return nil, "", fmt.Errorf("unknown absorption pattern %q", pat)
}

// shadowDraw takes a Bernoulli(eps) subsample, which is what §2.9's correction
// prescribes: the draw must be independent of x, so it cannot look at the record.
func shadowDraw(residual []Record, eps float64, seed uint64, pat AbsorptionPattern, rho float64) []Record {
	if eps >= 1 {
		return residual
	}
	// A permutation prefix rather than a per-record coin flip: at the sample sizes
	// here a Bernoulli draw's own variance would swamp the effect being measured,
	// and the expected count is what the paper's epsilon refers to. The selection
	// is still independent of x, which is the property that matters.
	want := int(math.Round(eps * float64(len(residual))))
	if want < 1 {
		want = 1
	}
	key := seed ^ math.Float64bits(eps) ^ math.Float64bits(rho) ^ hashString(string(pat))
	idx := permutation(len(residual), key)
	out := make([]Record, 0, want)
	for _, i := range idx[:want] {
		out = append(out, residual[i])
	}
	return out
}

// stripShadowFlags clears Record.Shadow on a copy.
//
// Calibrate prefers the shadow subset of whatever it is handed, and every record
// in the profiled corpus carries Shadow: true (Profile sets it unconditionally —
// invariant #8). Left alone, that internal preference would fire on both arms of
// this sweep and make the uncorrected arm silently identical to the corrected one,
// which would look like "shadow sampling has no effect" and be an artifact.
//
// The sweep models shadow sampling *explicitly*, by choosing which stream to
// calibrate on. So the flag is cleared to keep the two mechanisms from stacking.
// This is safe because it happens on copies in offline analysis; the flag on the
// real records, and Calibrate's preference for it, are untouched.
func stripShadowFlags(recs []Record) []Record {
	out := make([]Record, len(recs))
	copy(out, recs)
	for i := range out {
		out[i].Shadow = false
	}
	return out
}

// usableRecords applies the same exclusions Calibrate does, so a sweep row's n
// matches what a real certificate over the same corpus would use.
func usableRecords(recs []Record) []Record {
	var out []Record
	for _, r := range recs {
		if r.Contaminated || r.OracleUnsound {
			continue
		}
		out = append(out, r)
	}
	return out
}

func tier0Correct(r Record) bool {
	return len(r.Tiers) > 0 && r.Tiers[0].truth()
}

func tier0Score(r Record) float64 {
	if len(r.Tiers) == 0 {
		return 0
	}
	return r.Tiers[0].Score
}

// servedAtTier0 reports whether the policy tau would have accepted at tier 0, so
// a cache warmed by the running cascade absorbs exactly what it served.
func servedAtTier0(r Record, tau []float64) bool {
	if len(r.Tiers) == 0 || len(tau) == 0 {
		return false
	}
	return r.Tiers[0].Score >= tau[0]
}

// tier0Accuracy is ground-truth cheap-tier accuracy on a stream. It is the shift
// detector: if this does not move between D and the residual, the pattern absorbed
// nothing selective and no certificate difference is attributable to warmth.
func tier0Accuracy(recs []Record) float64 {
	if len(recs) == 0 {
		return 0
	}
	var ok float64
	for _, r := range recs {
		if tier0Correct(r) {
			ok++
		}
	}
	return ok / float64(len(recs))
}

// permutation returns a deterministic permutation of [0,n) from seed. It uses
// splitmix64 rather than math/rand so the sweep is reproducible across Go
// versions, whose global generator is explicitly not stable.
func permutation(n int, seed uint64) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	s := seed | 1
	for i := n - 1; i > 0; i-- {
		s += 0x9e3779b97f4a7c15
		z := s
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z ^= z >> 31
		j := int(z % uint64(i+1))
		idx[i], idx[j] = idx[j], idx[i]
	}
	return idx
}

func hashString(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func joinNote(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}
