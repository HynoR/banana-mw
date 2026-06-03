package config

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	level, err := ParseLogLevel("debug")
	if err != nil || level != slog.LevelDebug {
		t.Fatalf("got %v err=%v", level, err)
	}
	if _, err := ParseLogLevel("verbose"); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestDebugLog(t *testing.T) {
	cfg := &Config{LogLevel: "debug"}
	if !cfg.DebugLog() {
		t.Fatal("expected debug")
	}
	cfg.LogLevel = "info"
	if cfg.DebugLog() {
		t.Fatal("expected not debug")
	}
}
