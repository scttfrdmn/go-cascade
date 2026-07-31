package cascade

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scttfrdmn/go-cascade/internal/calibrate"
	"github.com/scttfrdmn/go-cascade/internal/config"
	"github.com/scttfrdmn/go-cascade/internal/model"
	"github.com/scttfrdmn/go-cascade/internal/prompt"
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

// validateOracle must accept a spec whose tests pass the reference and reject a
// spec whose tests refute it. This is the mechanism that turns a spec-model test
// bug into a skipped problem instead of a spurious model error (invariant #4).
func TestValidateOracleDetectsUnsoundTests(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the reference against generated tests")
	}
	r := newRouter(t, testConfig(t), nil)
	ctx := context.Background()

	const ref = `package solution

// Double returns n*2.
func Double(n int) int { return n * 2 }
`
	const api = `package solution

// Double returns n*2.
func Double(n int) int { panic("not implemented") }
`
	soundSpec := &prompt.Spec{
		API: api,
		VisibleTests: `package solution
import "testing"
func TestVDouble(t *testing.T) { if Double(3) != 6 { t.Fatal("want 6") } }`,
		HiddenTests: `package solution
import "testing"
func TestHDoubleZero(t *testing.T) { if Double(0) != 0 { t.Fatal("want 0") } }`,
	}
	r.SetReferences(map[string]string{"dbl": ref})
	if v, diag := r.validateOracle(ctx, "dbl", soundSpec); v != OracleSound {
		t.Errorf("sound generated tests were not classified sound (v=%d): %s", v, diag)
	}

	// An id with no registered reference is unchecked: the default path is
	// unchanged for benchmarks that ship no reference.
	if v, _ := r.validateOracle(ctx, "no-ref", soundSpec); v != OracleSound {
		t.Error("a problem with no reference must pass the check unchanged")
	}

	// A hidden test asserting the wrong expected value refutes the correct
	// reference — exactly the scale_two_sum failure mode observed live. The
	// reference compiles (Double exists), so this is a behavioural refutation:
	// unsound, not inconclusive.
	buggySpec := &prompt.Spec{
		API:          api,
		VisibleTests: soundSpec.VisibleTests,
		HiddenTests: `package solution
import "testing"
func TestHDoubleWrong(t *testing.T) { if Double(2) != 5 { t.Fatal("test asserts a wrong answer") } }`,
	}
	v, diag := r.validateOracle(ctx, "dbl", buggySpec)
	if v != OracleUnsoundVerdict {
		t.Errorf("a test that refutes the compiling reference must be unsound, got v=%d", v)
	}
	if !strings.Contains(diag, "reference refuted") {
		t.Errorf("diagnostic should explain the refutation, got: %q", diag)
	}

	// A spec whose tests call a DIFFERENT function name than the reference
	// implements must be INCONCLUSIVE (API mismatch), not unsound — the reference
	// cannot compile, which says nothing about whether the tests are correct for a
	// candidate written against the generated API. This is the class that
	// over-fired 12x when misclassified as unsound.
	mismatchSpec := &prompt.Spec{
		API: `package solution
// Twice returns n*2.
func Twice(n int) int { panic("not implemented") }`,
		VisibleTests: `package solution
import "testing"
func TestVTwice(t *testing.T) { if Twice(3) != 6 { t.Fatal("want 6") } }`,
		HiddenTests: `package solution
import "testing"
func TestHTwiceZero(t *testing.T) { if Twice(0) != 0 { t.Fatal("want 0") } }`,
	}
	v, diag = r.validateOracle(ctx, "dbl", mismatchSpec)
	if v != OracleInconclusive {
		t.Errorf("an API-name mismatch must be inconclusive, not unsound; got v=%d diag=%q", v, diag)
	}

	// A generated test that uses a stdlib package it forgot to import does not
	// compile against ANY candidate (or the reference), so it is a broken/unsound
	// oracle — distinct from an API mismatch, where the tests compile against
	// their own API. This is the third spec-model noise class the pinned run
	// surfaced (missing import in the test). It must be UNSOUND, not inconclusive.
	missingImportSpec := &prompt.Spec{
		API: api, // matches the reference's Double, so it is not a signature mismatch
		VisibleTests: `package solution
import "testing"
func TestVDouble(t *testing.T) { if Double(3) != 6 { t.Fatal("want 6") } }`,
		// Uses strings.Repeat but never imports "strings": will not compile.
		HiddenTests: `package solution
import "testing"
func TestHDoubleBig(t *testing.T) { _ = strings.Repeat("x", 3); if Double(0) != 0 { t.Fatal("want 0") } }`,
	}
	v, diag = r.validateOracle(ctx, "dbl", missingImportSpec)
	if v != OracleUnsoundVerdict {
		t.Errorf("a test that will not compile against its own API must be unsound; got v=%d diag=%q", v, diag)
	}
	if !strings.Contains(diag, "do not compile against their own API") {
		t.Errorf("diagnostic should name the malformed-test cause, got: %q", diag)
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

// A two-stage tier's planner authors the code, so a planner equal to the oracle
// author violates invariant #3 and must be rejected at config time, not silently
// calibrated. (Fast: no toolchain.)
func TestValidateRejectsPlannerEqualsTestModel(t *testing.T) {
	cfg := testConfig(t)
	cfg.Tiers[0].PlannerModel = cfg.TestModel // planner == oracle author
	if err := cfg.Validate(); err == nil {
		t.Error("a planner_model equal to test_model must be rejected (invariant #3)")
	}
	cfg.Tiers[0].PlannerModel = model.MockLarge // distinct planner is fine
	if err := cfg.Validate(); err != nil {
		t.Errorf("a planner distinct from the oracle author must be accepted: %v", err)
	}
}

// recordingProvider wraps a Provider and tallies calls by purpose, plus the last
// user text seen per purpose, so a test can assert the planner ran and that its
// output reached the coder.
type recordingProvider struct {
	inner   model.Provider
	mu      sync.Mutex
	byModel map[string]int
	byPurp  map[model.Purpose]int
	// planReachedCoder is true if any code-purpose prompt carried a plan. It is a
	// bool, not "the last prompt", because a two-stage tier 0 can escalate to
	// single-stage higher tiers whose plan-free prompts would otherwise mask it.
	planReachedCoder bool
}

func newRecording(inner model.Provider) *recordingProvider {
	return &recordingProvider{inner: inner, byModel: map[string]int{}, byPurp: map[model.Purpose]int{}}
}

func (p *recordingProvider) Name() string { return p.inner.Name() }

func (p *recordingProvider) Generate(ctx context.Context, req model.Request) (*model.Response, error) {
	p.mu.Lock()
	p.byModel[req.ModelID]++
	p.byPurp[req.Purpose]++
	if req.Purpose == model.PurposeCode && len(req.Messages) > 0 &&
		strings.Contains(req.Messages[len(req.Messages)-1].Text, "implementation plan") {
		p.planReachedCoder = true
	}
	p.mu.Unlock()
	return p.inner.Generate(ctx, req)
}

// A two-stage tier must (a) make exactly one planner call to the planner model,
// (b) thread the plan into the coder prompt, (c) charge the planner's cost, and
// (d) leave the oracle uncontaminated when the planner differs from TestModel.
func TestTwoStageTierRunsPlanner(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs candidates")
	}
	cfg := testConfig(t)
	cfg.CacheDir = "" // bypass arm zero so the tier actually samples
	// Make tier 0 two-stage: MockLarge plans, MockSmall codes. Both differ from
	// the MockOracle test model, so the record must be uncontaminated.
	cfg.Tiers[0].PlannerModel = model.MockLarge
	cfg.Tiers[0].Samples = 2

	rec := newRecording(model.Mock{})
	r, err := New(cfg, rec, nil)
	if err != nil {
		t.Skipf("cannot build router (no go toolchain?): %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	res, err := r.Solve(context.Background(), seqProblem)
	if err != nil {
		t.Fatal(err)
	}
	if p := rec.byPurp[model.PurposePlan]; p != 1 {
		t.Errorf("two-stage tier made %d planner calls, want exactly 1", p)
	}
	if rec.byModel[model.MockLarge] < 1 {
		t.Error("the planner model was never called")
	}
	if !rec.planReachedCoder {
		t.Error("the plan never reached any coder prompt")
	}
	if res.OracleContaminated {
		t.Error("a planner distinct from TestModel must not contaminate the oracle")
	}
	if res.Cost.ModelCalls < 1 {
		t.Error("planner cost was not accounted")
	}
}
