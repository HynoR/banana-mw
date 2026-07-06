package secure

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/middleware"
	"hynor/banana-mw/internal/requtil"
)

var activeTokenPattern = regexp.MustCompile(`^[a-z0-9]{32}$`)

func ActiveHandler(ttl time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := requtil.QueryParam(r, "t")
		session := requtil.QueryParam(r, "s")

		if !activeTokenPattern.MatchString(token) {
			attrs := append(requtil.TokenLogAttrs(token), "client_ip", requtil.ClientIP(r))
			slog.Warn("invalid token format in /active", attrs...)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := setToken(ctx, token, session, ttl); err != nil {
			errAttrs := append([]any{"error", err.Error()}, requtil.TokenLogAttrs(token)...)
			slog.Error("failed to set secure token", errAttrs...)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		attrs := append(requtil.TokenLogAttrs(token), "session_empty", session == "", "ttl", ttl.String())
		slog.Info("secure token activated", attrs...)
		w.WriteHeader(http.StatusOK)
	}
}

func Middleware(cfg *config.Config) func(http.Handler) http.Handler {
	failOpen := cfg.SecureFailOpen()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := requtil.RequestToken(r)
			if token == "" || !requtil.ValidProxyToken(token) {
				middleware.MarkSecure(r.Context(), "forbidden_token")
				middleware.MarkBlocked(r.Context(), "secure", http.StatusForbidden)
				w.WriteHeader(http.StatusForbidden)
				return
			}

			reqSession := requtil.QueryParam(r, "s")

			ctx, cancel := context.WithTimeout(r.Context(), cfg.RedisReadTimeoutDuration())
			defer cancel()
			session, lookup := getSession(ctx, token)
			switch lookup {
			case SessionRedisError:
				if failOpen {
					middleware.MarkSecure(r.Context(), "redis_error_open")
					next.ServeHTTP(w, r)
					return
				}
				middleware.MarkSecure(r.Context(), "redis_error")
				middleware.MarkBlocked(r.Context(), "secure", http.StatusUnauthorized)
				attrs := []any{"method", r.Method, "path", r.URL.Path, "client_ip", requtil.ClientIP(r)}
				attrs = append(attrs, requtil.TokenLogAttrs(token)...)
				slog.Warn("secure redis unavailable (fail closed)", attrs...)
				w.WriteHeader(http.StatusUnauthorized)
				return
			case SessionMiss:
				middleware.MarkSecure(r.Context(), "not_found")
				middleware.MarkBlocked(r.Context(), "secure", http.StatusUnauthorized)
				attrs := []any{"method", r.Method, "path", r.URL.Path, "client_ip", requtil.ClientIP(r)}
				attrs = append(attrs, requtil.TokenLogAttrs(token)...)
				//slog.Info("secure token not found", attrs...)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if session == "" || session == "1" {
				middleware.MarkSecure(r.Context(), "pass")
				next.ServeHTTP(w, r)
				return
			}

			if reqSession == "" || reqSession != session {
				middleware.MarkSecure(r.Context(), "session_mismatch")
				middleware.MarkBlocked(r.Context(), "secure", http.StatusUnauthorized)
				attrs := []any{
					"method", r.Method,
					"path", r.URL.Path,
					"client_ip", requtil.ClientIP(r),
				}
				attrs = append(attrs, requtil.TokenLogAttrs(token)...)
				attrs = append(attrs, "expected_session", session, "request_session", reqSession)
				slog.Warn("secure session mismatch", attrs...)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			middleware.MarkSecure(r.Context(), "pass")
			scheduleTokenDelete(token)
			next.ServeHTTP(w, r)
		})
	}
}
