// Package ratelimit implements a thread-safe token bucket limiter keyed by API key.
package ratelimit

import (
	"sync"
	"time"
)

// Bucket is a classic token bucket: it refills at Rate tokens/second up to Burst.
type Bucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

// NewBucket creates a full bucket.
func NewBucket(rate float64, burst int, now func() time.Time) *Bucket {
	if now == nil {
		now = time.Now
	}
	return &Bucket{rate: rate, burst: float64(burst), tokens: float64(burst), last: now(), now: now}
}

// Allow consumes one token if available. It returns whether the request is allowed,
// the remaining whole tokens, and how long until the next token is available.
func (b *Bucket) Allow() (bool, int, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	elapsed := t.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = t
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, int(b.tokens), 0
	}
	var wait time.Duration
	if b.rate > 0 {
		wait = time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
	}
	return false, 0, wait
}

// Limiter holds one bucket per key.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*Bucket
	now     func() time.Time
}

// NewLimiter creates an empty limiter. now may be nil.
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{buckets: map[string]*Bucket{}, now: now}
}

// Allow checks the bucket for key, creating it with rate/burst on first use.
func (l *Limiter) Allow(key string, rate float64, burst int) (bool, int, time.Duration) {
	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = NewBucket(rate, burst, l.now)
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.Allow()
}
