package verify

import (
	"bytes"
	"context"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
	"time"
)

const goodSrc = `package solution

// LongestIncreasingRun returns the length of the longest strictly increasing run.
func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(xs); i++ {
		if xs[i] > xs[i-1] {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
	}
	return best
}`

const visibleSrc = `package solution

import "testing"

func TestV_Basic(t *testing.T) {
	if got := LongestIncreasingRun([]int{1, 2, 3}); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
	if got := LongestIncreasingRun(nil); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}`

const hiddenSrc = `package solution

import "testing"

func TestH_Plateau(t *testing.T) {
	if got := LongestIncreasingRun([]int{1, 2, 2, 3}); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}`

func newTestRunner(t *testing.T) *Runner {
	t.Helper()
	r, err := NewRunner("", nil)
	if err != nil {
		t.Skipf("no go toolchain available: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func opts() Options {
	return Options{StdlibOnly: true, TestTimeout: 60 * time.Second, RaceCount: 1}
}

func run(t *testing.T, r *Runner, l *Ladder, src string) (*Report, *Workspace) {
	t.Helper()
	ws, err := r.NewWorkspace(src, visibleSrc, hiddenSrc)
	if err != nil {
		t.Fatal(err)
	}
	return l.Run(context.Background(), ws, src, opts()), ws
}

func TestLadderAcceptsCorrectSolution(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	r := newTestRunner(t)
	l := NewLadder()
	rep, ws := run(t, r, l, goodSrc)
	defer ws.Remove() //nolint:errcheck // scratch
	if !rep.OK {
		t.Fatalf("correct solution refuted at %s: %s", rep.FailedAt, rep.Diagnostic)
	}
	acc := l.Accept(context.Background(), ws, opts())
	if !acc.OK {
		t.Fatalf("held-out partition refuted a correct solution: %s", acc.Diagnostic)
	}
	if len(rep.Tests) == 0 {
		t.Error("no per-test outcomes captured; behavioural clustering would be blind")
	}
}

// The one-sided soundness property: every one of these must be refuted, and at
// the cheapest stage capable of refuting it.
func TestLadderRefutations(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	r := newTestRunner(t)
	l := NewLadder()

	cases := []struct {
		name  string
		src   string
		stage Stage
		want  string
	}{
		{
			name: "syntax error",
			src: `package solution
func LongestIncreasingRun(xs []int) int {
	return 0`,
			stage: StageParse,
		},
		{
			name: "hallucinated stdlib API",
			src: `package solution

import "slices"

func LongestIncreasingRun(xs []int) int {
	return slices.MaxRunFunc(xs, func(a, b int) bool { return a < b })
}`,
			stage: StageTypes,
			want:  "MaxRunFunc",
		},
		{
			name: "non-stdlib import",
			src: `package solution

import "github.com/pkg/errors"

func LongestIncreasingRun(xs []int) int {
	_ = errors.New("x")
	return 0
}`,
			stage: StageStdlib,
		},
		{
			name: "wrong signature",
			src: `package solution

func LongestIncreasingRun(xs []string) int { return len(xs) }`,
			// `go build` does not compile _test.go files, so a mismatch between
			// the solution and the oracle survives it. Vet is the cheapest
			// stage that typechecks the tests against the solution, which is
			// what earns it a slot in the ladder at all.
			stage: StageVet,
		},
		{
			name: "wrong answer",
			src: `package solution

func LongestIncreasingRun(xs []int) int { return 42 }`,
			stage: StageTest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep, ws := run(t, r, l, tc.src)
			defer ws.Remove() //nolint:errcheck // scratch
			if rep.OK {
				t.Fatalf("ladder failed to refute %q", tc.name)
			}
			if rep.FailedAt != tc.stage {
				t.Errorf("refuted at %s, expected %s (diagnostic: %s)",
					rep.FailedAt, tc.stage, firstLine(rep.Diagnostic))
			}
			if tc.want != "" && !strings.Contains(rep.Diagnostic, tc.want) {
				t.Errorf("diagnostic %q does not mention %q", firstLine(rep.Diagnostic), tc.want)
			}
		})
	}
}

// The subtle defect must survive the visible partition and die on the held-out
// one. If this ever passes acceptance, the holdout has stopped working.
func TestHiddenPartitionCatchesWhatVisibleMisses(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	const subtle = `package solution

func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(xs); i++ {
		if xs[i] >= xs[i-1] {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
	}
	return best
}`
	r := newTestRunner(t)
	l := NewLadder()
	rep, ws := run(t, r, l, subtle)
	defer ws.Remove() //nolint:errcheck // scratch
	if !rep.OK {
		t.Fatalf("the visible partition should not catch this defect, but it failed at %s", rep.FailedAt)
	}
	acc := l.Accept(context.Background(), ws, opts())
	if acc.OK {
		t.Fatal("the held-out partition failed to catch a non-strict comparison")
	}
}

// A partition whose test functions do not match the stage's -run filter executes
// nothing and `go test` exits 0. That is a *vacuous* pass and must be reported as a
// refutation: an oracle that runs nothing cannot refute anything, so treating it as
// OK would clear every candidate through that rung (invariant #4) and, at the
// acceptance stage, certify a risk bound against an oracle that never ran
// (invariant #6). The misnamed suite below is not hypothetical — it is exactly
// MultiPL-E's naming (`TestHas_Close_Elements`), which starts with "TestH" and so
// slips past ^TestV while looking like a normal suite.
func TestEmptyPartitionIsRefutedNotPassed(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	const misnamedVisible = `package solution

import "testing"

func TestHas_Close_Elements(t *testing.T) {
	if got := LongestIncreasingRun([]int{1, 2, 3}); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}`
	// Named so it matches neither ^TestV nor ^TestH.
	const misnamedHidden = `package solution

import "testing"

func TestX_Plateau(t *testing.T) {
	if got := LongestIncreasingRun([]int{1, 2, 2, 3}); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}`

	r := newTestRunner(t)
	l := NewLadder()

	t.Run("visible", func(t *testing.T) {
		ws, err := r.NewWorkspace(goodSrc, misnamedVisible, "")
		if err != nil {
			t.Fatal(err)
		}
		defer ws.Remove() //nolint:errcheck // scratch
		rep := l.Run(context.Background(), ws, goodSrc, opts())
		if rep.OK {
			t.Fatal("a visible partition that executed zero tests reported a pass; " +
				"every candidate would clear this rung against an oracle that never ran")
		}
		if rep.FailedAt != StageTest {
			t.Errorf("refuted at %s, want %s", rep.FailedAt, StageTest)
		}
		if !strings.Contains(rep.Diagnostic, "no tests") {
			t.Errorf("diagnostic does not name the cause: %q", firstLine(rep.Diagnostic))
		}
	})

	t.Run("hidden", func(t *testing.T) {
		ws, err := r.NewWorkspace(goodSrc, visibleSrc, misnamedHidden)
		if err != nil {
			t.Fatal(err)
		}
		defer ws.Remove() //nolint:errcheck // scratch
		if rep := l.Run(context.Background(), ws, goodSrc, opts()); !rep.OK {
			t.Fatalf("the visible partition is well-named and should pass: %s", rep.Diagnostic)
		}
		acc := l.Accept(context.Background(), ws, opts())
		if acc.OK {
			t.Fatal("acceptance against a partition that executed zero tests succeeded; " +
				"a certificate over this oracle would be vacuous")
		}
		if acc.FailedAt != StageAccept {
			t.Errorf("failed at %s, want %s", acc.FailedAt, StageAccept)
		}
		// The recorded stage must carry the failure too: calibration reads the stage
		// vector, not just Report.OK.
		last := acc.Stages[len(acc.Stages)-1]
		if last.OK {
			t.Error("Report.OK is false but the recorded accept stage still says OK")
		}
		if !strings.Contains(last.Diagnostic, "no tests") {
			t.Errorf("stage diagnostic does not name the cause: %q", firstLine(last.Diagnostic))
		}
	})
}

// RunAllTests is the estimator's independent-oracle entry point. Its defining
// property is the one the first run of experiment 19 got wrong: it must execute the
// TestH* half of a human-authored suite as well as the TestV* half. Going through
// the ladder's visible stage applied `-run ^TestV` and silently dropped the
// adversarial tests, measuring η_fa at a fraction of the oracle's real strength.
func TestRunAllTestsRunsEveryTest(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	r := newTestRunner(t)
	l := NewLadder()

	// The defect in `subtle` is caught ONLY by TestH_Plateau. If RunAllTests still
	// filtered on ^TestV this would report passed=true with one test — which is
	// precisely how a weakened oracle looks from the outside.
	const subtle = `package solution

func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(xs); i++ {
		if xs[i] >= xs[i-1] {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
	}
	return best
}`

	cases := []struct {
		name             string
		src              string
		visible, hidden  string
		wantRan, wantPas bool
		wantN            int
		wantDiag         string
	}{
		{"correct", goodSrc, visibleSrc, hiddenSrc, true, true, 2, ""},
		{"wrong only under TestH", subtle, visibleSrc, hiddenSrc, true, false, 2, ""},
		{"does not compile", `package solution

func LongestIncreasingRun(xs []int) int { return "no" }`, visibleSrc, hiddenSrc, false, false, 0, "does not compile"},
		// The signature mismatch matters more than the syntax error: `go build` does
		// not compile _test.go files, so this one gets PAST the build check and only
		// fails when the test binary links. Without the exit-status branch it is
		// reported as "suite executed no tests" — an API mismatch dressed up as a
		// vacuous oracle, which is exactly the distinction the estimator's records
		// need in order to separate a benchmark defect from an oracle defect.
		{"wrong signature", `package solution

func LongestIncreasingRun(xs []string) int { return 0 }`, visibleSrc, hiddenSrc, false, false, 0, "does not compile"},
		{"empty suite", goodSrc, "", "", false, false, 0, "executed no tests"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws, err := r.NewWorkspace(c.src, c.visible, c.hidden)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Remove() //nolint:errcheck // scratch
			ran, passed, n, diag := l.RunAllTests(context.Background(), ws, opts())
			if ran != c.wantRan || passed != c.wantPas {
				t.Errorf("ran=%v passed=%v, want ran=%v passed=%v (%s)",
					ran, passed, c.wantRan, c.wantPas, firstLine(diag))
			}
			if n != c.wantN {
				t.Errorf("executed %d tests, want %d — the whole suite must run, both partitions", n, c.wantN)
			}
			if c.wantDiag != "" && !strings.Contains(diag, c.wantDiag) {
				t.Errorf("diagnostic %q does not contain %q", firstLine(diag), c.wantDiag)
			}
			// A compile failure must not be reported as "ran nothing", so the two
			// no-label paths stay distinguishable to the caller.
			if c.wantDiag == "does not compile" && strings.Contains(diag, "executed no tests") {
				t.Error("a compile failure was reported as an empty suite; " +
					"the estimator could not tell an API mismatch from a vacuous run")
			}
		})
	}
}

func TestRaceGateAndDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	const racy = `package solution

import "sync"

// LongestIncreasingRun computes the answer with an unsynchronised shared write.
func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	best := 1
	var wg sync.WaitGroup
	for i := 1; i < len(xs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cur := 1
			for j := i; j > 0 && xs[j] > xs[j-1]; j-- {
				cur++
			}
			if cur > best {
				best = cur // data race
			}
		}(i)
	}
	wg.Wait()
	return best
}`
	st, err := Analyse(racy)
	if err != nil {
		t.Fatal(err)
	}
	if !st.UsesConcurrency {
		t.Fatal("the concurrency predicate missed goroutines and sync")
	}
	if st2, _ := Analyse(goodSrc); st2.UsesConcurrency {
		t.Error("the concurrency predicate fired on sequential code; the race stage would be wasted")
	}

	r := newTestRunner(t)
	l := NewLadder()
	ws, err := r.NewWorkspace(racy, visibleSrc, hiddenSrc)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Remove() //nolint:errcheck // scratch
	rep := l.Run(context.Background(), ws, racy, Options{
		StdlibOnly: true, TestTimeout: 90 * time.Second, RaceCount: 4,
	})
	if rep.OK {
		t.Skip("the race was not observed in this scheduling; ThreadSanitizer is sound but incomplete")
	}
	if rep.FailedAt != StageRace && rep.FailedAt != StageTest {
		t.Errorf("expected refutation at the race or test stage, got %s", rep.FailedAt)
	}
}

func TestStaticComplexityAndImports(t *testing.T) {
	st, err := Analyse(goodSrc)
	if err != nil {
		t.Fatal(err)
	}
	if st.MaxComplexity < 4 {
		t.Errorf("complexity %d looks too low for two branches in a loop", st.MaxComplexity)
	}
	if len(st.NonStdImports) != 0 {
		t.Errorf("stdlib-only source reported non-stdlib imports: %v", st.NonStdImports)
	}
	if !isStdlib("net/http") || isStdlib("github.com/x/y") || isStdlib("golang.org/x/sync") {
		t.Error("stdlib classification is wrong")
	}
}

// Mutation score is the oracle-gap estimate. A weak suite must score lower than
// a strong one, or the number is not measuring anything.
func TestMutationScoreDiscriminatesOracleStrength(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}

	r := newTestRunner(t)
	ctx := context.Background()

	strong, err := Mutate(ctx, r, goodSrc, visibleSrc, hiddenSrc, 12, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	const weakVisible = `package solution

import "testing"

func TestV_Trivial(t *testing.T) {
	_ = LongestIncreasingRun([]int{1, 2, 3})
}`
	weak, err := Mutate(ctx, r, goodSrc, weakVisible, "", 12, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if strong.Valid == 0 || weak.Valid == 0 {
		t.Fatalf("no valid mutants compiled (strong=%d weak=%d)", strong.Valid, weak.Valid)
	}
	if weak.Score >= strong.Score {
		t.Errorf("a suite that asserts nothing scored %.2f, at least as high as a real suite at %.2f",
			weak.Score, strong.Score)
	}
	// A suite that asserts nothing still kills some mutants: `i++ -> i--` hangs
	// and `< -> <=` panics, and a crash is detection. That floor is why the
	// meaningful comparison is between suites, not against zero.
	if weak.Score > 0.5 {
		t.Errorf("an assertion-free suite scored %.2f; the floor should come only from crashes", weak.Score)
	}
	t.Logf("mutation score: strong suite %.2f (%d/%d), weak suite %.2f (%d/%d)",
		strong.Score, strong.Killed, strong.Valid, weak.Score, weak.Killed, weak.Valid)
}

func TestKilledMutantsAreProvablyWrong(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}
	r := newTestRunner(t)
	ctx := context.Background()

	muts, err := KilledMutants(ctx, r, goodSrc, visibleSrc, hiddenSrc, 3, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(muts) == 0 {
		t.Fatal("expected at least one killed mutant from a mutable correct solution")
	}
	// Each returned mutant must compile AND be refuted by the suite: re-run it to
	// confirm the harvest only yields provably-wrong candidates.
	for _, m := range muts {
		ws, err := r.NewWorkspace(m.Source, visibleSrc, hiddenSrc)
		if err != nil {
			t.Fatal(err)
		}
		if b := ws.run(ctx, 60*time.Second, false, "build", "./..."); b.Err != nil {
			t.Errorf("mutant %q does not compile: %v", m.Desc, b.Err)
		}
		res := ws.run(ctx, 60*time.Second, true, "test", "-count=1", "./...")
		if res.Err == nil && !res.TimedOut {
			t.Errorf("mutant %q passed the suite; KilledMutants must return only refuted candidates", m.Desc)
		}
		_ = ws.Remove()
	}
}

func TestRaceKilledMutantsAreRaceRefuted(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates under -race")
	}
	// A correct, race-free concurrent solution: each worker writes its own slot,
	// and wg.Wait() ensures all writes finish before the result is read. Deleting
	// the Wait() lets the read race the writes -- caught only under -race.
	const src = `package solution

import "sync"

// SumParts adds xs concurrently, one goroutine per element, into private slots.
func SumParts(xs []int) int {
	parts := make([]int, len(xs))
	var wg sync.WaitGroup
	for i := range xs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			parts[i] = xs[i]
		}(i)
	}
	wg.Wait()
	total := 0
	for _, p := range parts {
		total += p
	}
	return total
}`
	const visible = `package solution

import "testing"

func TestV_Sum(t *testing.T) {
	xs := make([]int, 256)
	for i := range xs {
		xs[i] = i
	}
	want := 0
	for _, v := range xs {
		want += v
	}
	if got := SumParts(xs); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}`
	r := newTestRunner(t)
	ctx := context.Background()

	muts, err := RaceKilledMutants(ctx, r, src, visible, "", 2, 3, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(muts) == 0 {
		t.Fatal("expected at least one race-refuted mutant from deleting wg.Wait()")
	}
	// Each returned mutant must compile and fail specifically under -race.
	for _, m := range muts {
		ws, err := r.NewWorkspace(m.Source, visible, "")
		if err != nil {
			t.Fatal(err)
		}
		if b := ws.run(ctx, 60*time.Second, false, "build", "./..."); b.Err != nil {
			t.Errorf("mutant %q does not compile", m.Desc)
		}
		race := ws.run(ctx, 90*time.Second, true, "test", "-race", "-count=3", "./...")
		if race.Err == nil && !race.TimedOut {
			t.Errorf("mutant %q passed under -race; RaceKilledMutants must return only race-refuted candidates", m.Desc)
		}
		_ = ws.Remove()
	}
}

// scarFreeSrc is race-free and exercises all three scar-free operators: an
// RWMutex write under Lock/Unlock (downgrade), a two-statement critical section
// (escape), and a wg.Wait() with reads after it (defer).
const scarFreeSrc = `package solution

import "sync"

// Tally sums xs concurrently and also counts the contributions.
func Tally(xs []int) (int, int) {
	parts := make([]int, len(xs))
	var mu sync.RWMutex
	var wg sync.WaitGroup
	count := 0
	for i := range xs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			count++
			parts[i] = xs[i]
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	total := 0
	for _, p := range parts {
		total += p
	}
	return total, count
}`

const scarFreeVisible = `package solution

import "testing"

func TestV_Tally(t *testing.T) {
	xs := make([]int, 256)
	want := 0
	for i := range xs {
		xs[i] = i
		want += i
	}
	got, n := Tally(xs)
	if got != want || n != len(xs) {
		t.Fatalf("got (%d,%d) want (%d,%d)", got, n, want, len(xs))
	}
}`

// The three scar-free operators must all be found, since each targets a
// different construct and a silently-missing one would shrink the seed set
// without any test failing.
func TestScarFreeRaceSitesCoverAllThreeOperators(t *testing.T) {
	sites, _, _, err := collectScarFreeRaceSites(scarFreeSrc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RLock/RUnlock", "out of the mu critical section", "defer wg.Wait()"} {
		found := false
		for _, s := range sites {
			if strings.Contains(s.desc, want) {
				found = true
			}
		}
		if !found {
			var descs []string
			for _, s := range sites {
				descs = append(descs, s.desc)
			}
			t.Errorf("no site matching %q; got %v", want, descs)
		}
	}
}

// The defining property: every scar-free mutant keeps its synchronization
// scaffolding intact. Deletion-based mutants fail this by construction, which is
// precisely why the judge catches them by imbalance rather than by reasoning
// about interleaving. Asserted on the printed source, since that is what a judge
// would read.
func TestScarFreeMutantsKeepScaffoldingBalanced(t *testing.T) {
	sites, fset, f, err := collectScarFreeRaceSites(scarFreeSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) == 0 {
		t.Fatal("no scar-free sites; the fixture no longer exercises the operators")
	}
	for _, m := range sites {
		m.apply()
		var buf bytes.Buffer
		perr := printer.Fprint(&buf, fset, f)
		m.revert()
		if perr != nil {
			t.Fatalf("%s: print: %v", m.desc, perr)
		}
		mutant := buf.String()
		if _, err := parser.ParseFile(token.NewFileSet(), "m.go", mutant, 0); err != nil {
			t.Errorf("%s: mutant does not parse: %v", m.desc, err)
			continue
		}
		// Wait survives (possibly deferred), and lock/unlock stay paired: these
		// are the imbalances a reviewer spots without reasoning about the race.
		if !strings.Contains(mutant, "wg.Wait()") {
			t.Errorf("%s: wg.Wait() disappeared — that is a scar\n%s", m.desc, mutant)
		}
		if got, want := strings.Count(mutant, "mu.Lock()"), strings.Count(mutant, "mu.Unlock()"); got != want {
			t.Errorf("%s: %d Lock vs %d Unlock — unbalanced\n%s", m.desc, got, want, mutant)
		}
		if got, want := strings.Count(mutant, "mu.RLock()"), strings.Count(mutant, "mu.RUnlock()"); got != want {
			t.Errorf("%s: %d RLock vs %d RUnlock — unbalanced\n%s", m.desc, got, want, mutant)
		}
		if strings.Count(mutant, "wg.Add(1)") != 1 || strings.Count(mutant, "wg.Done()") != 1 {
			t.Errorf("%s: Add/Done scaffolding altered\n%s", m.desc, mutant)
		}
	}
}

// revert must restore the AST exactly, or later mutants in the same harvest
// would stack edits and the descriptions would stop matching the programs. The
// swap operator is the one at risk, since it mutates a slice rather than a field.
func TestScarFreeRaceSitesRevertCleanly(t *testing.T) {
	sites, fset, f, err := collectScarFreeRaceSites(scarFreeSrc)
	if err != nil {
		t.Fatal(err)
	}
	print := func() string {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, f); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	before := print()
	for _, m := range sites {
		m.apply()
		if during := print(); during == before {
			t.Errorf("%s: apply() changed nothing", m.desc)
		}
		m.revert()
		if after := print(); after != before {
			t.Errorf("%s: revert() did not restore the source\nwant:\n%s\ngot:\n%s", m.desc, before, after)
		}
	}
}

// A comment inside the statements being reordered is itself a tell (it would end
// up describing the wrong line), so the escape operator must decline that site
// rather than emit a mutant with a misplaced comment.
func TestScarFreeEscapeSkipsCommentedCriticalSection(t *testing.T) {
	const src = `package solution

import "sync"

func F(xs []int) int {
	var mu sync.Mutex
	total, count := 0, 0
	var wg sync.WaitGroup
	for _, x := range xs {
		wg.Add(1)
		go func(x int) {
			defer wg.Done()
			mu.Lock()
			count++
			// accumulate under the lock
			total += x
			mu.Unlock()
		}(x)
	}
	wg.Wait()
	_ = count
	return total
}`
	sites, _, _, err := collectScarFreeRaceSites(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sites {
		if strings.Contains(s.desc, "out of the") {
			t.Errorf("emitted a reordering site across a comment: %s", s.desc)
		}
	}
	// The downgrade operator does not move anything, so it must still be offered.
	found := false
	for _, s := range sites {
		if strings.Contains(s.desc, "RLock/RUnlock") {
			found = true
		}
	}
	if !found {
		t.Error("the comment veto should be limited to the reordering operator")
	}
}

// A solution with no eligible construct must yield no sites — reported as
// coverage by the caller, never as a judge result.
func TestScarFreeRaceSitesEmptyOnSequentialCode(t *testing.T) {
	sites, _, _, err := collectScarFreeRaceSites(goodSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Errorf("sequential code yielded %d scar-free race sites, want 0", len(sites))
	}
}

// Go 1.22 made loop variables per-iteration, so the loop-variable-capture
// operator proposed in results/race-seeded-2026-07-25.md:58-61 no longer races.
// This pins the fact the operator set relies on: if a future toolchain change
// made this race again, the operator would be worth adding, and this test is
// where that would surface.
func TestLoopVarCaptureDoesNotRaceOnThisToolchain(t *testing.T) {
	if testing.Short() {
		t.Skip("executes candidates under -race")
	}
	const src = `package solution

import "sync"

// SumParts adds xs concurrently, capturing the loop variable directly.
func SumParts(xs []int) int {
	parts := make([]int, len(xs))
	var wg sync.WaitGroup
	for i := range xs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parts[i] = xs[i]
		}()
	}
	wg.Wait()
	total := 0
	for _, p := range parts {
		total += p
	}
	return total
}`
	const visible = `package solution

import "testing"

func TestV_Sum(t *testing.T) {
	xs := make([]int, 256)
	want := 0
	for i := range xs {
		xs[i] = i
		want += i
	}
	if got := SumParts(xs); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}`
	r := newTestRunner(t)
	ctx := context.Background()
	ws, err := r.NewWorkspace(src, visible, "")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Remove() //nolint:errcheck // scratch dir
	res := ws.run(ctx, 90*time.Second, true, "test", "-race", "-count=3", "./...")
	if res.Err != nil || res.TimedOut {
		t.Errorf("loop-variable capture now races under -race; the operator set should be revisited: %v\n%s",
			res.Err, res.Output)
	}
}

// End to end: the scar-free operators must actually yield mutants that compile
// and are refuted by the race detector. Without this the operator set could be
// syntactically valid and behaviourally inert, which is exactly how the
// loop-variable-capture proposal would have failed silently.
func TestScarFreeRaceKilledMutantsAreRaceRefuted(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates under -race")
	}
	r := newTestRunner(t)
	ctx := context.Background()

	muts, err := ScarFreeRaceKilledMutants(ctx, r, scarFreeSrc, scarFreeVisible, "", 3, 3, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(muts) == 0 {
		t.Fatal("no scar-free race mutant survived the compile+race filter; the operators are inert")
	}
	for _, m := range muts {
		ws, err := r.NewWorkspace(m.Source, scarFreeVisible, "")
		if err != nil {
			t.Fatal(err)
		}
		if b := ws.run(ctx, 60*time.Second, false, "build", "./..."); b.Err != nil {
			t.Errorf("mutant %q does not compile", m.Desc)
		}
		race := ws.run(ctx, 90*time.Second, true, "test", "-race", "-count=3", "./...")
		if race.Err == nil && !race.TimedOut {
			t.Errorf("mutant %q passed under -race; only race-refuted candidates may be returned", m.Desc)
		}
		_ = ws.Remove()
	}
}

// hangingSrc compiles and typechecks but never returns, so the test stage can
// only end on the clock.
const hangingSrc = `package solution

// LongestIncreasingRun never returns.
func LongestIncreasingRun(xs []int) int {
	for {
		_ = xs
	}
}`

// TestTimeoutIsARefutationAndIsVisible pins BOTH halves of issue #63 at once,
// because they are in tension and a change that fixed one by breaking the other
// would otherwise look like a pass.
//
// The verdict must stay OK==false: a program that does not finish is not a program
// that works, and a timeout-is-inconclusive rung would make a ladder stage
// probabilistic (invariant #4). The TimedOut flag must ALSO be set, because a
// timeout is the one refutation whose cause can be external to the candidate, so a
// run cannot be audited afterwards unless the clock-caused ones are marked.
func TestTimeoutIsARefutationAndIsVisible(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}
	r := newTestRunner(t)
	ws, err := r.NewWorkspace(hangingSrc, visibleSrc, hiddenSrc)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Remove() //nolint:errcheck // scratch dir

	// Short enough to fire quickly, long enough that a loaded machine does not
	// reach it during `go build`/`go vet` (which use their own 60s bound anyway).
	o := opts()
	o.TestTimeout = 2 * time.Second

	rep := NewLadder().Run(context.Background(), ws, hangingSrc, o)
	if rep.OK {
		t.Fatal("a candidate that never returns was not refuted; a timeout must not be scored as a pass (invariant #4)")
	}
	if rep.FailedAt != StageTest {
		t.Errorf("refuted at %s, want %s: the hang is in the test stage", rep.FailedAt, StageTest)
	}
	if !rep.TimedOut() {
		t.Error("Report.TimedOut() is false on a stage killed by the clock; the refutation is correct but unauditable (issue #63)")
	}

	var sr *StageResult
	for i := range rep.Stages {
		if rep.Stages[i].Stage == StageTest {
			sr = &rep.Stages[i]
		}
	}
	if sr == nil {
		t.Fatal("no test stage in the report")
	}
	if sr.OK {
		t.Error("the timed-out stage reports OK; TimedOut must record the cause, never rescore the verdict")
	}
	if !sr.TimedOut {
		t.Error("StageResult.TimedOut is false on the stage that was killed")
	}
}

// TestTimedOutIsFalseWithoutATimeout is the null half: the flag must distinguish a
// clock-caused refutation from an ordinary one, so an ordinary refutation must not
// set it. Without this, "0 timeouts" in a summary would carry no information.
func TestTimedOutIsFalseWithoutATimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and executes candidates")
	}
	r := newTestRunner(t)
	l := NewLadder()

	rep, ws := run(t, r, l, goodSrc)
	defer ws.Remove() //nolint:errcheck // scratch dir
	if !rep.OK {
		t.Fatalf("the correct solution was refuted at %s: %s", rep.FailedAt, firstLine(rep.Diagnostic))
	}
	if rep.TimedOut() {
		t.Error("TimedOut() is true on a clean run")
	}

	// A genuinely wrong candidate: refuted on its output, not on the clock.
	const wrong = `package solution

func LongestIncreasingRun(xs []int) int { return 0 }`
	bad, bws := run(t, r, l, wrong)
	defer bws.Remove() //nolint:errcheck // scratch dir
	if bad.OK {
		t.Fatal("a wrong candidate survived the visible tests")
	}
	if bad.TimedOut() {
		t.Error("TimedOut() is true on an ordinary refutation; the flag would then mark every failure and distinguish nothing")
	}

	// And a nil report is not evidence of a timeout: recording sites hold one when
	// the ladder never ran.
	var nilRep *Report
	if nilRep.TimedOut() {
		t.Error("(*Report)(nil).TimedOut() is true")
	}
}

func TestParseAllocs(t *testing.T) {
	out := "BenchmarkX-8   1000   1234 ns/op   56 B/op   3 allocs/op\n" +
		"BenchmarkY-8   1000   99 ns/op   0 B/op   7 allocs/op"
	got, ok := parseAllocs(out)
	if !ok || got != 7 {
		t.Errorf("parseAllocs = %d, %v; want the worst case 7", got, ok)
	}
	if _, ok := parseAllocs("no benchmarks here"); ok {
		t.Error("parseAllocs found allocations in output that has none")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
