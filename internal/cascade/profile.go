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
	spec, err := r.spec(ctx, problem, &Result{})
	if err != nil {
		return nil, fmt.Errorf("spec phase: %w", err)
	}
	rec := &calibrate.Record{ID: id, Problem: problem, Shadow: true}

	// If a reference is registered, check the generated oracle is sound before
	// trusting any label it produces (invariant #4). Tiers are still profiled so
	// the record is complete, but an unsound oracle is excluded from calibration.
	if sound, diag := r.validateOracle(ctx, id, spec); !sound {
		rec.OracleUnsound = true
		rec.OracleUnsoundDiag = diag
	}

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return rec, err
		}
		local := &Result{}
		t0 := time.Now()
		cands, err := r.sampleTier(ctx, k, problem, spec, local, nil)
		if err != nil {
			return rec, err
		}
		score, winner := cluster.Score(cluster.Group(cands))

		obs := calibrate.TierObs{Tier: tier.Name, Score: score}
		if winner != nil {
			rep := cands[indexOf(cands, winner.Rep)]
			acc, cpu := r.acceptOne(ctx, rep.Source, spec)
			local.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
			obs.Correct = acc != nil && acc.OK
		}
		obs.Cost = local.Cost.TotalUSD
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
