package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type stubSession struct {
	id uuid.UUID
	ok bool
}

func (s stubSession) UserID(*http.Request) (uuid.UUID, bool) {
	return s.id, s.ok
}

func testHandler(t *testing.T, authed bool) *Handler {
	t.Helper()
	s := testStore(t)
	return NewHandler(s, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: authed})
}

func TestHomeUnauthorized(t *testing.T) {
	h := testHandler(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	rec := httptest.NewRecorder()
	h.Home(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHomeOK(t *testing.T) {
	h := testHandler(t, true)
	r := chi.NewRouter()
	r.Get("/api/v1/home", h.Home)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data Home `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data.Sections) != 3 {
		t.Fatalf("sections %d", len(env.Data.Sections))
	}
}

func TestCategoryRoute(t *testing.T) {
	h := testHandler(t, true)
	r := chi.NewRouter()
	r.Get("/api/v1/categories/{id}", h.Category)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories/cat_frida", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
