// Package calibrate turns a set of recorded cascade observations into
// thresholds that carry a finite-sample, distribution-free risk guarantee.
//
// The method is Learn-then-Test: enumerate a grid of candidate threshold
// vectors, treat "risk(tau) > alpha" as a null hypothesis for each, compute a
// valid p-value from the calibration sample, and correct for multiplicity. The
// selected tau then satisfies
//
//	P[ risk(tau_hat) <= alpha ] >= 1 - delta
//
// over the draw of the calibration set, assuming only exchangeability. No
// parametric model of the confidence score is required, and in particular the
// score does not need to be calibrated — only monotone in correctness.
package calibrate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"
)

// TierObs is what one tier did on one calibration problem, recorded by running
// every tier on every problem so that any threshold vector can be replayed
// offline without re-querying a model.
type TierObs struct {
	Tier  string  `json:"tier"`
	Score float64 `json:"score"` // largest verified behavioural cluster mass
	// Correct is the oracle's verdict: what the calibration arm treats as truth.
	// Under the execution oracle it is "survived the held-out partition"; under
	// the judge oracle it is the judge's PASS. Calibration is computed against
	// this field, so a noisy oracle calibrates against noisy labels.
	Correct bool `json:"correct"`
	// TrueCorrect is execution ground truth (survived the held-out partition),
	// recorded even in the judge arm so the certificate a judge issues can be
	// checked against reality. When it is unset (older records) RealizedRisk
	// falls back to Correct, i.e. it assumes the oracle was sound. See §5.5.
	TrueCorrect *bool   `json:"true_correct,omitempty"`
	Cost        float64 `json:"cost_usd"`
	// DisagreementSource is the accepted candidate's program text, retained ONLY
	// when the arm's oracle verdict differs from execution ground truth
	// (Correct != *TrueCorrect). It exists to make the judge's error *classes*
	// recoverable after the fact: experiment 21 measured eta_fa = 11/1096 with the
	// cheap-tier gradient §3.1 predicts, but could not say what the 11 programs
	// were wrong about, so the paper's claim that the blind spot is
	// *reading-invisible* defects stayed argued rather than confirmed.
	//
	// FORENSIC ONLY. Nothing on the acceptance path may read this field, and no
	// verifier stage may consume it. A field that influenced acceptance would be a
	// new oracle input with no soundness argument behind it (invariants #4, #6).
	//
	// Retained on disagreements alone, not on all observations: at n=409 that is
	// 166 programs rather than 1096, and agreeing observations carry no forensic
	// information by construction.
	DisagreementSource string `json:"disagreement_source,omitempty"`
	// TimedOut marks an observation where some verifier stage was killed by the
	// timeout rather than refuted by the program. The verdict stands (invariant
	// #4 — a program that does not finish is not correct), but a timeout is the
	// one refutation whose cause can be external to the candidate, so a run with
	// a nonzero count may have been measuring its own machine's load.
	//
	// FORENSIC ONLY, exactly as DisagreementSource: nothing on the acceptance
	// path may branch on it. Recorded so that `calibrate` can print a count and a
	// reader can tell "0 timeouts" from "never checked".
	TimedOut bool `json:"timed_out,omitempty"`
}

// truth returns the ground-truth correctness for a tier observation: execution
// truth if it was recorded, otherwise the oracle verdict (which is exact for the
// sound execution oracle and the only thing available for legacy records).
func (t TierObs) truth() bool {
	if t.TrueCorrect != nil {
		return *t.TrueCorrect
	}
	return t.Correct
}

// Disagrees reports whether this arm's oracle verdict differs from execution
// ground truth. It is false when TrueCorrect was never recorded: an observation
// with nothing to compare against is not evidence of agreement, so callers must
// not read false as "the oracle was right" (RealizedRisk documents the same
// caveat for its own fallback).
func (t TierObs) Disagrees() bool {
	return t.TrueCorrect != nil && t.Correct != *t.TrueCorrect
}

// RetainSourceOnDisagreement stores src for later defect classification, but only
// if the oracle and execution truth disagree. Centralised so every recording site
// applies the same rule and no site accidentally retains all 3n candidates.
func (t *TierObs) RetainSourceOnDisagreement(src string) {
	if t.Disagrees() {
		t.DisagreementSource = src
	}
}

// CountTimedOut returns how many records had a verifier stage killed by the
// timeout on any tier. Forensic: it is reported, never used to filter, since a
// timeout is a sound refutation and dropping the records it appears in would
// select the calibration sample on an outcome (invariants #4, #8).
func CountTimedOut(recs []Record) int {
	n := 0
	for _, r := range recs {
		if slices.ContainsFunc(r.Tiers, func(t TierObs) bool { return t.TimedOut }) {
			n++
		}
	}
	return n
}

// MarkTimedOut records that some verifier stage behind this observation was killed
// by the timeout. Sticky: an observation covers a whole fan-out plus an acceptance
// run, and one clock-caused verdict anywhere is enough to make the row suspect.
//
// Deliberately a setter and not a rescoring hook — see TierObs.TimedOut.
func (t *TierObs) MarkTimedOut(b bool) {
	if b {
		t.TimedOut = true
	}
}

// Record is one calibration problem.
type Record struct {
	ID           string    `json:"id"`
	Problem      string    `json:"problem"`
	Tiers        []TierObs `json:"tiers"`
	CacheHit     bool      `json:"cache_hit"`
	CacheCorrect bool      `json:"cache_correct"`
	CacheCost    float64   `json:"cache_cost_usd"`
	// Contaminated marks a record whose oracle was written by the same model
	// that wrote the code. Such records are excluded from calibration.
	Contaminated bool `json:"contaminated"`
	// OracleUnsound marks a record whose generated test suite is not a sound
	// oracle: a known-correct reference solution *compiled* against the generated
	// API but was refuted by a generated assertion (a test/race/accept failure).
	// Rejecting correct code violates invariant #4, so on that problem the labels
	// are noise, not truth. Such records are excluded from calibration exactly
	// like Contaminated ones — only set when calibrating with a reference set
	// (-refs). OracleUnsoundDiag carries the reference's failure output.
	OracleUnsound     bool   `json:"oracle_unsound,omitempty"`
	OracleUnsoundDiag string `json:"oracle_unsound_diag,omitempty"`
	// OracleInconclusive marks a record where the reference solution did not
	// compile against the generated tests — the spec model invented an API name
	// or signature that differs from the reference's canonical one. This is NOT
	// evidence the tests are wrong (a candidate matching the generated API could
	// still be judged soundly), so the record is KEPT in calibration; the flag is
	// recorded only so the run can report how often the reference check could not
	// reach a verdict. Distinguishing this from OracleUnsound is essential: a
	// naming mismatch is not an unsound oracle.
	OracleInconclusive bool `json:"oracle_inconclusive,omitempty"`
	// Shadow marks a record collected on the stream that bypasses the cache.
	// Calibration uses shadow records where available, because the router in a
	// warm system sees the conditional distribution of cache misses, not the
	// query distribution the naive sample would represent.
	Shadow bool `json:"shadow"`
}

// Outcome is the replayed result of a policy on one record.
type Outcome struct {
	Correct    bool
	Cost       float64
	AcceptedAt string
}

// Replay simulates the cascade under thresholds tau, where tau[k] is the accept
// threshold for tier k. The final tier has no threshold: it always accepts.
//
// The cache needs no threshold. Its gate is an executed verification, which is
// a sound refutation rather than a prediction, so there is nothing to calibrate
// and consulting it can only reduce cost at fixed risk.
func Replay(r Record, tau []float64) Outcome {
	if r.CacheHit {
		return Outcome{Correct: r.CacheCorrect, Cost: r.CacheCost, AcceptedAt: "cache"}
	}
	var cost float64
	for k, t := range r.Tiers {
		cost += t.Cost
		last := k == len(r.Tiers)-1
		if last || (k < len(tau) && t.Score >= tau[k]) {
			return Outcome{Correct: t.Correct, Cost: cost, AcceptedAt: t.Tier}
		}
	}
	return Outcome{Correct: false, Cost: cost, AcceptedAt: "exhausted"}
}

// Risk returns the empirical risk and mean cost of a threshold vector, measured
// against the oracle verdict the policy actually routed on. This is the
// quantity Learn-then-Test controls: P[Risk(tau_hat) <= alpha] >= 1 - delta.
//
// If the oracle is unsound, this is risk with respect to the oracle's labels,
// not with respect to truth. RealizedRisk measures the latter, and the gap
// between the two is the judge's noise floor made visible.
func Risk(recs []Record, tau []float64) (risk, cost float64) {
	if len(recs) == 0 {
		return 1, 0
	}
	var bad, total float64
	for _, r := range recs {
		o := Replay(r, tau)
		if !o.Correct {
			bad++
		}
		total += o.Cost
	}
	n := float64(len(recs))
	return bad / n, total / n
}

// RealizedRisk returns the ground-truth risk of a threshold vector: the policy
// accepts wherever its oracle said accept, but a returned solution counts as
// wrong whenever execution truth says it is wrong, regardless of what the oracle
// believed. For the sound execution oracle this equals Risk exactly (its labels
// are truth). For the judge oracle it can be strictly larger -- that difference
// is eta_fa, the false-acceptance rate the judge cannot see and therefore cannot
// certify against (paper §3.1, §5.5c).
func RealizedRisk(recs []Record, tau []float64) float64 {
	if len(recs) == 0 {
		return 1
	}
	var bad float64
	for _, r := range recs {
		if !replayTruth(r, tau) {
			bad++
		}
	}
	return bad / float64(len(recs))
}

// replayTruth mirrors Replay's acceptance decision but reports ground-truth
// correctness of whatever the policy accepted. It must track Replay's control
// flow exactly, or the two risks would describe different policies.
func replayTruth(r Record, tau []float64) bool {
	if r.CacheHit {
		return r.CacheCorrect // the cache gate is executed, so its verdict is truth
	}
	for k, t := range r.Tiers {
		last := k == len(r.Tiers)-1
		if last || (k < len(tau) && t.Score >= tau[k]) {
			return t.truth()
		}
	}
	return false
}

// Method selects the multiplicity correction.
type Method string

// Multiplicity corrections.
const (
	// FixedSequence walks a pre-specified ordering from most to least
	// conservative and stops at the first hypothesis it fails to reject. The
	// ordering must not depend on the data, so it is by threshold magnitude,
	// never by observed risk.
	FixedSequence Method = "fixed-sequence"
	// Bonferroni tests every grid point at delta/|Lambda|. More conservative,
	// but valid under any ordering and simpler to defend.
	Bonferroni Method = "bonferroni"
)

// Certificate is the artefact the router loads. A run without one is explicitly
// uncertified and must not claim a guarantee.
type Certificate struct {
	Valid               bool    `json:"valid"`
	Alpha               float64 `json:"alpha"`
	Delta               float64 `json:"delta"`
	N                   int     `json:"n_calibration"`
	NShadow             int     `json:"n_shadow"`
	NExcluded           int     `json:"n_excluded_contaminated"`
	NOracleUnsound      int     `json:"n_excluded_oracle_unsound,omitempty"`
	NOracleInconclusive int     `json:"n_oracle_inconclusive,omitempty"`
	// NTimedOut counts calibration records in which some verifier stage was killed
	// by the timeout rather than refuted by the program. Those records are KEPT and
	// fully counted: a timeout is a sound refutation (invariant #4), so excluding
	// them would be selecting the calibration sample on an outcome. The number is
	// reported so a run whose risk estimate may reflect its own machine's load is
	// visible rather than needing a log grep.
	//
	// Zero on records collected before this was recorded (pre-#63) means "not
	// recorded", not "none happened" — TimedOut is omitempty, so the two are
	// indistinguishable in a replayed file. A live `calibrate` run prints its own
	// count, where zero does mean zero.
	NTimedOut     int       `json:"n_timed_out,omitempty"`
	Method        Method    `json:"method"`
	GridSize      int       `json:"grid_size"`
	Tiers         []string  `json:"tiers"`
	Thresholds    []float64 `json:"thresholds"`
	EmpiricalRisk float64   `json:"empirical_risk"`
	// RealizedRisk is the ground-truth risk of the selected thresholds (§5.5).
	// Under the sound execution oracle it equals EmpiricalRisk. Under a judge
	// oracle it can exceed alpha even when the certificate is Valid: the judge
	// certified against its own labels, not against truth. When
	// RealizedRisk > Alpha on a Valid certificate, the guarantee is nominal
	// only -- the arm cannot honestly certify at this alpha.
	RealizedRisk float64   `json:"realized_risk"`
	PValue       float64   `json:"p_value"`
	ExpectedCost float64   `json:"expected_cost_usd"`
	MutationMean float64   `json:"mutation_score_mean"`
	Note         string    `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Options configures a calibration run.
type Options struct {
	Alpha  float64
	Delta  float64
	Step   float64 // grid resolution, e.g. 0.1
	Method Method
}

// Calibrate selects the cheapest threshold vector whose risk is certified at
// level alpha with confidence 1-delta.
func Calibrate(recs []Record, tiers []string, opts Options) (*Certificate, error) {
	if opts.Step <= 0 {
		opts.Step = 0.1
	}
	if opts.Delta <= 0 {
		opts.Delta = 0.1
	}
	excluded := 0
	unsound := 0
	inconclusive := 0
	var use []Record
	for _, r := range recs {
		if r.Contaminated {
			excluded++
			continue
		}
		// A generated test suite that refutes a known-correct reference is not a
		// sound oracle (invariant #4): its labels are noise. Drop it exactly like
		// a contaminated record so the risk estimate reflects real defects, not
		// spec-model test bugs. Only ever set when calibrating with -refs.
		if r.OracleUnsound {
			unsound++
			continue
		}
		// Inconclusive means the reference did not fit the generated API (a naming
		// mismatch, not an unsound test), so the check reached no verdict. Keep the
		// record — it is only tallied for reporting.
		if r.OracleInconclusive {
			inconclusive++
		}
		use = append(use, r)
	}
	// Prefer the shadow stream: a warm cache absorbs the head of the query
	// distribution, so records collected behind it are not exchangeable with
	// what the router actually sees.
	var shadow []Record
	for _, r := range use {
		if r.Shadow {
			shadow = append(shadow, r)
		}
	}
	note := ""
	if len(shadow) >= 30 {
		use, note = shadow, "calibrated on the cache-bypass shadow stream"
	} else if len(shadow) > 0 {
		note = fmt.Sprintf("only %d shadow records; calibrated on the full sample, which may be optimistic once the cache warms", len(shadow))
	} else {
		note = "no shadow records: calibration assumes a cold cache and will drift as it warms"
	}

	n := len(use)
	if n == 0 {
		if excluded > 0 {
			return nil, fmt.Errorf("all %d calibration records were excluded as contaminated: "+
				"the test model also appears as a code tier, so every oracle shares an author with "+
				"the code it judges. Set test_model to a model that is not in the tier list", excluded)
		}
		if unsound > 0 {
			return nil, fmt.Errorf("all %d calibration records were excluded as oracle-unsound: "+
				"every generated test suite refuted its reference solution. The spec model is "+
				"producing broken tests; inspect them before trusting any risk number", unsound)
		}
		return nil, fmt.Errorf("no calibration records")
	}

	free := max(len(tiers)-1, 0)
	grid := buildGrid(free, opts.Step)
	cert := &Certificate{
		Alpha: opts.Alpha, Delta: opts.Delta, N: n, NShadow: len(shadow),
		NExcluded: excluded, NOracleUnsound: unsound, NOracleInconclusive: inconclusive,
		Method: opts.Method, GridSize: len(grid),
		Tiers: tiers, Note: note, CreatedAt: time.Now().UTC(),
		// Counted over the stream actually calibrated on (shadow subset when it was
		// preferred), because that is the sample this certificate describes.
		NTimedOut: CountTimedOut(use),
	}
	if len(grid) == 0 {
		cert.Valid = true
		cert.EmpiricalRisk, cert.ExpectedCost = Risk(use, nil)
		cert.RealizedRisk = RealizedRisk(use, nil)
		flagIfNominal(cert)
		return cert, nil
	}

	type cand struct {
		tau  []float64
		risk float64
		cost float64
		p    float64
	}
	eval := make([]cand, 0, len(grid))
	for _, tau := range grid {
		risk, cost := Risk(use, tau)
		eval = append(eval, cand{tau: tau, risk: risk, cost: cost,
			p: HoeffdingBentkus(risk, n, opts.Alpha)})
	}

	var best *cand
	switch opts.Method {
	case Bonferroni:
		thresh := opts.Delta / float64(len(grid))
		for i := range eval {
			if eval[i].p <= thresh && (best == nil || eval[i].cost < best.cost) {
				best = &eval[i]
			}
		}
	default: // FixedSequence
		// Data-independent ordering: most conservative first. Higher
		// thresholds escalate more often, so they cost more and risk less.
		order := make([]int, len(eval))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			ta, tb := eval[order[a]].tau, eval[order[b]].tau
			sa, sb := sum(ta), sum(tb)
			if sa != sb {
				return sa > sb
			}
			return slices.Compare(tb, ta) < 0
		})
		for _, idx := range order {
			if eval[idx].p > opts.Delta {
				break // first failure to reject ends the sequence
			}
			c := eval[idx]
			if best == nil || c.cost < best.cost {
				best = &c
			}
		}
	}

	if best == nil {
		cert.Valid = false
		cheapest := slices.MinFunc(eval, func(a, b cand) int {
			return cmpFloat(a.risk, b.risk)
		})
		cert.EmpiricalRisk = cheapest.risk
		cert.PValue = cheapest.p
		cert.Thresholds = cheapest.tau
		cert.ExpectedCost = cheapest.cost
		cert.RealizedRisk = RealizedRisk(use, cheapest.tau)
		cert.Note = fmt.Sprintf("%s; no threshold vector certifies alpha=%.3f at delta=%.3f with n=%d. "+
			"Lowest achievable empirical risk is %.3f. Raise alpha, add calibration data, or add a tier.",
			note, opts.Alpha, opts.Delta, n, cheapest.risk)
		return cert, nil
	}

	cert.Valid = true
	cert.Thresholds = best.tau
	cert.EmpiricalRisk = best.risk
	cert.PValue = best.p
	cert.ExpectedCost = best.cost
	cert.RealizedRisk = RealizedRisk(use, best.tau)
	flagIfNominal(cert)
	return cert, nil
}

// flagIfNominal appends a warning when a valid certificate's ground-truth risk
// exceeds alpha. That is the judge oracle's tell: it certified against its own
// verdicts, not truth, so the Valid flag would otherwise imply a guarantee the
// arm has not earned. The sound execution oracle never trips this, since its
// labels are truth and realized risk equals empirical risk.
func flagIfNominal(cert *Certificate) {
	if cert.RealizedRisk <= cert.Alpha+1e-9 {
		return
	}
	msg := fmt.Sprintf("WARNING: realized (ground-truth) risk %.3f exceeds alpha %.3f. "+
		"The oracle certified against its own verdicts, not against execution truth; "+
		"this certificate is nominal only. This is the judge-oracle noise floor of §3.1",
		cert.RealizedRisk, cert.Alpha)
	if cert.Note == "" {
		cert.Note = msg
	} else {
		cert.Note = strings.TrimSpace(cert.Note) + ". " + msg
	}
}

func sum(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// buildGrid enumerates threshold vectors of the given arity.
func buildGrid(arity int, step float64) [][]float64 {
	if arity <= 0 {
		return nil
	}
	var levels []float64
	for v := 0.0; v <= 1.0+1e-9; v += step {
		levels = append(levels, math.Round(v*1000)/1000)
	}
	grid := [][]float64{{}}
	for range arity {
		var next [][]float64
		for _, g := range grid {
			for _, l := range levels {
				row := make([]float64, len(g), len(g)+1)
				copy(row, g)
				next = append(next, append(row, l))
			}
		}
		grid = next
	}
	return grid
}

// Save writes a certificate.
func Save(path string, c *Certificate) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads a certificate. A missing file is not an error: the router falls
// back to uncalibrated priors and says so.
func Load(path string) (*Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Certificate
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
