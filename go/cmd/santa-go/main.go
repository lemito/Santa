package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"santa/go/internal/app"
	"santa/go/internal/auth"
	"santa/go/internal/config"
	"santa/go/internal/engine"
	"santa/go/internal/httpapi"
	"santa/go/internal/store"
)

func main() {
	migrate := flag.Bool("migrate", false, "apply database migrations")
	flag.Parse()
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL or ConnectionStrings__DefaultConnection is required")
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	if *migrate {
		dir := filepath.Join("go", "migrations")
		if _, err := os.Stat(dir); err != nil {
			dir = "migrations"
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
		if err != nil || len(files) == 0 {
			log.Fatal("database migrations not found")
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				log.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, string(data)); err != nil {
				log.Fatalf("%s: %v", file, err)
			}
			log.Printf("applied %s", filepath.Base(file))
		}
		return
	}
	sqlStore := &store.Store{DB: db}
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			_ = sqlStore.CleanupSessions(context.Background())
		}
	}()
	appStore := store.AppStore{Store: sqlStore}
	service := app.Service{Store: appStore, Engine: engine.Native{Budget: engine.DefaultBudget()}}
	users := auth.UserStore{DB: db}
	sessions := auth.Manager{Store: auth.SQLSessionStore{DB: db}, TTL: cfg.SessionTTL, Secure: cfg.SecureCookies}
	templates, _ := httpapi.LoadTemplates(filepath.Join("go", "templates"))
	if templates == nil {
		templates, _ = httpapi.LoadTemplates("templates")
	}
	staticDir := filepath.Join("go", "static")
	if _, err := os.Stat(staticDir); err != nil {
		staticDir = "static"
	}
	srv := &httpapi.Server{Static: http.FileServer(http.Dir(staticDir)), Auth: httpapi.SQLAuth{Users: users, Sessions: sessions}, Games: httpapi.AppAdapter{Service: service}, Rate: httpapi.NewRateLimiter(20, time.Minute), Templates: templates, DrawGate: make(chan struct{}, 2), Ready: func(c context.Context) bool { return db.PingContext(c) == nil }}
	log.Printf("santa-go listening on %s", cfg.ListenAddr)
	h := &http.Server{Addr: cfg.ListenAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	if err := h.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
