package stats

import (
	"log/slog"
	"time"
)

// dur formats a duration for JSON logs (slog encodes time.Duration as nanoseconds).
func dur(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

func durAttr(name string, d time.Duration) slog.Attr {
	return slog.String(name, dur(d))
}
