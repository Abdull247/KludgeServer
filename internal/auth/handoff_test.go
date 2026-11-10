package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestDetectClient(t *testing.T) {
	cases := []struct {
		header string
		query  string
		want   string
	}{
		{"", "", clientWeb},
		{"app", "", clientApp},
		{"android", "", clientApp},
		{"ios", "", clientApp},
		{"mobile", "", clientApp},
		{"native", "", clientApp},
		{"web", "", clientWeb},
		{"", "app", clientApp},
		{"web", "app", clientWeb},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
		if tc.query != "" {
			req = httptest.NewRequest(http.MethodGet, "/api/auth/google?client="+tc.query, nil)
		}
		if tc.header != "" {
			req.Header.Set("X-Kludge-Client", tc.header)
		}
		if got := detectClient(req); got != tc.want {
			t.Fatalf("header=%q query=%q got %q want %q", tc.header, tc.query, got, tc.want)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	req.Header.Set("X-Client", "app")
	if got := detectClient(req); got != clientApp {
		t.Fatalf("X-Client: got %q", got)
	}
}

func TestAppDeepLink(t *testing.T) {
	got := appDeepLink("kludge://auth/complete", "abc")
	if got != "kludge://auth/complete?ticket=abc" {
		t.Fatalf("got %q", got)
	}
	got = appDeepLink("https://example.com/auth/complete", "t 1")
	if !strings.Contains(got, "ticket=t+1") && !strings.Contains(got, "ticket=t%201") {
		t.Fatalf("got %q", got)
	}
}

func TestHandoffMemoryRoundTrip(t *testing.T) {
	store := NewHandoffStore(nil, 300, 60)
	ctx := context.Background()
	userID := uuid.MustParse("00000000-0000-4000-8000-000000000009")

	if err := store.PutTransaction(ctx, "state-1", "google", clientApp, "sess"); err != nil {
		t.Fatal(err)
	}
	client, sess, ok, err := store.TakeTransaction(ctx, "state-1", "google")
	if err != nil || !ok {
		t.Fatalf("take: ok=%v err=%v", ok, err)
	}
	if client != clientApp || sess != "sess" {
		t.Fatalf("client=%q sess=%q", client, sess)
	}
	if _, _, ok, _ = store.TakeTransaction(ctx, "state-1", "google"); ok {
		t.Fatal("state should be single-use")
	}

	ticket, err := store.PutHandoff(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.ConsumeHandoff(ctx, ticket)
	if err != nil || !ok || got != userID {
		t.Fatalf("consume: id=%s ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ = store.ConsumeHandoff(ctx, ticket); ok {
		t.Fatal("ticket should be single-use")
	}
}

func TestHandoffExpiredTransaction(t *testing.T) {
	store := NewHandoffStore(nil, 1, 1)
	store.txnTTL = time.Millisecond
	ctx := context.Background()
	if err := store.PutTransaction(ctx, "stale", "github", clientApp, "x"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, ok, _ := store.TakeTransaction(ctx, "stale", "github"); ok {
		t.Fatal("expired transaction should not be taken")
	}
}

func TestExchangeAndComplete(t *testing.T) {
	cfg := &config.Config{
		SessionSecret:   "abcdefghijklmnopqrstuvwxyz012345",
		SessionCookie:   "kludge_session",
		AppLink:         "kludge://auth/complete",
		AppPackage:      "com.kludge.app",
		OAuthTxnTTL:     300,
		OAuthHandoffTTL: 60,
		FrontendURL:     "https://kludgekit.web.app",
	}
	sessions := NewSessionStore(cfg)
	handoffs := NewHandoffStore(nil, cfg.OAuthTxnTTL, cfg.OAuthHandoffTTL)
	h := NewHandler(cfg, nil, nil, sessions, handoffs)

	ticket, err := handoffs.PutHandoff(context.Background(), TestUserID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/exchange", strings.NewReader(`{"ticket":"`+ticket+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Exchange(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("exchange status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"tokenType":"Bearer"`) || !strings.Contains(body, `"session"`) {
		t.Fatalf("exchange body %s", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/oauth/exchange", strings.NewReader(`{"ticket":"`+ticket+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.Exchange(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("replay status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/complete?ticket=handoff-ticket-1", nil)
	rec = httptest.NewRecorder()
	h.Complete(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete status %d", rec.Code)
	}
	page := rec.Body.String()
	if !strings.Contains(page, "kludge://auth/complete?ticket=handoff-ticket-1") {
		t.Fatalf("missing deep link: %s", page)
	}
	if !strings.Contains(page, "package=com.kludge.app") {
		t.Fatalf("missing intent package: %s", page)
	}
}

func TestStartOAuthUnconfigured(t *testing.T) {
	cfg := &config.Config{FrontendURL: "https://kludgekit.web.app"}
	h := NewHandler(cfg, nil, nil, NewSessionStore(&config.Config{
		SessionSecret: "abcdefghijklmnopqrstuvwxyz012345",
		SessionCookie: "kludge_session",
		Mode:          "test",
	}), NewHandoffStore(nil, 300, 60))

	r := chi.NewRouter()
	r.Post("/api/auth/oauth/{provider}", h.StartOAuth)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/google", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHandoffPageEscapes(t *testing.T) {
	page := handoffPage(`kludge://auth/complete?ticket=a"b`, `intent://auth/complete#Intent;scheme=kludge;package=com.kludge.app;end`)
	if strings.Contains(page, `ticket=a"b`) {
		t.Fatal("unescaped quote in href")
	}
	if !strings.Contains(page, "Opening Kludge") {
		t.Fatal("missing copy")
	}
}
