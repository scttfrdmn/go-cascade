package verify

import (
	"context"
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
