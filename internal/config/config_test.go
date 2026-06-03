package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
upstream: https://upstream.example
allowed_prefixes:
  - /api/v1
cache_ttl_200: 2m
cache_ttl_4xx: 3m
port: "9090"
secure: 1
secure_time: 4m
token_redis_addr: redis:6379
token_redis_db: 2
stats_enabled: 1
stats_prefix: /api/v1
admin_token: secret
stats_queue_size: 32
stats_workers: 3
stats_write_timeout: 2s
stats_clean_min_count: 3
stats_clean_inactive_time: 48h
stats_clean_interval_hours: 1
redis_dial_timeout: 4s
redis_read_timeout: 5s
redis_write_timeout: 6s
read_header_timeout: 7s
read_timeout: 8s
write_timeout: 0s
idle_timeout: 9s
shutdown_timeout: 10s
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Upstream != "https://upstream.example" {
		t.Fatalf("unexpected upstream: %s", cfg.Upstream)
	}
	if got := cfg.CacheTTL200Duration(); got != 2*time.Minute {
		t.Fatalf("unexpected cache ttl 200: %s", got)
	}
	if got := cfg.StatsCleanInactiveDuration(); got != 48*time.Hour {
		t.Fatalf("unexpected stats clean inactive time: %s", got)
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("unexpected admin token: %s", cfg.AdminToken)
	}
	if cfg.StatsQueueSize != 32 || cfg.StatsWorkers != 3 {
		t.Fatalf("unexpected stats worker config: queue=%d workers=%d", cfg.StatsQueueSize, cfg.StatsWorkers)
	}
	if cfg.StatsWriteTimeoutDuration() != 2*time.Second {
		t.Fatalf("unexpected stats write timeout: %s", cfg.StatsWriteTimeoutDuration())
	}
	if cfg.RedisDialTimeoutDuration() != 4*time.Second || cfg.RedisReadTimeoutDuration() != 5*time.Second || cfg.RedisWriteTimeoutDuration() != 6*time.Second {
		t.Fatalf("unexpected redis timeouts: dial=%s read=%s write=%s",
			cfg.RedisDialTimeoutDuration(), cfg.RedisReadTimeoutDuration(), cfg.RedisWriteTimeoutDuration())
	}
	if cfg.ReadHeaderTimeoutDuration() != 7*time.Second || cfg.ReadTimeoutDuration() != 8*time.Second || cfg.WriteTimeoutDuration() != 0 || cfg.IdleTimeoutDuration() != 9*time.Second || cfg.ShutdownTimeoutDuration() != 10*time.Second {
		t.Fatalf("unexpected server timeouts")
	}
}

func TestLoadConfigJSONFallbackStillWorks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"upstream":"https://upstream.example","allowed_prefixes":["/api"],"stats_enabled":0}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port, got %s", cfg.Port)
	}
}

func TestLoadConfigDefaultAdminPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`upstream: https://upstream.example`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.AdminPort != "8081" {
		t.Fatalf("expected default ports 8080/8081, got %s/%s", cfg.Port, cfg.AdminPort)
	}
}

func TestLoadConfigRejectsAdminPortCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
upstream: https://upstream.example
port: "8080"
admin_port: "8080"
is_admin: 1
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected admin_port == port with is_admin=1 to fail")
	}
}

func TestLoadConfigDefaultSecureRedisKeyPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`upstream: https://upstream.example`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SecureRedisKeyPrefix != DefaultSecureRedisKeyPrefix {
		t.Fatalf("expected default prefix %q, got %q", DefaultSecureRedisKeyPrefix, cfg.SecureRedisKeyPrefix)
	}
}

func TestLoadConfigCustomSecureRedisKeyPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
upstream: https://upstream.example
secure_redis_key_prefix: "custom::secure::"
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SecureRedisKeyPrefix != "custom::secure::" {
		t.Fatalf("unexpected prefix: %q", cfg.SecureRedisKeyPrefix)
	}
}

func TestLoadConfigSecureFailModeAndCacheMaxBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
upstream: https://upstream.example
secure_fail_mode: open
cache_max_body_bytes: 1024
trust_proxy: true
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SecureFailMode != "open" || !cfg.SecureFailOpen() {
		t.Fatalf("secure_fail_mode: %q open=%v", cfg.SecureFailMode, cfg.SecureFailOpen())
	}
	if cfg.CacheMaxBodyBytes != 1024 {
		t.Fatalf("cache_max_body_bytes: %d", cfg.CacheMaxBodyBytes)
	}
	if !cfg.TrustProxy {
		t.Fatal("expected trust_proxy true")
	}
}

func TestLoadConfigRejectsInvalidSecureFailMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
upstream: https://upstream.example
secure_fail_mode: maybe
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid secure_fail_mode to fail")
	}
}

func TestLoadConfigDefaultCacheMaxBodyBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`upstream: https://upstream.example`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheMaxBodyBytes != 2*1024*1024 {
		t.Fatalf("expected 2MiB default, got %d", cfg.CacheMaxBodyBytes)
	}
	if cfg.SecureFailMode != "closed" {
		t.Fatalf("expected closed default, got %q", cfg.SecureFailMode)
	}
}

func TestLoadConfigRejectsPlaceholderUpstream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`upstream: https://example.com`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected placeholder upstream to fail")
	}
}
