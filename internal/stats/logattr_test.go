package stats

import (
	"testing"
	"time"
)

func TestDurFormatsHumanReadable(t *testing.T) {
	if got := dur(time.Second); got != "1s" {
		t.Fatalf("got %q", got)
	}
	if got := dur(time.Hour); got != "1h0m0s" {
		t.Fatalf("got %q", got)
	}
}
