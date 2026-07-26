package calibrate

import "testing"

func TestBaselines(t *testing.T) {
	tp, fp := true, false
	// Two tiers. Problem A: cheap correct. Problem B: cheap wrong, frontier
	// correct (score 0.9 at tier0). Costs: tier0=1, tier1=10 per problem.
	recs := []Record{
		{Tiers: []TierObs{
			{Tier: "small", Score: 0.9, Cost: 1, TrueCorrect: &tp},
			{Tier: "large", Score: 0, Cost: 10, TrueCorrect: &tp},
		}},
		{Tiers: []TierObs{
			{Tier: "small", Score: 0.9, Cost: 1, TrueCorrect: &fp}, // cheap is wrong here
			{Tier: "large", Score: 0, Cost: 10, TrueCorrect: &tp},
		}},
	}

	// tau below tier0's score => cascade always accepts at tier0 (never escalates).
	got := Baselines(recs, []float64{0.5})
	by := map[string]Policy{}
	for _, p := range got {
		by[p.Name] = p
	}

	// always-cheapest: accepts tier0 both times. Wrong on B => risk 0.5, cost 1.
	if c := by["always-cheapest"]; c.Risk != 0.5 || c.MeanUSD != 1 {
		t.Errorf("cheapest: risk=%.3f cost=%.3f, want 0.5 / 1", c.Risk, c.MeanUSD)
	}
	// always-frontier: last tier correct both times. risk 0, cost 10.
	if f := by["always-frontier"]; f.Risk != 0 || f.MeanUSD != 10 {
		t.Errorf("frontier: risk=%.3f cost=%.3f, want 0 / 10", f.Risk, f.MeanUSD)
	}
	// cascade with tau=0.5: tier0 score 0.9 >= 0.5, accepts tier0 both times ==
	// same as cheapest here (risk 0.5, cost 1). Demonstrates a too-loose tau.
	if c := by["cascade"]; c.Risk != 0.5 || c.MeanUSD != 1 {
		t.Errorf("cascade(tau=0.5): risk=%.3f cost=%.3f, want 0.5 / 1", c.Risk, c.MeanUSD)
	}

	// Tighten tau above tier0's score => cascade escalates to tier1 on both.
	// Now risk 0 (frontier correct) at cost 1+10=11 (paid both tiers).
	got2 := Baselines(recs, []float64{0.95})
	for _, p := range got2 {
		if p.Name == "cascade" {
			if p.Risk != 0 || p.MeanUSD != 11 {
				t.Errorf("cascade(tau=0.95): risk=%.3f cost=%.3f, want 0 / 11", p.Risk, p.MeanUSD)
			}
		}
	}
}

func TestBaselinesSkipsCacheAndEmpty(t *testing.T) {
	tp := true
	recs := []Record{
		{CacheHit: true, CacheCorrect: true, CacheCost: 0.001},
		{Tiers: nil},
		{Tiers: []TierObs{{Tier: "t", Score: 1, Cost: 2, TrueCorrect: &tp}}},
	}
	got := Baselines(recs, nil)
	for _, p := range got {
		if p.N != 1 {
			t.Errorf("%s: N=%d, want 1 (cache and empty records skipped)", p.Name, p.N)
		}
	}
}
