package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, ListenAddr, Environment string
	SecureCookies                        bool
	SessionTTL, RequestLimit             time.Duration
}

func Load() Config {
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
	if v, ok := os.LookupEnv("SANTA_GO_SECURE_COOKIES"); ok {
		secure, _ = strconv.ParseBool(v)
	}
	return Config{DatabaseURL: db, ListenAddr: addr, Environment: env, SecureCookies: secure, SessionTTL: 8 * time.Hour, RequestLimit: 1 << 20}
}
