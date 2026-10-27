package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Abdull247/KludgeServer/internal/auth"
	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/database"
	httpx "github.com/Abdull247/KludgeServer/internal/http"
	"github.com/Abdull247/KludgeServer/internal/profile"
	"github.com/Abdull247/KludgeServer/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sessions := auth.NewSessionStore(cfg)
	auth.ConfigureProviders(cfg)

	if cfg.IsTest() {
		log.Printf("MODE=test — serving catalog from mock JSON, skipping database")
		serve(cfg, auth.NewHandler(cfg, nil, nil, sessions), profile.NewHandler(nil, sessions), sessions)
		return
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	users := user.NewStore(pool)
	profiles := profile.NewStore(pool)
	serve(cfg, auth.NewHandler(cfg, users, profiles, sessions), profile.NewHandler(profiles, sessions), sessions)
}

func serve(cfg *config.Config, authHandler *auth.Handler, profileHandler *profile.Handler, sessions *auth.SessionStore) {
	catStore, err := catalog.NewStoreFromMocks()
	if err != nil {
		log.Fatalf("catalog mocks: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.NewRouter(cfg, authHandler, profileHandler, catalog.NewHandler(catStore, sessions)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("kludge api listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
