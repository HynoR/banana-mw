package secure

import (
	"context"
	"log/slog"
	"time"

	"hynor/banana-mw/internal/config"
	"hynor/banana-mw/internal/redisstore"
	"hynor/banana-mw/internal/requtil"

	"github.com/redis/go-redis/v9"
)

var redisKeyPrefix string

// SessionLookup describes the outcome of a secure Redis session read.
type SessionLookup int

const (
	SessionMiss SessionLookup = iota
	SessionHit
	SessionRedisError
)

// Configure sets the Redis key prefix for secure tokens from loaded config.
func Configure(cfg *config.Config) {
	redisKeyPrefix = cfg.SecureRedisKeyPrefix
}

func redisKey(token string) string {
	return redisKeyPrefix + token
}

func getSession(ctx context.Context, token string) (string, SessionLookup) {
	client := redisstore.Client()
	if client == nil {
		slog.Error("secure redis client not initialized", requtil.TokenLogAttrs(token)...)
		return "", SessionRedisError
	}
	val, err := client.Get(ctx, redisKey(token)).Result()
	if err == redis.Nil {
		return "", SessionMiss
	}
	if err != nil {
		attrs := append([]any{"error", err.Error()}, requtil.TokenLogAttrs(token)...)
		slog.Error("secure redis get error", attrs...)
		return "", SessionRedisError
	}
	return val, SessionHit
}

func setToken(ctx context.Context, token, session string, ttl time.Duration) error {
	return redisstore.Client().Set(ctx, redisKey(token), session, ttl).Err()
}

func deleteToken(ctx context.Context, token string) {
	if err := redisstore.Client().Del(ctx, redisKey(token)).Err(); err != nil {
		delAttrs := append([]any{"error", err.Error()}, requtil.TokenLogAttrs(token)...)
		slog.Error("secure redis delete error", delAttrs...)
	}
}
