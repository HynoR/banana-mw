package server

import (
	"net/http"
	"net/http/httputil"
	"strings"

	"hynor/banana-mw/internal/admin"
	"hynor/banana-mw/internal/cache"
	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/middleware"
	"hynor/banana-mw/internal/secure"
	"hynor/banana-mw/internal/stats"

	"github.com/go-chi/chi/v5"
)

func ProxyRouter(cfg *config.Config, cache200 *cache.Memory, cache4xx *cache.Memory4xx, proxy *httputil.ReverseProxy) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recovery)

	proxyChain := chi.NewRouter()
	proxyChain.Use(middleware.RequestTrace(cfg))
	proxyChain.Use(middleware.MethodGuard)
	proxyChain.Use(middleware.UserAgentGuard)
	proxyChain.Use(middleware.PathPrefixGuard(normalizeAllowedPrefixes(cfg.AllowedPrefixes)))
	proxyChain.Use(middleware.TokenGuard)

	if cfg.Secure == 1 {
		proxyChain.Use(secure.Middleware(cfg))
	}
	if cfg.StatsEnabled == 1 {
		proxyChain.Use(stats.Middleware(cfg.StatsPrefix))
	}

	proxyChain.Use(cache.Middleware4xx(cache4xx, cfg.CacheIncludeQuery))
	proxyChain.Use(cache.Middleware200(
		cache200, cache4xx, proxy,
		cfg.CacheTTL200Duration(), cfg.CacheTTL4xxDuration(), cfg.CacheIncludeQuery, cfg.CacheMaxBodyBytes,
	))
	proxyChain.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	r.NotFound(proxyChain.ServeHTTP)
	r.MethodNotAllowed(proxyChain.ServeHTTP)
	return r
}

func AdminRouter(cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recovery)
	admin.RegisterRoutes(r, cfg)
	return r
}

func normalizeAllowedPrefixes(prefixes []string) []string {
	normalized := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		normalized = append(normalized, p)
	}
	return normalized
}
