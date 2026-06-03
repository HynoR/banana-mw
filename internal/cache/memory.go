package cache

import (
	"bytes"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Entry struct {
	Status    int
	Header    http.Header
	Body      []byte
	ExpiresAt time.Time
}

type Memory struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

func NewMemory() *Memory {
	return &Memory{
		data: make(map[string]*Entry, 512),
	}
}

func (c *Memory) Get(key string) (*Entry, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		if entry, ok := c.data[key]; ok && time.Now().After(entry.ExpiresAt) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return entry, true
}

func (c *Memory) Set(key string, entry *Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = entry
}

func (c *Memory) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	beforeCount := len(c.data)
	now := time.Now()
	newMap := make(map[string]*Entry, beforeCount/2)
	for k, v := range c.data {
		if now.Before(v.ExpiresAt) {
			newMap[k] = v
		}
	}
	c.data = newMap

	slog.Info("cleanup expired 200 cache", "before", beforeCount, "after", len(c.data))
}

func (c *Memory) StartGC(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			c.CleanupExpired()
		}
	}()
}

type entry4xx struct {
	ExpiresAt time.Time
}

type Memory4xx struct {
	mu   sync.RWMutex
	data map[string]*entry4xx
}

func NewMemory4xx() *Memory4xx {
	return &Memory4xx{
		data: make(map[string]*entry4xx, 1024),
	}
}

func (c *Memory4xx) Get(key string) bool {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		if entry, ok := c.data[key]; ok && time.Now().After(entry.ExpiresAt) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		return false
	}

	return true
}

func (c *Memory4xx) Set(key string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = &entry4xx{ExpiresAt: expiresAt}
}

func (c *Memory4xx) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	beforeCount := len(c.data)
	now := time.Now()
	newMap := make(map[string]*entry4xx, beforeCount/2)
	for k, v := range c.data {
		if now.Before(v.ExpiresAt) {
			newMap[k] = v
		}
	}
	c.data = newMap
	slog.Info("cleanup expired 4xx cache", "before", beforeCount, "after", len(c.data))
}

func (c *Memory4xx) StartGC(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			c.CleanupExpired()
		}
	}()
}

func CloneHeader(h http.Header) http.Header {
	cloned := make(http.Header, len(h))
	for k, vv := range h {
		copyVals := make([]string, len(vv))
		copy(copyVals, vv)
		cloned[k] = copyVals
	}
	return cloned
}

type BodyCaptureWriter struct {
	http.ResponseWriter
	status          int
	body            *bytes.Buffer
	maxBodyBytes    int
	captureExceeded bool
}

// NewBodyCaptureWriter wraps w and buffers up to maxBodyBytes for cache storage.
func NewBodyCaptureWriter(w http.ResponseWriter, maxBodyBytes int) *BodyCaptureWriter {
	return &BodyCaptureWriter{
		ResponseWriter: w,
		body:           bytes.NewBuffer(nil),
		maxBodyBytes:   maxBodyBytes,
	}
}

// Cacheable reports whether the captured body is eligible for 200 caching.
func (w *BodyCaptureWriter) Cacheable() bool {
	return !w.captureExceeded && w.body.Len() > 0
}

func (w *BodyCaptureWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *BodyCaptureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if !w.captureExceeded && w.maxBodyBytes > 0 {
		remaining := w.maxBodyBytes - w.body.Len()
		if remaining <= 0 {
			w.captureExceeded = true
		} else if len(b) > remaining {
			_, _ = w.body.Write(b[:remaining])
			w.captureExceeded = true
		} else {
			_, _ = w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *BodyCaptureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
