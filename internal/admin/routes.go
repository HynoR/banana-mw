package admin

import (
	"embed"
	"net/http"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/stats"

	"github.com/go-chi/chi/v5"
)

//go:embed static/index.html
var assets embed.FS

var processStartedAt = time.Now()
var buildVersion = "dev"

// SetBuildVersion sets the version string reported by the health API.
func SetBuildVersion(version string) {
	if version != "" {
		buildVersion = version
	}
}

func RegisterRoutes(r chi.Router, cfg *config.Config) {
	r.Get("/_gogoadmin", redirectIndex)
	r.Get("/_gogoadmin/", serveIndex)
	r.Get("/_gogoadmin/api/stats", stats.AdminGetHandler(cfg))
	r.Get("/_gogoadmin/api/health", HealthHandler(cfg))
	r.Get("/banana-mw/api/get", stats.LegacyGetHandler(cfg))
}

func redirectIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/_gogoadmin/", http.StatusMovedPermanently)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "admin page unavailable", http.StatusInternalServerError)
		return
	}
	setPageHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func setPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; "+
			"script-src 'unsafe-inline' https://unpkg.com; "+
			"connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
}
