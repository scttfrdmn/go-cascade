package calibrate

import (
	"math"
	"strings"
	"testing"
)

// scRecords builds records whose per-tier costs are known, so the fan-out a
// matched budget buys is computable by hand rather than asserted against a golden
// value. Tier costs are for the WHOLE configured fan-out, which is the subtlety
// SelfConsistencyBudget exists to get right.
func scRecords(n int, cheapCost, midCost, frontCost float64, score0 float64) []Record {
	recs := make([]Record, 0, n)
	for i := range n {
		yes := true
		recs = append(recs, Record{
			ID: "p" + itoa(i), Shadow: true,
			Tiers: []TierObs{
				{Tier: "small", Score: score0, Correct: true, TrueCorrect: &yes, Cost: cheapCost},
				{Tier: "mid", Score: 0.5, Correct: true, TrueCorrect: &yes, Cost: midCost},
				{Tier: "large", Score: 0.9, Correct: true, TrueCorrect: &yes, Cost: frontCost},
			},
		})
	}
	return recs
}

func scFanout() map[string]int { return map[string]int{"small": 2, "mid": 1, "large": 1} }

// The load-bearing arithmetic: a TierObs cost covers the tier's whole fan-out, so
// per-sample cost is cost/samples. Getting this wrong scales the entire budget by
// the fan-out factor and would silently double the cheap tier's apparent capacity
// on the shipped 2:1:1 config.
func TestBudgetDividesTierCostByItsConfiguredFanout(t *testing.T) {
	// Cheap tier: $0.004 for 2 samples = $0.002 each. Escalate-always budget is
	// 0.004 + 0.010 + 0.020 = $0.034, so the cheap tier affords 17 samples.
	recs := scRecords(20, 0.004, 0.010, 0.020, 0.5)
	rows, err := SelfConsistencyBudget(recs, []float64{1, 1}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	byTier := map[string]SelfConsistencyFeasibility{}
	for _, r := range rows {
		byTier[r.Tier] = r
	}
	if got := byTier["small"]; math.Abs(got.MeanUnitUSD-0.002) > 1e-12 {
		t.Errorf("small unit cost %v, want 0.002 (= 0.004 for a fan-out of 2)", got.MeanUnitUSD)
	}
	if got := byTier["small"].MedianFanout; got != 17 {
		t.Errorf("small fan-out %d, want 17 (budget 0.034 / 0.002)", got)
	}
	// Frontier: $0.020 for 1 sample, budget $0.034 → 1 sample. This is the shape of
	// the real finding — the arm degenerates where the cascade's spend IS the
	// frontier call.
	if got := byTier["large"].MedianFanout; got != 1 {
		t.Errorf("large fan-out %d, want 1", got)
	}
}

// The finding that decides where the paid run points: at the frontier tier the
// cascade's own spend buys a single sample, so "self-consistency at matched cost"
// is always-frontier with a new label. A harness that pooled that with the cheap
// tier's 49-way vote would report a degenerate configuration as a result about
// self-consistency — the same trap as experiment 22's uniform absorption.
func TestFrontierTierIsFlaggedNotWorthRunning(t *testing.T) {
	recs := scRecords(50, 0.004, 0.010, 0.020, 0.5)
	rows, err := SelfConsistencyBudget(recs, []float64{1, 1}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	var large, small SelfConsistencyFeasibility
	for _, r := range rows {
		switch r.Tier {
		case "large":
			large = r
		case "small":
			small = r
		}
	}
	if large.DegenerateFrac < 0.5 {
		t.Fatalf("frontier degenerate fraction %.3f; the fixture no longer reproduces the "+
			"condition the verdict is about", large.DegenerateFrac)
	}
	if !strings.Contains(large.Verdict, "NOT WORTH RUNNING") {
		t.Errorf("frontier verdict = %q, want a refusal", large.Verdict)
	}
	if small.Degenerate != 0 {
		t.Errorf("cheap tier had %d degenerate problems, want 0", small.Degenerate)
	}
	if !strings.Contains(small.Verdict, "RUNNABLE") || strings.Contains(small.Verdict, "NOT WORTH") {
		t.Errorf("cheap verdict = %q, want RUNNABLE", small.Verdict)
	}
}

// MinVote is 3 for a reason, and the boundary is where a wrong inequality hides: a
// 2-sample vote either agrees (mass 1, no discrimination) or ties.
func TestMinVoteBoundaryIsExclusiveOfTwo(t *testing.T) {
	// Unit cost chosen so the budget buys exactly 2 at the frontier tier: budget
	// 0.004 + 0.010 + 0.010 = 0.024, frontier unit 0.010 → floor(2.4) = 2.
	recs := scRecords(10, 0.004, 0.010, 0.010, 0.5)
	rows, err := SelfConsistencyBudget(recs, []float64{1, 1}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Tier != "large" {
			continue
		}
		if r.MedianFanout != 2 {
			t.Fatalf("fixture drifted: frontier fan-out %d, want exactly 2", r.MedianFanout)
		}
		if r.Degenerate != r.N {
			t.Errorf("a 2-sample vote counted %d/%d degenerate, want all: two samples cannot "+
				"form a plurality", r.Degenerate, r.N)
		}
	}
}

// The budget must be matched PER PROBLEM. Matching on the mean would hand every
// problem the same money and hide that the expensive problems are exactly the ones
// where the arm degenerates — the correlation is the whole reason the frontier row
// is unrunnable.
func TestBudgetIsPerProblemNotMeanMatched(t *testing.T) {
	// Half the problems accept at tier 0 (cheap, score clears tau) and half escalate
	// all the way, so the per-problem budgets are bimodal and the mean is between.
	var recs []Record
	yes := true
	for i := range 40 {
		score := 0.9
		if i%2 == 1 {
			score = 0.0 // escalates
		}
		recs = append(recs, Record{
			ID: "p" + itoa(i), Shadow: true,
			Tiers: []TierObs{
				{Tier: "small", Score: score, Correct: true, TrueCorrect: &yes, Cost: 0.004},
				{Tier: "mid", Score: 0.0, Correct: true, TrueCorrect: &yes, Cost: 0.010},
				{Tier: "large", Score: 0.9, Correct: true, TrueCorrect: &yes, Cost: 0.020},
			},
		})
	}
	rows, err := SelfConsistencyBudget(recs, []float64{0.5, 0.5}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Tier != "small" {
			continue
		}
		// Accepting problems get $0.004 → 2 samples; escalating ones get $0.034 → 17.
		// A mean-matched budget would give every problem $0.019 → 9, so the p10 and
		// p90 would both be 9 and nothing would be flagged degenerate.
		if r.P10Fanout != 2 || r.P90Fanout != 17 {
			t.Errorf("fan-out spread p10=%d p90=%d, want 2 and 17; a single spread value means "+
				"the budget was averaged before matching", r.P10Fanout, r.P90Fanout)
		}
		if r.Degenerate == 0 {
			t.Error("no degenerate rows, so the cheap problems' 2-sample budget was hidden by " +
				"averaging")
		}
	}
}

// The estimate has to be the budget itself: arm (e) spends exactly what arm (b)
// spent, by construction. That number is what needs spend approval, so an
// understated one is not a rounding issue.
func TestEstimatedRunCostEqualsTheMatchedBudget(t *testing.T) {
	recs := scRecords(30, 0.004, 0.010, 0.020, 0.5)
	rows, err := SelfConsistencyBudget(recs, []float64{1, 1}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	want := 30 * 0.034
	for _, r := range rows {
		if math.Abs(r.EstimatedRunUSD-want) > 1e-9 {
			t.Errorf("tier %s estimated $%.4f, want $%.4f (the summed matched budget)",
				r.Tier, r.EstimatedRunUSD, want)
		}
	}
}

// A missing fan-out is an error, not a default of 1. Defaulting would multiply the
// per-sample cost by the real fan-out and understate capacity — silently, with a
// plausible-looking number out the other end.
func TestMissingFanoutIsAnErrorNotAssumedOne(t *testing.T) {
	recs := scRecords(5, 0.004, 0.010, 0.020, 0.5)
	if _, err := SelfConsistencyBudget(recs, []float64{1, 1}, map[string]int{"small": 2}); err == nil {
		t.Error("a samplesPerTier missing two tiers was accepted; per-sample cost would be wrong " +
			"by the fan-out factor with no sign of it")
	}
	if _, err := SelfConsistencyBudget(recs, []float64{1, 1}, nil); err == nil {
		t.Error("a nil samplesPerTier was accepted")
	}
}

// Excluded records must be dropped before budgeting, exactly as Calibrate drops
// them, or the budget is matched against a spend the certified cascade never had.
func TestExcludedRecordsAreDroppedBeforeBudgeting(t *testing.T) {
	recs := scRecords(10, 0.004, 0.010, 0.020, 0.5)
	recs[0].Contaminated = true
	recs[1].OracleUnsound = true
	rows, err := SelfConsistencyBudget(recs, []float64{1, 1}, scFanout())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].N != 8 {
		t.Errorf("N = %d, want 8 after excluding one contaminated and one unsound record", rows[0].N)
	}
	if _, err := SelfConsistencyBudget([]Record{{Contaminated: true}}, []float64{1}, scFanout()); err == nil {
		t.Error("a corpus with no usable records was accepted")
	}
}
