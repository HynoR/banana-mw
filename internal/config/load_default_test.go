package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultPrefersDataConfigYAML(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte(`upstream: https://from-data.example`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(`upstream: https://from-root.example`), 0644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Upstream != "https://from-data.example" {
		t.Fatalf("expected data config, got upstream %s", cfg.Upstream)
	}
}
