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
	resp, gerr := r.prov.Generate(ctx, model.Request{
		ModelID: judgeModel, Purpose: model.PurposeJudge,
		System:    prompt.JudgeSystem(),
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
