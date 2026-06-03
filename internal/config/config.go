package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// DefaultSecureRedisKeyPrefix is the Redis key prefix for secure token sessions.
const DefaultSecureRedisKeyPrefix = "secure_session::user::"

// Config holds service configuration loaded from YAML or JSON.
type Config struct {
	Upstream             string   `json:"upstream" yaml:"upstream"`
	AllowedPrefixes      []string `json:"allowed_prefixes" yaml:"allowed_prefixes"`
	CacheTTL200          string   `json:"cache_ttl_200" yaml:"cache_ttl_200"`
	CacheTTL4xx          string   `json:"cache_ttl_4xx" yaml:"cache_ttl_4xx"`
	CacheIncludeQuery    bool     `json:"cache_include_query" yaml:"cache_include_query"`
	CacheMaxBodyBytes    int      `json:"cache_max_body_bytes" yaml:"cache_max_body_bytes"`
	TrustProxy           bool     `json:"trust_proxy" yaml:"trust_proxy"`
	Port                 string   `json:"port" yaml:"port"`
	AdminPort            string   `json:"admin_port" yaml:"admin_port"`
	LogLevel             string   `json:"log_level" yaml:"log_level"`
	Secure               int      `json:"secure" yaml:"secure"`
	SecureTime           string   `json:"secure_time" yaml:"secure_time"`
	SecureRedisKeyPrefix string   `json:"secure_redis_key_prefix" yaml:"secure_redis_key_prefix"`
	SecureFailMode       string   `json:"secure_fail_mode" yaml:"secure_fail_mode"`
	TokenRedisAddr       string   `json:"token_redis_addr" yaml:"token_redis_addr"`
	TokenRedisPwd        string   `json:"token_redis_pwd" yaml:"token_redis_pwd"`
	TokenRedisDB         int      `json:"token_redis_db" yaml:"token_redis_db"`
	RedisDialTimeout     string   `json:"redis_dial_timeout" yaml:"redis_dial_timeout"`
	RedisReadTimeout     string   `json:"redis_read_timeout" yaml:"redis_read_timeout"`
	RedisWriteTimeout    string   `json:"redis_write_timeout" yaml:"redis_write_timeout"`
	StatsEnabled         int      `json:"stats_enabled" yaml:"stats_enabled"`
	StatsPrefix          string   `json:"stats_prefix" yaml:"stats_prefix"`
	StatsQueueSize       int      `json:"stats_queue_size" yaml:"stats_queue_size"`
	StatsWorkers         int      `json:"stats_workers" yaml:"stats_workers"`
	StatsWriteTimeout    string   `json:"stats_write_timeout" yaml:"stats_write_timeout"`
	IsAdmin              int      `json:"is_admin" yaml:"is_admin"`
	AdminToken           string   `json:"admin_token" yaml:"admin_token"`
	ReadHeaderTimeout    string   `json:"read_header_timeout" yaml:"read_header_timeout"`
	ReadTimeout          string   `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout         string   `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout          string   `json:"idle_timeout" yaml:"idle_timeout"`
	ShutdownTimeout      string   `json:"shutdown_timeout" yaml:"shutdown_timeout"`

	StatsCleanMinCount      int    `json:"stats_clean_min_count" yaml:"stats_clean_min_count"`
	StatsCleanInactiveTime  string `json:"stats_clean_inactive_time" yaml:"stats_clean_inactive_time"`
	StatsCleanIntervalHours int    `json:"stats_clean_interval_hours" yaml:"stats_clean_interval_hours"`

	cacheTTL200            time.Duration
	cacheTTL4xx            time.Duration
	secureTime             time.Duration
	redisDialTimeout       time.Duration
	redisReadTimeout       time.Duration
	redisWriteTimeout      time.Duration
	statsWriteTimeout      time.Duration
	statsCleanInactiveTime time.Duration
	readHeaderTimeout      time.Duration
	readTimeout            time.Duration
	writeTimeout           time.Duration
	idleTimeout            time.Duration
	shutdownTimeout        time.Duration
}

func (c *Config) CacheTTL200Duration() time.Duration       { return c.cacheTTL200 }
func (c *Config) CacheTTL4xxDuration() time.Duration       { return c.cacheTTL4xx }
func (c *Config) SecureTimeDuration() time.Duration        { return c.secureTime }
func (c *Config) RedisDialTimeoutDuration() time.Duration  { return c.redisDialTimeout }
func (c *Config) RedisReadTimeoutDuration() time.Duration  { return c.redisReadTimeout }
func (c *Config) RedisWriteTimeoutDuration() time.Duration { return c.redisWriteTimeout }
func (c *Config) StatsWriteTimeoutDuration() time.Duration { return c.statsWriteTimeout }
func (c *Config) StatsCleanInactiveDuration() time.Duration {
	return c.statsCleanInactiveTime
}
func (c *Config) ReadHeaderTimeoutDuration() time.Duration { return c.readHeaderTimeout }
func (c *Config) ReadTimeoutDuration() time.Duration       { return c.readTimeout }
func (c *Config) WriteTimeoutDuration() time.Duration      { return c.writeTimeout }
func (c *Config) IdleTimeoutDuration() time.Duration       { return c.idleTimeout }
func (c *Config) ShutdownTimeoutDuration() time.Duration   { return c.shutdownTimeout }

// SecureFailOpen reports whether Redis errors bypass secure checks.
func (c *Config) SecureFailOpen() bool {
	return c.SecureFailMode == "open"
}

// Load reads and validates configuration from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &cfg)
	case ".json", "":
		err = json.Unmarshal(data, &cfg)
	default:
		err = fmt.Errorf("unsupported config file extension: %s", filepath.Ext(path))
	}
	if err != nil {
		return nil, err
	}

	if cfg.Upstream == "" {
		return nil, fmt.Errorf("upstream is empty in config")
	}
	if cfg.Upstream == "https://example.com" {
		return nil, fmt.Errorf("upstream must be changed from placeholder https://example.com")
	}

	if cfg.CacheTTL200 == "" {
		cfg.cacheTTL200 = 5 * time.Minute
	} else {
		d, err := time.ParseDuration(cfg.CacheTTL200)
		if err != nil {
			return nil, fmt.Errorf("invalid cache_ttl_200: %w", err)
		}
		cfg.cacheTTL200 = d
	}

	if cfg.CacheTTL4xx == "" {
		cfg.cacheTTL4xx = 10 * time.Minute
	} else {
		d, err := time.ParseDuration(cfg.CacheTTL4xx)
		if err != nil {
			return nil, fmt.Errorf("invalid cache_ttl_4xx: %w", err)
		}
		cfg.cacheTTL4xx = d
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if cfg.AdminPort == "" {
		cfg.AdminPort = "8081"
	}
	if cfg.IsAdmin == 1 && cfg.AdminPort == cfg.Port {
		return nil, fmt.Errorf("admin_port (%s) must differ from port when is_admin is 1", cfg.AdminPort)
	}

	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return nil, err
	}

	if cfg.SecureTime == "" {
		cfg.secureTime = 30 * time.Minute
	} else {
		d, err := time.ParseDuration(cfg.SecureTime)
		if err != nil {
			return nil, fmt.Errorf("invalid secure_time: %w", err)
		}
		cfg.secureTime = d
	}

	if cfg.SecureRedisKeyPrefix == "" {
		cfg.SecureRedisKeyPrefix = DefaultSecureRedisKeyPrefix
	}

	failMode := strings.ToLower(strings.TrimSpace(cfg.SecureFailMode))
	if failMode == "" {
		failMode = "closed"
	}
	if failMode != "closed" && failMode != "open" {
		return nil, fmt.Errorf("invalid secure_fail_mode %q (want closed or open)", cfg.SecureFailMode)
	}
	cfg.SecureFailMode = failMode

	if cfg.CacheMaxBodyBytes <= 0 {
		cfg.CacheMaxBodyBytes = 2 * 1024 * 1024
	}

	if cfg.TokenRedisAddr == "" {
		cfg.TokenRedisAddr = "localhost:6379"
	}

	var errDuration error
	if cfg.redisDialTimeout, errDuration = parseDurationDefault(cfg.RedisDialTimeout, 3*time.Second, "redis_dial_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.redisReadTimeout, errDuration = parseDurationDefault(cfg.RedisReadTimeout, time.Second, "redis_read_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.redisWriteTimeout, errDuration = parseDurationDefault(cfg.RedisWriteTimeout, time.Second, "redis_write_timeout"); errDuration != nil {
		return nil, errDuration
	}

	if cfg.StatsQueueSize <= 0 {
		cfg.StatsQueueSize = 10000
	}
	if cfg.StatsWorkers <= 0 {
		cfg.StatsWorkers = 2
	}
	if cfg.statsWriteTimeout, errDuration = parseDurationDefault(cfg.StatsWriteTimeout, time.Second, "stats_write_timeout"); errDuration != nil {
		return nil, errDuration
	}

	if cfg.StatsEnabled == 1 && cfg.StatsPrefix == "" {
		return nil, fmt.Errorf("stats_prefix is required when stats_enabled is 1")
	}

	if cfg.StatsCleanInactiveTime != "" {
		d, err := time.ParseDuration(cfg.StatsCleanInactiveTime)
		if err != nil {
			return nil, fmt.Errorf("invalid stats_clean_inactive_time: %w", err)
		}
		cfg.statsCleanInactiveTime = d
	}

	if cfg.readHeaderTimeout, errDuration = parseDurationDefault(cfg.ReadHeaderTimeout, 5*time.Second, "read_header_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.readTimeout, errDuration = parseDurationDefault(cfg.ReadTimeout, 30*time.Second, "read_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.writeTimeout, errDuration = parseDurationDefault(cfg.WriteTimeout, 0, "write_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.idleTimeout, errDuration = parseDurationDefault(cfg.IdleTimeout, 120*time.Second, "idle_timeout"); errDuration != nil {
		return nil, errDuration
	}
	if cfg.shutdownTimeout, errDuration = parseDurationDefault(cfg.ShutdownTimeout, 10*time.Second, "shutdown_timeout"); errDuration != nil {
		return nil, errDuration
	}

	return &cfg, nil
}

func parseDurationDefault(raw string, fallback time.Duration, name string) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return d, nil
}

// LoadDefault loads runtime config from data/, then legacy root paths.
func LoadDefault() (*Config, error) {
	candidates := []string{
		"data/config.yaml",
		"config.yaml",
		"data/config.json",
		"config.json",
	}
	var lastErr error
	for _, path := range candidates {
		cfg, err := Load(path)
		if err == nil {
			return cfg, nil
		}
		if os.IsNotExist(err) {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no config file found")
	}
	return nil, lastErr
}
