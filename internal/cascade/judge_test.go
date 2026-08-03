package cascade

import (
	"context"
	"testing"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
)

// ProfileStrictnessReplay must judge the SAME candidate stream at every
// strictness level: one shared execution record, and every level's judge record
// sharing per-tier score and execution truth with it. That shared stream is what
// makes the strictness sweep a controlled A/B rather than a re-sampling
// comparison, so assert it.
func TestProfileStrictnessReplaySharesStream(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)
	levels := []prompt.JudgeStrictness{prompt.JudgeStrict, prompt.JudgeBalanced, prompt.JudgePermissive}

	er, jrs, err := r.ProfileStrictnessReplay(context.Background(), "seq", seqProblem, "", levels)
	if err != nil {
		t.Fatal(err)
	}
	if len(jrs) != len(levels) {
		t.Fatalf("expected %d judge records, got %d", len(levels), len(jrs))
	}
	for _, lvl := range levels {
		jr := jrs[lvl]
		if jr == nil || len(jr.Tiers) != len(er.Tiers) {
			t.Fatalf("level %s: judge record shape mismatch", lvl)
		}
		for i := range er.Tiers {
			if er.Tiers[i].Score != jr.Tiers[i].Score {
				t.Errorf("level %s tier %d: score differs from execution (%.3f vs %.3f); stream not shared",
					lvl, i, er.Tiers[i].Score, jr.Tiers[i].Score)
			}
			et, jt := er.Tiers[i].TrueCorrect, jr.Tiers[i].TrueCorrect
			if (et == nil) != (jt == nil) || (et != nil && *et != *jt) {
				t.Errorf("level %s tier %d: TrueCorrect differs from execution; stream not shared", lvl, i)
			}
		}
	}
}

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

// ProfilePaired must return two records drawn from ONE shared candidate stream:
// same spec, same samples, same representative per tier. The pairing is what
// removes the sampling-variance confounder from the execution-vs-judge
// comparison, so assert the invariants that encode it: identical per-tier
// scores, identical execution ground truth (TrueCorrect), and a sound execution
// arm (Correct == TrueCorrect everywhere).
func TestProfilePairedSharesCandidateStream(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)

	er, jr, err := r.ProfilePaired(context.Background(), "seq", seqProblem, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(er.Tiers) != len(jr.Tiers) || len(er.Tiers) != len(cfg.Tiers) {
		t.Fatalf("tier count mismatch: exec=%d judge=%d cfg=%d",
			len(er.Tiers), len(jr.Tiers), len(cfg.Tiers))
	}
	for i := range er.Tiers {
		e, j := er.Tiers[i], jr.Tiers[i]
		// Shared stream => identical score and identical execution truth.
		if e.Score != j.Score {
			t.Errorf("tier %d: scores differ (exec %.3f, judge %.3f); candidates not shared",
				i, e.Score, j.Score)
		}
		if (e.TrueCorrect == nil) != (j.TrueCorrect == nil) ||
			(e.TrueCorrect != nil && *e.TrueCorrect != *j.TrueCorrect) {
			t.Errorf("tier %d: TrueCorrect differs between arms; ground truth not shared", i)
		}
		// Execution arm is sound: its verdict IS truth.
		if e.TrueCorrect != nil && e.Correct != *e.TrueCorrect {
			t.Errorf("tier %d: execution arm Correct=%v but TrueCorrect=%v",
				i, e.Correct, *e.TrueCorrect)
		}
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

// Issue #49: when the judge and execution disagree, the record must keep the
// program so its defect class can be recovered later. Experiment 21 measured
// η_fa = 11/1096 and could not say what any of the 11 got wrong, which is why
// the "reading-invisible defects" mechanism stayed argued.
//
// The assertion is two-sided on purpose: source present on every disagreement,
// absent on every agreement. Retaining agreements too would store 3n programs
// per run rather than the disagreement count.
func TestPairedRetainsSourceOnlyWhereOraclesDisagree(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)

	er, jr, err := r.ProfilePaired(context.Background(), "seq", seqProblem, "")
	if err != nil {
		t.Fatal(err)
	}

	disagreements := 0
	for i := range jr.Tiers {
		j := jr.Tiers[i]
		if j.Disagrees() {
			disagreements++
			if j.DisagreementSource == "" {
				t.Errorf("tier %q: judge said %v, truth %v, but no source retained — "+
					"the defect class is unrecoverable", j.Tier, j.Correct, *j.TrueCorrect)
			}
		} else if j.DisagreementSource != "" {
			t.Errorf("tier %q: oracles agree yet source was retained (%d bytes); "+
				"retention must be limited to disagreements",
				j.Tier, len(j.DisagreementSource))
		}
	}

	// If the mock ever stops producing a divergence this test proves nothing, so
	// fail loudly rather than passing vacuously (same guard as
	// TestJudgeArmDivergesFromExecutionArm).
	if disagreements == 0 {
		t.Fatalf("no judge/execution disagreement in the paired run; test is vacuous. judge tiers: %+v", jr.Tiers)
	}

	// The execution arm cannot disagree with itself (Correct and TrueCorrect come
	// from one value), so it must never retain source — that would be dead weight
	// in every execution records file.
	for _, e := range er.Tiers {
		if e.DisagreementSource != "" {
			t.Errorf("execution arm tier %q retained source; it is sound by construction", e.Tier)
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
