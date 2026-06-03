package stats

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"hynor/banana-mw/internal/adminauth"
	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/redisstore"
	"hynor/banana-mw/internal/requtil"

	"github.com/redis/go-redis/v9"
)

type Response struct {
	Count    int64    `json:"count"`
	Count48h int64    `json:"count_48h"`
	IPs      []string `json:"ips"`
	UAs      []string `json:"uas"`
	Hourly   []int64  `json:"hourly"`
}

type TokenResponse struct {
	Token    string   `json:"token"`
	Count    int64    `json:"count"`
	Count48h int64    `json:"count_48h"`
	IPs      []string `json:"ips"`
	UAs      []string `json:"uas"`
	Paths    []string `json:"paths"`
	Hourly   []int64  `json:"hourly"`
}

type AdminResponse struct {
	WindowSeconds int64                    `json:"window_seconds"`
	GeneratedAt   int64                    `json:"generated_at"`
	Paths         map[string]Response      `json:"paths"`
	Tokens        map[string]TokenResponse `json:"tokens"`
}

func LegacyGetHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminauth.Authorize(w, r, cfg) {
			return
		}
		if cfg.StatsEnabled != 1 {
			requtil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "stats disabled"})
			return
		}
		result, err := pathStats(r.Context())
		if err != nil {
			slog.Error("stats get error", "error", err.Error())
			requtil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		requtil.WriteJSON(w, http.StatusOK, result)
	}
}

func AdminGetHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminauth.Authorize(w, r, cfg) {
			return
		}
		if cfg.StatsEnabled != 1 {
			requtil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "stats disabled"})
			return
		}
		paths, err := pathStats(r.Context())
		if err != nil {
			slog.Error("path stats get error", "error", err.Error())
			requtil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		tokens, err := tokenStats(r.Context())
		if err != nil {
			slog.Error("token stats get error", "error", err.Error())
			requtil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		requtil.WriteJSON(w, http.StatusOK, AdminResponse{
			WindowSeconds: int64(TTL.Seconds()),
			GeneratedAt:   time.Now().Unix(),
			Paths:         paths,
			Tokens:        tokens,
		})
	}
}

func pathStats(ctx context.Context) (map[string]Response, error) {
	client := redisstore.Client()
	if client == nil {
		return nil, fmt.Errorf("redis is not initialized")
	}

	var cursor uint64
	var keys []string
	for {
		batch, next, err := client.Scan(ctx, cursor, "stats::*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range batch {
			if !strings.HasSuffix(key, "::ips") &&
				!strings.HasSuffix(key, "::uas") &&
				!strings.HasSuffix(key, "::hours") &&
				!strings.HasSuffix(key, "::paths") &&
				!strings.HasPrefix(key, "stats::token::") &&
				key != "stats::tokens" {
				keys = append(keys, key)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	result := make(map[string]Response)
	now := time.Now()
	nowHour := hourBucket(now)
	since := now.Unix() - int64(TTL.Seconds())

	for _, mainKey := range keys {
		normalizedPath := strings.TrimPrefix(mainKey, "stats::")
		ipKey := statsIPKey(normalizedPath)
		uaKey := statsUAKey(normalizedPath)
		hourKey := statsHourKey(normalizedPath)

		pipe := client.Pipeline()
		countCmd := pipe.HGet(ctx, mainKey, "count")
		hoursCmd := pipe.HGetAll(ctx, hourKey)
		ipsCmd := pipe.ZRangeByScore(ctx, ipKey, &redis.ZRangeBy{Min: fmt.Sprintf("%d", since), Max: "+inf"})
		uasCmd := pipe.ZRangeByScore(ctx, uaKey, &redis.ZRangeBy{Min: fmt.Sprintf("%d", since), Max: "+inf"})
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			slog.Warn("stats get error", "error", err.Error(), "path", normalizedPath)
			continue
		}

		count := int64(0)
		if countStr, err := countCmd.Result(); err == nil {
			count, _ = strconv.ParseInt(countStr, 10, 64)
		}
		ips, _ := ipsCmd.Result()
		uas, _ := uasCmd.Result()
		buckets, _ := hoursCmd.Result()
		count48h, hourly := summarizeHours(buckets, nowHour)
		sort.Strings(ips)
		sort.Strings(uas)

		result[normalizedPath] = Response{
			Count:    count,
			Count48h: count48h,
			IPs:      ips,
			UAs:      uas,
			Hourly:   hourly,
		}
	}
	return result, nil
}

func tokenStats(ctx context.Context) (map[string]TokenResponse, error) {
	client := redisstore.Client()
	if client == nil {
		return nil, fmt.Errorf("redis is not initialized")
	}

	now := time.Now()
	nowHour := hourBucket(now)
	since := now.Unix() - int64(TTL.Seconds())

	tokens, err := client.ZRangeByScore(ctx, "stats::tokens", &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", since),
		Max: "+inf",
	}).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	sort.Strings(tokens)

	result := make(map[string]TokenResponse, len(tokens))
	for _, token := range tokens {
		tokenKey := statsTokenKey(token)
		ipKey := statsTokenIPKey(token)
		uaKey := statsTokenUAKey(token)
		pathKey := statsTokenPathKey(token)
		hourKey := statsTokenHourKey(token)

		pipe := client.Pipeline()
		countCmd := pipe.HGet(ctx, tokenKey, "count")
		hoursCmd := pipe.HGetAll(ctx, hourKey)
		ipsCmd := pipe.ZRangeByScore(ctx, ipKey, &redis.ZRangeBy{Min: fmt.Sprintf("%d", since), Max: "+inf"})
		uasCmd := pipe.ZRangeByScore(ctx, uaKey, &redis.ZRangeBy{Min: fmt.Sprintf("%d", since), Max: "+inf"})
		pathsCmd := pipe.ZRangeByScore(ctx, pathKey, &redis.ZRangeBy{Min: fmt.Sprintf("%d", since), Max: "+inf"})
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			return nil, err
		}

		count := int64(0)
		if countStr, err := countCmd.Result(); err == nil {
			count, _ = strconv.ParseInt(countStr, 10, 64)
		}
		ips, _ := ipsCmd.Result()
		uas, _ := uasCmd.Result()
		paths, _ := pathsCmd.Result()
		buckets, _ := hoursCmd.Result()
		count48h, hourly := summarizeHours(buckets, nowHour)
		sort.Strings(ips)
		sort.Strings(uas)
		sort.Strings(paths)

		result[token] = TokenResponse{
			Token:    token,
			Count:    count,
			Count48h: count48h,
			IPs:      ips,
			UAs:      uas,
			Paths:    paths,
			Hourly:   hourly,
		}
	}
	return result, nil
}
