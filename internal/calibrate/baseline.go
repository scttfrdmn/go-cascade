package calibrate

// Baselines compares the cascade against the two single-model policies it must
// beat, answering the project's central unrun question (README "known gaps"):
// is the cascade cheaper than always calling the frontier model at equal
// correctness, and more correct than always calling the cheapest?
//
// All three policies are computed offline from the same profiled records — each
// tier was run on every problem, so each TierObs carries that tier's standalone
// cost and ground-truth correctness. Correctness here is ALWAYS execution truth
// (TierObs.truth()), never an oracle verdict, so the arms are compared on what
// they actually deliver, not on what any oracle believed.
//
// tau is the cascade's calibrated threshold vector (nil = escalate-only priors
// via the always-accept-last-tier rule; pass the certified thresholds to score
// the calibrated cascade).

// Policy is one routing strategy's measured cost and correctness over a record set.
type Policy struct {
	Name    string  `json:"name"`
	Risk    float64 `json:"risk"`     // ground-truth error rate (1 - accuracy)
	MeanUSD float64 `json:"mean_usd"` // mean cost per query
	N       int     `json:"n"`
}

// Baselines evaluates always-cheapest, the cascade under tau, and
// always-frontier over recs. It skips records with no usable tier data.
func Baselines(recs []Record, tau []float64) []Policy {
	var cheapN, cascN, frontN int
	var cheapBad, cascBad, frontBad float64
	var cheapCost, cascCost, frontCost float64

	for _, r := range recs {
		if r.CacheHit || len(r.Tiers) == 0 {
			continue // baselines are about the model tiers, not the cache path
		}
		last := len(r.Tiers) - 1

		// Always-cheapest: only ever tier 0, at tier 0's standalone cost.
		cheapN++
		cheapCost += r.Tiers[0].Cost
		if !r.Tiers[0].truth() {
			cheapBad++
		}

		// Always-frontier: only ever the last tier, at its standalone cost.
		frontN++
		frontCost += r.Tiers[last].Cost
		if !r.Tiers[last].truth() {
			frontBad++
		}

		// Cascade: escalate by tau, accumulating cost, accept where the rule fires.
		var cost float64
		accepted := last
		for k, t := range r.Tiers {
			cost += t.Cost
			if k == last || (k < len(tau) && t.Score >= tau[k]) {
				accepted = k
				break
			}
		}
		cascN++
		cascCost += cost
		if !r.Tiers[accepted].truth() {
			cascBad++
		}
	}

	mk := func(name string, bad, cost float64, n int) Policy {
		p := Policy{Name: name, N: n}
		if n > 0 {
			p.Risk = bad / float64(n)
			p.MeanUSD = cost / float64(n)
		}
		return p
	}
	return []Policy{
		mk("always-cheapest", cheapBad, cheapCost, cheapN),
		mk("cascade", cascBad, cascCost, cascN),
		mk("always-frontier", frontBad, frontCost, frontN),
	}
}
