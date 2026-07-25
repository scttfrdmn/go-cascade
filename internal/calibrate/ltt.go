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
	"time"
)

// TierObs is what one tier did on one calibration problem, recorded by running
// every tier on every problem so that any threshold vector can be replayed
// offline without re-querying a model.
type TierObs struct {
	Tier    string  `json:"tier"`
	Score   float64 `json:"score"`   // largest verified behavioural cluster mass
	Correct bool    `json:"correct"` // survived the held-out partition
	Cost    float64 `json:"cost_usd"`
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

// Risk returns the empirical risk and mean cost of a threshold vector.
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
	Valid         bool      `json:"valid"`
	Alpha         float64   `json:"alpha"`
	Delta         float64   `json:"delta"`
	N             int       `json:"n_calibration"`
	NShadow       int       `json:"n_shadow"`
	NExcluded     int       `json:"n_excluded_contaminated"`
	Method        Method    `json:"method"`
	GridSize      int       `json:"grid_size"`
	Tiers         []string  `json:"tiers"`
	Thresholds    []float64 `json:"thresholds"`
	EmpiricalRisk float64   `json:"empirical_risk"`
	PValue        float64   `json:"p_value"`
	ExpectedCost  float64   `json:"expected_cost_usd"`
	MutationMean  float64   `json:"mutation_score_mean"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
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
	var use []Record
	for _, r := range recs {
		if r.Contaminated {
			excluded++
			continue
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
		return nil, fmt.Errorf("no calibration records")
	}

	free := max(len(tiers)-1, 0)
	grid := buildGrid(free, opts.Step)
	cert := &Certificate{
		Alpha: opts.Alpha, Delta: opts.Delta, N: n, NShadow: len(shadow),
		NExcluded: excluded, Method: opts.Method, GridSize: len(grid),
		Tiers: tiers, Note: note, CreatedAt: time.Now().UTC(),
	}
	if len(grid) == 0 {
		cert.Valid = true
		cert.EmpiricalRisk, cert.ExpectedCost = Risk(use, nil)
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
	return cert, nil
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
