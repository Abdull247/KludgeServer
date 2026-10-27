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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
		serve(cfg, auth.NewHandler(cfg, nil, nil, sessions), profile.NewHandler(nil, sessions), sessions, nil, nil)
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

	if cfg.AutomationEmail != "" {
		if err := ensureAutomationAccount(ctx, users, profiles, cfg.AutomationEmail); err != nil {
			log.Fatalf("automation account: %v", err)
		}
	}

	serve(cfg, auth.NewHandler(cfg, users, profiles, sessions), profile.NewHandler(profiles, sessions), sessions, pool, profiles)
}

func serve(cfg *config.Config, authHandler *auth.Handler, profileHandler *profile.Handler, sessions *auth.SessionStore, pool *pgxpool.Pool, profiles *profile.Store) {
	catStore, err := catalog.NewStoreFromMocks()
	if err != nil {
		log.Fatalf("catalog mocks: %v", err)
	}
	if pool != nil {
		if err := catStore.AttachDB(context.Background(), pool); err != nil {
			log.Printf("catalog db attach: %v — continuing with mocks", err)
		}
	}

	var identities catalog.IdentityLookup
	if profiles != nil {
		identities = profileIdentityAdapter{profiles: profiles}
	}
	catHandler := catalog.NewHandler(catStore, sessions, identities, cfg.AutomationEmail)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.NewRouter(cfg, authHandler, profileHandler, catHandler),
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

func ensureAutomationAccount(ctx context.Context, users *user.Store, profiles *profile.Store, email string) error {
	u, err := users.EnsureByEmail(ctx, email)
	if err != nil {
		return err
	}
	_, err = profiles.Ensure(ctx, profile.EnsureInput{
		UserID:   u.ID,
		FullName: "Kludge Ingest",
		Email:    email,
	})
	return err
}

// profileIdentityAdapter adapts the profile store to the catalog
// IdentityLookup interface, keeping catalog free of profile imports.
type profileIdentityAdapter struct {
	profiles *profile.Store
}

func (a profileIdentityAdapter) ByUserID(ctx context.Context, userID uuid.UUID) (catalog.Identity, bool) {
	p, err := a.profiles.GetByUserID(ctx, userID)
	if err != nil || p == nil {
		return catalog.Identity{}, false
	}
	avatar := ""
	if p.AvatarURL != nil {
		avatar = *p.AvatarURL
	}
	return catalog.Identity{
		UserID:    p.UserID.String(),
		Email:     p.Email,
		Username:  p.Username,
		FullName:  p.FullName,
		AvatarURL: avatar,
	}, true
}
