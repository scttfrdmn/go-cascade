package solution

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestVBasicConsume(t *testing.T) {
	r := NewRateLimiter(10)
	if !r.Allow(4) {
		t.Fatal("expected first Allow(4) to succeed")
	}
	if !r.Allow(6) {
		t.Fatal("expected Allow(6) to succeed, bucket had 6 left")
	}
	if r.Allow(1) {
		t.Fatal("expected Allow(1) to fail, bucket empty")
	}
	if got := r.Available(); got != 0 {
		t.Fatalf("Available = %d, want 0", got)
	}
}

func TestVRefillSaturates(t *testing.T) {
	r := NewRateLimiter(5)
	if !r.Allow(5) {
		t.Fatal("expected Allow(5) to succeed")
	}
	r.Refill(100) // way over capacity
	if got := r.Available(); got != 5 {
		t.Fatalf("Available after saturating refill = %d, want 5", got)
	}
}

func TestVNonPositive(t *testing.T) {
	r := NewRateLimiter(3)
	if !r.Allow(0) {
		t.Fatal("Allow(0) should be true")
	}
	if !r.Allow(-1) {
		t.Fatal("Allow(-1) should be true")
	}
	r.Refill(-5) // no-op
	if got := r.Available(); got != 3 {
		t.Fatalf("Available = %d, want 3", got)
	}
}

// TestHNeverOverGrant hammers Allow(1) from many goroutines against a bucket
// that is never refilled. The total number of successful grants must equal
// exactly the capacity — never more. A check-then-act race would let two
// goroutines both observe tokens>=1 for the last token and both succeed,
// pushing the granted total over capacity.
func TestHNeverOverGrant(t *testing.T) {
	const capacity = 1000
	const goroutines = 200

	r := NewRateLimiter(capacity)
	var granted int64
	var wg sync.WaitGroup

	// Each goroutine tries far more than its fair share of the tokens so
	// there is heavy contention on the final tokens.
	perG := capacity / goroutines * 4
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perG {
				if r.Allow(1) {
					atomic.AddInt64(&granted, 1)
				}
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt64(&granted); got != capacity {
		t.Fatalf("granted = %d, want exactly %d (over-grant indicates a race)", got, capacity)
	}
	if av := r.Available(); av != 0 {
		t.Fatalf("Available = %d, want 0", av)
	}
}

// TestHRefillBoundedGrant runs producers (Refill) and consumers (Allow)
// concurrently and asserts that total granted never exceeds initial fill plus
// total refilled. This is the fundamental supply bound for a token bucket.
func TestHRefillBoundedGrant(t *testing.T) {
	const capacity = 500
	const consumers = 64
	const refillRounds = 2000

	r := NewRateLimiter(capacity)
	var granted int64
	var refilled int64
	var wg sync.WaitGroup

	// One producer that refills 1 token at a time, many rounds.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range refillRounds {
			r.Refill(1)
			atomic.AddInt64(&refilled, 1)
		}
	}()

	stop := make(chan struct{})
	for range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					// Drain a bit more so we don't leave supply on the table
					// unaccounted; then exit.
					for range 100 {
						if r.Allow(1) {
							atomic.AddInt64(&granted, 1)
						}
					}
					return
				default:
					if r.Allow(1) {
						atomic.AddInt64(&granted, 1)
					}
				}
			}
		}()
	}

	// Wait until the producer is done, then signal consumers to stop.
	// We can't wg.Wait yet since consumers loop forever until stop.
	for atomic.LoadInt64(&refilled) < refillRounds {
	}
	close(stop)
	wg.Wait()

	maxSupply := int64(capacity) + atomic.LoadInt64(&refilled)
	if g := atomic.LoadInt64(&granted); g > maxSupply {
		t.Fatalf("granted = %d exceeds max supply %d (over-grant race)", g, maxSupply)
	}
}
