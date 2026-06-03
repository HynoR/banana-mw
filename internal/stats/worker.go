package stats

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/redisstore"
	"hynor/banana-mw/internal/requtil"

	"github.com/redis/go-redis/v9"
)

type event struct {
	normalizedPath string
	token          string
	clientIP       string
	userAgent      string
	now            time.Time
}

var (
	queue   chan event
	dropped atomic.Uint64
)

// DroppedCount returns the number of stats events dropped due to a full queue.
func DroppedCount() uint64 {
	return dropped.Load()
}

// QueueInfo returns capacity and length when the queue is initialized.
func QueueInfo() (capacity, length int) {
	if queue == nil {
		return 0, 0
	}
	return cap(queue), len(queue)
}

func StartWorkers(ctx context.Context, cfg *config.Config) {
	if cfg.StatsEnabled != 1 {
		return
	}
	queue = make(chan event, cfg.StatsQueueSize)
	for i := 0; i < cfg.StatsWorkers; i++ {
		go runWorker(ctx, cfg, i)
	}
	slog.Info("stats workers started",
		"workers", cfg.StatsWorkers,
		"queue_size", cfg.StatsQueueSize,
		durAttr("write_timeout", cfg.StatsWriteTimeoutDuration()),
	)
}

func runWorker(ctx context.Context, cfg *config.Config, workerID int) {
	_ = workerID
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-queue:
			writeCtx, cancel := context.WithTimeout(ctx, cfg.StatsWriteTimeoutDuration())
			record(writeCtx, ev.normalizedPath, ev.token, ev.clientIP, ev.userAgent, ev.now)
			cancel()
		}
	}
}

func Middleware(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fullPath := r.URL.RequestURI()
			normalizedPath, shouldTrack := NormalizePath(prefix, fullPath)
			if !shouldTrack {
				next.ServeHTTP(w, r)
				return
			}

			sw := &requtil.StatusCaptureWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			if sw.Status() == http.StatusOK {
				enqueue(event{
					normalizedPath: normalizedPath,
					token:          requtil.RequestToken(r),
					clientIP:       requtil.ClientIP(r),
					userAgent:      r.UserAgent(),
					now:            time.Now(),
				})
			}
		})
	}
}

func enqueue(ev event) {
	if queue == nil {
		return
	}
	select {
	case queue <- ev:
	default:
		n := dropped.Add(1)
		if n == 1 || n%1000 == 0 {
			slog.Warn("stats queue full, dropping events", "dropped", n)
		}
	}
}

func record(ctx context.Context, normalizedPath, token, clientIP, userAgent string, now time.Time) {
	client := redisstore.Client()
	if client == nil {
		return
	}

	timestamp := now.Unix()
	hour := hourBucket(now)
	hourField := strconv.FormatInt(hour, 10)
	staleField := strconv.FormatInt(hour-windowHours, 10)

	mainKey := statsKey(normalizedPath)
	ipKey := statsIPKey(normalizedPath)
	uaKey := statsUAKey(normalizedPath)
	hourKey := statsHourKey(normalizedPath)

	pipe := client.Pipeline()
	pipe.HIncrBy(ctx, mainKey, "count", 1)
	pipe.HIncrBy(ctx, hourKey, hourField, 1)
	pipe.HDel(ctx, hourKey, staleField)
	pipe.ZAdd(ctx, ipKey, redis.Z{Score: float64(timestamp), Member: clientIP})
	pipe.ZAdd(ctx, uaKey, redis.Z{Score: float64(timestamp), Member: userAgent})
	pipe.Expire(ctx, mainKey, TTL)
	pipe.Expire(ctx, ipKey, TTL)
	pipe.Expire(ctx, uaKey, TTL)
	pipe.Expire(ctx, hourKey, TTL)

	cutoff := timestamp - int64(TTL.Seconds())
	pipe.ZRemRangeByScore(ctx, ipKey, "0", fmt.Sprintf("%d", cutoff))
	pipe.ZRemRangeByScore(ctx, uaKey, "0", fmt.Sprintf("%d", cutoff))

	if token != "" {
		tokenKey := statsTokenKey(token)
		tokenIPKey := statsTokenIPKey(token)
		tokenUAKey := statsTokenUAKey(token)
		tokenPathKey := statsTokenPathKey(token)
		tokenHourKey := statsTokenHourKey(token)

		pipe.ZAdd(ctx, "stats::tokens", redis.Z{Score: float64(timestamp), Member: token})
		pipe.HIncrBy(ctx, tokenKey, "count", 1)
		pipe.HIncrBy(ctx, tokenHourKey, hourField, 1)
		pipe.HDel(ctx, tokenHourKey, staleField)
		pipe.ZAdd(ctx, tokenIPKey, redis.Z{Score: float64(timestamp), Member: clientIP})
		pipe.ZAdd(ctx, tokenUAKey, redis.Z{Score: float64(timestamp), Member: userAgent})
		pipe.ZAdd(ctx, tokenPathKey, redis.Z{Score: float64(timestamp), Member: normalizedPath})

		pipe.Expire(ctx, "stats::tokens", TTL)
		pipe.Expire(ctx, tokenKey, TTL)
		pipe.Expire(ctx, tokenIPKey, TTL)
		pipe.Expire(ctx, tokenUAKey, TTL)
		pipe.Expire(ctx, tokenPathKey, TTL)
		pipe.Expire(ctx, tokenHourKey, TTL)
		pipe.ZRemRangeByScore(ctx, tokenIPKey, "0", fmt.Sprintf("%d", cutoff))
		pipe.ZRemRangeByScore(ctx, tokenUAKey, "0", fmt.Sprintf("%d", cutoff))
		pipe.ZRemRangeByScore(ctx, tokenPathKey, "0", fmt.Sprintf("%d", cutoff))
		pipe.ZRemRangeByScore(ctx, "stats::tokens", "0", fmt.Sprintf("%d", cutoff))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		slog.Error("stats redis error", "error", err.Error(), "path", normalizedPath, "client_ip", clientIP)
	}
}
