package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"santa/go/internal/app"
	"santa/go/internal/auth"
	"santa/go/internal/config"
	"santa/go/internal/engine"
	"santa/go/internal/httpapi"
	"santa/go/internal/migrations"
	appRuntime "santa/go/internal/runtime"
	"santa/go/internal/store"
)

func main() {
	migrate := flag.Bool("migrate", false, "apply database migrations")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL or ConnectionStrings__DefaultConnection is required")
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	if *migrate {
		dir := filepath.Join("go", "migrations")
		if _, err := os.Stat(dir); err != nil {
			dir = "migrations"
		}
		if err := migrations.Run(ctx, db, dir); err != nil {
			log.Fatal(err)
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
	service := app.Service{Store: appStore, Engine: engine.Native{Budget: engine.DefaultBudget()}}
	users := auth.UserStore{DB: db}
	sessions := auth.Manager{Store: auth.SQLSessionStore{DB: db}, TTL: cfg.SessionTTL, Secure: cfg.SecureCookies}
	templates, templateErr := httpapi.LoadTemplates(filepath.Join("go", "templates"))
	if templates == nil {
		templates, templateErr = httpapi.LoadTemplates("templates")
	}
	if templateErr != nil || templates == nil {
		log.Fatalf("load templates: %v", templateErr)
	}
	staticDir := filepath.Join("go", "static")
	if _, err := os.Stat(staticDir); err != nil {
		staticDir = "static"
	}
	srv := &httpapi.Server{Static: http.FileServer(http.Dir(staticDir)), Auth: httpapi.SQLAuth{Users: users, Sessions: sessions}, Games: httpapi.AppAdapter{Service: service}, Rate: httpapi.NewRateLimiter(cfg.RateLimit, cfg.RateWindow), Templates: templates, DrawGate: make(chan struct{}, cfg.DrawConcurrency), BodyLimit: int64(cfg.RequestLimit), Ready: func(c context.Context) bool { return db.PingContext(c) == nil }}
	log.Printf("santa-go listening on %s", cfg.ListenAddr)
	h := &http.Server{Addr: cfg.ListenAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	if err := appRuntime.Run(rootCtx, &httpServer{Server: h}, cfg.ShutdownTimeout); err != nil {
		log.Fatal(err)
	}
	<-cleanupDone
}

type httpServer struct{ *http.Server }

func (s *httpServer) Serve() error { return s.ListenAndServe() }
