package stats

import "strings"

// windowCleanupAction decides how to trim 48h window data for a stats Redis key.
// kind is the Redis TYPE (zset, hash, ...). Suffix-only matching is unsafe because
// path main hashes can end with "::ips" when the normalized path contains that segment.
func windowCleanupAction(kind, key string) string {
	switch kind {
	case "zset":
		return "zrem_window"
	case "hash":
		if strings.HasSuffix(key, "::hours") {
			return "prune_hours"
		}
	}
	return "skip"
}
