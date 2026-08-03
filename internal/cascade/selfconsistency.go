package cascade

import (
	"context"
	"fmt"
	"math"

	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
)

// Arm (e) of §5.5(2): self-consistency at matched cost.
//
// The last unimplemented arm of the evaluation design. Arms (a), (b) and (d) are
// derived offline from profiled records (calibrate.Baselines) because every tier
// ran on every problem; arm (e) cannot be, because "matched cost" means a sampling
// budget equal to the cascade's realized spend and the vote is over candidates the
// cascade never drew.
//
// # What makes this a real comparison rather than a strawman
//
// §3.5's claim is specifically that for *code*, execution inverts the usual
// tradeoff: self-consistency votes on how candidates are written, behavioural
// clustering votes on what they do, and the latter dominates because running Go is
// cheap. So the arm has to be given every advantage the claim allows:
//
//   - The vote is over normalised source (cluster.TextVote), so formatting and
//     comments do not split agreeing candidates.
//   - The budget is matched per problem, not on average. Matching on the mean would
//     hand every problem the same budget and hide that the expensive problems are
//     exactly the ones where the arm degenerates.
//   - The winner is scored by execution ground truth, the same oracle every other
//     arm is scored by, so the comparison is on delivered correctness.
//
// What the arm must NOT do is consult a verifier to pick its winner. A text vote
// that peeked at execution would be behavioural clustering with extra steps, and
// the comparison would be rigged for §3.5. Hence SelfConsistencyObs records the
// verified-cluster choice too, but only as a *parallel* measurement on the same
// candidates: the two selectors are scored against each other, neither informs the
// other.
//
// # This is a measurement, not a routing path
//
// Nothing here participates in acceptance and no certificate reads it (invariants
// #4, #6). Arm (e) is a baseline the cascade is compared against, and its accuracy
// is descriptive.

// SelfConsistencyObs is one problem's arm (e) result.
type SelfConsistencyObs struct {
	ID   string `json:"id"`
	Tier string `json:"tier"`
	// BudgetUSD is the cascade's realized spend on this problem — the cost arm (e)
	// was given to match. SpentUSD is what the sampling actually cost, which can
	// undershoot (the next sample would have exceeded the budget) but must not
	// overshoot, or the arms are not matched.
	BudgetUSD float64 `json:"budget_usd"`
	SpentUSD  float64 `json:"spent_usd"`
	// OverBudgetUSD is any residual overshoot, which is bounded but not zero: a
	// batch's per-sample cost is only known after it returns, so the final batch can
	// come in above its estimate. Recorded rather than swallowed — an arm that
	// quietly outspends the arm it is "matched" to is a rigged comparison, and this
	// is the number that says whether it did.
	OverBudgetUSD float64 `json:"over_budget_usd,omitempty"`
	// SpecUSD is the shared-oracle cost: the spec (and cascade plan, if configured)
	// this problem needed. Deliberately NOT charged to BudgetUSD — arm (b)'s recorded
	// spend excludes it too, so charging one arm alone would shift the match by a
	// constant. Recorded because it is still real money on the bill, and the figure
	// that gets approved should be the bill rather than the matched portion.
	SpecUSD float64 `json:"spec_usd"`
	// Fanout is how many candidates the budget bought. Below calibrate.MinVote the
	// row is Degenerate: a plurality among two is a coin flip and among one is a
	// single draw, so such rows must be reported separately rather than pooled.
	Fanout     int  `json:"fanout"`
	Degenerate bool `json:"degenerate"`

	// TextCorrect is arm (e) proper: execution ground truth of the candidate the
	// self-consistency vote selected. TextMass is the winning class's raw share.
	TextCorrect bool    `json:"text_correct"`
	TextMass    float64 `json:"text_mass"`

	// ClusterCorrect is the same measurement for §3.5's selector on the SAME
	// candidates — behavioural clustering's representative, scored by the same
	// oracle. This is the head-to-head the arm exists to produce: it isolates the
	// selector, since both arms saw an identical candidate set at identical cost.
	// ClusterScore is the Wilson lower bound (invariant #9), not raw mass, because
	// that is the statistic the router actually uses.
	ClusterCorrect bool    `json:"cluster_correct"`
	ClusterScore   float64 `json:"cluster_score"`
	// ClusterAbstained records that no candidate survived the ladder, so §3.5's
	// selector had nothing to pick. That is a sound refutation of the whole sample
	// (invariant #4) and an escalation in a real cascade — NOT a wrong answer, and
	// scoring it as one would flatter the text vote. Rows with this set must be
	// reported separately.
	ClusterAbstained bool `json:"cluster_abstained"`

	// Agreed reports whether the two selectors picked behaviourally equivalent
	// candidates, which bounds how much the selector choice can matter at all.
	Agreed bool `json:"agreed"`

	Skipped string `json:"skipped,omitempty"`
}

// SelfConsistency runs arm (e) on one problem: draw as many candidates from the
// given tier as budgetUSD affords, pick a winner by text vote, and score it by
// execution ground truth. §3.5's selector is scored on the same candidates.
//
// budgetUSD comes from the profiled records (calibrate.SelfConsistencyBudget),
// which is also where the feasibility of running this at all is decided — at the
// frontier tier the cascade's whole spend buys one sample on 99.5% of problems,
// making the arm arm (a) relabelled. Check that before paying for this.
func (r *Router) SelfConsistency(ctx context.Context, id, problem string, tierIdx int, budgetUSD float64) (*SelfConsistencyObs, error) {
	if tierIdx < 0 || tierIdx >= len(r.cfg.Tiers) {
		return nil, fmt.Errorf("tier index %d out of range for %d tiers", tierIdx, len(r.cfg.Tiers))
	}
	tier := r.cfg.Tiers[tierIdx]
	obs := &SelfConsistencyObs{ID: id, Tier: tier.Name, BudgetUSD: budgetUSD}

	// The spec is the shared oracle every arm is scored against, so its cost is NOT
	// charged to the budget: arm (b)'s recorded spend excludes it too (Profile
	// attributes only tier sampling and the plan), and charging it to one arm alone
	// would make the "matched" budgets differ by a constant. It is still recorded, in
	// SpecUSD, because it is real money.
	specRes := &Result{}
	spec, err := r.spec(ctx, problem, r.pinnedAPI[id], specRes)
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}
	plan, _, _, planUSD := r.cascadePlan(ctx, problem, spec)
	obs.SpecUSD = specRes.Cost.TotalUSD + planUSD

	// One probe sample establishes the per-sample cost for this problem, then the
	// remaining budget decides the fan-out. Estimating from the recorded mean
	// instead would mismatch on exactly the problems that matter: long problems cost
	// more per sample AND are where the cascade escalates, so the two errors
	// compound in the same direction.
	probe, err := r.sampleN(ctx, tierIdx, problem, spec, plan, 1, 0)
	if err != nil {
		return nil, err
	}
	if probe.spent <= 0 {
		obs.Skipped = "probe sample reported zero cost; per-sample cost is unknown so no " +
			"budget can be matched (a mock provider will do this)"
		return obs, nil
	}
	obs.SpentUSD = probe.spent

	// Fan-out is sized off the budget remaining after the probe, which is the same
	// count as floor(budget/unit) — the probe is one of the samples, not an extra.
	//
	// The overshoot this arm actually exhibits comes from somewhere else: the probe's
	// cost is an ESTIMATE of the unit cost, and the remaining samples are drawn in one
	// batch whose true cost is only known once it returns. Output length varies per
	// sample, so a batch priced off a cheap probe comes in above budget. Measured live
	// at 1.7%-17.8% of the matched budget, always upward — because a short probe buys
	// a larger fan-out AND underprices it, so the two errors compound in the same
	// direction. That is recorded (OverBudgetUSD) rather than swallowed: an arm that
	// quietly outspends the arm it is "matched" to is a rigged comparison, and this is
	// the number that says whether it did. Eliminating it would mean drawing one sample
	// at a time and re-pricing after each, which serialises ~50 calls per problem for a
	// few tenths of a cent.
	cands := probe.cands
	if want := int(math.Floor((budgetUSD - probe.spent) / probe.spent)); want > 0 {
		// Seeds continue from the probe so the extra draws are distinct samples rather
		// than repeats of seed 0.
		more, err := r.sampleN(ctx, tierIdx, problem, spec, plan, want, 1)
		if err != nil {
			return nil, err
		}
		cands = append(cands, more.cands...)
		obs.SpentUSD += more.spent
	}
	if obs.SpentUSD > budgetUSD {
		obs.OverBudgetUSD = obs.SpentUSD - budgetUSD
	}
	obs.Fanout = len(cands)
	obs.Degenerate = obs.Fanout < minVote

	// Arm (e)'s own selector: text agreement, no verifier consulted.
	winner, mass, ok := cluster.TextVote(cands)
	if !ok {
		obs.Skipped = "no candidates drawn"
		return obs, nil
	}
	obs.TextMass = mass
	textSrc := cands[indexOf(cands, winner)].Source
	acc, _ := r.acceptOne(ctx, textSrc, spec)
	obs.TextCorrect = acc != nil && acc.OK

	// §3.5's selector on the identical candidate set, so the comparison isolates the
	// selector rather than the sampling budget.
	score, cl := cluster.Score(cluster.Group(cands))
	obs.ClusterScore = score
	if cl == nil {
		// Nothing survived the ladder. An abstention, not an error — see the field doc.
		obs.ClusterAbstained = true
		return obs, nil
	}
	clusterSrc := cands[indexOf(cands, cl.Rep)].Source
	cacc, _ := r.acceptOne(ctx, clusterSrc, spec)
	obs.ClusterCorrect = cacc != nil && cacc.OK
	obs.Agreed = cluster.Behaviour(cands[indexOf(cands, winner)]) ==
		cluster.Behaviour(cands[indexOf(cands, cl.Rep)])
	return obs, nil
}

// minVote mirrors calibrate.MinVote. Duplicated rather than imported to keep the
// dependency pointing one way (calibrate does not import cascade and must not);
// the const is asserted equal in the tests.
const minVote = 3

type sampleBatch struct {
	cands []cluster.Candidate
	spent float64
}

// sampleN draws n candidates from a tier with seeds offset by seed0, reporting
// what they cost. It exists because sampleTier's fan-out is fixed by config, and
// arm (e)'s whole point is a fan-out set by a budget instead.
func (r *Router) sampleN(ctx context.Context, tierIdx int, problem string, spec *prompt.Spec,
	plan string, n, seed0 int,
) (sampleBatch, error) {
	local := &Result{}
	cands, err := r.sampleTierN(ctx, tierIdx, problem, spec, local, nil, plan, n, seed0)
	if err != nil {
		return sampleBatch{}, err
	}
	return sampleBatch{cands: cands, spent: local.Cost.TotalUSD}, nil
}
