package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func handler(t *testing.T) http.Handler {
	clients := map[string]Client{"k1": {Name: "alice", Key: "k1"}}
	return Middleware(clients, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := FromContext(r.Context())
		if !ok || c.Name != "alice" {
			t.Error("client not in context")
		}
		w.WriteHeader(204)
	}))
}

func TestMissingKey(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestWrongKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "nope")
	rec := httptest.NewRecorder()
	handler(t).ServeHTTP(rec, r)
	if rec.Code != 401 {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestHeaderAndBearer(t *testing.T) {
	for _, set := range []func(*http.Request){
		func(r *http.Request) { r.Header.Set("X-API-Key", "k1") },
		func(r *http.Request) { r.Header.Set("Authorization", "Bearer k1") },
	} {
		r := httptest.NewRequest("GET", "/", nil)
		set(r)
		rec := httptest.NewRecorder()
		handler(t).ServeHTTP(rec, r)
		if rec.Code != 204 {
			t.Fatalf("code %d", rec.Code)
		}
	}
}
