package redisstore

import (
	"context"
	"time"

	"hynor/banana-mw/internal/config"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// Init creates the shared Redis client from configuration.
func Init(cfg *config.Config) {
	addr := cfg.TokenRedisAddr
	if addr == "" {
		addr = "localhost:6379"
	}
	client = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.TokenRedisPwd,
		DB:           cfg.TokenRedisDB,
		DialTimeout:  cfg.RedisDialTimeoutDuration(),
		ReadTimeout:  cfg.RedisReadTimeoutDuration(),
		WriteTimeout: cfg.RedisWriteTimeoutDuration(),
	})
}

// Client returns the shared Redis client, or nil if not initialized.
func Client() *redis.Client {
	return client
}

// Ping checks Redis connectivity with the configured read timeout.
func Ping(ctx context.Context, readTimeout time.Duration) error {
	if client == nil {
		return redis.Nil
	}
	pingCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	return client.Ping(pingCtx).Err()
}
