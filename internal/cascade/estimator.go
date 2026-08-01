package cascade

import (
	"context"
	"fmt"

	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/verify"
)

// EstimatorObs is one (problem, tier) measurement for the §3.7 estimator test:
// does mutation score M predict the oracle's false-acceptance rate η_fa on the
// MODEL's real defect distribution?
//
// The construct-validity trap §3.7 warns about is *circularity*: M is, by
// construction, 1 − η_fa measured on the mutation-operator defect distribution
// (a survivor is exactly a wrong program the generated suite accepts). Measuring
// η_fa against those same mutants would tautologically reproduce M and prove
// nothing. The whole point of §3.7's "unknown bias" is that the model's defect
// distribution (whole-algorithm errors, spec misreads) is NOT the operator
// distribution. So η_fa here is measured with an INDEPENDENT oracle: the
// human-authored canonical reference test suite (refs/<id>/solution_test.go),
// which the mutation operators never touch.
//
// A single observation is an η_fa "event" iff the generated suite ACCEPTED the
// candidate (V=1) yet the canonical suite REJECTS it (Y=0): a wrong program the
// generated oracle passed. Correlating M against the rate of such events across
// problems is the non-circular §3.7 test.
type EstimatorObs struct {
	ID   string `json:"id"`
	Tier string `json:"tier"`
	// GeneratedAccept is V: the representative passed the generated visible+hidden
	// suite (the oracle the cascade actually uses). Only V=1 rows carry η_fa
	// signal — a candidate the oracle already rejected cannot be a false accept.
	GeneratedAccept bool `json:"generated_accept"`
	// CanonicalCorrect is Y from the INDEPENDENT reference suite. Meaningful only
	// when CanonicalRan is true (the candidate compiled against the canonical
	// tests, which requires the pinned API so names match the reference).
	CanonicalCorrect bool `json:"canonical_correct"`
	CanonicalRan     bool `json:"canonical_ran"`
	// FalseAccept is the η_fa event: GeneratedAccept && CanonicalRan && !CanonicalCorrect.
	FalseAccept bool `json:"false_accept"`
	// MutationScore M of the accepted candidate against the GENERATED suite, and
	// the mutant counts behind it. Valid==0 means M is undefined (no compiling
	// mutant), so the row carries no M signal.
	MutationScore  float64 `json:"mutation_score"`
	MutationValid  int     `json:"mutation_valid"`
	MutationKilled int     `json:"mutation_killed"`
	// CanonicalDiag is the reference suite's refutation output on a false accept,
	// kept so the defect class can be inspected rather than only counted.
	CanonicalDiag string `json:"canonical_diag,omitempty"`
	// Skipped notes why a (problem, tier) produced no usable row (no reference, no
	// verified representative, canonical suite would not compile, etc.).
	Skipped string `json:"skipped,omitempty"`
}

// SetCanonicalTests installs the human-authored reference test suites
// (refs/<id>/solution_test.go), keyed by problem id — the INDEPENDENT oracle for
// the §3.7 estimator test. Kept separate from SetReferences (which holds
// solution.go): the oracle-soundness gate needs the reference solution, the
// estimator needs the reference tests. Empty unless EstimateOracleGap is used.
func (r *Router) SetCanonicalTests(tests map[string]string) { r.canonicalTests = tests }

// EstimateOracleGap measures, for one problem, whether the generated oracle's
// false-acceptance rate lines up with the mutation-score estimate of it — the
// §3.7 estimator test, run per tier so a cheap tier (which produces most η_fa
// events) is measured alongside the accurate ones.
//
// This is a MEASUREMENT, not a certificate. It never feeds a threshold or a risk
// budget (invariants #4/#6): it deliberately runs a second, independent oracle
// purely to check the first oracle's estimator, and its output is descriptive.
// It requires a pinned API — without it the candidate implements a differently
// named API than the reference's canonical tests and they will not compile, so
// there is no independent label to compare against.
func (r *Router) EstimateOracleGap(ctx context.Context, id, problem string, mutants int) ([]EstimatorObs, error) {
	canon, haveCanon := r.canonicalTests[id]
	spec, err := r.spec(ctx, problem, r.pinnedAPI[id], &Result{})
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}

	// The plan (if a cascade planner is configured) is drawn once and threaded
	// into every tier, exactly as calibration profiles it.
	plan, _, _, _ := r.cascadePlan(ctx, problem, spec)

	out := make([]EstimatorObs, 0, len(r.cfg.Tiers))
	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		obs := EstimatorObs{ID: id, Tier: tier.Name}
		if !haveCanon {
			obs.Skipped = "no canonical reference test suite for this problem"
			out = append(out, obs)
			continue
		}

		local := &Result{}
		cands, serr := r.sampleTier(ctx, k, problem, spec, local, nil, plan)
		if serr != nil {
			return out, serr
		}
		_, winner := cluster.Score(cluster.Group(cands))
		if winner == nil {
			obs.Skipped = "no representative survived the generated verifier ladder"
			out = append(out, obs)
			continue
		}
		rep := cands[indexOf(cands, winner.Rep)]

		// V: does the GENERATED oracle accept this representative? This is the
		// acceptance decision the cascade actually makes.
		acc, _ := r.acceptOne(ctx, rep.Source, spec)
		obs.GeneratedAccept = acc != nil && acc.OK

		// Y: the INDEPENDENT label from the human-authored canonical suite. Run the
		// candidate against the reference's own tests, not the generated ones.
		ran, correct, diag := r.canonicalVerdict(ctx, rep.Source, canon)
		obs.CanonicalRan, obs.CanonicalCorrect = ran, correct
		// Keep the diagnostic whenever the canonical suite had something to say: a
		// false-acceptance refutation (the finding) OR a compile mismatch that
		// explains why no label was produced (needed to tell an API mismatch from a
		// real gap when reading a run).
		if !ran || (ran && !correct) {
			obs.CanonicalDiag = diag
		}
		if ran && obs.GeneratedAccept && !correct {
			obs.FalseAccept = true
		}

		// M: mutation score against the GENERATED suite — the estimator whose bias
		// §3.7 questions. Measured on the accepted candidate.
		if mutants > 0 {
			ms, merr := verify.Mutate(ctx, r.runner, rep.Source, spec.VisibleTests, spec.HiddenTests,
				mutants, r.cfg.TestTimeout)
			if merr == nil && ms != nil {
				obs.MutationScore, obs.MutationValid, obs.MutationKilled = ms.Score, ms.Valid, ms.Killed
			}
		}
		out = append(out, obs)
	}
	return out, nil
}

// canonicalVerdict runs a candidate against the independent, human-authored
// reference test suite. It reports whether the suite RAN (the candidate compiled
// against the canonical tests — which needs the pinned API so the exported names
// match) and, if so, whether it PASSED. A compile failure is reported as
// ran=false, not as an incorrect verdict: a name mismatch says nothing about
// correctness, exactly as the oracle-soundness gate treats the mirror case.
func (r *Router) canonicalVerdict(ctx context.Context, src, canonicalTests string) (ran, correct bool, diag string) {
	ws, err := r.runner.NewWorkspace(src, canonicalTests, "")
	if err != nil {
		return false, false, fmt.Sprintf("workspace: %v", err)
	}
	defer ws.Remove() //nolint:errcheck // scratch dir

	rep := r.ladder.Run(ctx, ws, src, r.verifyOpts())
	if !rep.OK {
		// A behavioural refutation (test ran and failed) is a genuine incorrect
		// verdict. A compile-stage failure means the candidate does not fit the
		// canonical API — no independent label is available, so ran=false.
		if rep.FailedAt >= verify.StageTest {
			return true, false, rep.Diagnostic
		}
		return false, false, fmt.Sprintf("candidate does not compile against canonical tests (%s): %s",
			rep.FailedAt, rep.Diagnostic)
	}
	return true, true, ""
}
