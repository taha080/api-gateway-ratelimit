// Package stats keeps in-memory usage counters and renders Prometheus metrics.
package stats

import (
	"fmt"
	"io"
	"sort"
	"sync"
)

// KeyStats are counters for one API key.
type KeyStats struct {
	Allowed int64 `json:"allowed"`
	Limited int64 `json:"limited"`
}

// Store is a concurrency-safe in-memory stats store.
type Store struct {
	mu           sync.Mutex
	keys         map[string]*KeyStats
	Unauthorized int64
}

func New() *Store { return &Store{keys: map[string]*KeyStats{}} }

func (s *Store) get(k string) *KeyStats {
	ks, ok := s.keys[k]
	if !ok {
		ks = &KeyStats{}
		s.keys[k] = ks
	}
	return ks
}

func (s *Store) Allowed(k string) { s.mu.Lock(); s.get(k).Allowed++; s.mu.Unlock() }
func (s *Store) Limited(k string) { s.mu.Lock(); s.get(k).Limited++; s.mu.Unlock() }
func (s *Store) Unauth()          { s.mu.Lock(); s.Unauthorized++; s.mu.Unlock() }

// Snapshot returns a copy of all counters.
func (s *Store) Snapshot() map[string]KeyStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]KeyStats, len(s.keys))
	for k, v := range s.keys {
		out[k] = *v
	}
	return out
}

// WritePrometheus writes metrics in the Prometheus text exposition format.
func (s *Store) WritePrometheus(w io.Writer) {
	snap := s.Snapshot()
	names := make([]string, 0, len(snap))
	for k := range snap {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Fprintln(w, "# HELP gateway_requests_total Requests handled by the gateway.")
	fmt.Fprintln(w, "# TYPE gateway_requests_total counter")
	for _, k := range names {
		fmt.Fprintf(w, "gateway_requests_total{key=%q,result=\"allowed\"} %d\n", k, snap[k].Allowed)
		fmt.Fprintf(w, "gateway_requests_total{key=%q,result=\"rate_limited\"} %d\n", k, snap[k].Limited)
	}
	s.mu.Lock()
	u := s.Unauthorized
	s.mu.Unlock()
	fmt.Fprintln(w, "# HELP gateway_unauthorized_total Requests rejected for a missing or invalid API key.")
	fmt.Fprintln(w, "# TYPE gateway_unauthorized_total counter")
	fmt.Fprintf(w, "gateway_unauthorized_total %d\n", u)
}
