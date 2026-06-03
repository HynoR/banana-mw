package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"hynor/banana-mw/internal/config"
)

func TestRequestTraceLogsBlockedRequest(t *testing.T) {
	SetDebugRequests(true)
	t.Cleanup(func() { SetDebugRequests(false) })

	cfg := config.Test(config.TestOptions{AllowedPrefixes: []string{"/api"}})
	var gotStatus int
	handler := RequestTrace(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		MarkBlocked(r.Context(), "token", http.StatusForbidden)
		w.WriteHeader(http.StatusForbidden)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	gotStatus = rec.Code

	if gotStatus != http.StatusForbidden {
		t.Fatalf("status %d", gotStatus)
	}

	ctx := context.Background()
	trace := traceFrom(ctx)
	if trace != nil {
		t.Fatal("trace should not leak outside request")
	}
}
