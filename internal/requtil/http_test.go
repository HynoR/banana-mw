package requtil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustProxyOffIgnoresForwarded(t *testing.T) {
	ConfigureTrustProxy(false)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:12345"
	r.Header.Set("X-Forwarded-For", "198.51.100.1")
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPTrustProxyOnUsesForwarded(t *testing.T) {
	ConfigureTrustProxy(true)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.9:12345"
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.2")
	if got := ClientIP(r); got != "198.51.100.1" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenLogAttrsRedacts(t *testing.T) {
	attrs := TokenLogAttrs("abcdefghijklmnop")
	if len(attrs) != 4 {
		t.Fatalf("unexpected attrs len %d", len(attrs))
	}
	if attrs[0] != "token_prefix" || attrs[1] != "abcd" {
		t.Fatalf("prefix attrs: %v", attrs[:2])
	}
	if attrs[2] != "token_length" || attrs[3] != 16 {
		t.Fatalf("length attrs: %v", attrs[2:])
	}
}
