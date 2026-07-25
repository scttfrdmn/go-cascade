package calibrate

import (
	"strings"
	"testing"
)

func boolp(b bool) *bool { return &b }

// A sound oracle records TrueCorrect == Correct, so empirical and realized risk
// coincide at every threshold. This is the execution arm's defining property
// (β=0): what it certifies is what it delivers.
func TestSoundOracleRealizedEqualsEmpirical(t *testing.T) {
	recs := []Record{
		{Tiers: []TierObs{{Tier: "small", Score: 0.9, Correct: true, TrueCorrect: boolp(true)}}},
		{Tiers: []TierObs{{Tier: "small", Score: 0.9, Correct: false, TrueCorrect: boolp(false)}}},
		{Tiers: []TierObs{{Tier: "small", Score: 0.9, Correct: true, TrueCorrect: boolp(true)}}},
	}
	for _, tau := range [][]float64{nil, {0.0}, {0.5}, {0.95}} {
		emp, _ := Risk(recs, tau)
		real := RealizedRisk(recs, tau)
		if emp != real {
			t.Errorf("tau=%v: empirical %.3f != realized %.3f for a sound oracle", tau, emp, real)
		}
	}
}

// A judge oracle that passes wrong programs (Correct=true, TrueCorrect=false)
// reports zero empirical risk while realized risk is positive. This is η_fa:
// the gap the judge cannot see and therefore cannot certify against (§3.1).
func TestJudgeOracleHidesFalseAcceptance(t *testing.T) {
	// Single-tier records so the final tier always accepts and thresholds don't
	// enter: this isolates the oracle from the routing decision.
	recs := []Record{
		{Tiers: []TierObs{{Tier: "large", Correct: true, TrueCorrect: boolp(true)}}},
		{Tiers: []TierObs{{Tier: "large", Correct: true, TrueCorrect: boolp(false)}}}, // judge missed a defect
		{Tiers: []TierObs{{Tier: "large", Correct: true, TrueCorrect: boolp(false)}}}, // and another
		{Tiers: []TierObs{{Tier: "large", Correct: true, TrueCorrect: boolp(true)}}},
	}
	emp, _ := Risk(recs, nil)
	real := RealizedRisk(recs, nil)
	if emp != 0 {
		t.Fatalf("judge reported empirical risk %.3f, expected 0 (it passed everything)", emp)
	}
	if real != 0.5 {
		t.Fatalf("realized risk = %.3f, expected 0.5 (two of four accepted programs are wrong)", real)
	}
}

// The headline of the comparison experiment: a judge arm can produce a Valid
// certificate at a small alpha whose realized risk exceeds that alpha. The
// certificate is nominal only, and Calibrate must flag it. The execution arm on
// the identical ground truth certifies honestly.
func TestJudgeCertificateIsNominalOnly(t *testing.T) {
	// 20 records; 30% of accepted programs are wrong but read as correct.
	var judge, exec []Record
	for i := range 20 {
		wrong := i%10 < 3 // 30% carry a subtle defect
		truth := !wrong
		// Judge arm: oracle says correct (it can't see the defect); truth recorded.
		judge = append(judge, Record{Tiers: []TierObs{
			{Tier: "large", Correct: true, TrueCorrect: boolp(truth)},
		}})
		// Execution arm on the same programs: oracle verdict IS truth.
		exec = append(exec, Record{Tiers: []TierObs{
			{Tier: "large", Correct: truth, TrueCorrect: boolp(truth)},
		}})
	}
	tiers := []string{"large"} // one tier => empty grid => always-valid at any alpha
	opts := Options{Alpha: 0.05, Delta: 0.1, Step: 0.1, Method: FixedSequence}

	jc, err := Calibrate(judge, tiers, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !jc.Valid {
		t.Fatalf("judge certificate should be nominally valid, got invalid")
	}
	if jc.EmpiricalRisk != 0 {
		t.Errorf("judge empirical risk = %.3f, expected 0", jc.EmpiricalRisk)
	}
	if jc.RealizedRisk <= jc.Alpha {
		t.Errorf("judge realized risk %.3f should exceed alpha %.3f", jc.RealizedRisk, jc.Alpha)
	}
	if !strings.Contains(jc.Note, "realized") {
		t.Errorf("judge certificate should carry a realized-risk warning; note = %q", jc.Note)
	}

	ec, err := Calibrate(exec, tiers, opts)
	if err != nil {
		t.Fatal(err)
	}
	if ec.RealizedRisk != ec.EmpiricalRisk {
		t.Errorf("execution arm: realized %.3f != empirical %.3f", ec.RealizedRisk, ec.EmpiricalRisk)
	}
	if ec.RealizedRisk != jc.RealizedRisk {
		t.Errorf("both arms ran the same programs, so ground-truth risk must match: exec %.3f, judge %.3f",
			ec.RealizedRisk, jc.RealizedRisk)
	}
}

// Legacy records without TrueCorrect fall back to the oracle verdict, so
// RealizedRisk reduces to Risk and old certificates are unaffected.
func TestRealizedRiskLegacyFallback(t *testing.T) {
	recs := []Record{
		{Tiers: []TierObs{{Tier: "small", Score: 1, Correct: true}}},
		{Tiers: []TierObs{{Tier: "small", Score: 1, Correct: false}}},
	}
	emp, _ := Risk(recs, nil)
	if got := RealizedRisk(recs, nil); got != emp {
		t.Errorf("legacy records: realized %.3f should fall back to empirical %.3f", got, emp)
	}
}
