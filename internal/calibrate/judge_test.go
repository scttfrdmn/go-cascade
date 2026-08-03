package calibrate

import (
	"encoding/json"
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

// Source is retained on exactly the observations where the oracle and execution
// truth disagree, in both directions, and on no others. The negative cases carry
// the cost argument: retaining agreements would store 3n programs per run instead
// of the disagreement count (1096 vs 166 at n=409).
func TestRetainSourceOnlyOnDisagreement(t *testing.T) {
	for _, tc := range []struct {
		name       string
		obs        TierObs
		wantRetain bool
	}{
		{"over-acceptance is eta_fa, the dangerous direction",
			TierObs{Correct: true, TrueCorrect: boolp(false)}, true},
		{"over-rejection is beta, costly but safe",
			TierObs{Correct: false, TrueCorrect: boolp(true)}, true},
		{"agreement on correct carries no forensic information",
			TierObs{Correct: true, TrueCorrect: boolp(true)}, false},
		{"agreement on wrong carries no forensic information",
			TierObs{Correct: false, TrueCorrect: boolp(false)}, false},
		// A legacy record has nothing to compare against. Treating a missing
		// TrueCorrect as agreement would be the same unsound fallback the
		// analysis scripts must avoid: for the judge arm it forces agreement.
		{"missing TrueCorrect is not evidence of agreement",
			TierObs{Correct: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs := tc.obs
			if got := obs.Disagrees(); got != tc.wantRetain {
				t.Errorf("Disagrees() = %v, want %v", got, tc.wantRetain)
			}
			obs.RetainSourceOnDisagreement("package main // candidate")
			if got := obs.DisagreementSource != ""; got != tc.wantRetain {
				t.Errorf("retained source = %v, want %v (source %q)",
					got, tc.wantRetain, obs.DisagreementSource)
			}
		})
	}
}

// The retained source is forensic only: it must not change any risk figure. If
// adding it moved empirical or realized risk, it would have become an oracle
// input with no soundness argument behind it (invariants #4, #6).
func TestRetainedSourceDoesNotAffectRisk(t *testing.T) {
	base := []Record{
		{Tiers: []TierObs{{Tier: "small", Score: 1, Correct: true, TrueCorrect: boolp(false)}}},
		{Tiers: []TierObs{{Tier: "small", Score: 1, Correct: false, TrueCorrect: boolp(true)}}},
		{Tiers: []TierObs{{Tier: "small", Score: 1, Correct: true, TrueCorrect: boolp(true)}}},
	}
	withSrc := make([]Record, len(base))
	for i, r := range base {
		tiers := make([]TierObs, len(r.Tiers))
		copy(tiers, r.Tiers)
		for j := range tiers {
			tiers[j].RetainSourceOnDisagreement("func Solve() int { return 0 }")
		}
		withSrc[i] = Record{Tiers: tiers}
	}
	// Sanity: the fixture must actually exercise retention, or this proves nothing.
	retained := 0
	for _, r := range withSrc {
		for _, o := range r.Tiers {
			if o.DisagreementSource != "" {
				retained++
			}
		}
	}
	if retained != 2 {
		t.Fatalf("fixture retained %d sources, want 2; test would be vacuous", retained)
	}

	empBase, _ := Risk(base, nil)
	empSrc, _ := Risk(withSrc, nil)
	if empBase != empSrc {
		t.Errorf("empirical risk changed with retained source: %.3f -> %.3f", empBase, empSrc)
	}
	if r1, r2 := RealizedRisk(base, nil), RealizedRisk(withSrc, nil); r1 != r2 {
		t.Errorf("realized risk changed with retained source: %.3f -> %.3f", r1, r2)
	}
}

// Records written before this field existed must still load, and a record
// carrying it must round-trip. The field is omitempty, so agreeing observations
// add no bytes to any records file in results/.
func TestDisagreementSourceRoundTripsAndOmits(t *testing.T) {
	legacy := `{"tiers":[{"tier":"small","score":1,"correct":true}]}`
	var rec Record
	if err := json.Unmarshal([]byte(legacy), &rec); err != nil {
		t.Fatalf("legacy record no longer loads: %v", err)
	}
	if rec.Tiers[0].DisagreementSource != "" {
		t.Errorf("legacy record invented a source: %q", rec.Tiers[0].DisagreementSource)
	}

	agree := TierObs{Tier: "small", Correct: true, TrueCorrect: boolp(true)}
	b, err := json.Marshal(agree)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "disagreement_source") {
		t.Errorf("agreeing observation serialised the field: %s", b)
	}

	src := "package main\n\nfunc Solve() int { return 0 }\n"
	disagree := TierObs{Tier: "small", Correct: true, TrueCorrect: boolp(false)}
	disagree.RetainSourceOnDisagreement(src)
	b, err = json.Marshal(disagree)
	if err != nil {
		t.Fatal(err)
	}
	var back TierObs
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.DisagreementSource != src {
		t.Errorf("source did not round-trip: got %q want %q", back.DisagreementSource, src)
	}
}
