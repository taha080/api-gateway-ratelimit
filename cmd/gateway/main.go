// Command gateway is a single-binary API gateway with API-key auth,
// per-key token-bucket rate limiting, reverse proxying and metrics.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/taha080/api-gateway-ratelimit/internal/auth"
	"github.com/taha080/api-gateway-ratelimit/internal/proxy"
	"github.com/taha080/api-gateway-ratelimit/internal/ratelimit"
	"github.com/taha080/api-gateway-ratelimit/internal/stats"
	"gopkg.in/yaml.v3"
)

// Config is the YAML configuration file.
type Config struct {
	Listen  string        `yaml:"listen"`
	Clients []auth.Client `yaml:"clients"`
	Routes  []proxy.Route `yaml:"routes"`
}

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(b))), &c); err != nil {
		return nil, err
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if p := os.Getenv("PORT"); p != "" { // Render and similar platforms
		c.Listen = ":" + p
	}
	for i := range c.Clients {
		if c.Clients[i].Rate <= 0 {
			c.Clients[i].Rate = 5
		}
		if c.Clients[i].Burst <= 0 {
			c.Clients[i].Burst = 10
		}
	}
	return &c, nil
}

// NewHandler wires the whole gateway together.
func NewHandler(cfg *Config, now func() time.Time) (http.Handler, error) {
	router, err := proxy.New(cfg.Routes)
	if err != nil {
		return nil, err
	}
	clients := map[string]auth.Client{}
	for _, c := range cfg.Clients {
		clients[c.Key] = c
	}
	st := stats.New()
	lim := ratelimit.NewLimiter(now)

	limited := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := auth.FromContext(r.Context())
		ok, remaining, wait := lim.Allow(c.Key, c.Rate, c.Burst)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(c.Burst))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			st.Limited(c.Name)
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		st.Allowed(c.Name)
		router.ServeHTTP(w, r)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		st.WritePrometheus(w)
	})
	mux.Handle("/stats", auth.Middleware(clients, st.Unauth, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"clients": st.Snapshot(), "unauthorized": st.Unauthorized})
	})))
	mux.Handle("/", auth.Middleware(clients, st.Unauth, limited))
	return mux, nil
}

func main() {
	path := flag.String("config", "config.yaml", "path to YAML config")
	flag.Parse()
	cfg, err := loadConfig(*path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	h, err := NewHandler(cfg, nil)
	if err != nil {
		log.Fatalf("setup: %v", err)
	}
	log.Printf("api-gateway listening on %s (%d clients, %d routes)", cfg.Listen, len(cfg.Clients), len(cfg.Routes))
	srv := &http.Server{Addr: cfg.Listen, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
