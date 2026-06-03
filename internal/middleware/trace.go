package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/requtil"
)

type traceKey struct{}

type requestTrace struct {
	start          time.Time
	status         int
	blockedBy      string
	secure         string
	cache4xxHit    bool
	cache200Hit    bool
	responseSource string
	proxyPanic     bool
}

var debugRequests bool

// SetDebugRequests enables per-request debug tracing (from config log_level=debug).
func SetDebugRequests(enabled bool) {
	debugRequests = enabled
}

// DebugRequests reports whether request tracing is active.
func DebugRequests() bool {
	return debugRequests
}

func traceFrom(ctx context.Context) *requestTrace {
	t, _ := ctx.Value(traceKey{}).(*requestTrace)
	return t
}

// MarkBlocked records an early guard rejection.
func MarkBlocked(ctx context.Context, by string, status int) {
	if t := traceFrom(ctx); t != nil {
		t.blockedBy = by
		t.status = status
		t.responseSource = "blocked"
	}
}

// MarkSecure records secure middleware outcome (pass, not_found, ...).
func MarkSecure(ctx context.Context, status string) {
	if t := traceFrom(ctx); t != nil {
		t.secure = status
	}
}

// MarkCache4xxHit records a 4xx cache short-circuit.
func MarkCache4xxHit(ctx context.Context) {
	if t := traceFrom(ctx); t != nil {
		t.cache4xxHit = true
		t.blockedBy = "cache_4xx"
		t.status = http.StatusForbidden
		t.responseSource = "cache_4xx"
	}
}

// MarkCache200Hit records a 200 cache hit.
func MarkCache200Hit(ctx context.Context, status int) {
	if t := traceFrom(ctx); t != nil {
		t.cache200Hit = true
		t.status = status
		t.responseSource = "cache_200"
	}
}

// MarkUpstream records an upstream response after cache miss.
func MarkUpstream(ctx context.Context, status int) {
	if t := traceFrom(ctx); t != nil {
		t.status = status
		t.responseSource = "upstream"
	}
}

// MarkProxyPanic records proxy handler panic/abort.
func MarkProxyPanic(ctx context.Context) {
	if t := traceFrom(ctx); t != nil {
		t.proxyPanic = true
		t.responseSource = "upstream_aborted"
	}
}

// RequestTrace wraps the proxy chain and emits one debug log per request when enabled.
func RequestTrace(cfg *config.Config) func(http.Handler) http.Handler {
	secureEnabled := cfg.Secure == 1
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !debugRequests {
				next.ServeHTTP(w, r)
				return
			}

			trace := &requestTrace{
				start:  time.Now(),
				secure: "disabled",
			}
			if secureEnabled {
				trace.secure = "pending"
			}
			ctx := context.WithValue(r.Context(), traceKey{}, trace)
			tw := &traceResponseWriter{ResponseWriter: w, trace: trace}

			next.ServeHTTP(tw, r.WithContext(ctx))
			logRequestTrace(r, trace)
		})
	}
}

type traceResponseWriter struct {
	http.ResponseWriter
	trace *requestTrace
}

func (w *traceResponseWriter) WriteHeader(statusCode int) {
	if w.trace.status == 0 {
		w.trace.status = statusCode
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *traceResponseWriter) Write(b []byte) (int, error) {
	if w.trace.status == 0 {
		w.trace.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func logRequestTrace(r *http.Request, t *requestTrace) {
	status := t.status
	if status == 0 {
		status = http.StatusOK
	}
	secure := t.secure
	if secure == "pending" {
		if t.blockedBy != "" {
			secure = "not_reached"
		} else {
			secure = "pass"
		}
	}
	attrs := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"latency", time.Since(t.start).String(),
		"client_ip", requtil.ClientIP(r),
	}
	attrs = append(attrs, requtil.TokenLogAttrs(requtil.RequestToken(r))...)
	attrs = append(attrs,
		"user_agent", r.UserAgent(),
		"secure", secure,
		"cache_4xx_hit", t.cache4xxHit,
		"cache_200_hit", t.cache200Hit,
		"response_source", t.responseSource,
	)
	if t.blockedBy != "" {
		attrs = append(attrs, "blocked_by", t.blockedBy)
	}
	if t.proxyPanic {
		attrs = append(attrs, "proxy_panic", true)
	}
	slog.Debug("request", attrs...)
}
