package config

import (
	"os"
	"testing"
)

func TestLoadFailsFastWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x/y")
	os.Unsetenv("PORT")
	os.Unsetenv("LOG_LEVEL")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
