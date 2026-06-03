package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"hynor/banana-mw/internal/middleware"
	"hynor/banana-mw/internal/requtil"
)

func generateCacheKey(r *http.Request, includeQuery bool) (key, token, ua string) {
	token = requtil.RequestToken(r)
	ua = r.UserAgent()

	path := r.URL.Path
	if includeQuery {
		if canonicalQuery := canonicalQuery(r.URL.Query()); canonicalQuery != "" {
			path += "?" + canonicalQuery
		}
	}
	paramsStr := fmt.Sprintf("%s|%s|%s", r.Method, path, ua)
	hash := md5.Sum([]byte(paramsStr))
	paramsHash := hex.EncodeToString(hash[:])
	key = fmt.Sprintf("%s_%s", paramsHash, token)
	return key, token, ua
}

func canonicalQuery(values url.Values) string {
	filtered := make(url.Values, len(values))
	for key, vals := range values {
		if key == "token" || key == "s" {
			continue
		}
		copied := append([]string(nil), vals...)
		sort.Strings(copied)
		filtered[key] = copied
	}
	return strings.ReplaceAll(filtered.Encode(), "+", "%20")
}

func Middleware4xx(cache4xx *Memory4xx, includeQuery bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, _, _ := generateCacheKey(r, includeQuery)
			if cache4xx.Get(key) {
				middleware.MarkCache4xxHit(r.Context())
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("forbidden"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Middleware200(cache200 *Memory, cache4xx *Memory4xx, proxy *httputil.ReverseProxy, ttl200, ttl4xx time.Duration, includeQuery bool, maxBodyBytes int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			clientIP := requtil.ClientIP(r)
			key, token, ua := generateCacheKey(r, includeQuery)

			if entry, ok := cache200.Get(key); ok {
				middleware.MarkCache200Hit(r.Context(), entry.Status)
				writeCache200Response(w, entry)
				logCacheHit(r, entry.Status, start, clientIP, token, ua, key)
				return
			}

			bw, proxyPanic, panicValue := forwardToUpstream(w, r, proxy, maxBodyBytes)
			if proxyPanic {
				middleware.MarkProxyPanic(r.Context())
				logProxyAborted(r, start, clientIP, token, ua, key, panicValue)
				return
			}

			statusCode := getStatusCode(bw)
			middleware.MarkUpstream(r.Context(), statusCode)
			storeResponse(cache200, cache4xx, key, statusCode, bw, ttl200, ttl4xx)
			logCacheMiss(r, statusCode, start, clientIP, token, ua, key, ttl200, ttl4xx)
		})
	}
}

func writeCache200Response(w http.ResponseWriter, entry *Entry) {
	for k := range w.Header() {
		w.Header().Del(k)
	}
	for k, vv := range entry.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(entry.Status)
	_, _ = w.Write(entry.Body)
}

func forwardToUpstream(w http.ResponseWriter, r *http.Request, proxy *httputil.ReverseProxy, maxBodyBytes int) (*BodyCaptureWriter, bool, interface{}) {
	bw := NewBodyCaptureWriter(w, maxBodyBytes)

	var proxyPanic bool
	var panicValue interface{}

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				proxyPanic = true
				panicValue = recovered
			}
		}()
		proxy.ServeHTTP(bw, r)
	}()

	return bw, proxyPanic, panicValue
}

func getStatusCode(bw *BodyCaptureWriter) int {
	if bw.status == 0 {
		return http.StatusOK
	}
	return bw.status
}

func storeResponse(cache200 *Memory, cache4xx *Memory4xx, key string, statusCode int, bw *BodyCaptureWriter, ttl200, ttl4xx time.Duration) {
	switch {
	case bw.status != 0 && bw.Cacheable() && statusCode == http.StatusOK:
		entry := &Entry{
			Status:    statusCode,
			Header:    CloneHeader(bw.Header()),
			Body:      bw.body.Bytes(),
			ExpiresAt: time.Now().Add(ttl200),
		}
		cache200.Set(key, entry)
	case statusCode >= 400 && statusCode < 500:
		cache4xx.Set(key, time.Now().Add(ttl4xx))
	}
}

func logCacheHit(r *http.Request, status int, start time.Time, clientIP, token, ua, key string) {
	if middleware.DebugRequests() {
		return
	}
	attrs := []any{
		"cache_hit", true,
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"latency", time.Since(start).String(),
		"client_ip", clientIP,
	}
	attrs = append(attrs, requtil.TokenLogAttrs(token)...)
	attrs = append(attrs,
		"user_agent", ua,
		"cache_key", key,
		"source", "cache_200",
	)
	slog.Info("access", attrs...)
}

func logCacheMiss(r *http.Request, statusCode int, start time.Time, clientIP, token, ua, key string, ttl200, ttl4xx time.Duration) {
	if middleware.DebugRequests() {
		return
	}
	var ttlStr string
	switch {
	case statusCode == http.StatusOK:
		ttlStr = ttl200.String()
	case statusCode >= 400 && statusCode < 500:
		ttlStr = ttl4xx.String()
	default:
		ttlStr = "0s"
	}

	attrs := []any{
		"cache_hit", false,
		"method", r.Method,
		"path", r.URL.Path,
		"status", statusCode,
		"latency", time.Since(start).String(),
		"client_ip", clientIP,
	}
	attrs = append(attrs, requtil.TokenLogAttrs(token)...)
	attrs = append(attrs,
		"user_agent", ua,
		"cache_key", key,
		"source", "upstream",
		"ttl", ttlStr,
	)
	slog.Info("access", attrs...)
}

func logProxyAborted(r *http.Request, start time.Time, clientIP, token, ua, key string, panicValue interface{}) {
	attrs := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"latency", time.Since(start).String(),
		"client_ip", clientIP,
	}
	attrs = append(attrs, requtil.TokenLogAttrs(token)...)
	attrs = append(attrs,
		"user_agent", ua,
		"cache_key", key,
		"panic", fmt.Sprintf("%v", panicValue),
	)
	slog.Warn("proxy aborted", attrs...)
}
