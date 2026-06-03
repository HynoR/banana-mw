package server

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"hynor/banana-mw/internal/admin"
	"hynor/banana-mw/internal/cache"
	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/middleware"
	"hynor/banana-mw/internal/redisstore"
	"hynor/banana-mw/internal/requtil"
	"hynor/banana-mw/internal/secure"
	"hynor/banana-mw/internal/stats"
)

// Run starts the HTTP service until ctx is cancelled.
func Run(ctx context.Context, buildVersion string) error {
	admin.SetBuildVersion(buildVersion)

	cfg, err := config.LoadDefault()
	if err != nil {
		return err
	}

	logLevel, err := config.ParseLogLevel(cfg.LogLevel)
	if err != nil {
		return err
	}
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.String("time", a.Value.Time().Format("2006-01-02T15:04:05"))
				}
				return a
			},
		}),
	)
	slog.SetDefault(logger)
	middleware.SetDebugRequests(cfg.DebugLog())
	if cfg.DebugLog() {
		slog.Debug("debug request logging enabled")
	}

	upstreamURL, err := url.Parse(cfg.Upstream)
	if err != nil {
		return err
	}
	proxy := newReverseProxy(upstreamURL)

	cache200 := cache.NewMemory()
	cache200.StartGC(3 * cfg.CacheTTL200Duration())
	cache4xx := cache.NewMemory4xx()
	cache4xx.StartGC(3 * cfg.CacheTTL4xxDuration())

	requtil.ConfigureTrustProxy(cfg.TrustProxy)
	secure.Configure(cfg)
	if cfg.Secure == 1 || cfg.StatsEnabled == 1 || cfg.IsAdmin == 1 {
		redisstore.Init(cfg)
	}
	if cfg.Secure == 1 {
		secure.StartDelayDeleter(ctx, time.Minute, 0)
		if err := redisstore.Ping(ctx, cfg.RedisReadTimeoutDuration()); err != nil {
			slog.Warn("secure redis ping failed at startup", "error", err.Error())
		}
	}
	if cfg.StatsEnabled == 1 || cfg.IsAdmin == 1 {
		stats.StartWorkers(ctx, cfg)
		stats.StartCleaner(ctx, cfg)
	}

	// Reverse proxy and admin panel are served by separate HTTP servers on
	// separate ports, so the public proxy port never exposes the admin routes.
	servers := []namedServer{
		{name: "proxy", server: newHTTPServer(cfg, ":"+cfg.Port, ProxyRouter(cfg, cache200, cache4xx, proxy))},
	}
	if cfg.IsAdmin == 1 {
		servers = append(servers, namedServer{
			name:   "admin",
			server: newHTTPServer(cfg, ":"+cfg.AdminPort, AdminRouter(cfg)),
		})
	}

	return runHTTPServers(ctx, cfg, servers...)
}

func newReverseProxy(upstreamURL *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("proxy error", "method", r.Method, "path", r.URL.Path, "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
	}
	return proxy
}

// MustRun loads config and runs until shutdown; exits the process on fatal errors.
func MustRun(ctx context.Context, buildVersion string) {
	if err := Run(ctx, buildVersion); err != nil {
		log.Fatal(err)
	}
}
