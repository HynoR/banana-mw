package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestReverseProxyErrorHandlerWritesBadGateway(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("unreachable"))
	}))
	upstreamURL, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamServer.Close()

	proxy := newReverseProxy(upstreamURL)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sub", nil)
	resp := httptest.NewRecorder()

	proxy.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 from proxy error handler, got %d", resp.Code)
	}
}
