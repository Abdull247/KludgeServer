package upload

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/google/uuid"
)

type stubSession struct {
	ok bool
}

func (s stubSession) UserID(*http.Request) (uuid.UUID, bool) {
	return uuid.MustParse("00000000-0000-4000-8000-000000000001"), s.ok
}

func TestLimits(t *testing.T) {
	h := NewHandler(stubSession{ok: false}, nil, &config.Config{MaxUploadBytes: 20 << 20})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/limits", nil)
	rec := httptest.NewRecorder()
	h.Limits(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !contains(got, "20971520") || !contains(got, "20 MB") {
		t.Fatalf("body %s", got)
	}
}

func TestPutUnauthorized(t *testing.T) {
	h := NewHandler(stubSession{ok: false}, nil, &config.Config{MaxUploadBytes: 1024})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", nil)
	rec := httptest.NewRecorder()
	h.Put(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPutUnconfigured(t *testing.T) {
	h := NewHandler(stubSession{ok: true}, nil, &config.Config{MaxUploadBytes: 1024})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", nil)
	rec := httptest.NewRecorder()
	h.Put(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()))
}
