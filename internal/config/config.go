package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                string
	Mode               string
	Port               string
	PublicURL          string
	FrontendURL        string
	DatabaseURL        string
	SessionSecret      string
	SessionCookie      string
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
	AutomationEmail    string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:                get("APP_ENV", "development"),
		Mode:               strings.ToLower(get("MODE", "live")),
		Port:               get("APP_PORT", "8080"),
		PublicURL:          strings.TrimRight(get("APP_PUBLIC_URL", "http://localhost:8080"), "/"),
		FrontendURL:        strings.TrimRight(get("FRONTEND_URL", "http://localhost:3000"), "/"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		SessionCookie:      get("SESSION_COOKIE", "kludge_session"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AutomationEmail:    strings.ToLower(strings.TrimSpace(os.Getenv("AUTOMATION_EMAIL"))),
	}

	if cfg.IsTest() {
		if cfg.SessionSecret == "" {
			cfg.SessionSecret = "test-mode-session-secret-do-not-use-live"
		}
		if cfg.DatabaseURL == "" {
			cfg.DatabaseURL = "postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable"
		}
		if cfg.AutomationEmail == "" {
			cfg.AutomationEmail = "ingest@kludge.dev"
		}
		return cfg, nil
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func (c *Config) IsDev() bool {
	return c.Env != "production"
}

func (c *Config) IsTest() bool {
	return c.Mode == "test" || c.Mode == "js"
}

func (c *Config) CallbackURL(provider string) string {
	return fmt.Sprintf("%s/api/auth/%s/callback", c.PublicURL, provider)
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
