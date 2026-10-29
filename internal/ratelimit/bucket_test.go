package ratelimit

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestBurstThenDeny(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	b := NewBucket(1, 3, c.now)
	for i := 0; i < 3; i++ {
		if ok, _, _ := b.Allow(); !ok {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	ok, _, wait := b.Allow()
	if ok {
		t.Fatal("4th request should be denied")
	}
	if wait != time.Second {
		t.Fatalf("wait = %v, want 1s", wait)
	}
}

func TestRefill(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	b := NewBucket(2, 2, c.now)
	b.Allow()
	b.Allow()
	if ok, _, _ := b.Allow(); ok {
		t.Fatal("should be empty")
	}
	c.t = c.t.Add(500 * time.Millisecond)
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("should refill 1 token after 0.5s at 2/s")
	}
}

func TestRefillCapsAtBurst(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	b := NewBucket(10, 2, c.now)
	c.t = c.t.Add(time.Hour)
	n := 0
	for i := 0; i < 10; i++ {
		if ok, _, _ := b.Allow(); ok {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("allowed %d, want 2", n)
	}
}

func TestLimiterPerKey(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.now)
	l.Allow("a", 1, 1)
	if ok, _, _ := l.Allow("a", 1, 1); ok {
		t.Fatal("key a should be limited")
	}
	if ok, _, _ := l.Allow("b", 1, 1); !ok {
		t.Fatal("key b has its own bucket")
	}
}
