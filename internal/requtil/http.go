package requtil

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var (
	tokenGuardPattern = regexp.MustCompile(`^[a-zA-Z0-9]{16,34}$`)
	trustProxy        bool
)

const tokenLogPrefixLen = 4

// ConfigureTrustProxy sets whether ClientIP may use X-Forwarded-For / X-Real-IP.
func ConfigureTrustProxy(trust bool) {
	trustProxy = trust
}

// TokenLogAttrs returns slog key/value pairs that redact a proxy token for logs.
func TokenLogAttrs(token string) []any {
	prefix := token
	if len(token) > tokenLogPrefixLen {
		prefix = token[:tokenLogPrefixLen]
	}
	return []any{"token_prefix", prefix, "token_length", len(token)}
}

// ValidProxyToken reports whether token satisfies proxy guard rules.
func ValidProxyToken(token string) bool {
	return token != "" && tokenGuardPattern.MatchString(token)
}

func QueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func PostFormParam(r *http.Request, key string) string {
	return r.PostFormValue(key)
}

func RequestToken(r *http.Request) string {
	token := QueryParam(r, "token")
	if token == "" {
		token = PostFormParam(r, "token")
	}
	if token == "" && strings.HasPrefix(r.URL.Path, "/link/") {
		token = strings.TrimPrefix(r.URL.Path, "/link/")
	}
	return token
}

func ClientIP(r *http.Request) string {
	if trustProxy {
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			if ip := strings.TrimSpace(strings.Split(forwardedFor, ",")[0]); ip != "" {
				return ip
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type StatusCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (w *StatusCaptureWriter) WriteHeader(statusCode int) {
	if w.status == 0 {
		w.status = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *StatusCaptureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *StatusCaptureWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *StatusCaptureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
