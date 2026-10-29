// Package proxy routes requests by path prefix to upstream services.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
)

// Route maps a path prefix to an upstream URL.
type Route struct {
	Prefix      string `yaml:"prefix"`
	Upstream    string `yaml:"upstream"`
	StripPrefix bool   `yaml:"strip_prefix"`
}

type compiled struct {
	Route
	rp *httputil.ReverseProxy
}

// Router is an http.Handler that picks the longest matching prefix.
type Router struct{ routes []compiled }

// New builds a router from routes.
func New(routes []Route) (*Router, error) {
	r := &Router{}
	for _, rt := range routes {
		u, err := url.Parse(rt.Upstream)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid upstream %q for prefix %q", rt.Upstream, rt.Prefix)
		}
		rt := rt
		rp := httputil.NewSingleHostReverseProxy(u)
		orig := rp.Director
		rp.Director = func(req *http.Request) {
			if rt.StripPrefix {
				p := strings.TrimPrefix(req.URL.Path, strings.TrimSuffix(rt.Prefix, "/"))
				if !strings.HasPrefix(p, "/") {
					p = "/" + p
				}
				req.URL.Path = p
				req.URL.RawPath = ""
			}
			orig(req)
			req.Host = u.Host
			req.Header.Del("X-API-Key")
			req.Header.Del("Authorization")
		}
		rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		}
		r.routes = append(r.routes, compiled{rt, rp})
	}
	sort.Slice(r.routes, func(i, j int) bool { return len(r.routes[i].Prefix) > len(r.routes[j].Prefix) })
	return r, nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range r.routes {
		if strings.HasPrefix(req.URL.Path, rt.Prefix) {
			rt.rp.ServeHTTP(w, req)
			return
		}
	}
	http.Error(w, `{"error":"no route"}`, http.StatusNotFound)
}
