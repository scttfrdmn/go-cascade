// Package cascade implements the router.
//
// The policy shape is not a design choice, it is forced. Writing the Lagrangian
// of "minimise expected cost subject to risk <= alpha" and solving backward
// gives, at each stage, accept iff lambda*(1-p_k(s)) <= V_{k+1}. When the score
// is monotone in correctness the accept region is an interval, so one threshold
// per stage is optimal and nothing more expressive helps. Everything below is
// that rule plus the machinery to make each p_k cheap to evaluate.
package cascade

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/scttfrdmn/go-cascade/internal/cache"
	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/cluster"
	"github.com/scttfrdmn/go-cascade/internal/config"
	"github.com/scttfrdmn/go-cascade/internal/model"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
	"github.com/scttfrdmn/go-cascade/internal/verify"
)

// Router routes a coding problem down the cheapest sufficient path.
type Router struct {
	cfg    *config.Config
	prov   model.Provider
	runner *verify.Runner
	ladder *verify.Ladder
	cache  *cache.Cache
	cert   *calibrate.Certificate

	// Observe, if set, receives a calibration record per solve.
	Observe func(calibrate.Record)

	// refs, if set, maps a problem id to an execution-validated reference
	// solution's source. During calibration the profiler runs the reference
	// against the freshly-generated test suite; if the reference is refuted the
	// generated oracle is unsound (it rejects correct code, invariant #4) and the
	// record is flagged OracleUnsound and excluded. Empty by default: reference
	// validation is opt-in via `calibrate -refs`.
	refs map[string]string

	// pinnedAPI, if set, maps a problem id to the exact API block the spec model
	// must write its tests against (extracted from the reference). It removes the
	// name/signature mismatch that otherwise leaves the reference unable to compile
	// against the generated tests, so the -refs gate can reach a verdict on the
	// whole benchmark instead of ~40% of it. Empty by default; opt-in via
	// `calibrate -pin-api` (which implies -refs). Read-only after construction.
	pinnedAPI map[string]string

	limit int // max concurrent verifications
}

// SetReferences installs the reference solutions used to detect an unsound
// generated oracle during calibration. Keyed by problem id.
func (r *Router) SetReferences(refs map[string]string) { r.refs = refs }

// SetPinnedAPIs installs the per-problem API contracts the spec model must write
// its tests against. Keyed by problem id. See the pinnedAPI field.
func (r *Router) SetPinnedAPIs(apis map[string]string) { r.pinnedAPI = apis }

// New builds a router. cert may be nil, in which case the run is uncertified.
func New(cfg *config.Config, prov model.Provider, cert *calibrate.Certificate) (*Router, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	runner, err := verify.NewRunner("", cfg.ExecWrapper)
	if err != nil {
		return nil, err
	}
	c, err := cache.Open(cfg.CacheDir)
	if err != nil {
		return nil, err
	}
	l := verify.NewLadder()
	// Pre-warm the shared importer so the first candidate does not pay the
	// 200ms+ cold cost of typechecking the standard library from source.
	l.Warm("fmt", "sync", "sync/atomic", "sort", "slices", "strings", "errors", "context", "maps", "cmp")

	return &Router{
		cfg: cfg, prov: prov, runner: runner, ladder: l, cache: c, cert: cert,
		limit: max(runtime.NumCPU(), 1),
	}, nil
}

// Close releases scratch state.
func (r *Router) Close() error { return r.runner.Close() }

// Cache exposes the store for reporting.
func (r *Router) Cache() *cache.Cache { return r.cache }

// calibrated reports whether a valid certificate governs this run. The final
// tier has no threshold -- it accepts by construction -- so certification is a
// property of the policy, not of whichever tier happened to accept.
func (r *Router) calibrated() bool {
	return r.cert != nil && r.cert.Valid
}

// threshold returns the accept threshold for tier k.
func (r *Router) threshold(k int) (float64, bool) {
	if r.calibrated() && k < len(r.cert.Thresholds) {
		return r.cert.Thresholds[k], true
	}
	// Uncalibrated priors: require a strict behavioural majority at the cheap
	// tier and a bare majority thereafter. These are guesses. They are not a
	// bound, and Result.Certified stays false.
	if k == 0 {
		return 0.6, false
	}
	return 0.5, false
}

func (r *Router) verifyOpts() verify.Options {
	return verify.Options{
		StdlibOnly:    r.cfg.StdlibOnly,
		MaxComplexity: r.cfg.MaxComplexity,
		MaxAllocsOp:   r.cfg.MaxAllocsOp,
		TestTimeout:   r.cfg.TestTimeout,
		RaceCount:     r.cfg.RaceCount,
		// Under a latency bound the race stage is 30x a plain test run, so it
		// is the first thing to drop.
		SkipRace: r.cfg.Deadline > 0,
	}
}

// Solve runs the cascade.
func (r *Router) Solve(ctx context.Context, problem string) (*Result, error) {
	start := time.Now()
	if r.cfg.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.cfg.Deadline)
		defer cancel()
	}

	res := &Result{Problem: problem}
	res.Shadow = rand.Float64() < r.cfg.ShadowRate

	defer func() {
		res.Elapsed = time.Since(start)
	}()

	// Phase 1: the contract. Tests are written before any solution exists and
	// by a different model, which is what keeps the oracle independent.
	spec, err := r.spec(ctx, problem, "", res)
	if err != nil {
		return res, fmt.Errorf("spec phase: %w", err)
	}
	res.API, res.VisibleTests, res.HiddenTests = spec.API, spec.VisibleTests, spec.HiddenTests

	// Phase 2: arm zero. Retrieved solutions are re-executed against this
	// query's tests, so a hit is a verified transfer rather than a predicted
	// one and costs nothing against the risk budget.
	if !res.Shadow {
		if ok, err := r.tryCache(ctx, problem, spec, res); err != nil {
			return res, err
		} else if ok {
			r.finish(ctx, res, spec)
			return res, nil
		}
	} else {
		res.Trace = append(res.Trace, Step{
			Stage: "cache", Action: ActEscalate,
			Reason: "shadow sample: routed past the cache to keep calibration unbiased",
		})
	}

	// Phase 3: the model cascade.
	if r.cfg.Deadline > 0 {
		err = r.speculative(ctx, problem, spec, res)
	} else {
		err = r.sequential(ctx, problem, spec, res)
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return res, err
	}
	r.finish(ctx, res, spec)
	return res, nil
}

// finish runs the post-acceptance work: oracle-gap estimation and admission.
func (r *Router) finish(ctx context.Context, res *Result, spec *prompt.Spec) {
	if !res.Solved {
		return
	}
	if r.cfg.Mutants > 0 {
		ms, err := verify.Mutate(ctx, r.runner, res.Solution, spec.VisibleTests, spec.HiddenTests,
			r.cfg.Mutants, r.cfg.TestTimeout)
		if err == nil {
			res.Mutation = ms
			res.Cost.addCompute(ms.Elapsed, r.cfg.ComputeUSDPerCoreSecond)
		}
	}
	// Admission is stricter than acceptance: a cached entry is reused many
	// times, so it should clear a higher bar than a one-shot answer.
	if res.AcceptedAt != "cache" && res.Score >= r.cfg.CacheAdmitAt && !res.OracleContaminated {
		mut := 0.0
		if res.Mutation != nil {
			mut = res.Mutation.Score
		}
		_ = r.cache.PutSolution(cache.Entry{
			Problem: res.Problem, ProblemHash: cache.ProblemHash(res.Problem),
			API: res.API, Solution: res.Solution, Tier: res.AcceptedAt,
			Score: res.Score, MutationScore: mut,
		})
	}
	if r.Observe != nil {
		r.Observe(r.record(res))
	}
}

func (r *Router) record(res *Result) calibrate.Record {
	rec := calibrate.Record{
		ID: cache.ProblemHash(res.Problem), Problem: res.Problem,
		Contaminated: res.OracleContaminated, Shadow: res.Shadow,
	}
	if res.AcceptedAt == "cache" {
		rec.CacheHit, rec.CacheCorrect, rec.CacheCost = true, res.Solved, res.Cost.TotalUSD
		return rec
	}
	for _, s := range res.Trace {
		if s.Tier == "" || s.Action == ActRepair {
			continue
		}
		rec.Tiers = append(rec.Tiers, calibrate.TierObs{
			Tier: s.Tier, Score: s.Score,
			Correct: s.Action == ActAccept && res.Solved,
			Cost:    s.CostSoFar,
		})
	}
	return rec
}

// spec derives or retrieves the API contract and both test partitions.
func (r *Router) spec(ctx context.Context, problem, pinnedAPI string, res *Result) (*prompt.Spec, error) {
	// A pinned spec is a different oracle than an unpinned one for the same
	// problem (its tests are written against a fixed API), so it must not collide
	// with an unpinned cache entry. Salt the cache key with the pinned API.
	ph := cache.ProblemHash(problem)
	if pinnedAPI != "" {
		ph = cache.ProblemHash(problem + "\x00pinned\x00" + pinnedAPI)
	}
	if s, ok := r.cache.GetSpec(ph); ok {
		res.Trace = append(res.Trace, Step{
			Stage: "spec", Action: ActAccept, Reason: "test cache hit: oracle reused at zero model cost",
		})
		return &prompt.Spec{API: s.API, VisibleTests: s.VisibleTests, HiddenTests: s.HiddenTests}, nil
	}

	sys, user := prompt.SpecSystem(), prompt.SpecUser(problem)
	if pinnedAPI != "" {
		sys, user = prompt.SpecSystemPinned(), prompt.SpecUserPinned(problem, pinnedAPI)
	}
	t0 := time.Now()
	resp, err := r.prov.Generate(ctx, model.Request{
		ModelID: r.cfg.TestModel, Purpose: model.PurposeSpec,
		System:    sys,
		Messages:  []model.Message{{Role: model.RoleUser, Text: user}},
		MaxTokens: 8000, Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}
	usd := (float64(resp.Usage.InputTokens)/1e6)*r.cfg.TestInCost +
		(float64(resp.Usage.OutputTokens)/1e6)*r.cfg.TestOutCost
	res.Cost.addModel(resp.Usage.InputTokens, resp.Usage.OutputTokens, usd)

	spec, err := prompt.ParseSpec(resp.Text)
	if err != nil {
		return nil, err
	}
	res.Trace = append(res.Trace, Step{
		Stage: "spec", Action: ActAccept, Reason: "oracle generated by " + r.cfg.TestModel,
		CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
	})
	_ = r.cache.PutSpec(cache.Spec{
		ProblemHash: ph, API: spec.API, VisibleTests: spec.VisibleTests,
		HiddenTests: spec.HiddenTests, Author: r.cfg.TestModel,
	})
	return spec, nil
}

// tryCache is arm zero: retrieve, then verify by execution.
func (r *Router) tryCache(ctx context.Context, problem string, spec *prompt.Spec, res *Result) (bool, error) {
	if r.cache.Disabled() {
		return false, nil
	}
	t0 := time.Now()
	hits, err := r.cache.Retrieve(problem, r.cfg.CacheTopK, r.cfg.CacheMinSim)
	if err != nil || len(hits) == 0 {
		res.Trace = append(res.Trace, Step{
			Stage: "cache", Action: ActEscalate, Reason: "no candidate above the similarity floor",
			Elapsed: time.Since(t0),
		})
		return false, nil //nolint:nilerr // a cache read failure is not fatal
	}
	for _, h := range hits {
		rep, acc, cpu, err := r.verifyOne(ctx, h.Solution, spec)
		res.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
		if err != nil {
			continue
		}
		if rep.OK && acc != nil && acc.OK {
			res.Solution, res.Solved, res.AcceptedAt = h.Solution, true, "cache"
			res.Score, res.Static = 1.0, rep.Static
			res.Trace = append(res.Trace, Step{
				Stage: "cache", Action: ActAccept, Score: 1.0,
				Reason: fmt.Sprintf("verified transfer from a prior solution (similarity %.2f); "+
					"executed against this query's tests, not predicted", h.Similarity),
				CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
			})
			return true, nil
		}
	}
	res.Trace = append(res.Trace, Step{
		Stage: "cache", Action: ActEscalate,
		Reason:    fmt.Sprintf("%d retrieved solutions all refuted by this query's tests", len(hits)),
		CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
	})
	return false, nil
}

// sequential is the cost-optimal topology: one tier at a time.
func (r *Router) sequential(ctx context.Context, problem string, spec *prompt.Spec, res *Result) error {
	ph := cache.ProblemHash(problem)
	var carried []string // diagnostics carried forward so the next tier starts informed

	for k, tier := range r.cfg.Tiers {
		if err := ctx.Err(); err != nil {
			return err
		}
		t0 := time.Now()
		cands, err := r.sampleTier(ctx, k, problem, spec, res, carried)
		if err != nil {
			return err
		}
		clusters := cluster.Group(cands)
		score, winner := cluster.Score(clusters)
		tau, certified := r.threshold(k)
		last := k == len(r.cfg.Tiers)-1

		// No survivor: the whole sample is soundly refuted. Repair or escalate
		// by marginal correctness per dollar, not by a fixed rule.
		if winner == nil {
			best := bestRefuted(cands)
			r.recordFailure(ph, cands)
			if best != nil && r.preferRepair(k) {
				repaired, ok := r.repairLoop(ctx, k, problem, spec, best, res)
				if ok {
					cands = append(cands, *repaired)
					clusters = cluster.Group(cands)
					score, winner = cluster.Score(clusters)
				}
			}
			if winner == nil {
				if best != nil {
					carried = append(carried, summarise(best.Report))
				}
				res.Trace = append(res.Trace, Step{
					Stage: "tier", Tier: tier.Name, Action: ActEscalate, Clusters: clusters,
					Reason:    "every sample refuted by the verifier ladder",
					CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
				})
				continue
			}
		}

		rep := cands[indexOf(cands, winner.Rep)]
		if score < tau && !last {
			carried = append(carried, fmt.Sprintf("behavioural agreement was only %.2f", score))
			res.Trace = append(res.Trace, Step{
				Stage: "tier", Tier: tier.Name, Action: ActEscalate, Score: score, Threshold: tau,
				Clusters: clusters, Reason: thresholdReason(score, tau, certified),
				CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
			})
			continue
		}

		// Acceptance: the held-out partition. A failure here escalates and is
		// never fed to repair, or the holdout would stop being held out.
		acc, cpu := r.acceptOne(ctx, rep.Source, spec)
		res.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
		if acc == nil || !acc.OK {
			diag := ""
			if acc != nil {
				diag = acc.Diagnostic
			}
			r.recordFailure(ph, []cluster.Candidate{rep})
			carried = append(carried, "a prior candidate passed the visible tests but failed the held-out partition")
			res.Trace = append(res.Trace, Step{
				Stage: "tier", Tier: tier.Name, Action: ActReject, Score: score, Threshold: tau,
				Clusters: clusters, Diagnostic: diag,
				Reason:    "passed the visible tests, refuted by the held-out partition",
				CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
			})
			continue
		}

		res.Solution, res.Solved, res.AcceptedAt = rep.Source, true, tier.Name
		res.Score, res.Static = score, rep.Report.Static
		res.Certified = r.calibrated()
		res.Certificate = r.cert
		res.OracleContaminated = tier.ModelID == r.cfg.TestModel
		res.Trace = append(res.Trace, Step{
			Stage: "tier", Tier: tier.Name, Action: ActAccept, Score: score, Threshold: tau,
			Clusters: clusters, Reason: acceptReason(score, tau, certified, last),
			CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
		})
		return nil
	}
	return nil
}

// speculative is the latency-bounded topology. Under a deadline the optimal
// shape is not a cascade: parallelism buys latency with dollars, so every tier
// starts at once and the cheapest verified answer wins.
func (r *Router) speculative(ctx context.Context, problem string, spec *prompt.Spec, res *Result) error {
	type outcome struct {
		k     int
		cands []cluster.Candidate
		err   error
	}
	ch := make(chan outcome, len(r.cfg.Tiers))
	var wg sync.WaitGroup
	sub, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	for k := range r.cfg.Tiers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			local := &Result{}
			mu.Unlock()
			c, err := r.sampleTier(sub, k, problem, spec, local, nil)
			mu.Lock()
			res.Cost.ModelUSD += local.Cost.ModelUSD
			res.Cost.ComputeUSD += local.Cost.ComputeUSD
			res.Cost.InputTokens += local.Cost.InputTokens
			res.Cost.OutputTokens += local.Cost.OutputTokens
			res.Cost.ModelCalls += local.Cost.ModelCalls
			res.Cost.TotalUSD = res.Cost.ModelUSD + res.Cost.ComputeUSD
			mu.Unlock()
			ch <- outcome{k: k, cands: c, err: err}
		}()
	}
	go func() { wg.Wait(); close(ch) }()

	// Collect every tier's best verified candidate, then try acceptance in
	// cost order. Falling through to the next tier on an acceptance failure
	// mirrors what the sequential cascade does when it escalates, so the
	// held-out partition is consulted once per tier either way and the
	// speculative path does not shop harder against the holdout.
	type winner struct {
		k        int
		cand     cluster.Candidate
		score    float64
		clusters []cluster.Cluster
	}
	var winners []winner
	for o := range ch {
		if o.err != nil || len(o.cands) == 0 {
			continue
		}
		cs := cluster.Group(o.cands)
		score, w := cluster.Score(cs)
		if w == nil {
			continue
		}
		winners = append(winners, winner{
			k: o.k, cand: o.cands[indexOf(o.cands, w.Rep)], score: score, clusters: cs,
		})
	}
	slices.SortFunc(winners, func(a, b winner) int { return a.k - b.k })

	best := -1
	var bestCand *cluster.Candidate
	var bestScore float64
	var bestClusters []cluster.Cluster
	for _, w := range winners {
		acc, cpu := r.acceptOne(ctx, w.cand.Source, spec)
		res.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
		if acc == nil || !acc.OK {
			res.Trace = append(res.Trace, Step{
				Stage: "speculative", Tier: r.cfg.Tiers[w.k].Name, Action: ActReject,
				Score: w.score, Clusters: w.clusters,
				Reason:    "passed the visible tests, refuted by the held-out partition",
				CostSoFar: res.Cost.TotalUSD,
			})
			continue
		}
		c := w.cand
		best, bestCand, bestScore, bestClusters = w.k, &c, w.score, w.clusters
		break
	}
	if bestCand == nil {
		res.Trace = append(res.Trace, Step{
			Stage: "speculative", Action: ActEscalate,
			Reason:    "no tier produced a candidate that survived both partitions within the deadline",
			CostSoFar: res.Cost.TotalUSD,
		})
		return nil
	}

	tier := r.cfg.Tiers[best]
	res.Solution, res.Solved, res.AcceptedAt = bestCand.Source, true, tier.Name
	res.Score, res.Static = bestScore, bestCand.Report.Static
	res.Certified, res.Certificate = r.calibrated(), r.cert
	res.OracleContaminated = tier.ModelID == r.cfg.TestModel
	res.Trace = append(res.Trace, Step{
		Stage: "speculative", Tier: tier.Name, Action: ActAccept, Score: bestScore,
		Clusters: bestClusters, CostSoFar: res.Cost.TotalUSD,
		Reason: "cheapest tier whose candidate survived; all tiers were run concurrently to meet the deadline",
	})
	return nil
}

// sampleTier draws n candidates and verifies them concurrently.
func (r *Router) sampleTier(ctx context.Context, k int, problem string, spec *prompt.Spec,
	res *Result, carried []string,
) ([]cluster.Candidate, error) {
	tier := r.cfg.Tiers[k]
	avoid := carried
	if !r.cache.Disabled() {
		for _, f := range r.cache.Failures(cache.ProblemHash(problem)) {
			avoid = append(avoid, f.Stage+": "+f.Summary)
		}
	}

	out := make([]cluster.Candidate, tier.Samples)
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(r.limit)
	for i := range tier.Samples {
		g.Go(func() error {
			resp, err := r.prov.Generate(gctx, model.Request{
				ModelID: tier.ModelID, Purpose: model.PurposeCode,
				System: prompt.CodeSystem(),
				Messages: []model.Message{{Role: model.RoleUser,
					Text: prompt.CodeUser(problem, spec.API, i, avoid)}},
				MaxTokens: 8000, Temperature: tier.Temperature, Seed: i,
			})
			if err != nil {
				return err
			}
			usd := tier.Cost(resp.Usage.InputTokens, resp.Usage.OutputTokens)
			mu.Lock()
			res.Cost.addModel(resp.Usage.InputTokens, resp.Usage.OutputTokens, usd)
			mu.Unlock()

			src, err := prompt.ExtractCode(resp.Text)
			if err != nil {
				out[i] = cluster.Candidate{Index: i, Source: resp.Text,
					Report: &verify.Report{OK: false, Diagnostic: err.Error()}}
				return nil
			}
			rep, _, cpu, err := r.verifyOne(gctx, src, spec)
			mu.Lock()
			res.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
			mu.Unlock()
			if err != nil {
				return err
			}
			out[i] = cluster.Candidate{Index: i, Source: src, Report: rep}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// verifyOne runs the ladder against one candidate. It returns the visible
// report; the acceptance run is deliberately separate.
func (r *Router) verifyOne(ctx context.Context, src string, spec *prompt.Spec) (
	*verify.Report, *verify.Report, time.Duration, error,
) {
	ws, err := r.runner.NewWorkspace(src, spec.VisibleTests, spec.HiddenTests)
	if err != nil {
		return nil, nil, 0, err
	}
	defer ws.Remove() //nolint:errcheck // scratch dir

	rep := r.ladder.Run(ctx, ws, src, r.verifyOpts())
	if !rep.OK {
		return rep, nil, rep.CPUTime, nil
	}
	acc := r.ladder.Accept(ctx, ws, r.verifyOpts())
	return rep, acc, rep.CPUTime + acc.CPUTime, nil
}

func (r *Router) acceptOne(ctx context.Context, src string, spec *prompt.Spec) (*verify.Report, time.Duration) {
	ws, err := r.runner.NewWorkspace(src, spec.VisibleTests, spec.HiddenTests)
	if err != nil {
		return nil, 0
	}
	defer ws.Remove() //nolint:errcheck // scratch dir
	acc := r.ladder.Accept(ctx, ws, r.verifyOpts())
	return acc, acc.CPUTime
}

// OracleVerdict is the three-way result of checking a generated test suite
// against a known-correct reference solution.
type OracleVerdict int

// Oracle-soundness verdicts.
const (
	// OracleSound: the reference compiled against the generated API and passed
	// every generated test. The oracle accepts correct code, as it must.
	OracleSound OracleVerdict = iota
	// OracleUnsoundVerdict: the reference compiled but a generated *assertion*
	// refuted it (test/race/accept stage). The tests reject correct code, so the
	// oracle is unsound (invariant #4) and its labels on this problem are noise.
	OracleUnsoundVerdict
	// OracleInconclusive: the reference did not even compile against the
	// generated tests — a parse/type/build/vet failure. This means the spec model
	// invented an API name or signature that differs from the reference's
	// canonical one, so the reference simply does not fit *this* run's contract.
	// It is NOT evidence that the tests are wrong (a candidate written against the
	// generated API could still be judged soundly), so the record is kept. Only a
	// behavioural refutation of a compiling reference proves unsoundness.
	OracleInconclusive
)

// validateOracle checks whether this problem's generated test suite is a sound
// oracle by running the known-correct reference solution through the full verify
// ladder plus acceptance, then classifying *how* it failed. The distinction is
// load-bearing:
//
//   - A compile/static failure (parse, types, build, vet) means the reference's
//     canonical signature does not match the API this run's spec model invented.
//     The reference does not fit the contract, which says nothing about whether
//     the tests are correct for a candidate that *does* fit it. Inconclusive:
//     keep the record. (Confusing this with unsoundness over-excludes ~12x and
//     drives the measured floor spuriously to zero.)
//   - A behavioural failure of a *compiling* reference (test, race, accept) means
//     the generated tests reject provably-correct code: an unsound oracle.
//
// Returns OracleSound when no reference is registered (nothing to check) so the
// default path is unchanged. diag carries the reference's refutation output.
func (r *Router) validateOracle(ctx context.Context, id string, spec *prompt.Spec) (OracleVerdict, string) {
	ref, ok := r.refs[id]
	if !ok {
		return OracleSound, ""
	}
	rep, acc, _, err := r.verifyOne(ctx, ref, spec)
	if err != nil {
		return OracleInconclusive, fmt.Sprintf("reference workspace error: %v", err)
	}
	if !rep.OK {
		// StageTest is the first stage that runs the reference rather than just
		// compiling/vetting it. A failure at or after it is behavioural; anything
		// earlier is an API/compile mismatch and is inconclusive.
		if rep.FailedAt >= verify.StageTest {
			return OracleUnsoundVerdict, fmt.Sprintf("reference refuted at %s: %s", rep.FailedAt, strings.TrimSpace(rep.Diagnostic))
		}
		return OracleInconclusive, fmt.Sprintf("reference does not fit generated API (%s): %s", rep.FailedAt, strings.TrimSpace(rep.Diagnostic))
	}
	if acc == nil || !acc.OK {
		stage, d := "hidden", ""
		if acc != nil {
			stage, d = acc.FailedAt.String(), acc.Diagnostic
		}
		// The reference already passed the visible ladder (rep.OK), so reaching
		// acceptance means it compiled; an acceptance failure is behavioural.
		return OracleUnsoundVerdict, fmt.Sprintf("reference refuted at %s stage: %s", stage, strings.TrimSpace(d))
	}
	return OracleSound, ""
}

// repairLoop feeds the exact verifier diagnostic back to the same tier.
//
// Repair attempts on a fixed model are strongly positively correlated: if it
// cannot fix the defect in two rounds it will not fix it in five. The depth cap
// is the point of that observation, not an arbitrary limit.
func (r *Router) repairLoop(ctx context.Context, k int, problem string, spec *prompt.Spec,
	prev *cluster.Candidate, res *Result,
) (*cluster.Candidate, bool) {
	tier := r.cfg.Tiers[k]
	cur := prev
	for round := range tier.RepairDepth {
		t0 := time.Now()
		resp, err := r.prov.Generate(ctx, model.Request{
			ModelID: tier.ModelID, Purpose: model.PurposeRepair,
			System: prompt.CodeSystem(),
			Messages: []model.Message{{Role: model.RoleUser, Text: prompt.RepairUser(
				problem, spec.API, cur.Source, cur.Report.FailedAt.String(), cur.Report.Diagnostic)}},
			MaxTokens: 8000, Temperature: tier.Temperature,
		})
		if err != nil {
			return nil, false
		}
		res.Cost.addModel(resp.Usage.InputTokens, resp.Usage.OutputTokens,
			tier.Cost(resp.Usage.InputTokens, resp.Usage.OutputTokens))

		src, err := prompt.ExtractCode(resp.Text)
		if err != nil {
			return nil, false
		}
		rep, _, cpu, err := r.verifyOne(ctx, src, spec)
		res.Cost.addCompute(cpu, r.cfg.ComputeUSDPerCoreSecond)
		if err != nil {
			return nil, false
		}
		cand := &cluster.Candidate{Index: 1000 + round, Source: src, Report: rep}
		action, reason := ActEscalate, "repair did not clear the ladder"
		if rep.OK {
			action, reason = ActRepair, "repaired using the verifier diagnostic"
		}
		res.Trace = append(res.Trace, Step{
			Stage: "repair", Tier: tier.Name, Action: action,
			Reason:    fmt.Sprintf("round %d/%d: %s", round+1, tier.RepairDepth, reason),
			CostSoFar: res.Cost.TotalUSD, Elapsed: time.Since(t0),
		})
		if rep.OK {
			return cand, true
		}
		cur = cand
	}
	return nil, false
}

// preferRepair applies the marginal rule: take repair over escalation when it
// buys more correctness per dollar. Repair costs roughly one call at the
// current tier; escalation costs a whole sample at the next one.
//
// Without a certificate the gain terms are priors, so this is a heuristic and
// the run stays uncertified. With one, they come from measured per-tier rates.
func (r *Router) preferRepair(k int) bool {
	if r.cfg.Tiers[k].RepairDepth <= 0 {
		return false
	}
	if k == len(r.cfg.Tiers)-1 {
		return true // nowhere left to escalate
	}
	tier := r.cfg.Tiers[k]
	next := r.cfg.Tiers[k+1]
	// One repair call, versus a full sample at the next tier.
	cRepair := tier.OutPerMTok
	cEsc := next.OutPerMTok * float64(next.Samples)
	const (
		gainRepair = 0.45 // a compiler diagnostic localises the defect precisely
		gainEsc    = 0.35 // a stronger model, but starting cold
	)
	return gainRepair/cRepair > gainEsc/cEsc
}

func (r *Router) recordFailure(problemHash string, cands []cluster.Candidate) {
	for _, c := range cands {
		if c.Report == nil || c.Report.OK {
			continue
		}
		h, err := cache.CanonicalHash(c.Source)
		if err != nil {
			continue
		}
		_ = r.cache.AddFailure(problemHash, cache.Failure{
			CanonHash: h, Stage: c.Report.FailedAt.String(),
			Summary: firstLine(c.Report.Diagnostic),
		})
	}
}

func bestRefuted(cands []cluster.Candidate) *cluster.Candidate {
	best := -1
	var bestStage verify.Stage = -1
	for i, c := range cands {
		if c.Report == nil {
			continue
		}
		if c.Report.FailedAt > bestStage {
			best, bestStage = i, c.Report.FailedAt
		}
	}
	if best < 0 {
		return nil
	}
	return &cands[best]
}

func indexOf(cands []cluster.Candidate, idx int) int {
	for i, c := range cands {
		if c.Index == idx {
			return i
		}
	}
	return 0
}

// acceptReason explains an acceptance. The last tier has nowhere to escalate,
// so it accepts by construction and quoting a threshold there would imply a
// comparison that never happened.
func acceptReason(score, tau float64, certified, last bool) string {
	if last {
		return fmt.Sprintf("final tier: accepts by construction (behavioural agreement %.2f)", score)
	}
	return thresholdReason(score, tau, certified)
}

func thresholdReason(score, tau float64, certified bool) string {
	kind := "calibrated threshold"
	if !certified {
		kind = "uncalibrated prior"
	}
	rel := ">="
	if score < tau {
		rel = "<"
	}
	return fmt.Sprintf("behavioural agreement %.2f %s %.2f (%s)", score, rel, tau, kind)
}

func summarise(rep *verify.Report) string {
	if rep == nil {
		return ""
	}
	return rep.FailedAt.String() + ": " + firstLine(rep.Diagnostic)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
