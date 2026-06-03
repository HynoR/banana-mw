package stats

import (
	"fmt"
	"strings"
	"time"
)

const TTL = 48 * time.Hour

const windowHours = 48

// NormalizePath returns the path segment used for stats after applying prefix rules.
func NormalizePath(prefix, path string) (string, bool) {
	if prefix == "*" {
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		path = strings.TrimPrefix(path, "/")
		return path, true
	}

	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	normalized := strings.TrimPrefix(path, prefix)
	if normalized == "" {
		normalized = "/" + normalized
	}
	if idx := strings.Index(normalized, "?"); idx != -1 {
		normalized = normalized[:idx]
	}
	return normalized, true
}

func statsKey(normalizedPath string) string       { return fmt.Sprintf("stats::%s", normalizedPath) }
func statsIPKey(normalizedPath string) string     { return fmt.Sprintf("stats::%s::ips", normalizedPath) }
func statsUAKey(normalizedPath string) string     { return fmt.Sprintf("stats::%s::uas", normalizedPath) }
func statsHourKey(normalizedPath string) string   { return fmt.Sprintf("stats::%s::hours", normalizedPath) }
func statsTokenKey(token string) string           { return fmt.Sprintf("stats::token::%s", token) }
func statsTokenIPKey(token string) string         { return fmt.Sprintf("stats::token::%s::ips", token) }
func statsTokenUAKey(token string) string         { return fmt.Sprintf("stats::token::%s::uas", token) }
func statsTokenPathKey(token string) string       { return fmt.Sprintf("stats::token::%s::paths", token) }
func statsTokenHourKey(token string) string       { return fmt.Sprintf("stats::token::%s::hours", token) }

func hourBucket(t time.Time) int64 {
	return t.Unix() / 3600
}
