package main

import (
	"context"
	"database/sql"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lemito/Santa/go/internal/app"
	"github.com/lemito/Santa/go/internal/auth"
	"github.com/lemito/Santa/go/internal/config"
	"github.com/lemito/Santa/go/internal/engine"
	"github.com/lemito/Santa/go/internal/httpapi"
	"github.com/lemito/Santa/go/internal/logging"
	"github.com/lemito/Santa/go/internal/migrations"
	appRuntime "github.com/lemito/Santa/go/internal/runtime"
	"github.com/lemito/Santa/go/internal/store"
	"go.uber.org/zap"
)

func main() {
	logger, err := logging.New()
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()
	migrate := flag.Bool("migrate", false, "apply database migrations")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("config_load_failed", zap.Error(err))
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.DatabaseURL == "" {
		logger.Fatal("database_url_missing")
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("database_open_failed", zap.Error(err))
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Fatal("database_ping_failed", zap.Error(err))
	}
	if *migrate {
		dir := filepath.Join("go", "migrations")
		if _, err := os.Stat(dir); err != nil {
			dir = "migrations"
		}
		if err := migrations.Run(ctx, db, dir); err != nil {
			logger.Fatal("migration_failed", zap.Error(err))
		}
		return
	}
	sqlStore := &store.Store{DB: db}
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-t.C:
				cleanupCtx, cleanupCancel := context.WithTimeout(rootCtx, 10*time.Second)
				_ = sqlStore.CleanupSessions(cleanupCtx)
				cleanupCancel()
			}
		}
	}()
	appStore := store.AppStore{Store: sqlStore}
	service := app.Service{Store: appStore, Engine: engine.Native{Budget: engine.DefaultBudget(), Logger: logger}, Logger: logger}
	users := auth.UserStore{DB: db}
	sessions := auth.Manager{Store: auth.SQLSessionStore{DB: db}, TTL: cfg.SessionTTL, Secure: cfg.SecureCookies}
	templates, templateErr := httpapi.LoadTemplates(filepath.Join("go", "templates"))
	if templates == nil {
		templates, templateErr = httpapi.LoadTemplates("templates")
	}
	if templateErr != nil || templates == nil {
		logger.Fatal("template_load_failed", zap.Error(templateErr))
	}
	staticDir := filepath.Join("go", "static")
	if _, err := os.Stat(staticDir); err != nil {
		staticDir = "static"
	}
	srv := &httpapi.Server{Static: http.FileServer(http.Dir(staticDir)), Auth: httpapi.SQLAuth{Users: users, Sessions: sessions}, Games: httpapi.AppAdapter{Service: service}, Rate: httpapi.NewRateLimiter(cfg.RateLimit, cfg.RateWindow), Templates: templates, DrawGate: make(chan struct{}, cfg.DrawConcurrency), BodyLimit: int64(cfg.RequestLimit), Ready: func(c context.Context) bool { return db.PingContext(c) == nil }, Logger: logger}
	logger.Info("santa_go_listening", zap.String("address", cfg.ListenAddr))
	h := &http.Server{Addr: cfg.ListenAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	if err := appRuntime.Run(rootCtx, &httpServer{Server: h}, cfg.ShutdownTimeout); err != nil {
		logger.Fatal("server_failed", zap.Error(err))
	}
	<-cleanupDone
}

type httpServer struct{ *http.Server }

func (s *httpServer) Serve() error { return s.ListenAndServe() }
