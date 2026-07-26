package cascade

import (
	"context"
	"fmt"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/model"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
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
	spec, err := r.spec(ctx, problem, &Result{})
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}
	rec := &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return rec, err
		}
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil)
		if err != nil {
			return rec, err
		}
		score, winner := cluster.Score(cluster.Group(cands))

		obs := calibrate.TierObs{Tier: tier.Name, Score: score}
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
		}
		obs.Cost = local.Cost.TotalUSD
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
	spec, err := r.spec(ctx, problem, &Result{})
	if err != nil {
		return nil, nil, fmt.Errorf("spec phase: %w", err)
	}
	execRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	judgeRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return execRec, judgeRec, err
		}
		// Sampling is shared: draw the candidates once and cost them once. Both
		// arms carry this identical sampling cost.
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil)
		if err != nil {
			return execRec, judgeRec, err
		}
		sampleCost := local.Cost.TotalUSD
		score, winner := cluster.Score(cluster.Group(cands))

		execObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
		judgeObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}

		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]

			// Execution oracle: run the held-out tests on the shared
			// representative. This is both the execution arm's acceptance
			// decision and the ground truth used to check the judge arm.
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			truth := acc != nil && acc.OK
			execObs.Cost += cpu.Seconds() * r.cfg.ComputeUSDPerCoreSecond

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
	spec, err := r.spec(ctx, problem, &Result{})
	if err != nil {
		return nil, nil, fmt.Errorf("spec phase: %w", err)
	}
	execRec = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	judgeRecs = make(map[prompt.JudgeStrictness]*calibrate.Record, len(levels))
	for _, lvl := range levels {
		judgeRecs[lvl] = &calibrate.Record{ID: id, Problem: problem, Shadow: true}
	}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return execRec, judgeRecs, err
		}
		local := &Result{}
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil)
		if err != nil {
			return execRec, judgeRecs, err
		}
		sampleCost := local.Cost.TotalUSD
		score, winner := cluster.Score(cluster.Group(cands))

		execObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
		var truth bool
		var repSource string
		haveRep := false
		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]
			repSource = rep.Source
			haveRep = true
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			truth = acc != nil && acc.OK
			execObs.Cost += cpu.Seconds() * r.cfg.ComputeUSDPerCoreSecond
			execObs.Correct = truth
			execObs.TrueCorrect = &truth
		}
		execRec.Tiers = append(execRec.Tiers, execObs)
		if tier.ModelID == r.cfg.TestModel {
			execRec.Contaminated = true
		}

		for _, lvl := range levels {
			jObs := calibrate.TierObs{Tier: tier.Name, Score: score, Cost: sampleCost}
			if haveRep {
				pass, jcost, jerr := r.judgeAt(ctx, lvl, judgeModel, problem, spec.API, repSource)
				if jerr != nil {
					return execRec, judgeRecs, jerr
				}
				t := truth
				jObs.Correct = pass
				jObs.TrueCorrect = &t
				jObs.Cost += jcost
			}
			judgeRecs[lvl].Tiers = append(judgeRecs[lvl].Tiers, jObs)
			if tier.ModelID == judgeModel {
				judgeRecs[lvl].Contaminated = true
			}
		}
	}
	return execRec, judgeRecs, nil
}
