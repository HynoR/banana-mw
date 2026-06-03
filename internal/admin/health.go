package admin

import (
	"net/http"
	"time"

	"hynor/banana-mw/internal/adminauth"
	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/redisstore"
	"hynor/banana-mw/internal/requtil"
	"hynor/banana-mw/internal/stats"
)

type queueHealth struct {
	Capacity int    `json:"capacity"`
	Length   int    `json:"length"`
	Dropped  uint64 `json:"dropped"`
	Workers  int    `json:"workers"`
}

type redisHealth struct {
	Enabled   bool   `json:"enabled"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type healthResponse struct {
	Now           int64       `json:"now"`
	StartedAt     int64       `json:"started_at"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	Version       string      `json:"version"`
	StatsEnabled  bool        `json:"stats_enabled"`
	SecureEnabled bool        `json:"secure_enabled"`
	IsAdmin       bool        `json:"is_admin"`
	WindowSeconds int64       `json:"window_seconds"`
	Queue         queueHealth `json:"queue"`
	Redis         redisHealth `json:"redis"`
}

func HealthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminauth.Authorize(w, r, cfg) {
			return
		}

		now := time.Now()
		capacity, length := stats.QueueInfo()
		resp := healthResponse{
			Now:           now.Unix(),
			StartedAt:     processStartedAt.Unix(),
			UptimeSeconds: int64(now.Sub(processStartedAt).Seconds()),
			Version:       buildVersion,
			StatsEnabled:  cfg.StatsEnabled == 1,
			SecureEnabled: cfg.Secure == 1,
			IsAdmin:       cfg.IsAdmin == 1,
			WindowSeconds: int64(stats.TTL.Seconds()),
			Queue: queueHealth{
				Capacity: capacity,
				Length:   length,
				Dropped:  stats.DroppedCount(),
				Workers:  cfg.StatsWorkers,
			},
			Redis: checkRedis(r, cfg),
		}
		requtil.WriteJSON(w, http.StatusOK, resp)
	}
}

func checkRedis(r *http.Request, cfg *config.Config) redisHealth {
	h := redisHealth{Enabled: redisstore.Client() != nil}
	if !h.Enabled {
		return h
	}
	start := time.Now()
	err := redisstore.Ping(r.Context(), cfg.RedisReadTimeoutDuration())
	h.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.OK = true
	return h
}
