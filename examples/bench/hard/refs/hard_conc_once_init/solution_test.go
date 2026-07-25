package solution

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestVComputesAndCaches(t *testing.T) {
	c := NewLazyCache()
	var calls int32
	compute := func() any {
		atomic.AddInt32(&calls, 1)
		return 42
	}
	if got := c.Get("k", compute); got != 42 {
		t.Fatalf("Get = %v, want 42", got)
	}
	if got := c.Get("k", compute); got != 42 {
		t.Fatalf("second Get = %v, want 42", got)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("compute ran %d times, want 1", n)
	}
}

func TestVDistinctKeys(t *testing.T) {
	c := NewLazyCache()
	a := c.Get("a", func() any { return "va" })
	b := c.Get("b", func() any { return "vb" })
	if a != "va" || b != "vb" {
		t.Fatalf("got a=%v b=%v, want va/vb", a, b)
	}
}

// TestHExactlyOncePerKey launches a large number of goroutines that all race
// to Get the same set of keys. For each key we keep an atomic counter that the
// compute closure increments; after the storm every counter must read exactly
// 1. A check-then-compute race (map lookup, then compute if absent, without a
// per-key Once) would let multiple goroutines compute the same key, driving a
// counter above 1. All callers must also observe the identical value.
func TestHExactlyOncePerKey(t *testing.T) {
	const keys = 50
	const goroutinesPerKey = 100

	c := NewLazyCache()
	counters := make([]int32, keys)

	// Expected value for key i is i*7+1; observers assert they see it.
	var wg sync.WaitGroup
	var mismatches int32

	for k := range keys {
		key := fmt.Sprintf("key-%d", k)
		want := k*7 + 1
		for range goroutinesPerKey {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v := c.Get(key, func() any {
					atomic.AddInt32(&counters[k], 1)
					return want
				})
				if v.(int) != want {
					atomic.AddInt32(&mismatches, 1)
				}
			}()
		}
	}
	wg.Wait()

	if m := atomic.LoadInt32(&mismatches); m != 0 {
		t.Fatalf("%d callers observed a wrong value", m)
	}
	for k := range keys {
		if got := atomic.LoadInt32(&counters[k]); got != 1 {
			t.Fatalf("key %d: compute ran %d times, want exactly 1 (run-once violation)", k, got)
		}
	}
}

// TestHConcurrentDistinctKeys stresses the map itself: many goroutines writing
// many distinct keys concurrently. This surfaces unsynchronized map access
// (fatal "concurrent map writes") and confirms every compute ran once.
func TestHConcurrentDistinctKeys(t *testing.T) {
	const keys = 5000

	c := NewLazyCache()
	var calls int64
	var wg sync.WaitGroup
	for k := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("d-%d", k)
			c.Get(key, func() any {
				atomic.AddInt64(&calls, 1)
				return k
			})
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt64(&calls); got != keys {
		t.Fatalf("total computes = %d, want %d", got, keys)
	}
}
