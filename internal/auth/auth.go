// Package auth provides API-key authentication middleware.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type ctxKey struct{}

// Client describes an API key and its rate limit.
type Client struct {
	Name  string  `yaml:"name"`
	Key   string  `yaml:"key"`
	Rate  float64 `yaml:"rate"`
	Burst int     `yaml:"burst"`
}

// FromContext returns the authenticated client.
func FromContext(ctx context.Context) (Client, bool) {
	c, ok := ctx.Value(ctxKey{}).(Client)
	return c, ok
}

// ExtractKey reads the key from X-API-Key or "Authorization: Bearer <key>".
func ExtractKey(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	return ""
}

// Middleware rejects requests without a known key with 401.
func Middleware(clients map[string]Client, onUnauth func(), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := clients[ExtractKey(r)]
		if !ok {
			if onUnauth != nil {
				onUnauth()
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("WWW-Authenticate", `Bearer realm="api-gateway"`)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid API key"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
	})
}
