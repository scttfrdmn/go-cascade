package model

import (
	"context"
	"fmt"
	"strings"
)

// Mock is a deterministic Provider used for tests and for exercising the whole
// cascade without Bedrock credentials. It injects the defect classes the
// verifier ladder is designed to catch, with competence increasing by tier:
//
//	mock-small  hallucinated stdlib API | visible-test failure | data race |
//	            a subtle defect that passes the visible tests and fails hidden
//	mock-mid    occasionally the subtle defect
//	mock-large  correct
//
// Repair succeeds only where the diagnostic actually localises the defect,
// which is what makes repair-depth exhaustion and escalation observable.
type Mock struct{}

// Name implements Provider.
func (Mock) Name() string { return "mock" }

// Mock model IDs.
const (
	MockSmall = "mock-small"
	MockMid   = "mock-mid"
	MockLarge = "mock-large"
	// MockOracle writes tests only. Keeping the oracle author distinct from
	// every code tier is what stops the contamination guard rejecting the
	// whole calibration set.
	MockOracle = "mock-oracle"
)

// Generate implements Provider.
func (m Mock) Generate(_ context.Context, req Request) (*Response, error) {
	last := ""
	if n := len(req.Messages); n > 0 {
		last = req.Messages[n-1].Text
	}
	p := pickProblem(last)

	var text string
	switch req.Purpose {
	case PurposeSpec:
		text = p.spec
	case PurposeRepair:
		text = fence(m.repair(p, last))
	case PurposeJudge:
		text = m.judge(p, last)
	case PurposePlan:
		// A stipulated plan; the mock exists to exercise the two-stage code path,
		// not to model planning quality. Never cite this as planner behaviour.
		text = "Plan: implement the API directly. Handle empty/nil input and the " +
			"stated boundary cases; standard library only."
	default:
		// Fold the problem into the seed so that a benchmark of many problems
		// produces a varied defect distribution rather than one repeated
		// outcome, which would make calibration degenerate.
		text = fence(m.code(p, req.ModelID, req.Seed+problemNonce(last)))
	}
	return &Response{
		Text: text,
		Usage: Usage{
			InputTokens:  len(last) / 4,
			OutputTokens: len(text) / 4,
		},
	}, nil
}

func (Mock) code(p *mockProblem, modelID string, seed int) string {
	switch modelID {
	case MockLarge:
		return p.correct
	case MockMid:
		if seed%3 == 0 {
			return p.subtle
		}
		return p.correct
	default: // MockSmall and anything else
		switch seed % 4 {
		case 0:
			return p.correct
		case 1:
			return p.badAPI
		case 2:
			return p.badLogic
		default:
			return p.subtle
		}
	}
}

// repair models a real asymmetry: a compiler diagnostic names the defect, so
// the fix lands; a failing assertion is weaker evidence and the small model
// often reproduces its own mistake.
func (Mock) repair(p *mockProblem, prompt string) string {
	switch {
	case strings.Contains(prompt, p.badAPIMarker):
		return p.correct
	case strings.Contains(prompt, p.badLogicMarker):
		return p.badLogic // cannot see it; burns the repair budget, then escalates
	default:
		return p.correct
	}
}

// judge models an LLM code reviewer ruling on a candidate by reading alone. It
// exists to make the judge-oracle arm's failure mode observable and
// deterministic, so it must reproduce the one asymmetry that matters:
//
//   - Gross defects are visible on the page. A hallucinated API or an off-by-one
//     that a doc comment contradicts gets a FAIL.
//   - A *subtle* semantic defect -- here the non-strict `>=` that satisfies the
//     stated behaviour on every ordinary input and only diverges on the held-out
//     boundary case -- reads as correct. The judge PASSes it.
//
// That second case is eta_fa > 0: a wrong program the oracle accepts. Execution
// against the hidden partition catches it; a reader does not. The whole point of
// paper §3.1 is that a judge cannot certify below the rate at which this
// happens, and this method is what lets the mock demonstrate it rather than
// merely assert it.
//
// The prompt (JudgeUser) carries the candidate source, so the mock recognises
// each stipulated defect by the same marker the repair path uses.
func (Mock) judge(p *mockProblem, prompt string) string {
	verdict := func(pass bool) string {
		if pass {
			return "VERDICT: PASS"
		}
		return "VERDICT: FAIL"
	}
	switch {
	case strings.Contains(prompt, p.badAPIMarker):
		return verdict(false) // invented API: a careful reader catches it
	case strings.Contains(prompt, p.badLogicMarker):
		return verdict(false) // the doc comment contradicts the code: caught
	case strings.Contains(prompt, p.subtleMarker):
		return verdict(true) // reads as correct; only execution refutes it
	default:
		return verdict(true) // the correct solution
	}
}

func fence(code string) string { return "```go\n" + code + "\n```" }

type mockProblem struct {
	spec           string
	correct        string
	badAPI         string
	badAPIMarker   string
	badLogic       string
	badLogicMarker string
	subtle         string
	subtleMarker   string // a substring unique to the subtle variant's source
}

func pickProblem(text string) *mockProblem {
	l := strings.ToLower(text)
	for _, kw := range []string{"parallel", "concurrent", "worker", "goroutine"} {
		if strings.Contains(l, kw) {
			return &parallelMap
		}
	}
	return &longestRun
}

func specBlocks(api, visible, hidden string) string {
	return fmt.Sprintf("### API\n```go\n%s\n```\n\n### VISIBLE TESTS\n```go\n%s\n```\n\n### HIDDEN TESTS\n```go\n%s\n```\n",
		api, visible, hidden)
}

var longestRun = mockProblem{
	spec: specBlocks(
		`package solution

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs. It returns 0 for an empty slice.
func LongestIncreasingRun(xs []int) int { panic("not implemented") }`,
		`package solution

import "testing"

func TestV_Empty(t *testing.T) {
	if got := LongestIncreasingRun(nil); got != 0 {
		t.Fatalf("nil: got %d, want 0", got)
	}
}

func TestV_Single(t *testing.T) {
	if got := LongestIncreasingRun([]int{7}); got != 1 {
		t.Fatalf("single: got %d, want 1", got)
	}
}

func TestV_Ascending(t *testing.T) {
	if got := LongestIncreasingRun([]int{1, 2, 3}); got != 3 {
		t.Fatalf("ascending: got %d, want 3", got)
	}
}

func TestV_Descending(t *testing.T) {
	if got := LongestIncreasingRun([]int{3, 2, 1}); got != 1 {
		t.Fatalf("descending: got %d, want 1", got)
	}
}`,
		`package solution

import "testing"

func TestH_Plateau(t *testing.T) {
	// Equal neighbours break the run: strictly increasing, not non-decreasing.
	if got := LongestIncreasingRun([]int{1, 2, 2, 3}); got != 2 {
		t.Fatalf("plateau: got %d, want 2", got)
	}
}

func TestH_RunAfterReset(t *testing.T) {
	if got := LongestIncreasingRun([]int{5, 1, 2, 3, 0}); got != 3 {
		t.Fatalf("reset: got %d, want 3", got)
	}
}

func TestH_AllEqual(t *testing.T) {
	if got := LongestIncreasingRun([]int{4, 4, 4, 4}); got != 1 {
		t.Fatalf("all equal: got %d, want 1", got)
	}
}`),

	correct: `package solution

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs. It returns 0 for an empty slice.
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
}`,

	// Hallucinated stdlib API: refuted in-process by go/types at V1, never
	// reaching the compiler or the test runner.
	badAPI: `package solution

import "slices"

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs.
func LongestIncreasingRun(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	return slices.MaxRunFunc(xs, func(a, b int) bool { return a < b })
}`,
	badAPIMarker: "slices.MaxRunFunc",

	// Off-by-one on the seed value: fails a visible test, so the repair loop
	// gets a precise assertion to work from.
	badLogic: `package solution

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs.
func LongestIncreasingRun(xs []int) int {
	best, cur := 0, 1
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
}`,
	badLogicMarker: "best, cur := 0, 1",

	// Non-strict comparison: passes every visible test, fails the held-out
	// plateau case. This is the candidate the hidden partition exists for.
	subtle: `package solution

// LongestIncreasingRun returns the length of the longest strictly increasing
// contiguous run in xs.
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
}`,
	subtleMarker: "xs[i] >= xs[i-1]",
}

var parallelMap = mockProblem{
	spec: specBlocks(
		`package solution

// ParallelMap applies f to every element of xs using at most workers
// goroutines and returns the results in input order. A workers value of zero
// or less means one worker per element.
func ParallelMap[T, U any](xs []T, workers int, f func(T) U) []U {
	panic("not implemented")
}`,
		`package solution

import "testing"

func TestV_Order(t *testing.T) {
	xs := []int{1, 2, 3, 4, 5}
	got := ParallelMap(xs, 2, func(v int) int { return v * v })
	want := []int{1, 4, 9, 16, 25}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestV_Empty(t *testing.T) {
	if got := ParallelMap(nil, 4, func(v int) int { return v }); len(got) != 0 {
		t.Fatalf("empty: got len %d, want 0", len(got))
	}
}`,
		`package solution

import (
	"sync/atomic"
	"testing"
)

func TestH_ZeroWorkers(t *testing.T) {
	got := ParallelMap([]int{1, 2, 3}, 0, func(v int) int { return v + 1 })
	want := []int{2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestH_AppliedExactlyOnce(t *testing.T) {
	var calls atomic.Int64
	xs := make([]int, 512)
	for i := range xs {
		xs[i] = i
	}
	got := ParallelMap(xs, 8, func(v int) int {
		calls.Add(1)
		return v * 2
	})
	if n := calls.Load(); n != int64(len(xs)) {
		t.Fatalf("f called %d times, want %d", n, len(xs))
	}
	for i := range xs {
		if got[i] != i*2 {
			t.Fatalf("index %d: got %d, want %d", i, got[i], i*2)
		}
	}
}`),

	correct: `package solution

import "sync"

// ParallelMap applies f to every element of xs using at most workers
// goroutines and returns the results in input order.
func ParallelMap[T, U any](xs []T, workers int, f func(T) U) []U {
	out := make([]U, len(xs))
	if len(xs) == 0 {
		return out
	}
	if workers <= 0 || workers > len(xs) {
		workers = len(xs)
	}
	idx := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range idx {
				out[i] = f(xs[i])
			}
		}()
	}
	for i := range xs {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return out
}`,

	badAPI: `package solution

import "sync"

// ParallelMap applies f to every element of xs using at most workers goroutines.
func ParallelMap[T, U any](xs []T, workers int, f func(T) U) []U {
	out := make([]U, len(xs))
	var wg sync.WaitGroup
	wg.AddDelta(len(xs))
	for i := range xs {
		go func(i int) {
			defer wg.Done()
			out[i] = f(xs[i])
		}(i)
	}
	wg.Wait()
	return out
}`,
	badAPIMarker: "wg.AddDelta",

	badLogic: `package solution

import "sync"

// ParallelMap applies f to every element of xs using at most workers goroutines.
func ParallelMap[T, U any](xs []T, workers int, f func(T) U) []U {
	out := make([]U, len(xs))
	if len(xs) == 0 {
		return out
	}
	if workers <= 0 {
		workers = 1
	}
	idx := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range idx {
				out[i] = f(xs[i])
			}
		}()
	}
	// Off by one: the final element is never enqueued.
	for i := 0; i < len(xs)-1; i++ {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return out
}`,
	badLogicMarker: "i < len(xs)-1",

	// Unsynchronised shared counter. Every functional test can pass; only the
	// race detector refutes it.
	subtle: `package solution

import "sync"

// ParallelMap applies f to every element of xs using at most workers goroutines.
func ParallelMap[T, U any](xs []T, workers int, f func(T) U) []U {
	out := make([]U, len(xs))
	if len(xs) == 0 {
		return out
	}
	if workers <= 0 || workers > len(xs) {
		workers = len(xs)
	}
	next := 0
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for {
				i := next
				if i >= len(xs) {
					return
				}
				next++
				out[i] = f(xs[i])
			}
		}()
	}
	wg.Wait()
	return out
}`,
	subtleMarker: "i := next",
}

// problemNonce derives a small stable integer from the problem statement.
func problemNonce(s string) int {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h % 97
}
