package solution

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestVOrderAndValues(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	out := FanOut(in, 3, func(i, v int) int { return v * 10 }, nil)
	if len(out) != len(in) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(in))
	}
	for i := range out {
		if out[i].Index != i {
			t.Fatalf("out[%d].Index = %d, want %d", i, out[i].Index, i)
		}
		if out[i].Value != in[i]*10 {
			t.Fatalf("out[%d].Value = %d, want %d", i, out[i].Value, in[i]*10)
		}
	}
}

func TestVEmpty(t *testing.T) {
	out := FanOut([]int{}, 4, func(i, v int) int { return v }, nil)
	if len(out) != 0 {
		t.Fatalf("len(out) = %d, want 0", len(out))
	}
}

func TestVProgressReachesLen(t *testing.T) {
	in := make([]int, 100)
	var ctr *atomic.Int64
	out := FanOut(in, 8, func(i, v int) int { return i }, func(c *atomic.Int64) {
		ctr = c
	})
	if ctr == nil {
		t.Fatal("progress callback never invoked")
	}
	if got := ctr.Load(); got != int64(len(in)) {
		t.Fatalf("final counter = %d, want %d", got, len(in))
	}
	_ = out
}

// TestHOrderExactlyOnceProgress runs a large workload under heavy parallelism
// and asserts three invariants that a racy implementation violates:
//   - every result is in input order (Index == position, Value derived from
//     the correct input);
//   - exactly one result per input (no duplicates, no drops);
//   - the progress counter equals len(input) at the end.
//
// A racy counter (plain int++ instead of atomic) trips the race detector; an
// out-of-order or duplicated write (e.g. appending results from workers) trips
// the order/coverage checks.
func TestHOrderExactlyOnceProgress(t *testing.T) {
	const n = 20000
	in := make([]int, n)
	for i := range in {
		in[i] = i
	}

	var ctr *atomic.Int64
	// Reader goroutine hammers the counter concurrently to catch unsynchronized
	// reads under -race.
	stop := make(chan struct{})
	var reader sync.WaitGroup

	out := FanOut(in, 64, func(i, v int) int {
		if v != i {
			t.Errorf("fn saw input[%d] = %d, mismatch", i, v)
		}
		return v * 3
	}, func(c *atomic.Int64) {
		ctr = c
		reader.Add(1)
		go func() {
			defer reader.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.Load()
				}
			}
		}()
	})
	close(stop)
	reader.Wait()

	if got := ctr.Load(); got != int64(n) {
		t.Fatalf("final counter = %d, want %d", got, n)
	}
	if len(out) != n {
		t.Fatalf("len(out) = %d, want %d", len(out), n)
	}

	seen := make([]bool, n)
	for pos := range out {
		r := out[pos]
		if r.Index != pos {
			t.Fatalf("out[%d].Index = %d, want %d (out of order)", pos, r.Index, pos)
		}
		if r.Value != pos*3 {
			t.Fatalf("out[%d].Value = %d, want %d", pos, r.Value, pos*3)
		}
		if seen[r.Index] {
			t.Fatalf("duplicate result for index %d", r.Index)
		}
		seen[r.Index] = true
	}
	for i, s := range seen {
		if !s {
			t.Fatalf("missing result for index %d", i)
		}
	}
}

// TestHBoundedWorkers verifies that no more than `workers` invocations of fn
// run concurrently, using an atomic in-flight gauge. Exceeding the bound means
// the concurrency limit is not enforced.
func TestHBoundedWorkers(t *testing.T) {
	const n = 5000
	const workers = 16
	in := make([]int, n)

	var inFlight atomic.Int64
	var maxSeen atomic.Int64

	FanOut(in, workers, func(i, v int) int {
		cur := inFlight.Add(1)
		for {
			m := maxSeen.Load()
			if cur <= m || maxSeen.CompareAndSwap(m, cur) {
				break
			}
		}
		inFlight.Add(-1)
		return v
	}, nil)

	if m := maxSeen.Load(); m > workers {
		t.Fatalf("max concurrent workers = %d, want <= %d", m, workers)
	}
}
