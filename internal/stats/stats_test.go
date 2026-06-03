package stats

import (
	"testing"
	"time"
)

func TestEnqueueDropsWhenFull(t *testing.T) {
	oldQueue := queue
	oldDropped := dropped.Load()
	t.Cleanup(func() {
		queue = oldQueue
		dropped.Store(oldDropped)
	})

	queue = make(chan event, 1)
	dropped.Store(0)

	ev := event{normalizedPath: "/sub", now: time.Now()}
	enqueue(ev)
	enqueue(ev)

	if got := dropped.Load(); got != 1 {
		t.Fatalf("expected one dropped stats event, got %d", got)
	}
}

func TestSummarizeHours(t *testing.T) {
	nowHour := int64(1000)
	buckets := map[string]string{
		"1000": "3",
		"999":  "2",
		"953":  "5",
		"952":  "9",
		"bad":  "1",
		"970":  "oops",
	}

	count, hourly := summarizeHours(buckets, nowHour)
	if count != 10 {
		t.Fatalf("expected 48h count 10, got %d", count)
	}
	if len(hourly) != windowHours {
		t.Fatalf("expected %d hourly buckets, got %d", windowHours, len(hourly))
	}
	if hourly[windowHours-1] != 3 {
		t.Fatalf("expected current hour bucket 3, got %d", hourly[windowHours-1])
	}
	if hourly[windowHours-2] != 2 {
		t.Fatalf("expected previous hour bucket 2, got %d", hourly[windowHours-2])
	}
	if hourly[0] != 5 {
		t.Fatalf("expected oldest in-window bucket 5, got %d", hourly[0])
	}
}
