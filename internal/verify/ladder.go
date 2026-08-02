// Package verify is the verifier ladder: it refutes candidate solutions by
// execution rather than scoring them by confidence. Each stage is sound, so a
// failure means incorrect with no probability attached, which is why a refuted
// candidate costs nothing against the risk budget. Stages are ordered by
// measured cost on a warm build cache and stop at the first refutation.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stage is a rung of the verifier ladder. Stages are ordered by measured cost
// on a warm build cache, which is why Build precedes Vet: on a single vCPU
// `go build` is ~43ms and `go vet` ~113ms, so building first fails codegen
// errors sooner and warms the cache for everything after it.
type Stage int

// Ladder stages.
const (
	StageParse  Stage = iota // in-process, ~0.1ms
	StageStdlib              // in-process, free: hard filter on imports
	StageTypes               // in-process with a shared source importer, ~1ms
	StageBuild               // ~43ms
	StageVet                 // ~113ms
	StageTest                // ~120ms, visible partition only
	StageRace                // ~1.4s, gated on the concurrency predicate
	StageBench               // gated on MaxAllocsOp
	StageAccept              // hidden partition; the acceptance oracle
)

var stageNames = [...]string{
	"V0:parse", "V0:stdlib", "V1:types", "V2:build",
	"V3:vet", "V4:test", "V5:race", "V6:bench", "VA:accept",
}

func (s Stage) String() string {
	if int(s) < len(stageNames) {
		return stageNames[s]
	}
	return "V?:" + strconv.Itoa(int(s))
}

// StageResult records one rung.
type StageResult struct {
	Stage      Stage         `json:"stage"`
	Name       string        `json:"name"`
	OK         bool          `json:"ok"`
	Skipped    bool          `json:"skipped,omitempty"`
	Reason     string        `json:"skip_reason,omitempty"`
	Diagnostic string        `json:"diagnostic,omitempty"`
	Elapsed    time.Duration `json:"elapsed"`
}

// Report is the outcome of running the ladder against one candidate.
//
// Soundness: OK==false means the candidate is incorrect, with no probability
// attached. That is why failed stages cost nothing against the risk budget.
// OK==true means "not refuted", which is a weaker claim; the residual is the
// oracle gap, estimated separately by mutation testing.
type Report struct {
	OK         bool          `json:"ok"`
	FailedAt   Stage         `json:"failed_at,omitempty"`
	Diagnostic string        `json:"diagnostic,omitempty"`
	Stages     []StageResult `json:"stages"`
	Static     *Static       `json:"static,omitempty"`

	// Tests is the per-test outcome vector used for behavioural clustering.
	Tests map[string]bool `json:"tests,omitempty"`

	AllocsPerOp int           `json:"allocs_per_op,omitempty"`
	CPUTime     time.Duration `json:"cpu_time"`
}

// Options controls which optional rungs run.
type Options struct {
	StdlibOnly    bool
	MaxComplexity int
	MaxAllocsOp   int
	TestTimeout   time.Duration
	RaceCount     int
	// SkipRace forces the race stage off even when the predicate fires, for
	// latency-bounded runs.
	SkipRace bool
}

// Ladder runs the verifier stages. It is safe for concurrent use; the shared
// source importer is serialised internally.
type Ladder struct {
	mu   sync.Mutex
	fset *token.FileSet
	imp  types.Importer
}

// NewLadder builds a ladder with a shared source importer. The importer caches
// typechecked standard-library packages across candidates, which is what turns
// the type stage from a 237ms cold check into a ~1ms warm one.
func NewLadder() *Ladder {
	fset := token.NewFileSet()
	return &Ladder{fset: fset, imp: importer.ForCompiler(fset, "source", nil)}
}

// Warm pre-loads the importer cache for the given stdlib packages so the first
// real candidate does not pay for them.
func (l *Ladder) Warm(pkgs ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, p := range pkgs {
		_, _ = l.imp.Import(p)
	}
}

// Run executes the ladder against a workspace holding the candidate and the
// visible test partition. It stops at the first refutation.
func (l *Ladder) Run(ctx context.Context, ws *Workspace, src string, opts Options) *Report {
	r := &Report{OK: true}
	add := func(sr StageResult) bool {
		sr.Name = sr.Stage.String()
		r.Stages = append(r.Stages, sr)
		r.CPUTime += sr.Elapsed
		if !sr.OK && !sr.Skipped {
			r.OK = false
			r.FailedAt = sr.Stage
			r.Diagnostic = sr.Diagnostic
			return false
		}
		return true
	}

	// V0: parse. Also yields the static facts the later gates need.
	t0 := time.Now()
	st, err := Analyse(src)
	if err != nil {
		add(StageResult{Stage: StageParse, Elapsed: time.Since(t0), Diagnostic: err.Error()})
		return r
	}
	r.Static = st
	if !add(StageResult{Stage: StageParse, OK: true, Elapsed: time.Since(t0)}) {
		return r
	}

	// V0b: import filter. Deterministic, so it costs nothing against risk.
	t0 = time.Now()
	sr := StageResult{Stage: StageStdlib, OK: true, Elapsed: time.Since(t0)}
	if opts.StdlibOnly && len(st.NonStdImports) > 0 {
		sr.OK = false
		sr.Diagnostic = "non-stdlib imports are not permitted: " + strings.Join(st.NonStdImports, ", ")
	}
	if !add(sr) {
		return r
	}

	// V1: types. The highest-yield rung per millisecond: it refutes invented
	// APIs and wrong signatures without invoking the compiler.
	t0 = time.Now()
	terr := l.typecheck(src)
	sr = StageResult{Stage: StageTypes, OK: terr == nil, Elapsed: time.Since(t0)}
	if terr != nil {
		sr.Diagnostic = terr.Error()
	}
	if !add(sr) {
		return r
	}

	// V2: build.
	res := ws.run(ctx, 60*time.Second, false, "build", "./...")
	if !add(stageFrom(StageBuild, res)) {
		return r
	}

	// V3: vet.
	res = ws.run(ctx, 60*time.Second, false, "vet", "./...")
	if !add(stageFrom(StageVet, res)) {
		return r
	}

	// V4: visible tests. The hidden partition is deliberately excluded: it is
	// the acceptance oracle, and repairing against it would destroy the
	// holdout that makes acceptance meaningful.
	res = ws.run(ctx, opts.TestTimeout, true, "test", "-json", "-count=1", "-run", "^TestV", "./...")
	r.Tests = parseTestJSON(res.Output)
	sr = stageFrom(StageTest, res)
	sr.Diagnostic = summariseTestOutput(res.Output, sr.Diagnostic)
	// A stage that ran ZERO tests must not report OK. `go test -run ^TestV` on a
	// suite whose functions are named otherwise exits 0 with "[no tests to run]",
	// so a vacuous pass would look identical to a real one and every candidate
	// would clear this rung — an unsound oracle, which invariant #4 forbids. This
	// is a soundness guard, not a style check: it converts a silently-empty
	// partition into a refutation with a diagnostic that names the cause.
	if sr.OK && len(r.Tests) == 0 {
		sr.OK = false
		sr.Diagnostic = "visible partition ran no tests: no test function matches ^TestV " +
			"(a suite that runs nothing cannot refute anything)"
	}
	if !add(sr) {
		return r
	}

	// V5: race, gated on a free AST predicate. The gate is deterministic, so
	// skipping costs nothing against risk.
	switch {
	case opts.SkipRace:
		add(StageResult{Stage: StageRace, OK: true, Skipped: true, Reason: "latency-bounded run"})
	case !st.UsesConcurrency:
		add(StageResult{Stage: StageRace, OK: true, Skipped: true, Reason: "no goroutines, channels, select or sync"})
	default:
		n := max(opts.RaceCount, 1)
		res = ws.run(ctx, opts.TestTimeout*time.Duration(n)+30*time.Second, true,
			"test", "-race", "-count="+strconv.Itoa(n), "-run", "^TestV", "./...")
		sr = stageFrom(StageRace, res)
		sr.Diagnostic = summariseTestOutput(res.Output, sr.Diagnostic)
		if !add(sr) {
			return r
		}
	}

	// V6: allocation bound. Deterministic given a benchmark, so it is a
	// measurement gate rather than a probabilistic one.
	if opts.MaxAllocsOp > 0 {
		res = ws.run(ctx, opts.TestTimeout, true, "test", "-run", "^$", "-bench", ".", "-benchmem", "./...")
		sr = StageResult{Stage: StageBench, OK: true, Elapsed: res.Duration}
		if allocs, ok := parseAllocs(res.Output); ok {
			r.AllocsPerOp = allocs
			if allocs > opts.MaxAllocsOp {
				sr.OK = false
				sr.Diagnostic = fmt.Sprintf("%d allocs/op exceeds limit %d", allocs, opts.MaxAllocsOp)
			}
		} else {
			sr.Skipped = true
			sr.Reason = "no benchmark in the test partition"
		}
		if !add(sr) {
			return r
		}
	}

	// Cyclomatic complexity is computed from the AST, so it is checked last
	// only because it never needs to short-circuit anything expensive.
	if opts.MaxComplexity > 0 && st.MaxComplexity > opts.MaxComplexity {
		add(StageResult{
			Stage: StageParse,
			Diagnostic: fmt.Sprintf("cyclomatic complexity %d in %s exceeds limit %d",
				st.MaxComplexity, st.ComplexFunc, opts.MaxComplexity),
		})
	}
	return r
}

// Accept runs the held-out partition. It is called only on a candidate that has
// already survived Run, and its result is never fed back into repair.
func (l *Ladder) Accept(ctx context.Context, ws *Workspace, opts Options) *Report {
	r := &Report{OK: true}
	res := ws.run(ctx, opts.TestTimeout, true, "test", "-json", "-count=1", "-run", "^TestH", "./...")
	sr := stageFrom(StageAccept, res)
	sr.Name = sr.Stage.String()
	sr.Diagnostic = summariseTestOutput(res.Output, sr.Diagnostic)
	r.Stages = append(r.Stages, sr)
	r.CPUTime = res.Duration
	r.Tests = parseTestJSON(res.Output)
	// The same soundness guard as the visible stage, and it matters more here:
	// this is the acceptance oracle. A hidden partition with no ^TestH function
	// exits 0 having executed nothing, which would accept every candidate and
	// certify a risk bound against an oracle that never ran (invariants #4/#6).
	if sr.OK && len(r.Tests) == 0 {
		sr.OK = false
		sr.Diagnostic = "hidden partition ran no tests: no test function matches ^TestH " +
			"(acceptance against an empty oracle is not acceptance)"
		r.Stages[len(r.Stages)-1] = sr
	}
	if !sr.OK {
		r.OK = false
		r.FailedAt = StageAccept
		r.Diagnostic = sr.Diagnostic
	}
	return r
}

// RunAllTests executes EVERY test in the workspace, with no ^TestV/^TestH filter,
// and reports whether the candidate compiled, how many tests reported, and the
// diagnostic.
//
// This exists for the estimator's independent (canonical) oracle, which is a
// whole human-authored suite rather than a generated V/H partition. Running it
// through the ladder's visible stage would apply `-run ^TestV` and silently skip
// every TestH* function in it — measuring η_fa against a fraction of the suite,
// with the adversarial half (boundaries, overflow) exactly the part dropped. It
// is deliberately NOT on the acceptance path: the V/H split is what protects the
// holdout (invariant #1), so nothing that routes may use this.
//
// ran=false means the candidate did not compile against the suite (no label is
// available — an API mismatch is not an incorrectness signal) or the suite
// executed no tests at all.
func (l *Ladder) RunAllTests(ctx context.Context, ws *Workspace, opts Options) (ran, passed bool, ntests int, diag string) {
	// Compile first and separately, so "does not compile" is distinguishable from
	// "compiled and failed" — the caller reports those as different verdicts.
	if res := ws.run(ctx, 60*time.Second, false, "build", "./..."); res.Err != nil {
		return false, false, 0, "candidate does not compile against the suite: " +
			summariseTestOutput(res.Output, res.Err.Error())
	}
	res := ws.run(ctx, opts.TestTimeout, true, "test", "-json", "-count=1", "./...")
	tests := parseTestJSON(res.Output)
	diag = summariseTestOutput(res.Output, "")
	if len(tests) == 0 {
		// Two different no-label causes, told apart by the exit status rather than by
		// matching on go's output. `go build` above does not compile _test.go files,
		// so a solution/suite signature mismatch reaches here: the test binary fails
		// to link, nothing reports, and the exit is non-zero. An actually-empty suite
		// exits ZERO having executed nothing. Conflating them would report an API
		// mismatch as a vacuous suite, which is the one distinction a reader of the
		// records needs in order to tell a benchmark defect from an oracle defect.
		if res.Err != nil {
			return false, false, 0, "candidate does not compile against the suite: " + diag
		}
		return false, false, 0, "suite executed no tests: " + diag
	}
	return true, res.Err == nil, len(tests), diag
}

func (l *Ladder) typecheck(src string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := parser.ParseFile(l.fset, "solution.go", src, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	var first error
	conf := types.Config{
		Importer: l.imp,
		Error: func(e error) {
			if first == nil {
				first = e
			}
		},
	}
	if _, err := conf.Check("solution", l.fset, []*ast.File{f}, nil); err != nil {
		if first != nil {
			return first
		}
		return err
	}
	return first
}

func stageFrom(s Stage, res cmdResult) StageResult {
	sr := StageResult{Stage: s, OK: res.Err == nil && !res.TimedOut, Elapsed: res.Duration}
	if !sr.OK {
		sr.Diagnostic = res.Output
		if sr.Diagnostic == "" && res.Err != nil {
			sr.Diagnostic = res.Err.Error()
		}
	}
	return sr
}

// testEvent is the subset of `go test -json` we consume.
type testEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
	Output string `json:"Output"`
}

// parseTestJSON builds the per-test outcome vector. This vector, not the source
// text, is what candidates are clustered on: two implementations that agree on
// every observable outcome belong in the same behavioural class however
// differently they are written.
func parseTestJSON(out string) map[string]bool {
	res := map[string]bool{}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass":
			res[ev.Test] = true
		case "fail":
			res[ev.Test] = false
		}
	}
	return res
}

// summariseTestOutput turns -json noise into something a repair prompt can use.
func summariseTestOutput(out, fallback string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			if line != "" {
				b.WriteString(line + "\n")
			}
			continue
		}
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Action == "output" {
			t := strings.TrimRight(ev.Output, "\n")
			if t == "" || strings.HasPrefix(t, "=== RUN") || strings.HasPrefix(t, "--- PASS") ||
				strings.HasPrefix(t, "ok ") || t == "PASS" {
				continue
			}
			b.WriteString(t + "\n")
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		return s
	}
	return fallback
}

var allocsRE = regexp.MustCompile(`(\d+)\s+allocs/op`)

// parseAllocs extracts the worst allocs/op across all benchmarks.
func parseAllocs(out string) (int, bool) {
	ms := allocsRE.FindAllStringSubmatch(out, -1)
	if len(ms) == 0 {
		return 0, false
	}
	worst := 0
	for _, m := range ms {
		if n, err := strconv.Atoi(m[1]); err == nil && n > worst {
			worst = n
		}
	}
	return worst, true
}
