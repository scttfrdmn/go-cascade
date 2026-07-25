package cache

import (
	"testing"
)

func TestCanonicalHashIgnoresRenamingAndComments(t *testing.T) {
	a := `package solution

// LongestIncreasingRun does a thing.
func LongestIncreasingRun(xs []int) int {
	best := 1
	for i := 1; i < len(xs); i++ {
		if xs[i] > xs[i-1] {
			best++
		}
	}
	return best
}`
	// Same program, different local names, different comments and spacing.
	b := `package solution

// Completely different doc comment.
func LongestIncreasingRun( items []int ) int {
	longest := 1
	for k := 1; k < len(items); k++ {
		// inline note
		if items[k] > items[k-1] {
			longest++
		}
	}
	return longest
}`
	ha, err := CanonicalHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := CanonicalHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Errorf("alpha-equivalent programs hashed differently:\n  %s\n  %s", ha, hb)
	}

	// A real semantic change must not collide.
	c := `package solution

func LongestIncreasingRun(xs []int) int {
	best := 1
	for i := 1; i < len(xs); i++ {
		if xs[i] >= xs[i-1] {
			best++
		}
	}
	return best
}`
	hc, err := CanonicalHash(c)
	if err != nil {
		t.Fatal(err)
	}
	if hc == ha {
		t.Error("changing > to >= did not change the canonical hash")
	}
}

// Selector fields must not be renamed, or canonicalization would corrupt the
// program it is summarising.
func TestCanonicalHashPreservesSelectors(t *testing.T) {
	src := `package solution

import "sync"

func F() int {
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Done()
	wg.Wait()
	return 0
}`
	if _, err := CanonicalHash(src); err != nil {
		t.Fatal(err)
	}
	other := `package solution

import "sync"

func F() int {
	var group sync.WaitGroup
	group.Add(1)
	group.Done()
	group.Wait()
	return 0
}`
	h1, _ := CanonicalHash(src)
	h2, _ := CanonicalHash(other)
	if h1 != h2 {
		t.Error("renaming a sync.WaitGroup variable changed the canonical hash")
	}
}

func TestSimilarity(t *testing.T) {
	a := "write a function that returns the longest increasing run in a slice of integers"
	b := "Write a function that returns the LONGEST INCREASING RUN in a slice of integers."
	c := "implement a concurrent worker pool that maps a function over a channel"
	if s := Similarity(a, b); s < 0.85 {
		t.Errorf("near-identical statements scored %.2f", s)
	}
	if s := Similarity(a, c); s > 0.35 {
		t.Errorf("unrelated statements scored %.2f", s)
	}
	if s := Similarity("", ""); s != 0 {
		t.Errorf("empty similarity = %v, want 0", s)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	problem := "return the longest strictly increasing contiguous run"
	sol := "package solution\n\nfunc F() int { return 1 }"
	if err := c.PutSolution(Entry{
		Problem: problem, ProblemHash: ProblemHash(problem),
		Solution: sol, Tier: "small", Score: 1.0,
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := c.Retrieve(problem, 5, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Similarity != 1.0 {
		t.Fatalf("exact match not retrieved at similarity 1: %+v", hits)
	}
	near, err := c.Retrieve("return the longest strictly increasing run", 5, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if len(near) != 1 {
		t.Errorf("paraphrase retrieved %d entries, want 1", len(near))
	}
	far, err := c.Retrieve("parse a TOML configuration file into a struct", 5, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if len(far) != 0 {
		t.Errorf("unrelated problem retrieved %d entries", len(far))
	}

	spec := Spec{ProblemHash: ProblemHash(problem), API: "api", VisibleTests: "v", HiddenTests: "h"}
	if err := c.PutSpec(spec); err != nil {
		t.Fatal(err)
	}
	if got, ok := c.GetSpec(ProblemHash(problem)); !ok || got.API != "api" {
		t.Error("spec cache round trip failed")
	}

	ph := ProblemHash(problem)
	if err := c.AddFailure(ph, Failure{CanonHash: "abc", Stage: "V1:types", Summary: "undefined"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddFailure(ph, Failure{CanonHash: "abc", Stage: "V1:types", Summary: "dup"}); err != nil {
		t.Fatal(err)
	}
	if fs := c.Failures(ph); len(fs) != 1 {
		t.Errorf("failure cache holds %d entries, want 1 after a duplicate", len(fs))
	}

	if s := c.Stats(); s.Solutions != 1 || s.Specs != 1 || s.Failures != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
}

func TestDisabledCacheIsInert(t *testing.T) {
	c, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Disabled() {
		t.Fatal("empty dir should disable the cache")
	}
	if err := c.PutSolution(Entry{Solution: "package solution"}); err != nil {
		t.Errorf("writes to a disabled cache should be no-ops, got %v", err)
	}
	hits, err := c.Retrieve("anything", 5, 0)
	if err != nil || len(hits) != 0 {
		t.Errorf("disabled cache returned %d hits, err %v", len(hits), err)
	}
}
