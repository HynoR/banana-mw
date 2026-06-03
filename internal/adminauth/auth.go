package adminauth

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/requtil"
)

func SetAPIHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
}

func Authorize(w http.ResponseWriter, r *http.Request, cfg *config.Config) bool {
	SetAPIHeaders(w)
	if cfg.AdminToken == "" {
		requtil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "admin disabled"})
		return false
	}
	rawAuth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(rawAuth, "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) != 1 {
		slog.Warn("admin auth failed",
			"method", r.Method,
			"path", r.URL.Path,
			"client_ip", requtil.ClientIP(r),
			"user_agent", r.UserAgent(),
			"has_bearer", ok,
		)
		requtil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}
