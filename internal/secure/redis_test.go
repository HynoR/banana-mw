package secure

import (
	"testing"

	"hynor/banana-mw/internal/config"
)

func TestConfigureRedisKeyPrefix(t *testing.T) {
	Configure(&config.Config{SecureRedisKeyPrefix: "custom::token::"})
	if got := redisKey("abc"); got != "custom::token::abc" {
		t.Fatalf("got %q", got)
	}

	Configure(&config.Config{SecureRedisKeyPrefix: config.DefaultSecureRedisKeyPrefix})
	if got := redisKey("xyz"); got != config.DefaultSecureRedisKeyPrefix+"xyz" {
		t.Fatalf("got %q", got)
	}
}
