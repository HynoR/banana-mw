package config

import (
	"fmt"
	"log/slog"
	"strings"
)

// ParseLogLevel maps config log_level to slog levels (default info).
func ParseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log_level %q: want info, debug, warn, or error", raw)
	}
}

// DebugLog reports whether per-request debug tracing is enabled.
func (c *Config) DebugLog() bool {
	level, err := ParseLogLevel(c.LogLevel)
	if err != nil {
		return false
	}
	return level == slog.LevelDebug
}
