package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStoreResponseSkipsUnwrittenResponse(t *testing.T) {
	cache200 := NewMemory()
	cache4xx := NewMemory4xx()
	bw := NewBodyCaptureWriter(httptest.NewRecorder(), 0)
	bw.status = 0

	storeResponse(cache200, cache4xx, "key", getStatusCode(bw), bw, time.Minute, time.Minute)

	if _, ok := cache200.Get("key"); ok {
		t.Fatal("unwritten response must not be cached as 200")
	}
}

func TestStoreResponseSkipsEmpty200Body(t *testing.T) {
	cache200 := NewMemory()
	cache4xx := NewMemory4xx()
	bw := NewBodyCaptureWriter(httptest.NewRecorder(), 0)
	bw.status = http.StatusOK

	storeResponse(cache200, cache4xx, "key", getStatusCode(bw), bw, time.Minute, time.Minute)

	if _, ok := cache200.Get("key"); ok {
		t.Fatal("empty 200 response must not be cached")
	}
}

func TestStoreResponseSkipsOversizedBody(t *testing.T) {
	cache200 := NewMemory()
	cache4xx := NewMemory4xx()
	rec := httptest.NewRecorder()
	bw := NewBodyCaptureWriter(rec, 4)
	bw.status = http.StatusOK
	if _, err := bw.Write([]byte("123456789")); err != nil {
		t.Fatal(err)
	}
	if got := rec.Body.String(); got != "123456789" {
		t.Fatalf("client body = %q", got)
	}
	if bw.Cacheable() {
		t.Fatal("oversized capture must not be cacheable")
	}
	storeResponse(cache200, cache4xx, "key", getStatusCode(bw), bw, time.Minute, time.Minute)
	if _, ok := cache200.Get("key"); ok {
		t.Fatal("oversized 200 response must not be cached")
	}
}
