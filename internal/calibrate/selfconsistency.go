package calibrate

import (
	"fmt"
	"math"
	"sort"
)

// Matched-cost budgeting for arm (e) of §5.5(2): self-consistency at matched cost.
//
// Arms (a), (b) and (d) are computed offline from profiled records (see
// Baselines), because every tier was run on every problem and each TierObs
// carries that tier's standalone cost and ground-truth correctness. Arm (e)
// cannot be derived that way: "matched cost" means a sampling budget equal to the
// cascade's realized spend, and self-consistency votes over candidates the
// cascade never drew. It needs its own sampling pass.
//
// What CAN be derived offline is whether the arm is well-posed at all — the
// question that has to be answered before paying for it.
//
// # Why the feasibility check comes first
//
// Self-consistency needs a plurality to be meaningful. A vote among two samples
// is a coin flip dressed up as a method, and a vote among one is just that model
// answering once — which is arm (a) or arm (d) with a new label, not a fifth arm.
// So the budget the cascade actually spent has to buy enough samples from the
// model being voted, and whether it does is a property of the recorded costs.
//
// Measured on the n=409 execution records (experiment 21, tau = [1,1], mean
// cascade spend $0.0101/query), it does not, except at the bottom tier:
//
//	tier    median fan-out at matched cost    fan-out < 3
//	small   49                                0/409   (0.0%)
//	mid     2                                 324/409 (79.2%)
//	large   1                                 407/409 (99.5%)
//
// The cascade's whole spend is roughly one frontier sample plus change, because
// escalation to the frontier tier is what the spend mostly *is*. So a
// frontier-model self-consistency arm at matched cost degenerates into
// always-frontier on 99.5% of problems, and a mid-tier one is a two-way vote on
// 79%. Only the cheap tier has real room, and there the arm is informative: 49
// Maverick samples for the price of one cascade query is a genuine alternative
// use of the money, and it is the comparison §3.5 is actually about — 49 votes on
// how the code is *written* against 2 votes on what it *does*.
//
// This is the same trap as experiment 22's uniform absorption: the arm as
// specified ("self-consistency at matched cost", tier unspecified) would have
// reported a degenerate configuration as a null result about self-consistency.
// Hence Feasibility, which is free, and which decides where the paid run points.
//
// Nothing here reaches an acceptance path or a certificate. It is budgeting
// arithmetic over recorded costs.

// SelfConsistencyFeasibility is what a matched-cost budget buys from one tier,
// computed from recorded costs before any sampling is paid for.
type SelfConsistencyFeasibility struct {
	Tier string `json:"tier"`
	// Samples is the tier's configured fan-out in the profiled run, needed because
	// TierObs.Cost is the cost of that whole fan-out, not of one sample.
	Samples int `json:"samples"`
	// MeanBudgetUSD is the cascade's mean realized spend per query under Tau — the
	// budget arm (e) has to match. MeanUnitUSD is this tier's mean per-sample cost.
	MeanBudgetUSD float64 `json:"mean_budget_usd"`
	MeanUnitUSD   float64 `json:"mean_unit_usd"`
	// Fan-out the budget buys, per problem. Median rather than mean because the
	// distribution is heavily skewed by a few expensive problems, and the median is
	// what a reader needs to judge whether the vote is a vote.
	MedianFanout int `json:"median_fanout"`
	P10Fanout    int `json:"p10_fanout"`
	P90Fanout    int `json:"p90_fanout"`
	// Degenerate counts problems where the budget buys fewer than MinVote samples,
	// i.e. where "self-consistency" is not distinguishable from a single draw or a
	// coin flip. A high rate here means the arm as configured measures nothing.
	Degenerate      int     `json:"degenerate"`
	DegenerateFrac  float64 `json:"degenerate_frac"`
	N               int     `json:"n"`
	EstimatedRunUSD float64 `json:"estimated_run_usd"`
	// Verdict states plainly whether this tier is worth paying to sample, since the
	// numbers above are easy to read past.
	Verdict string `json:"verdict"`
}

// MinVote is the smallest fan-out at which a plurality vote carries information
// beyond a single draw. Two samples either agree (mass 1, no discrimination) or
// disagree (a tie broken arbitrarily), so three is the floor.
const MinVote = 3

// SelfConsistencyBudget reports, per tier, what the cascade's realized spend
// under tau would buy in self-consistency samples — the arm (e) feasibility
// check. samplesPerTier gives each tier's configured fan-out in the profiled run,
// because a TierObs's cost covers that whole fan-out rather than one sample.
//
// It costs nothing and must be run before the paid sampling pass: the answer
// determines which tier the arm can meaningfully be run at, and on the n=409
// records it rules out the frontier tier entirely (see the file comment).
func SelfConsistencyBudget(recs []Record, tau []float64, samplesPerTier map[string]int) ([]SelfConsistencyFeasibility, error) {
	use := usableRecords(recs)
	if len(use) == 0 {
		return nil, fmt.Errorf("no usable records: all %d were excluded as contaminated or oracle-unsound", len(recs))
	}
	if len(samplesPerTier) == 0 {
		return nil, fmt.Errorf("samplesPerTier is required: a TierObs cost covers the whole " +
			"configured fan-out, so per-sample cost cannot be recovered without it")
	}

	// Per-problem cascade spend under tau, which is the budget to match. Recomputed
	// here rather than taken from Baselines because arm (e) needs it per problem,
	// not averaged: matching on the mean would hand every problem the same budget
	// and hide that the expensive problems are exactly the degenerate ones.
	budgets := make([]float64, 0, len(use))
	units := map[string][]float64{}
	for _, r := range use {
		if r.CacheHit || len(r.Tiers) == 0 {
			continue
		}
		budgets = append(budgets, cascadeSpend(r, tau))
		for _, t := range r.Tiers {
			n := samplesPerTier[t.Tier]
			if n < 1 {
				return nil, fmt.Errorf("tier %q has no configured fan-out in samplesPerTier; "+
					"per-sample cost is unrecoverable and the budget would be off by that factor", t.Tier)
			}
			units[t.Tier] = append(units[t.Tier], t.Cost/float64(n))
		}
	}
	if len(budgets) == 0 {
		return nil, fmt.Errorf("every usable record was a cache hit or had no tiers; no budget to match")
	}

	// Tier order follows the records, not map iteration, so output is stable.
	var order []string
	for _, t := range use[0].Tiers {
		order = append(order, t.Tier)
	}

	out := make([]SelfConsistencyFeasibility, 0, len(order))
	for _, name := range order {
		u := units[name]
		if len(u) != len(budgets) {
			return nil, fmt.Errorf("tier %q appears in %d records but %d budgets were computed; "+
				"the records are not rectangular and a per-problem match is not defined",
				name, len(u), len(budgets))
		}
		f := SelfConsistencyFeasibility{
			Tier: name, Samples: samplesPerTier[name], N: len(budgets),
			MeanBudgetUSD: mean(budgets), MeanUnitUSD: mean(u),
		}
		fan := make([]int, len(budgets))
		for i := range budgets {
			if u[i] > 0 {
				fan[i] = int(math.Floor(budgets[i] / u[i]))
			}
			if fan[i] < MinVote {
				f.Degenerate++
			}
			// The paid run would spend the matched budget on every problem by
			// construction, so the estimate is the budget itself — stated explicitly
			// because "we will spend exactly what arm (b) spent" is the point of the arm
			// and also the number that needs approval.
			f.EstimatedRunUSD += budgets[i]
		}
		sort.Ints(fan)
		f.MedianFanout = fan[len(fan)/2]
		f.P10Fanout = fan[len(fan)/10]
		f.P90Fanout = fan[9*len(fan)/10]
		f.DegenerateFrac = float64(f.Degenerate) / float64(f.N)

		switch {
		case f.DegenerateFrac >= 0.5:
			f.Verdict = fmt.Sprintf("NOT WORTH RUNNING: the matched budget buys < %d samples on %.1f%% "+
				"of problems, so the arm collapses into a single draw from this tier — that is arm (a) "+
				"or (d) relabelled, and reporting it as self-consistency would report a degenerate "+
				"configuration as a result about the method", MinVote, 100*f.DegenerateFrac)
		case f.DegenerateFrac > 0:
			f.Verdict = fmt.Sprintf("RUNNABLE WITH A CAVEAT: %.1f%% of problems fall below a %d-sample "+
				"vote and must be reported separately rather than pooled", 100*f.DegenerateFrac, MinVote)
		default:
			f.Verdict = fmt.Sprintf("RUNNABLE: every problem affords at least %d samples "+
				"(median %d), so the vote is a vote", MinVote, f.MedianFanout)
		}
		out = append(out, f)
	}
	return out, nil
}

// SelfConsistencyBudgets is the per-problem budget the paid sampling pass must
// match, keyed by record id: exactly what the cascade spent on that problem under
// tau. It is the same quantity SelfConsistencyBudget aggregates, exposed per
// problem because the arm has to be funded problem by problem — a mean-matched
// budget would overfund the cheap problems and underfund the expensive ones, and
// the expensive ones are precisely where the arm degenerates.
//
// Records the calibrator would exclude are excluded here too, and cache hits are
// dropped: there is no tier spend to match on a query arm zero served.
func SelfConsistencyBudgets(recs []Record, tau []float64) (map[string]float64, error) {
	use := usableRecords(recs)
	if len(use) == 0 {
		return nil, fmt.Errorf("no usable records: all %d were excluded as contaminated or oracle-unsound", len(recs))
	}
	out := make(map[string]float64, len(use))
	for _, r := range use {
		if r.CacheHit || len(r.Tiers) == 0 {
			continue
		}
		if r.ID == "" {
			return nil, fmt.Errorf("a usable record has no id; budgets cannot be matched to problems")
		}
		out[r.ID] = cascadeSpend(r, tau)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("every usable record was a cache hit or had no tiers; no budget to match")
	}
	return out, nil
}

// cascadeSpend is what the cascade spends on one problem under tau: the summed
// cost of every tier it reaches, stopping where the acceptance rule fires. It
// mirrors the loop in Baselines; kept separate because arm (e) needs the
// per-problem figure rather than the mean.
func cascadeSpend(r Record, tau []float64) float64 {
	var cost float64
	last := len(r.Tiers) - 1
	for k, t := range r.Tiers {
		cost += t.Cost
		if k == last || (k < len(tau) && t.Score >= tau[k]) {
			break
		}
	}
	return cost
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
