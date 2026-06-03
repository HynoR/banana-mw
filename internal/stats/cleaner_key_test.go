package stats

import "testing"

func TestWindowCleanupAction(t *testing.T) {
	tests := []struct {
		kind string
		key  string
		want string
	}{
		{"zset", "stats::tokens", "zrem_window"},
		{"zset", "stats::api/v1/foo::ips", "zrem_window"},
		{"zset", "stats::token::abc::paths", "zrem_window"},
		{"hash", "stats::api/v1/foo::hours", "prune_hours"},
		{"hash", "stats::api/v1/foo::ips", "skip"},
		{"hash", "stats::api/v1/foo", "skip"},
		{"hash", "stats::token::abc", "skip"},
		{"string", "stats::legacy", "skip"},
	}

	for _, tt := range tests {
		if got := windowCleanupAction(tt.kind, tt.key); got != tt.want {
			t.Fatalf("windowCleanupAction(%q, %q) = %q, want %q", tt.kind, tt.key, got, tt.want)
		}
	}
}
