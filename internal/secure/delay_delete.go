package secure

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"hynor/banana-mw/internal/requtil"
)

const defaultDelayDeleteMaxPending = 4096

var delayDeleter *tokenDelayDeleter

type tokenDelayDeleter struct {
	ctx        context.Context
	delay      time.Duration
	maxPending int

	mu      sync.Mutex
	pending map[string]*time.Timer
}

// StartDelayDeleter schedules bounded delayed token deletes until ctx is cancelled.
func StartDelayDeleter(ctx context.Context, delay time.Duration, maxPending int) {
	if delayDeleter != nil {
		return
	}
	if maxPending <= 0 {
		maxPending = defaultDelayDeleteMaxPending
	}
	d := &tokenDelayDeleter{
		ctx:        ctx,
		delay:      delay,
		maxPending: maxPending,
		pending:    make(map[string]*time.Timer),
	}
	delayDeleter = d
	go func() {
		<-ctx.Done()
		d.stopAll()
	}()
}

func scheduleTokenDelete(token string) {
	if delayDeleter == nil {
		return
	}
	delayDeleter.schedule(token)
}

func (d *tokenDelayDeleter) schedule(token string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx.Err() != nil {
		return
	}
	if t, ok := d.pending[token]; ok {
		t.Stop()
		delete(d.pending, token)
	}
	if len(d.pending) >= d.maxPending {
		d.deleteNow(token)
		return
	}
	tokenCopy := token
	timer := time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.pending, tokenCopy)
		d.mu.Unlock()
		if d.ctx.Err() != nil {
			return
		}
		d.deleteNow(tokenCopy)
	})
	d.pending[token] = timer
}

func (d *tokenDelayDeleter) deleteNow(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	deleteToken(ctx, token)
	slog.Info("secure token deleted after delay", requtil.TokenLogAttrs(token)...)
}

func (d *tokenDelayDeleter) stopAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, t := range d.pending {
		t.Stop()
	}
	d.pending = nil
}
