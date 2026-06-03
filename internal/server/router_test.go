package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"hynor/banana-mw/internal/cache"
	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/stats"
)

func TestAdminRoutesNotServedByProxy(t *testing.T) {
	upstream, err := url.Parse("https://upstream.example")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Test(config.TestOptions{AllowedPrefixes: []string{"/api/v1"}})
	router := ProxyRouter(cfg, cache.NewMemory(), cache.NewMemory4xx(), httputil.NewSingleHostReverseProxy(upstream))

	// Admin routes live on a separate server/port now. On the proxy router the
	// admin path is no longer special-cased: it flows through the proxy guards
	// and is rejected by the path-prefix guard instead of serving the panel.
	req := httptest.NewRequest(http.MethodGet, "/_gogoadmin/", nil)
	req.Header.Set("User-Agent", "unit-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected admin path blocked by proxy guards, got %d", resp.Code)
	}
	if ct := resp.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
		t.Fatalf("admin HTML must not be served from the proxy port, got content-type %q", ct)
	}
}

func TestProxyPathStillUsesMiddlewareAndUpstream(t *testing.T) {
	var upstreamHits int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&upstreamHits, 1)
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("payload"))
	}))
	t.Cleanup(upstreamServer.Close)

	upstream, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Test(config.TestOptions{AllowedPrefixes: []string{"/api/v1"}})
	router := ProxyRouter(cfg, cache.NewMemory(), cache.NewMemory4xx(), httputil.NewSingleHostReverseProxy(upstream))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sub?token=abcdefghijklmnop", nil)
	req.Header.Set("User-Agent", "unit-test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "payload" {
		t.Fatalf("unexpected body: %q", got)
	}
	if got := atomic.LoadInt64(&upstreamHits); got != 1 {
		t.Fatalf("expected upstream hit once, got %d", got)
	}
}

func TestProxyGuardsRemainInOrder(t *testing.T) {
	upstream, err := url.Parse("https://upstream.example")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Test(config.TestOptions{AllowedPrefixes: []string{"/api/v1"}})
	router := ProxyRouter(cfg, cache.NewMemory(), cache.NewMemory4xx(), httputil.NewSingleHostReverseProxy(upstream))

	tests := []struct {
		name   string
		method string
		path   string
		ua     string
		want   int
	}{
		{name: "method", method: http.MethodPut, path: "/api/v1/sub?token=abcdefghijklmnop", ua: "unit-test", want: http.StatusMethodNotAllowed},
		{name: "ua", method: http.MethodGet, path: "/api/v1/sub?token=abcdefghijklmnop", ua: "", want: http.StatusForbidden},
		{name: "prefix", method: http.MethodGet, path: "/blocked?token=abcdefghijklmnop", ua: "unit-test", want: http.StatusForbidden},
		{name: "token", method: http.MethodGet, path: "/api/v1/sub?token=short", ua: "unit-test", want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.ua != "" {
				req.Header.Set("User-Agent", tt.ua)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, resp.Code)
			}
		})
	}
}

func TestCache200StillKeysByTokenPathMethodUA(t *testing.T) {
	var upstreamHits int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := atomic.AddInt64(&upstreamHits, 1)
		_, _ = fmt.Fprintf(w, "payload-%d", hit)
	}))
	t.Cleanup(upstreamServer.Close)

	upstream, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Test(config.TestOptions{AllowedPrefixes: []string{"/api/v1"}})
	router := ProxyRouter(cfg, cache.NewMemory(), cache.NewMemory4xx(), httputil.NewSingleHostReverseProxy(upstream))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sub?token=abcdefghijklmnop&nonce=ignored", nil)
		req.Header.Set("User-Agent", "unit-test")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i, resp.Code)
		}
		if !strings.Contains(resp.Body.String(), "payload-1") {
			t.Fatalf("request %d did not use cached first payload: %q", i, resp.Body.String())
		}
	}
	if got := atomic.LoadInt64(&upstreamHits); got != 1 {
		t.Fatalf("expected one upstream hit due to cache, got %d", got)
	}
}

func TestCacheIncludeQueryChangesCacheKey(t *testing.T) {
	var upstreamHits int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := atomic.AddInt64(&upstreamHits, 1)
		_, _ = fmt.Fprintf(w, "payload-%d", hit)
	}))
	t.Cleanup(upstreamServer.Close)

	upstream, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Test(config.TestOptions{
		AllowedPrefixes:   []string{"/api/v1"},
		CacheIncludeQuery: true,
	})
	router := ProxyRouter(cfg, cache.NewMemory(), cache.NewMemory4xx(), httputil.NewSingleHostReverseProxy(upstream))

	for _, path := range []string{
		"/api/v1/sub?token=abcdefghijklmnop&variant=a",
		"/api/v1/sub?token=abcdefghijklmnop&variant=b",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", "unit-test")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.Code)
		}
	}

	if got := atomic.LoadInt64(&upstreamHits); got != 2 {
		t.Fatalf("expected two upstream hits with cache_include_query, got %d", got)
	}
}

func TestAdminStatsRequiresToken(t *testing.T) {
	cfg := config.Test(config.TestOptions{AdminToken: "secret", StatsEnabled: 0})
	router := AdminRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/_gogoadmin/api/stats", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/_gogoadmin/api/stats", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected stats disabled, got %d", resp.Code)
	}
}

func TestAdminStatsDisabledWhenNoAdminToken(t *testing.T) {
	cfg := config.Test(config.TestOptions{StatsEnabled: 1})
	router := AdminRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/_gogoadmin/api/stats", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected admin disabled, got %d", resp.Code)
	}
}

func TestNormalizePathDropsQuery(t *testing.T) {
	got, ok := stats.NormalizePath("/api/v1", "/api/v1/sub?token=abc")
	if !ok {
		t.Fatal("expected path to match prefix")
	}
	if got != "/sub" {
		t.Fatalf("unexpected normalized path: %s", got)
	}

	got, ok = stats.NormalizePath("*", "/api/v1/sub?token=abc")
	if !ok {
		t.Fatal("expected wildcard to match")
	}
	if got != "api/v1/sub" {
		t.Fatalf("unexpected wildcard path: %s", got)
	}
}

func TestAdminHealthRequiresToken(t *testing.T) {
	cfg := config.Test(config.TestOptions{AdminToken: "secret"})
	router := AdminRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/_gogoadmin/api/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/_gogoadmin/api/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.Code)
	}
	if cc := resp.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected no-store on health response, got %q", cc)
	}
}

func TestAdminPageSetsSecurityHeaders(t *testing.T) {
	cfg := config.Test(config.TestOptions{AdminToken: "secret"})
	router := AdminRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/_gogoadmin/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected admin page 200, got %d", resp.Code)
	}
	if xfo := resp.Header().Get("X-Frame-Options"); xfo != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", xfo)
	}
	if csp := resp.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("expected CSP with frame-ancestors none, got %q", csp)
	}
	if cc := resp.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected no-store on admin page, got %q", cc)
	}
}
