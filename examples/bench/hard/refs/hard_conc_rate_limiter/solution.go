// Package solution implements a concurrency-safe token-bucket rate limiter.
package solution

import "sync"

// RateLimiter is a token-bucket rate limiter. A bucket holds up to Capacity
// tokens. Allow(n) atomically consumes n tokens when at least n are available,
// and Refill(n) adds up to n tokens without exceeding the capacity.
//
// All methods are safe for concurrent use by multiple goroutines. The bucket
// guarantees that the total number of tokens granted across all successful
// Allow calls never exceeds the initial fill plus everything added by Refill,
// because consumption and the availability check happen under a single lock
// (no check-then-act race).
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int64
	capacity int64
}

// NewRateLimiter returns a rate limiter with the given capacity, initially
// full (tokens == capacity). Capacity is clamped to be non-negative.
func NewRateLimiter(capacity int64) *RateLimiter {
	if capacity < 0 {
		capacity = 0
	}
	return &RateLimiter{tokens: capacity, capacity: capacity}
}

// Allow attempts to consume n tokens. It returns true and consumes exactly n
// tokens if at least n are currently available; otherwise it consumes nothing
// and returns false. A non-positive n is treated as always allowed and
// consumes nothing.
func (r *RateLimiter) Allow(n int64) bool {
	if n <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens >= n {
		r.tokens -= n
		return true
	}
	return false
}

// Refill adds n tokens to the bucket, saturating at the capacity. A
// non-positive n is a no-op.
func (r *RateLimiter) Refill(n int64) {
	if n <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens += n
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}
}

// Available reports the number of tokens currently in the bucket.
func (r *RateLimiter) Available() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens
}
