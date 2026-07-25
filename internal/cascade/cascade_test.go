package cascade

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/config"
	"github.com/scttfrdmn/go-cascade/internal/model"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Alpha = 0.1
	cfg.Mutants = 0
	cfg.RaceCount = 1
	cfg.CacheDir = t.TempDir()
	cfg.ShadowRate = 0 // deterministic: never bypass the cache in tests
	ids := []string{model.MockSmall, model.MockMid, model.MockLarge}
	for i := range cfg.Tiers {
		cfg.Tiers[i].ModelID = ids[i]
	}
	cfg.TestModel = model.MockOracle
	return cfg
}

func newRouter(t *testing.T, cfg *config.Config, cert *calibrate.Certificate) *Router {
	t.Helper()
	r, err := New(cfg, model.Mock{}, cert)
	if err != nil {
		t.Skipf("cannot build router (no go toolchain?): %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

const seqProblem = "Return the length of the longest strictly increasing contiguous run in a slice of integers."

func TestSolveEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	r := newRouter(t, testConfig(t), nil)
	res, err := r.Solve(context.Background(), seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved {
		t.Fatalf("nothing survived the ladder; trace: %+v", res.Trace)
	}
	if !strings.Contains(res.Solution, "package solution") {
		t.Errorf("accepted solution is not a Go file:\n%s", res.Solution)
	}
	if res.Cost.TotalUSD <= 0 || res.Cost.ModelCalls == 0 {
		t.Errorf("cost accounting is empty: %+v", res.Cost)
	}
	// Without a certificate the run must not claim a bound.
	if res.Certified {
		t.Error("an uncalibrated run reported itself as certified")
	}
	if !strings.Contains(res.RiskStatement(), "UNCERTIFIED") {
		t.Errorf("risk statement hides the lack of calibration: %q", res.RiskStatement())
	}
	if res.API == "" || res.VisibleTests == "" || res.HiddenTests == "" {
		t.Error("the contract or one of the test partitions is missing")
	}
}

// A run with a valid certificate must say so, including when the final tier
// accepts (that tier has no threshold, which is not the same as uncalibrated).
func TestSolveReportsCertificationFromPolicyNotTier(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cert := &calibrate.Certificate{
		Valid: true, Alpha: 0.15, Delta: 0.1, N: 40,
		Method: calibrate.FixedSequence, Thresholds: []float64{1.0, 1.0},
	}
	r := newRouter(t, testConfig(t), cert)
	res, err := r.Solve(context.Background(), seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved {
		t.Fatalf("nothing survived; trace: %+v", res.Trace)
	}
	// Thresholds of 1.0 force escalation all the way to the final tier.
	if res.AcceptedAt != "large" {
		t.Errorf("accepted at %q; thresholds of 1.0 should escalate to the final tier", res.AcceptedAt)
	}
	if !res.Certified {
		t.Error("a valid certificate was loaded but the run reported itself uncertified")
	}
	if strings.Contains(res.RiskStatement(), "UNCERTIFIED") {
		t.Errorf("risk statement contradicts the certificate: %q", res.RiskStatement())
	}
}

// The concurrency path: the race stage must gate on the AST predicate and the
// trace must show it separating racy candidates from clean ones.
func TestSolveConcurrentProblemUsesRaceStage(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the race detector")
	}
	cfg := testConfig(t)
	r := newRouter(t, cfg, nil)
	res, err := r.Solve(context.Background(),
		"Implement a generic ParallelMap applying a function over a slice with at most N worker goroutines, preserving order.")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved {
		t.Fatalf("nothing survived; trace: %+v", res.Trace)
	}
	if res.Static == nil || !res.Static.UsesConcurrency {
		t.Error("the concurrency predicate did not fire on a goroutine-based solution")
	}
	var sawRace bool
	for _, s := range res.Trace {
		for _, c := range s.Clusters {
			if strings.Contains(c.Key, "V5:race") {
				sawRace = true
			}
		}
	}
	if !sawRace {
		t.Log("no racy candidate was sampled in this run; ThreadSanitizer is sound but incomplete")
	}
}

// Arm zero: the second solve of a problem must reuse the cached oracle, which
// is the highest-leverage cache layer because test generation is O(1) per query
// while candidate generation is O(tiers).
func TestSpecCacheReducesCost(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	r := newRouter(t, cfg, nil)
	ctx := context.Background()

	first, err := r.Solve(ctx, seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Solve(ctx, seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if second.Cost.ModelCalls >= first.Cost.ModelCalls {
		t.Errorf("second run made %d model calls, first made %d; the oracle was not reused",
			second.Cost.ModelCalls, first.Cost.ModelCalls)
	}
	var hit bool
	for _, s := range second.Trace {
		if s.Stage == "spec" && strings.Contains(s.Reason, "cache hit") {
			hit = true
		}
	}
	if !hit {
		t.Error("no spec cache hit recorded on the second solve")
	}
}

func TestDisabledCacheStillSolves(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = ""
	r := newRouter(t, cfg, nil)
	res, err := r.Solve(context.Background(), seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved {
		t.Fatal("disabling arm zero broke the cascade")
	}
}

// Under a deadline the topology flips from sequential to speculative parallel.
func TestSpeculativeModeUnderDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.Deadline = 90 * time.Second
	r := newRouter(t, cfg, nil)
	res, err := r.Solve(context.Background(), seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Solved {
		t.Fatalf("speculative mode solved nothing; trace: %+v", res.Trace)
	}
	var spec bool
	for _, s := range res.Trace {
		if s.Stage == "speculative" {
			spec = true
		}
	}
	if !spec {
		t.Error("a deadline did not switch the router to speculative routing")
	}
	// Parallelism buys latency with dollars: every tier runs, so this must not
	// look cheaper than a cascade.
	if res.Cost.ModelCalls < len(cfg.Tiers) {
		t.Errorf("speculative mode made %d calls; it should start every tier", res.Cost.ModelCalls)
	}
}

func TestValidateRejectsDualKnobs(t *testing.T) {
	cfg := config.Default()
	cfg.Alpha, cfg.Budget = 0.05, 1.0
	if err := cfg.Validate(); err == nil {
		t.Error("alpha and budget are dual forms of one constraint; setting both must be rejected")
	}
	cfg.Alpha, cfg.Budget = 0, 0
	if err := cfg.Validate(); err == nil {
		t.Error("neither alpha nor budget set must be rejected")
	}
}
