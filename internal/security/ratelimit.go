package security

import (
	"sync"
	"time"
)

// TokenBucket is a simple token-bucket rate limiter. It guards resolve-intent
// against target enumeration / policy probing (draft-cui §12.2) and bounds
// self-reported telemetry (brief §5.5), limiting flooding DoS (§12.8).
type TokenBucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	rate     float64 // tokens per second
	last     time.Time
	now      func() time.Time
}

// NewTokenBucket creates a bucket that starts full, holding `capacity` tokens
// and refilling at `ratePerSec`.
func NewTokenBucket(capacity, ratePerSec float64) *TokenBucket {
	// `last` is left zero and initialised on first use, so injecting a custom
	// clock (b.now) after construction works correctly in tests.
	return &TokenBucket{capacity: capacity, tokens: capacity, rate: ratePerSec, now: time.Now}
}

// Allow consumes one token, returning false when the bucket is empty.
func (b *TokenBucket) Allow() bool { return b.AllowN(1) }

// AllowN consumes n tokens if available.
func (b *TokenBucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if b.last.IsZero() {
		b.last = now
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
	if b.tokens >= n {
		b.tokens -= n
		return true
	}
	return false
}

// KeyedLimiter applies a per-key token bucket, e.g. per requester or per agent.
type KeyedLimiter struct {
	mu       sync.Mutex
	capacity float64
	rate     float64
	buckets  map[string]*TokenBucket
	now      func() time.Time
}

// NewKeyedLimiter builds a per-key limiter.
func NewKeyedLimiter(capacity, ratePerSec float64) *KeyedLimiter {
	return &KeyedLimiter{capacity: capacity, rate: ratePerSec, buckets: map[string]*TokenBucket{}, now: time.Now}
}

// Allow consumes a token for key.
func (k *KeyedLimiter) Allow(key string) bool {
	k.mu.Lock()
	b := k.buckets[key]
	if b == nil {
		b = NewTokenBucket(k.capacity, k.rate)
		b.now = k.now
		k.buckets[key] = b
	}
	k.mu.Unlock()
	return b.Allow()
}
