// Package solution provides a bounded-concurrency parallel sum.
package solution

import "sync"

// ParallelSum returns the sum of nums using up to workers goroutines, each
// adding a disjoint partition of the slice concurrently.
//
// If workers <= 0, one worker per element is used. The result equals the
// sequential sum for any partitioning. An empty slice sums to 0. Each
// goroutine accumulates into its own local variable and writes to a
// dedicated slot, so the computation is data-race-free.
func ParallelSum(nums []int, workers int) int {
	n := len(nums)
	if n == 0 {
		return 0
	}
	if workers <= 0 || workers > n {
		workers = n
	}

	partials := make([]int, workers)
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for w := range workers {
		start := w * chunk
		if start >= n {
			break
		}
		end := min(start+chunk, n)
		wg.Add(1)
		go func(w, start, end int) {
			defer wg.Done()
			local := 0
			for i := start; i < end; i++ {
				local += nums[i]
			}
			partials[w] = local
		}(w, start, end)
	}
	wg.Wait()

	total := 0
	for _, p := range partials {
		total += p
	}
	return total
}
