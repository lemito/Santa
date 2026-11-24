package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"ASPNETCORE_ENVIRONMENT", "SANTA_GO_ADDR", "DATABASE_URL",
		"ConnectionStrings__DefaultConnection", "SANTA_GO_SECURE_COOKIES",
		"SANTA_GO_SESSION_TTL", "SANTA_GO_REQUEST_LIMIT", "SANTA_GO_DRAW_CONCURRENCY",
		"SANTA_GO_RATE_LIMIT", "SANTA_GO_RATE_WINDOW", "SANTA_GO_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
		if key == "DATABASE_URL" || key == "ConnectionStrings__DefaultConnection" {
			_ = os.Unsetenv(key)
		}
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":5080" || cfg.SessionTTL != 8*time.Hour || cfg.RequestLimit != 1<<20 || cfg.DrawConcurrency != 2 || cfg.RateLimit != 20 || cfg.RateWindow != time.Minute || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadOverridesAndRejectsInvalidValues(t *testing.T) {
	t.Setenv("SANTA_GO_ADDR", "127.0.0.1:6000")
	t.Setenv("SANTA_GO_SECURE_COOKIES", "true")
	t.Setenv("SANTA_GO_SESSION_TTL", "2h")
	t.Setenv("SANTA_GO_REQUEST_LIMIT", "2048")
	t.Setenv("SANTA_GO_DRAW_CONCURRENCY", "4")
	t.Setenv("SANTA_GO_RATE_LIMIT", "7")
	t.Setenv("SANTA_GO_RATE_WINDOW", "30s")
	t.Setenv("SANTA_GO_SHUTDOWN_TIMEOUT", "3s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:6000" || !cfg.SecureCookies || cfg.SessionTTL != 2*time.Hour || cfg.RequestLimit != 2048 || cfg.DrawConcurrency != 4 || cfg.RateLimit != 7 || cfg.RateWindow != 30*time.Second || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}

	for _, tc := range []struct {
		name, key, value string
	}{
		{"bad duration", "SANTA_GO_SESSION_TTL", "nope"},
		{"zero body", "SANTA_GO_REQUEST_LIMIT", "0"},
		{"negative concurrency", "SANTA_GO_DRAW_CONCURRENCY", "-1"},
		{"zero rate", "SANTA_GO_RATE_LIMIT", "0"},
		{"zero window", "SANTA_GO_RATE_WINDOW", "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%q", tc.key, tc.value)
			}
		})
	}
}
