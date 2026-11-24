package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, ListenAddr, Environment     string
	SecureCookies                            bool
	SessionTTL, RateWindow, ShutdownTimeout  time.Duration
	RequestLimit, DrawConcurrency, RateLimit int
}

func Load() (Config, error) {
	env := os.Getenv("ASPNETCORE_ENVIRONMENT")
	if env == "" {
		env = "Production"
	}
	addr := os.Getenv("SANTA_GO_ADDR")
	if addr == "" {
		addr = ":5080"
	}
	db := os.Getenv("ConnectionStrings__DefaultConnection")
	if db == "" {
		db = os.Getenv("DATABASE_URL")
	}
	secure := env != "Development" && env != "Testing"
	if v, ok := os.LookupEnv("SANTA_GO_SECURE_COOKIES"); ok && v != "" {
		var err error
		secure, err = strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("SANTA_GO_SECURE_COOKIES: %w", err)
		}
	}
	cfg := Config{
		DatabaseURL: db, ListenAddr: addr, Environment: env, SecureCookies: secure,
		SessionTTL: 8 * time.Hour, RequestLimit: 1 << 20, DrawConcurrency: 2,
		RateLimit: 20, RateWindow: time.Minute, ShutdownTimeout: 10 * time.Second,
	}
	var err error
	if cfg.SessionTTL, err = durationEnv("SANTA_GO_SESSION_TTL", cfg.SessionTTL); err != nil {
		return Config{}, err
	}
	if cfg.RateWindow, err = durationEnv("SANTA_GO_RATE_WINDOW", cfg.RateWindow); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("SANTA_GO_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.RequestLimit, err = positiveIntEnv("SANTA_GO_REQUEST_LIMIT", cfg.RequestLimit); err != nil {
		return Config{}, err
	}
	if cfg.DrawConcurrency, err = positiveIntEnv("SANTA_GO_DRAW_CONCURRENCY", cfg.DrawConcurrency); err != nil {
		return Config{}, err
	}
	if cfg.RateLimit, err = positiveIntEnv("SANTA_GO_RATE_LIMIT", cfg.RateLimit); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		if err == nil {
			err = fmt.Errorf("must be positive")
		}
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		if err == nil {
			err = fmt.Errorf("must be positive")
		}
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}
