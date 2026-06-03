package config

import "time"

// TestOptions builds a Config for unit tests outside this package.
type TestOptions struct {
	AllowedPrefixes   []string
	CacheTTL200       time.Duration
	CacheTTL4xx       time.Duration
	CacheIncludeQuery bool
	AdminToken        string
	StatsEnabled      int
}

// Test returns a minimally valid Config for router and handler tests.
func Test(opts TestOptions) *Config {
	ttl200 := opts.CacheTTL200
	if ttl200 <= 0 {
		ttl200 = time.Minute
	}
	ttl4xx := opts.CacheTTL4xx
	if ttl4xx <= 0 {
		ttl4xx = time.Minute
	}
	return &Config{
		AllowedPrefixes:   opts.AllowedPrefixes,
		CacheIncludeQuery: opts.CacheIncludeQuery,
		CacheMaxBodyBytes: 2 * 1024 * 1024,
		AdminToken:        opts.AdminToken,
		StatsEnabled:      opts.StatsEnabled,
		SecureFailMode:    "closed",
		Port:              "8080",
		cacheTTL200:       ttl200,
		cacheTTL4xx:       ttl4xx,
		redisReadTimeout:  time.Second,
		shutdownTimeout:   10 * time.Second,
		readHeaderTimeout: 5 * time.Second,
		readTimeout:       30 * time.Second,
		idleTimeout:       120 * time.Second,
	}
}
