// Package limits implements the MVP rate-limit and build-concurrency gates
// (RB§30 R10). Limits are enforced per token/IP with an elevated admin tier —
// never disabled outright.
package limits

import (
	"fmt"
	"sync"
	"time"
)

const (
	// TokenReqPerMinute is the per-token API budget.
	TokenReqPerMinute = 100
	// LoginIPPerMinute is the per-IP budget for auth endpoints.
	LoginIPPerMinute = 10
	// AdminReqPerMinute keeps admins throttled at a high ceiling instead of
	// disabling limiting entirely.
	AdminReqPerMinute = 600
	// BuildConcurrencyFree caps concurrent builds per org on the free tier.
	BuildConcurrencyFree = 1
	// BuildRetryAfter is the hint returned when the build gate rejects.
	BuildRetryAfter = 30 * time.Second
)

// TokenBucket refills continuously up to its capacity.
type TokenBucket struct {
	lastRefill time.Time
	tokens     float64
	capacity   float64
	perMinute  float64
}

func newTokenBucket(capacity, perMinute float64, now time.Time) *TokenBucket {
	return &TokenBucket{tokens: capacity, capacity: capacity, perMinute: perMinute, lastRefill: now}
}

func (b *TokenBucket) allow(now time.Time) (bool, time.Duration) {
	elapsed := now.Sub(b.lastRefill).Minutes()
	if elapsed > 0 {
		b.tokens += elapsed * b.perMinute
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.lastRefill = now
	}
	if b.tokens < 1 {
		wait := time.Duration((1 - b.tokens) / b.perMinute * float64(time.Minute))
		return false, wait
	}
	b.tokens -= 1
	return true, 0
}

// RateLimiter tracks one bucket per key (token or IP).
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*TokenBucket
	capacity  float64
	perMinute float64
}

func NewRateLimiter(capacity, perMinute float64) *RateLimiter {
	return &RateLimiter{buckets: map[string]*TokenBucket{}, capacity: capacity, perMinute: perMinute}
}

// Allow consumes one unit for the key at the given instant.
func (r *RateLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket, ok := r.buckets[key]
	if !ok {
		bucket = newTokenBucket(r.capacity, r.perMinute, now)
		r.buckets[key] = bucket
	}
	return bucket.allow(now)
}

// BuildGate caps concurrent builds per org (R10 free tier: one at a time).
type BuildGate struct {
	inFlight map[string]int
	limit    int
	mu       sync.Mutex
}

func NewBuildGate(limit int) *BuildGate {
	return &BuildGate{inFlight: map[string]int{}, limit: limit}
}

// TryAcquire admits the build or reports how long until a slot frees up.
func (g *BuildGate) TryAcquire(orgID string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inFlight[orgID] >= g.limit {
		return false, BuildRetryAfter
	}
	g.inFlight[orgID]++
	return true, 0
}

// Release frees the slot after the build settles.
func (g *BuildGate) Release(orgID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inFlight[orgID] > 0 {
		g.inFlight[orgID]--
	}
}

// ConflictError renders as HTTP 409/429 semantics upstream.
func ConflictError(action string) error {
	return fmt.Errorf("%s conflicts with an in-flight operation", action)
}
