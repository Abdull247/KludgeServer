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
	AdminEmails        []string
	R2AccessKeyID      string
	R2SecretAccessKey  string
	R2BucketName       string
	R2Endpoint         string
	R2PublicURL        string
	MaxUploadBytes     int64
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
		AdminEmails:        parseEmails(os.Getenv("ADMIN_EMAILS")),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2BucketName:       os.Getenv("R2_BUCKET_NAME"),
		R2Endpoint:         strings.TrimRight(os.Getenv("R2_ENDPOINT"), "/"),
		R2PublicURL:        strings.TrimRight(os.Getenv("R2_PUBLIC_URL"), "/"),
		MaxUploadBytes:     parseBytes(get("MAX_UPLOAD_BYTES", "52428800")),
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
		if len(cfg.AdminEmails) == 0 {
			cfg.AdminEmails = []string{"ada@kludge.dev"}
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

// parseEmails splits a comma-separated email list into lowercase entries.
func parseEmails(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseBytes(raw string) int64 {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	if raw == "" {
		return 52428800
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(raw, "GB"):
		mult = 1 << 30
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "GB"))
	case strings.HasSuffix(raw, "MB"):
		mult = 1 << 20
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "MB"))
	case strings.HasSuffix(raw, "KB"):
		mult = 1 << 10
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "KB"))
	case strings.HasSuffix(raw, "B"):
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "B"))
	}
	var n int64
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 52428800
		}
		n = n*10 + int64(r-'0')
	}
	if n <= 0 {
		return 52428800
	}
	return n * mult
}
