package stats

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/redisstore"

	"github.com/redis/go-redis/v9"
)

func StartCleaner(ctx context.Context, cfg *config.Config) bool {
	inactive := cfg.StatsCleanInactiveDuration()
	if inactive <= 0 {
		inactive = TTL
	}
	intervalHours := cfg.StatsCleanIntervalHours
	if intervalHours <= 0 {
		intervalHours = 1
	}
	interval := time.Duration(intervalHours) * time.Hour

	inactiveLog := cfg.StatsCleanInactiveTime
	if inactiveLog == "" {
		inactiveLog = inactive.String()
	}
	slog.Info("stats cleaner started",
		"min_count", cfg.StatsCleanMinCount,
		"inactive_time", inactiveLog,
		durAttr("interval", interval),
		"interval_hours", intervalHours,
	)

	go runCleaner(ctx, cfg, inactive, interval)
	return true
}

func runCleaner(ctx context.Context, cfg *config.Config, inactive time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	doCleanup(ctx, cfg, inactive)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			doCleanup(ctx, cfg, inactive)
		}
	}
}

func doCleanup(ctx context.Context, cfg *config.Config, inactive time.Duration) {
	client := redisstore.Client()
	if client == nil {
		return
	}

	startTime := time.Now()
	lastProgress := startTime
	const progressInterval = 5 * time.Second

	var cursor uint64
	var batches, redisKeys, mainKeysScanned, pathsDeleted int
	scanBatchSize := int64(100)
	sleepBetweenBatches := 100 * time.Millisecond

	slog.Info("stats cleanup started",
		"scan_pattern", "stats::*",
		"scan_batch_size", scanBatchSize,
		"min_count", cfg.StatsCleanMinCount,
		durAttr("inactive_threshold", inactive),
	)

	for {
		if ctx.Err() != nil {
			slog.Info("stats cleanup cancelled",
				durAttr("elapsed", time.Since(startTime)),
				"batches", batches,
				"main_keys_scanned", mainKeysScanned,
				"paths_deleted", pathsDeleted,
			)
			return
		}

		keys, nextCursor, err := client.Scan(ctx, cursor, "stats::*", scanBatchSize).Result()
		if err != nil {
			slog.Error("stats cleanup scan error", "error", err, "batches_done", batches)
			return
		}
		batches++
		redisKeys += len(keys)

		cleanWindows(ctx, keys)

		mainKeys := filterMainKeys(keys)
		mainKeysScanned += len(mainKeys)

		for _, key := range mainKeys {
			cleaned, err := checkAndCleanKey(ctx, cfg, key, inactive)
			if err != nil {
				slog.Error("stats cleanup key error", "key", key, "error", err)
				continue
			}
			if cleaned {
				pathsDeleted++
			}
		}

		if now := time.Now(); now.Sub(lastProgress) >= progressInterval {
			elapsed := now.Sub(startTime)
			slog.Info("stats cleanup progress",
				durAttr("elapsed", elapsed),
				"batches", batches,
				"redis_keys", redisKeys,
				"main_keys_scanned", mainKeysScanned,
				"paths_deleted", pathsDeleted,
			)
			lastProgress = now
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}

		select {
		case <-ctx.Done():
			slog.Info("stats cleanup cancelled",
				durAttr("elapsed", time.Since(startTime)),
				"batches", batches,
				"main_keys_scanned", mainKeysScanned,
				"paths_deleted", pathsDeleted,
			)
			return
		case <-time.After(sleepBetweenBatches):
		}
	}

	elapsed := time.Since(startTime)
	rate := ""
	if elapsed > 0 && mainKeysScanned > 0 {
		perSec := float64(mainKeysScanned) / elapsed.Seconds()
		rate = fmt.Sprintf("%.0f/s", perSec)
	}
	slog.Info("stats cleanup completed",
		durAttr("duration", elapsed),
		"batches", batches,
		"redis_keys", redisKeys,
		"main_keys_scanned", mainKeysScanned,
		"paths_deleted", pathsDeleted,
		"scan_rate", rate,
	)
}

func cleanWindows(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	client := redisstore.Client()
	cutoff := time.Now().Add(-TTL).Unix()
	cutoffStr := fmt.Sprintf("%d", cutoff)
	sinceHour := hourBucket(time.Now()) - windowHours + 1

	typePipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keys))
	for i, key := range keys {
		typeCmds[i] = typePipe.Type(ctx, key)
	}
	if _, err := typePipe.Exec(ctx); err != nil {
		slog.Warn("stats window cleanup type error", "error", err)
		return
	}

	var zsetErrors int
	for i, key := range keys {
		kind, err := typeCmds[i].Result()
		if err != nil {
			continue
		}
		switch windowCleanupAction(kind, key) {
		case "zrem_window":
			if err := client.ZRemRangeByScore(ctx, key, "0", cutoffStr).Err(); err != nil {
				zsetErrors++
				if zsetErrors == 1 {
					slog.Warn("stats window cleanup error", "key", key, "error", err)
				}
			}
		case "prune_hours":
			pruneHourBuckets(ctx, key, sinceHour)
		}
	}
	if zsetErrors > 1 {
		slog.Warn("stats window cleanup additional zset errors", "count", zsetErrors)
	}
}

func pruneHourBuckets(ctx context.Context, key string, sinceHour int64) {
	client := redisstore.Client()
	buckets, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		slog.Warn("stats hour bucket read error", "key", key, "error", err)
		return
	}
	var stale []string
	for field := range buckets {
		h, err := strconv.ParseInt(field, 10, 64)
		if err != nil || h < sinceHour {
			stale = append(stale, field)
		}
	}
	if len(stale) > 0 {
		if err := client.HDel(ctx, key, stale...).Err(); err != nil {
			slog.Warn("stats hour bucket prune error", "key", key, "error", err)
		}
	}
}

func filterMainKeys(keys []string) []string {
	var mainKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "::ips") &&
			!strings.HasSuffix(key, "::uas") &&
			!strings.HasSuffix(key, "::hours") &&
			!strings.HasSuffix(key, "::paths") &&
			!strings.HasPrefix(key, "stats::token::") &&
			key != "stats::tokens" {
			mainKeys = append(mainKeys, key)
		}
	}
	return mainKeys
}

func checkAndCleanKey(ctx context.Context, cfg *config.Config, mainKey string, inactive time.Duration) (bool, error) {
	if cfg.StatsCleanMinCount <= 0 {
		return false, nil
	}

	client := redisstore.Client()
	countStr, err := client.HGet(ctx, mainKey, "count").Result()
	if err != nil {
		return false, nil
	}

	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		return false, err
	}
	if count >= int64(cfg.StatsCleanMinCount) {
		return false, nil
	}

	path := strings.TrimPrefix(mainKey, "stats::")
	hourKey := statsHourKey(path)
	buckets, err := client.HGetAll(ctx, hourKey).Result()
	if err != nil || len(buckets) == 0 {
		return deletePathKeys(ctx, path)
	}

	var lastHour int64
	for field := range buckets {
		if h, err := strconv.ParseInt(field, 10, 64); err == nil && h > lastHour {
			lastHour = h
		}
	}

	lastAccessTime := time.Unix(lastHour*3600, 0)
	if time.Since(lastAccessTime) <= inactive {
		return false, nil
	}

	slog.Info("stats cleanup deleted path",
		"path", path,
		"count", count,
		"last_access", lastAccessTime.Format("2006-01-02 15:04:05"),
		durAttr("inactive_for", time.Since(lastAccessTime)),
	)
	return deletePathKeys(ctx, path)
}

func deletePathKeys(ctx context.Context, path string) (bool, error) {
	client := redisstore.Client()
	pipe := client.Pipeline()
	pipe.Del(ctx, statsKey(path))
	pipe.Del(ctx, statsIPKey(path))
	pipe.Del(ctx, statsUAKey(path))
	pipe.Del(ctx, statsHourKey(path))
	_, err := pipe.Exec(ctx)
	return err == nil, err
}
