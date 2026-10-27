package config

import (
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is empty")
	}
}

func TestLoadRequiresSessionSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable")
	t.Setenv("SESSION_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when SESSION_SECRET is too short")
	}
}

func TestCallbackURL(t *testing.T) {
	cfg := &Config{PublicURL: "http://localhost:8080"}
	got := cfg.CallbackURL("github")
	want := "http://localhost:8080/api/auth/github/callback"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadOK(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable")
	t.Setenv("SESSION_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("MODE", "live")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("port: got %q", cfg.Port)
	}
}

func TestLoadTestModeSkipsSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("MODE", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsTest() {
		t.Fatal("expected test mode")
	}
}
