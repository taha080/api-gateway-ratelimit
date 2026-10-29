package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRoutesAndStrips(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" {
			t.Error("api key leaked upstream")
		}
		io.WriteString(w, "up:"+r.URL.Path)
	}))
	defer up.Close()
	rt, err := New([]Route{{Prefix: "/svc/", Upstream: up.URL, StripPrefix: true}})
	if err != nil {
		t.Fatal(err)
	}
	gw := httptest.NewServer(rt)
	defer gw.Close()
	req, _ := http.NewRequest("GET", gw.URL+"/svc/hello", nil)
	req.Header.Set("X-API-Key", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "up:/hello" {
		t.Fatalf("got %q", b)
	}
}

func TestNoRoute(t *testing.T) {
	rt, _ := New(nil)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestBadUpstream(t *testing.T) {
	if _, err := New([]Route{{Prefix: "/", Upstream: "not a url"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpstreamDown(t *testing.T) {
	rt, _ := New([]Route{{Prefix: "/", Upstream: "http://127.0.0.1:1"}})
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 502 {
		t.Fatalf("code %d", rec.Code)
	}
}
