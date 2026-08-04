package cascade

import (
	"context"
	"fmt"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/cache"
	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cluster"
)

// Profile runs every tier on one problem and records what each did.
//
// Calibration deliberately does not run the cascade. Recording all tiers on all
// problems lets any threshold vector be replayed offline, so the grid search
// costs nothing beyond the one-time collection; running the cascade instead
// would only ever observe the tiers the current thresholds happened to reach.
//
// Records are collected on the cache-bypass path. A warm cache absorbs the head
// of the query distribution, so the router in production sees the conditional
// distribution of cache misses; calibrating behind the cache silently breaks
// exchangeability and voids the bound.
func (r *Router) Profile(ctx context.Context, id, problem string) (*calibrate.Record, error) {
	spec, err := r.spec(ctx, problem, r.pinnedAPI[id], &Result{})
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}
	rec := &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	// If a reference is registered, check the generated oracle is sound before
	// trusting any label it produces (invariant #4). Tiers are still profiled so
	// the record is complete. An unsound oracle (compiling reference refuted by an
	// assertion) is excluded; an inconclusive result (API mismatch) is only noted.
	switch v, diag := r.validateOracle(ctx, id, spec); v {
	case OracleUnsoundVerdict:
		rec.OracleUnsound, rec.OracleUnsoundDiag = true, diag
	case OracleInconclusive:
		rec.OracleInconclusive, rec.OracleUnsoundDiag = true, diag
	}

	// One cascade plan per problem, threaded into every profiled tier. Its single
	// USD charge is attributed to tier 0's cost below, so offline Replay — which
	// sums the cost of each reached tier — counts the plan exactly once per query
	// (tier 0 is always reached). A cascade planner equal to TestModel authors the
	// code every tier submits and so contaminates the record (invariant #3).
	plan, _, _, planUSD := r.cascadePlan(ctx, problem, spec)
	if r.cfg.PlannerModel == r.cfg.TestModel {
		rec.Contaminated = true
	}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return rec, err
		}
		local := &Result{}
		t0 := time.Now()
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil, plan)
		if err != nil {
			return rec, err
		}
		score, winner := cluster.Score(cluster.Group(cands))

		obs := calibrate.TierObs{Tier: tier.Name, Score: score}
		obs.MarkTimedOut(anyTimedOut(cands))
		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			local.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
			obs.Correct = acc != nil && acc.OK
			obs.MarkTimedOut(acc.TimedOut())
		}
		obs.Cost = local.Cost.TotalUSD
		if k == 0 {
			obs.Cost += planUSD // the one plan charge, counted once per query
		}
		rec.Tiers = append(rec.Tiers, obs)

		if tier.ModelID == r.cfg.TestModel {
			rec.Contaminated = true
		}
		_ = t0
	}
	rec.CacheCost = 0
	_ = cache.ProblemHash(problem)
	return rec, nil
}
