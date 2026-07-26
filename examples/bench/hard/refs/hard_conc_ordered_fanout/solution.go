// Package solution provides a bounded-parallelism map that preserves input
// order and exposes a concurrently-readable progress counter.
package solution

import (
	"sync"
	"sync/atomic"
)

// Result carries the outcome of processing a single input element together
// with the Index of that element in the original input slice.
type Result[T any] struct {
	Index int
	Value T
}

// FanOut applies fn to every element of input using at most workers concurrent
// goroutines. It returns a slice of Results in input order: Results[i].Index
// == i and Results[i].Value == fn(i, input[i]). Exactly one Result is produced
// per input element.
//
// The progress function, if non-nil, is invoked to report a live counter of
// completed elements. It is safe to read the counter concurrently from other
// goroutines while FanOut runs; when FanOut returns, the counter equals
// len(input). progress receives a pointer to the atomic counter once, before
// any work begins, so callers can observe intermediate progress.
//
// Ordering is achieved by writing each result into its fixed slot in a
// preallocated slice keyed by index, so no two goroutines ever touch the same
// slot and results are inherently in order without post-sorting.
func FanOut[T any](input []T, workers int, fn func(index int, in T) T, progress func(counter *atomic.Int64)) []Result[T] {
	results := make([]Result[T], len(input))
	var completed atomic.Int64

	if progress != nil {
		progress(&completed)
	}

	if len(input) == 0 {
		return results
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(input) {
		workers = len(input)
	}

	// Indices are dispatched over a channel; each worker owns whichever
	// indices it pulls, and writes only to results[idx] — a slot no other
	// worker will touch. This makes the writes data-race-free without a lock.
	indices := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indices {
				results[idx] = Result[T]{Index: idx, Value: fn(idx, input[idx])}
				completed.Add(1)
			}
		}()
	}

	for i := range input {
		indices <- i
	}
	close(indices)
	wg.Wait()

	return results
}
