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

func TestParseBytes(t *testing.T) {
	cases := map[string]int64{
		"":      52428800,
		"1024":  1024,
		"50MB":  50 << 20,
		"2gb":   2 << 30,
		"10KB":  10 << 10,
		"bogus": 52428800,
		"0":     52428800,
	}
	for in, want := range cases {
		if got := parseBytes(in); got != want {
			t.Fatalf("parseBytes(%q)=%d want %d", in, got, want)
		}
	}
}

func TestLoadR2AndUploadLimit(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable")
	t.Setenv("SESSION_SECRET", "abcdefghijklmnopqrstuvwxyz012345")
	t.Setenv("MODE", "live")
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	t.Setenv("R2_BUCKET_NAME", "kludge")
	t.Setenv("R2_ENDPOINT", "https://example.r2.cloudflarestorage.com/")
	t.Setenv("R2_PUBLIC_URL", "https://cdn.example.com/")
	t.Setenv("MAX_UPLOAD_BYTES", "20MB")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.R2BucketName != "kludge" || cfg.R2Endpoint != "https://example.r2.cloudflarestorage.com" {
		t.Fatalf("r2: %+v", cfg)
	}
	if cfg.MaxUploadBytes != 20<<20 {
		t.Fatalf("max upload %d", cfg.MaxUploadBytes)
	}
}
