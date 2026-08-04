package cascade

import (
	"context"
	"fmt"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/model"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
	"github.com/scttfrdmn/go-cascade/internal/verify"
)

// judgeAccept asks a model whether a candidate is correct. It is the oracle for
// the judge-comparison arm (paper §5.5c) and is deliberately kept out of the
// verifier ladder: a judge verdict is a noisy prediction, not a sound
// refutation, so it cannot be a ladder stage without breaking invariant #4 (a
// failed stage must imply incorrectness). It replaces exactly one thing -- the
// held-out acceptance oracle -- and nothing else about routing changes, which is
// what isolates the oracle as the single variable the experiment compares.
//
// The judge never sees the test suites. Handing it the tests would turn it back
// into an executable oracle and defeat the comparison; withholding them is what
// makes it a reviewer reading code, which is the arm we mean to measure.
//
// A reply with no parseable verdict is treated as a refusal to pass. The judge
// oracle is asymmetric on purpose: only an explicit PASS accepts.
func (r *Router) judgeAccept(ctx context.Context, judgeModel, problem, api, src string) (pass bool, cost float64, err error) {
	return r.judgeAt(ctx, r.judgeStrictness(), judgeModel, problem, api, src)
}

// judgeAt is judgeAccept at an explicit strictness, so the same candidate can be
// judged at several operating points without mutating config. This is what lets
// the strictness sweep be a true A/B on identical programs.
func (r *Router) judgeAt(ctx context.Context, strictness prompt.JudgeStrictness, judgeModel, problem, api, src string) (pass bool, cost float64, err error) {
	resp, gerr := r.prov.Generate(ctx, model.Request{
		ModelID: judgeModel, Purpose: model.PurposeJudge,
		System:    prompt.JudgeSystem(strictness),
		Messages:  []model.Message{{Role: model.RoleUser, Text: prompt.JudgeUser(problem, api, src)}},
		MaxTokens: 256, Temperature: 0.0,
	})
	if gerr != nil {
		return false, 0, gerr
	}
	// Judge tokens are billed at the test model's rate: it plays the oracle role
	// the test model plays in the execution arm, so the arms stay cost-comparable.
	cost = (float64(resp.Usage.InputTokens)/1e6)*r.cfg.TestInCost +
		(float64(resp.Usage.OutputTokens)/1e6)*r.cfg.TestOutCost
	verdict, perr := prompt.ParseJudge(resp.Text)
	if perr != nil {
		return false, cost, nil //nolint:nilerr // unparseable verdict = no PASS, not a fatal error
	}
	return verdict, cost, nil
}

// ProfileJudge is the judge-arm analogue of Profile: it runs every tier on one
// problem, but the acceptance oracle is a judge model instead of the held-out
// test partition. It records the judge's verdict as TierObs.Correct (what this
// arm calibrates on) and execution ground truth as TierObs.TrueCorrect (what
// the resulting certificate is checked against). The two diverge exactly when a
// wrong program reads as correct -- eta_fa, the quantity §3.1 says a judge
// cannot certify against.
//
// Like Profile, it profiles every tier so any threshold vector replays offline,
// and it runs on the cache-bypass path (Shadow) to preserve exchangeability.
func (r *Router) ProfileJudge(ctx context.Context, id, problem, judgeModel string) (*calibrate.Record, error) {
	// An empty judgeModel falls back to the configured test model, so a caller
	// that does not set --judge-model still names a real model rather than
	// sending an empty ModelID to the provider. Centralised here so every caller
	// (solve, calibrate, compare) inherits the fallback.
	if judgeModel == "" {
		judgeModel = r.judgeModelDefault()
	}
	spec, err := r.spec(ctx, problem, "", &Result{})
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}
	rec := &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	// One cascade plan threaded into every tier; charged once to tier 0 (see
	// Profile). The judge arm's oracle is the judge model, so the plan author
	// contaminates when it equals the judge, not the test model.
	plan, _, _, planUSD := r.cascadePlan(ctx, problem, spec)
	if r.cfg.PlannerModel == judgeModel {
		rec.Contaminated = true
	}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return rec, err
		}
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil, plan)
		if err != nil {
			return rec, err
		}
		score, winner := cluster.Score(cluster.Group(cands))

		obs := calibrate.TierObs{Tier: tier.Name, Score: score}
		obs.MarkTimedOut(anyTimedOut(cands))
		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]

			// The judge rules on the representative -- the same candidate the
			// execution arm would send to acceptance -- so the arms decide on
			// identical inputs and differ only in the oracle.
			pass, jcost, jerr := r.judgeAccept(ctx, judgeModel, problem, spec.API, rep.Source)
			if jerr != nil {
				return rec, jerr
			}
			local.Cost.addModel(0, 0, jcost)
			obs.Correct = pass

			// Ground truth, recorded regardless of the judge's verdict.
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			local.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
			truth := acc != nil && acc.OK
			obs.TrueCorrect = &truth
			obs.MarkTimedOut(acc.TimedOut())

			// Forensic retention on judge/truth disagreement (issue #49). Must
			// follow both assignments above, since the rule reads them.
			obs.RetainSourceOnDisagreement(rep.Source)
		}
		obs.Cost = local.Cost.TotalUSD
		if k == 0 {
			obs.Cost += planUSD
		}
		rec.Tiers = append(rec.Tiers, obs)

		// The judge is the oracle here, so contamination is judge-vs-code, not
		// test-model-vs-code: a judge that also wrote the code is not independent.
		if tier.ModelID == judgeModel {
			rec.Contaminated = true
		}
	}
	return rec, nil
}

// judgeModelDefault picks the judge model. It defaults to the configured test
// model, so the judge arm and the execution arm use the same oracle model and
// the comparison isolates *how* the oracle judges (reading vs execution) rather
// than *which* model does. Callers can override.
func (r *Router) judgeModelDefault() string { return r.cfg.TestModel }

// judgeStrictness resolves the judge's operating point from config, defaulting
// to strict (the behaviour before the knob existed).
func (r *Router) judgeStrictness() prompt.JudgeStrictness {
	if r.cfg.JudgeStrictness == "" {
		return prompt.JudgeStrict
	}
	return prompt.JudgeStrictness(r.cfg.JudgeStrictness)
}

// ProfilePaired profiles one problem under BOTH oracles against a single,
// shared candidate stream and returns (execution, judge) records.
//
// This is the fix for the confounder in the independent-sampling comparison:
// Profile and ProfileJudge each generated their own spec and re-sampled their
// own candidates, so the two arms never ruled on the same programs and any
// difference in risk was dominated by sampling variance rather than by the
// oracle. Here the spec is generated once, each tier is sampled once, and the
// same representative is submitted to both the held-out tests (execution) and
// the reading-only judge. The two records are therefore paired: they share
// tier, score, cost and — for the execution record — TrueCorrect equals
// Correct, while the judge record carries the judge verdict as Correct and the
// same execution truth as TrueCorrect. Any divergence between the arms is now
// attributable to the oracle alone, which is what §5.5c actually asks for.
func (r *Router) ProfilePaired(ctx context.Context, id, problem, judgeModel string) (execRec, judgeRec *calibrate.Record, err error) {
	if judgeModel == "" {
		judgeModel = r.judgeModelDefault()
	}
	spec, err := r.spec(ctx, problem, r.pinnedAPI[id], &Result{})
	if err != nil {
		return nil, nil, fmt.Errorf("spec phase: %w", err)
	}
	execRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	judgeRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	// Both arms rule on the same generated tests, so an unsound oracle taints
	// both. Flag each; calibration excludes unsound records (invariant #4) but
	// keeps inconclusive (API-mismatch) ones. The execution arm's soundness check
	// is exact here — it uses the same held-out tests.
	switch v, diag := r.validateOracle(ctx, id, spec); v {
	case OracleUnsoundVerdict:
		execRec.OracleUnsound, execRec.OracleUnsoundDiag = true, diag
		judgeRec.OracleUnsound, judgeRec.OracleUnsoundDiag = true, diag
	case OracleInconclusive:
		execRec.OracleInconclusive, execRec.OracleUnsoundDiag = true, diag
		judgeRec.OracleInconclusive, judgeRec.OracleUnsoundDiag = true, diag
	}

	// One shared cascade plan threaded into every tier for both arms; its single
	// charge is attributed to tier 0 of each record (see Profile). Plan-author
	// contamination differs per arm exactly like coder contamination below.
	plan, _, _, planUSD := r.cascadePlan(ctx, problem, spec)
	if r.cfg.PlannerModel == r.cfg.TestModel {
		execRec.Contaminated = true
	}
	if r.cfg.PlannerModel == judgeModel {
		judgeRec.Contaminated = true
	}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return execRec, judgeRec, err
		}
		// Sampling is shared: draw the candidates once and cost them once. Both
		// arms carry this identical sampling cost. Tier 0 also carries the one
		// plan charge, counted once per query.
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil, plan)
		if err != nil {
			return execRec, judgeRec, err
		}
		sampleCost := local.Cost.TotalUSD
		if k == 0 {
			sampleCost += planUSD
		}
		score, winner := cluster.Score(cluster.Group(cands))

		execObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
		judgeObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}

		// The candidate stream is shared, so a sampling timeout taints BOTH arms
		// identically — which is the point of the paired design and the reason the
		// flag must be set on both records rather than only the execution one.
		sampleTO := anyTimedOut(cands)
		execObs.MarkTimedOut(sampleTO)
		judgeObs.MarkTimedOut(sampleTO)

		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]

			// Execution oracle: run the held-out tests on the shared
			// representative. This is both the execution arm's acceptance
			// decision and the ground truth used to check the judge arm.
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			truth := acc != nil && acc.OK
			execObs.Cost += cpu.Seconds() * r.cfg.ComputeUSDPerCoreSecond
			// Both arms again: the judge record's TrueCorrect comes from this same
			// execution run, so a clock-caused verdict here makes the judge arm's
			// ground-truth column suspect too — and eta_fa is computed from it.
			execObs.MarkTimedOut(acc.TimedOut())
			judgeObs.MarkTimedOut(acc.TimedOut())

			// Judge oracle: rule on the same representative by reading only.
			pass, jcost, jerr := r.judgeAccept(ctx, judgeModel, problem, spec.API, rep.Source)
			if jerr != nil {
				return execRec, judgeRec, jerr
			}
			judgeObs.Cost += jcost

			// Execution arm: oracle verdict IS truth (sound, beta=0).
			execObs.Correct = truth
			execObs.TrueCorrect = &truth
			// Judge arm: oracle verdict is the judge's PASS; execution truth is
			// recorded alongside so realized risk can be checked against reality.
			judgeObs.Correct = pass
			judgeObs.TrueCorrect = &truth

			// Keep the program behind each judge disagreement so its defect class
			// can be recovered later (issue #49): experiment 21 measured
			// eta_fa = 11/1096 but could not say what those 11 got wrong.
			//
			// Only the judge arm needs this. The execution arm assigns both
			// Correct and TrueCorrect from the same `truth` value above, so it
			// cannot disagree with itself and the call would be dead code.
			judgeObs.RetainSourceOnDisagreement(rep.Source)
		}
		execRec.Tiers = append(execRec.Tiers, execObs)
		judgeRec.Tiers = append(judgeRec.Tiers, judgeObs)

		// Contamination differs per arm: execution's oracle is the test model,
		// judge's oracle is the judge model. Flag each against its own oracle.
		if tier.ModelID == r.cfg.TestModel {
			execRec.Contaminated = true
		}
		if tier.ModelID == judgeModel {
			judgeRec.Contaminated = true
		}
	}
	return execRec, judgeRec, nil
}

// ProfileStrictnessReplay samples each tier once and judges the SAME
// representative at every requested strictness level, returning one execution
// record plus one judge record per level (keyed by level string).
//
// This isolates strictness as the only variable: unlike running ProfilePaired once
// per level (which re-samples, so the levels judge different programs), here the
// candidate stream and execution truth are fixed and only the judge's tie-break
// instruction changes. A verdict that flips FAIL->PASS as the judge loosens is
// then attributable to strictness alone, which is what tracing the η_fa/β
// operating curve requires.
func (r *Router) ProfileStrictnessReplay(ctx context.Context, id, problem, judgeModel string,
	levels []prompt.JudgeStrictness,
) (execRec *calibrate.Record, judgeRecs map[prompt.JudgeStrictness]*calibrate.Record, err error) {
	if judgeModel == "" {
		judgeModel = r.judgeModelDefault()
	}
	spec, err := r.spec(ctx, problem, "", &Result{})
	if err != nil {
		return nil, nil, fmt.Errorf("spec phase: %w", err)
	}
	execRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	judgeRecs = make(map[prompt.JudgeStrictness]*calibrate.Record, len(levels))
	for _, lvl := range levels {
		judgeRecs[lvl] = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	}

	// One cascade plan threaded into every tier; charged once to tier 0 (see Profile).
	plan, _, _, planUSD := r.cascadePlan(ctx, problem, spec)

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return execRec, judgeRecs, err
		}
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil, plan)
		if err != nil {
			return execRec, judgeRecs, err
		}
		sampleCost := local.Cost.TotalUSD
		if k == 0 {
			sampleCost += planUSD
		}
		score, winner := cluster.Score(cluster.Group(cands))

		execObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
		var truth bool
		var repSource string
		haveRep := false
		timedOut := anyTimedOut(cands)
		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]
			repSource = rep.Source
			haveRep = true
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			truth = acc != nil && acc.OK
			execObs.Cost += cpu.Seconds() * r.cfg.ComputeUSDPerCoreSecond
			execObs.Correct = truth
			execObs.TrueCorrect = &truth
			timedOut = timedOut || acc.TimedOut()
		}
		execObs.MarkTimedOut(timedOut)
		execRec.Tiers = append(execRec.Tiers, execObs)
		if tier.ModelID == r.cfg.TestModel {
			execRec.Contaminated = true
		}

		for _, lvl := range levels {
			jObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
			// Every level replays against the one shared candidate and the one shared
			// execution run, so they all inherit its timeout status.
			jObs.MarkTimedOut(timedOut)
			if haveRep {
				pass, jcost, jerr := r.judgeAt(ctx, lvl, judgeModel, problem, spec.API, repSource)
				if jerr != nil {
					return execRec, judgeRecs, jerr
				}
				t := truth
				jObs.Correct = pass
				jObs.TrueCorrect = &t
				jObs.Cost += jcost
				// Same forensic retention as ProfilePaired. Here it also shows
				// which strictness level flipped a verdict on a given program,
				// since every level rules on this identical representative.
				jObs.RetainSourceOnDisagreement(repSource)
			}
			judgeRecs[lvl].Tiers = append(judgeRecs[lvl].Tiers, jObs)
			if tier.ModelID == judgeModel {
				judgeRecs[lvl].Contaminated = true
			}
		}
	}
	return execRec, judgeRecs, nil
}

// SeededJudgeResult tallies, per strictness level, how the judge ruled on a set
// of KNOWN-WRONG candidates. Because every candidate is provably wrong (execution
// refutes it), a PASS is an unambiguous false acceptance. This is what the other
// sweeps could not measure: they depended on the models happening to emit a wrong
// candidate, whereas here wrong candidates are seeded deliberately.
type SeededJudgeResult struct {
	Level       prompt.JudgeStrictness
	Judged      int // wrong candidates presented
	FalseAccept int // judge said PASS on a wrong candidate (η_fa)
}

// SeedKind selects which class of provably-wrong candidate the seeded test
// harvests.
type SeedKind int

// Seed kinds.
const (
	SeedLogic SeedKind = iota // single-edit logic mutants, killed by ordinary tests
	SeedRace                  // sync-deletion mutants, refuted only under -race
	// SeedScarFreeRace keeps the synchronization scaffolding intact and balanced,
	// so the mutant reads as self-consistent. SeedRace deletes a sync call, which
	// always leaves an imbalance a reviewer can spot — the judge scored 20/20 on
	// those while false-accepting a model-authored race live. Reported separately
	// from SeedRace on purpose: the difference between the two η_fa rates is the
	// mechanism result, and pooling them would average it away.
	SeedScarFreeRace
)

// ProfileSeeded runs the judge dangerous-mode experiment for one problem. It
// samples a correct-ish solution, harvests up to nSeed mutants that COMPILE and
// are KILLED by the hidden tests (provably wrong, but a single edit from
// correct), and judges each of those fixed wrong candidates at every strictness
// level. The same candidates are judged at every level, so any rise in false
// acceptances as the judge loosens is the strictness knob alone — the §3.1
// dangerous mode, isolated and non-trivial by construction.
//
// It returns nil (not an error) when no killed mutant could be produced for the
// problem, so the caller can skip it rather than count a vacuous zero.
//
// seedKind selects the defect class: SeedLogic harvests single-edit logic
// mutants killed by ordinary testing; SeedRace harvests sync-deletion mutants
// refuted only under the race detector; SeedScarFreeRace harvests races whose
// synchronization scaffolding is intact, which is the genuinely
// reading-invisible class (SeedRace's deletions leave an imbalance a reviewer
// catches without reasoning about interleaving).
func (r *Router) ProfileSeeded(ctx context.Context, problem, judgeModel string, nSeed int,
	levels []prompt.JudgeStrictness, seedKind SeedKind,
) (map[prompt.JudgeStrictness]*SeededJudgeResult, int, error) {
	if judgeModel == "" {
		judgeModel = r.judgeModelDefault()
	}
	spec, err := r.spec(ctx, problem, "", &Result{})
	if err != nil {
		return nil, 0, fmt.Errorf("spec phase: %w", err)
	}

	// Draw one seed solution from the cheapest tier and harvest wrong mutants.
	// Thread the cascade plan (if any) so the seed reflects the configured
	// generation path; the seeded experiment records no cost, so no attribution.
	plan, _, _, _ := r.cascadePlan(ctx, problem, spec)
	local := &Result{}
	cands, err := r.sampleTier(ctx, 0, problem, spec, local, nil, plan)
	if err != nil {
		return nil, 0, err
	}
	_, winner := cluster.Score(cluster.Group(cands))
	if winner == nil {
		return nil, 0, nil // nothing verified to mutate from
	}
	seed := cands[indexOf(cands, winner.Rep)].Source

	var mutants []verify.KilledMutant
	switch seedKind {
	case SeedRace:
		mutants, err = verify.RaceKilledMutants(ctx, r.runner, seed, spec.VisibleTests, spec.HiddenTests,
			nSeed, r.cfg.RaceCount, r.cfg.TestTimeout)
	case SeedScarFreeRace:
		mutants, err = verify.ScarFreeRaceKilledMutants(ctx, r.runner, seed, spec.VisibleTests, spec.HiddenTests,
			nSeed, r.cfg.RaceCount, r.cfg.TestTimeout)
	default:
		mutants, err = verify.KilledMutants(ctx, r.runner, seed, spec.VisibleTests, spec.HiddenTests,
			nSeed, r.cfg.TestTimeout)
	}
	if err != nil {
		return nil, 0, err
	}
	if len(mutants) == 0 {
		return nil, 0, nil // no provably-wrong candidate available for this problem
	}

	out := make(map[prompt.JudgeStrictness]*SeededJudgeResult, len(levels))
	for _, lvl := range levels {
		out[lvl] = &SeededJudgeResult{Level: lvl}
	}
	for _, m := range mutants {
		for _, lvl := range levels {
			pass, _, jerr := r.judgeAt(ctx, lvl, judgeModel, problem, spec.API, m.Source)
			if jerr != nil {
				return out, len(mutants), jerr
			}
			out[lvl].Judged++
			if pass {
				out[lvl].FalseAccept++ // PASS on a provably-wrong program
			}
		}
	}
	return out, len(mutants), nil
}
