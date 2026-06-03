package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"hynor/banana-mw/internal/requtil"
)

func MethodGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			MarkBlocked(r.Context(), "method", http.StatusMethodNotAllowed)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserAgentGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() == "" {
			slog.Warn("user agent is empty",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", requtil.ClientIP(r),
			)
			MarkBlocked(r.Context(), "user_agent", http.StatusForbidden)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TokenGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := requtil.RequestToken(r)

		isLinkToken := false
		if strings.HasPrefix(r.URL.Path, "/link/") {
			linkToken := strings.TrimPrefix(r.URL.Path, "/link/")
			if requtil.ValidProxyToken(linkToken) {
				isLinkToken = true
			}
		}

		if (!requtil.ValidProxyToken(token)) && !isLinkToken {
			slog.Warn("token is invalid",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", requtil.ClientIP(r),
				"token_length", len(token),
			)
			MarkBlocked(r.Context(), "token", http.StatusForbidden)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func PathPrefixGuard(allowedPrefixes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(path, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				MarkBlocked(r.Context(), "path_prefix", http.StatusForbidden)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
