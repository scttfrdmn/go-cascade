package calibrate

import (
	"math"
	"testing"
)

// binomCDFDirect sums the pmf exactly, for small n.
func binomCDFDirect(k, n int, p float64) float64 {
	if k >= n {
		return 1
	}
	var sum float64
	for i := 0; i <= k; i++ {
		lc, _ := math.Lgamma(float64(n + 1))
		li, _ := math.Lgamma(float64(i + 1))
		ln, _ := math.Lgamma(float64(n - i + 1))
		logp := lc - li - ln
		if i > 0 {
			logp += float64(i) * math.Log(p)
		}
		if n-i > 0 {
			logp += float64(n-i) * math.Log(1-p)
		}
		sum += math.Exp(logp)
	}
	return sum
}

func TestBinomCDFMatchesDirectSum(t *testing.T) {
	for _, n := range []int{1, 5, 20, 100, 500} {
		for _, p := range []float64{0.01, 0.05, 0.25, 0.5, 0.9} {
			for _, frac := range []float64{0, 0.1, 0.5, 0.9} {
				k := int(frac * float64(n))
				got := BinomCDF(k, n, p)
				want := binomCDFDirect(k, n, p)
				if math.Abs(got-want) > 1e-9 {
					t.Errorf("BinomCDF(%d,%d,%g) = %.12g, direct sum = %.12g", k, n, p, got, want)
				}
			}
		}
	}
}

// With zero observed errors the Hoeffding term collapses to the exact
// probability of seeing no errors when the true risk sits at alpha. This is the
// cheapest end-to-end check that h1 and the exponent are right.
func TestHoeffdingBentkusZeroRiskClosedForm(t *testing.T) {
	for _, n := range []int{10, 50, 200} {
		for _, alpha := range []float64{0.01, 0.05, 0.2} {
			exact := math.Pow(1-alpha, float64(n))
			got := HoeffdingBentkus(0, n, alpha)
			if got > exact+1e-12 {
				t.Errorf("n=%d alpha=%g: p=%.6g exceeds the closed form %.6g", n, alpha, got, exact)
			}
		}
	}
}

func TestHoeffdingBentkusIsAValidPValueShape(t *testing.T) {
	// No evidence when the empirical risk is at or above alpha.
	if p := HoeffdingBentkus(0.05, 100, 0.05); p != 1 {
		t.Errorf("rhat == alpha should give p=1, got %g", p)
	}
	if p := HoeffdingBentkus(0.2, 100, 0.05); p != 1 {
		t.Errorf("rhat > alpha should give p=1, got %g", p)
	}
	// More data at the same empirical risk must be at least as strong.
	prev := 1.0
	for _, n := range []int{10, 50, 100, 500, 2000} {
		p := HoeffdingBentkus(0.01, n, 0.05)
		if p > prev+1e-12 {
			t.Errorf("p-value increased with n: n=%d p=%g (previous %g)", n, p, prev)
		}
		prev = p
	}
	// A lower empirical risk must be at least as strong.
	hi := HoeffdingBentkus(0.04, 200, 0.05)
	lo := HoeffdingBentkus(0.005, 200, 0.05)
	if lo > hi {
		t.Errorf("lower empirical risk gave a weaker p-value: %g > %g", lo, hi)
	}
	if p := HoeffdingBentkus(0.01, 0, 0.05); p != 1 {
		t.Errorf("n=0 must give p=1, got %g", p)
	}
}

// The certificate must be unobtainable when the sample is too small to support
// it, which is the property that stops the tool claiming a bound it cannot back.
func TestSmallSampleCannotCertifyTightAlpha(t *testing.T) {
	if p := HoeffdingBentkus(0, 5, 0.01); p <= 0.1 {
		t.Errorf("n=5 should not certify alpha=0.01 at delta=0.1, got p=%g", p)
	}
	if p := HoeffdingBentkus(0, 400, 0.01); p > 0.1 {
		t.Errorf("n=400 with zero errors should certify alpha=0.01, got p=%g", p)
	}
}

func TestReplayAndRisk(t *testing.T) {
	mk := func(scores []float64, correct []bool) Record {
		r := Record{}
		for i := range scores {
			r.Tiers = append(r.Tiers, TierObs{
				Tier: string(rune('a' + i)), Score: scores[i],
				Correct: correct[i], Cost: float64(i+1) * 0.001,
			})
		}
		return r
	}
	// Cheap tier is confident and right: a low threshold accepts it.
	r1 := mk([]float64{0.9, 1.0}, []bool{true, true})
	// Cheap tier is confident and wrong: only a threshold above 0.8 escalates.
	r2 := mk([]float64{0.8, 1.0}, []bool{false, true})

	if o := Replay(r1, []float64{0.5}); o.AcceptedAt != "a" || !o.Correct {
		t.Errorf("expected acceptance at the cheap tier, got %+v", o)
	}
	if o := Replay(r2, []float64{0.9}); o.AcceptedAt != "b" || !o.Correct {
		t.Errorf("expected escalation to the second tier, got %+v", o)
	}
	risk, cost := Risk([]Record{r1, r2}, []float64{0.5})
	if risk != 0.5 {
		t.Errorf("risk = %g, want 0.5", risk)
	}
	if math.Abs(cost-0.001) > 1e-12 {
		t.Errorf("cost = %g, want 0.001", cost)
	}
	risk, cost = Risk([]Record{r1, r2}, []float64{0.9})
	if risk != 0 {
		t.Errorf("risk = %g, want 0 at the conservative threshold", risk)
	}
	if math.Abs(cost-0.002) > 1e-12 { // r1 accepts at a (0.001), r2 escalates (0.003)
		t.Errorf("cost = %g, want 0.002", cost)
	}

	// The cache short-circuits everything.
	rc := Record{CacheHit: true, CacheCorrect: true, CacheCost: 0.0001}
	if o := Replay(rc, []float64{0.9}); o.AcceptedAt != "cache" || o.Cost != 0.0001 {
		t.Errorf("cache replay wrong: %+v", o)
	}
}

// The headline monotonicity claim: adding the cache cannot raise cost at equal
// risk, because its gate can always reject.
func TestCacheIsCostMonotone(t *testing.T) {
	base := []Record{
		{Tiers: []TierObs{{Tier: "a", Score: 0.4, Correct: true, Cost: 0.01},
			{Tier: "b", Score: 1, Correct: true, Cost: 0.10}}},
		{Tiers: []TierObs{{Tier: "a", Score: 0.9, Correct: true, Cost: 0.01},
			{Tier: "b", Score: 1, Correct: true, Cost: 0.10}}},
	}
	withCache := make([]Record, len(base))
	copy(withCache, base)
	withCache[0].CacheHit = true
	withCache[0].CacheCorrect = true
	withCache[0].CacheCost = 0.0005

	tau := []float64{0.5}
	rNo, cNo := Risk(base, tau)
	rYes, cYes := Risk(withCache, tau)
	if rYes > rNo {
		t.Errorf("cache raised risk: %g > %g", rYes, rNo)
	}
	if cYes > cNo {
		t.Errorf("cache raised cost: %g > %g", cYes, cNo)
	}
}

func TestCalibrateSelectsCheapestValidThreshold(t *testing.T) {
	// 200 records. The cheap tier is right whenever its score is high, and
	// wrong when it is low, so a threshold near 0.5 should certify.
	var recs []Record
	for i := range 200 {
		high := i%10 != 0 // 90% high-confidence and correct
		score, correct := 0.2, false
		if high {
			score, correct = 0.9, true
		}
		recs = append(recs, Record{
			ID: string(rune(i)),
			Tiers: []TierObs{
				{Tier: "small", Score: score, Correct: correct, Cost: 0.001},
				{Tier: "large", Score: 1.0, Correct: true, Cost: 0.05},
			},
		})
	}
	cert, err := Calibrate(recs, []string{"small", "large"}, Options{
		Alpha: 0.05, Delta: 0.1, Step: 0.1, Method: FixedSequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cert.Valid {
		t.Fatalf("expected a valid certificate, got note %q", cert.Note)
	}
	if len(cert.Thresholds) != 1 {
		t.Fatalf("want 1 free threshold, got %v", cert.Thresholds)
	}
	if cert.Thresholds[0] <= 0.2 || cert.Thresholds[0] > 0.9 {
		t.Errorf("threshold %v does not separate the two score modes", cert.Thresholds)
	}
	if cert.EmpiricalRisk > 0.05 {
		t.Errorf("empirical risk %g exceeds alpha", cert.EmpiricalRisk)
	}
	// Bonferroni is more conservative but must also succeed here.
	cb, err := Calibrate(recs, []string{"small", "large"}, Options{
		Alpha: 0.05, Delta: 0.1, Step: 0.1, Method: Bonferroni,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cb.Valid {
		t.Errorf("bonferroni failed to certify: %s", cb.Note)
	}
}

// A tool that cannot certify must say so rather than emit a threshold anyway.
func TestCalibrateRefusesWhenNoThresholdWorks(t *testing.T) {
	var recs []Record
	for range 100 {
		recs = append(recs, Record{
			Tiers: []TierObs{
				{Tier: "small", Score: 0.9, Correct: false, Cost: 0.001},
				{Tier: "large", Score: 1.0, Correct: false, Cost: 0.05},
			},
		})
	}
	cert, err := Calibrate(recs, []string{"small", "large"}, Options{
		Alpha: 0.05, Delta: 0.1, Step: 0.25, Method: FixedSequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cert.Valid {
		t.Error("certified a policy whose top tier is always wrong")
	}
	if cert.Note == "" {
		t.Error("an invalid certificate must explain itself")
	}
}

func TestCalibrateExcludesContaminatedRecords(t *testing.T) {
	recs := []Record{
		{Contaminated: true, Tiers: []TierObs{{Tier: "a", Score: 1, Correct: true, Cost: 0.001}}},
		{Tiers: []TierObs{{Tier: "a", Score: 1, Correct: true, Cost: 0.001}}},
	}
	cert, err := Calibrate(recs, []string{"a"}, Options{Alpha: 0.5, Delta: 0.1, Step: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if cert.NExcluded != 1 || cert.N != 1 {
		t.Errorf("want 1 excluded and n=1, got excluded=%d n=%d", cert.NExcluded, cert.N)
	}
}
