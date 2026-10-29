package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/taha080/api-gateway-ratelimit/internal/auth"
	"github.com/taha080/api-gateway-ratelimit/internal/proxy"
)

func TestGatewayEndToEnd(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "pong") }))
	defer up.Close()
	fixed := time.Unix(0, 0)
	h, err := NewHandler(&Config{
		Clients: []auth.Client{{Name: "demo", Key: "k", Rate: 1, Burst: 2}},
		Routes:  []proxy.Route{{Prefix: "/", Upstream: up.URL}},
	}, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	do := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/ping", nil)
		if key != "" {
			r.Header.Set("X-API-Key", key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	if c := do("").Code; c != 401 {
		t.Fatalf("no key: %d", c)
	}
	for i := 0; i < 2; i++ {
		if rec := do("k"); rec.Code != 200 || rec.Body.String() != "pong" {
			t.Fatalf("req %d: %d %q", i, rec.Code, rec.Body.String())
		}
	}
	rec := do("k")
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	m := httptest.NewRecorder()
	h.ServeHTTP(m, httptest.NewRequest("GET", "/metrics", nil))
	body := m.Body.String()
	for _, want := range []string{`key="demo",result="allowed"} 2`, `result="rate_limited"} 1`, "gateway_unauthorized_total 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q:\n%s", want, body)
		}
	}
}
