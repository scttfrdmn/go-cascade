package cascade

import (
	"context"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
)

// An empty judgeModel must fall back to the configured test model rather than
// reaching the provider with an empty ModelID. This regresses a live failure
// where --compare passed the unset --judge-model default ("") straight through
// and every judge call errored with "modelId must not be empty".
func TestProfileJudgeEmptyModelFallsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)
	ctx := context.Background()

	rec, err := r.ProfileJudge(ctx, "seq", seqProblem, "") // empty on purpose
	if err != nil {
		t.Fatalf("ProfileJudge with empty judgeModel should fall back, got error: %v", err)
	}
	if len(rec.Tiers) != len(cfg.Tiers) {
		t.Fatalf("expected %d tiers profiled, got %d", len(cfg.Tiers), len(rec.Tiers))
	}
	// Contamination is judge-vs-code; with the fallback the judge is the test
	// model (mock-oracle), distinct from every code tier, so nothing is flagged.
	if rec.Contaminated {
		t.Errorf("fallback judge (test model) should not collide with any code tier")
	}
}

// The judge arm and the execution arm profile the same problem. The execution
// oracle is sound, so its recorded correctness is ground truth. The judge reads
// the code and, by construction of the mock, passes the subtle defect that only
// the held-out partition catches. This is the whole comparison: on the tier
// that emits the subtle bug, the judge's verdict and execution truth diverge.
func TestJudgeArmDivergesFromExecutionArm(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = "" // calibration runs on the cache-bypass path
	r := newRouter(t, cfg, nil)
	ctx := context.Background()

	judgeModel := r.judgeModelDefault()

	jrec, err := r.ProfileJudge(ctx, "seq", seqProblem, judgeModel)
	if err != nil {
		t.Fatalf("judge profile: %v", err)
	}
	erec, err := r.Profile(ctx, "seq", seqProblem)
	if err != nil {
		t.Fatalf("execution profile: %v", err)
	}

	if len(jrec.Tiers) != len(cfg.Tiers) {
		t.Fatalf("judge record has %d tiers, want %d", len(jrec.Tiers), len(cfg.Tiers))
	}

	// Every judge observation must carry execution ground truth, or the
	// certificate it produces cannot be checked against reality.
	for _, obs := range jrec.Tiers {
		if obs.TrueCorrect == nil {
			t.Errorf("tier %q: judge record is missing TrueCorrect", obs.Tier)
		}
	}

	// Look for the tell: some tier where the judge said correct but execution
	// says wrong. The small tier emits the subtle >=-for-> defect, which passes
	// the visible tests (so it verifies and the judge reads it as fine) yet
	// fails the hidden partition. If this never happens the mock has drifted and
	// the comparison is vacuous, so assert it explicitly.
	sawFalseAccept := false
	for _, obs := range jrec.Tiers {
		if obs.TrueCorrect != nil && obs.Correct && !*obs.TrueCorrect {
			sawFalseAccept = true
		}
	}
	if !sawFalseAccept {
		t.Errorf("expected the judge to pass at least one program the hidden tests refute; "+
			"judge tiers: %+v", jrec.Tiers)
	}

	// The execution arm's recorded correctness is ground truth by construction
	// (β=0): Correct and the (redundant) TrueCorrect must agree everywhere.
	for _, obs := range erec.Tiers {
		if obs.TrueCorrect != nil && obs.Correct != *obs.TrueCorrect {
			t.Errorf("execution arm tier %q: Correct=%v but TrueCorrect=%v; the oracle is supposed to be sound",
				obs.Tier, obs.Correct, *obs.TrueCorrect)
		}
	}
}

// End to end through Calibrate: the judge arm's certificate reports higher
// realized risk than empirical risk on a run where it passes a hidden-failing
// program, while the execution arm's two risks coincide.
func TestJudgeCertificateRealizedExceedsEmpirical(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)
	ctx := context.Background()
	names := make([]string, len(cfg.Tiers))
	for i, tr := range cfg.Tiers {
		names[i] = tr.Name
	}

	rec, err := r.ProfileJudge(ctx, "seq", seqProblem, r.judgeModelDefault())
	if err != nil {
		t.Fatal(err)
	}
	// Force acceptance at the cheapest tier: thresholds of 0 accept tier 0
	// unconditionally, so whatever the small tier's representative is gets
	// returned.
	tau := make([]float64, len(cfg.Tiers)-1)
	emp, _ := calibrate.Risk([]calibrate.Record{*rec}, tau)
	real := calibrate.RealizedRisk([]calibrate.Record{*rec}, tau)
	// The mock judge only ever false-accepts (it passes the subtle defect); it
	// never false-rejects. So realized risk can only meet or exceed empirical
	// risk, never fall below it. A strict excess means the returned program was
	// one the judge waved through and the hidden tests refute.
	if real < emp {
		t.Errorf("realized risk %.3f fell below empirical risk %.3f; the mock judge should be optimistic-only", real, emp)
	}
}
