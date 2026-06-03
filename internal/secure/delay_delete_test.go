package secure

import (
	"context"
	"testing"
	"time"
)

func TestDelayDeleterCancelsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	StartDelayDeleter(ctx, 50*time.Millisecond, 8)
	scheduleTokenDelete("tok-a")
	scheduleTokenDelete("tok-a")
	cancel()
	time.Sleep(20 * time.Millisecond)
	delayDeleter = nil
}
