package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/catalog"
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

type stubIdentity struct {
	iden catalog.Identity
	ok   bool
}

func (s stubIdentity) ByUserID(context.Context, uuid.UUID) (catalog.Identity, bool) {
	return s.iden, s.ok
}

func adminHandler(t *testing.T, emails []string, identities catalog.IdentityLookup, authed bool) *Handler {
	t.Helper()
	s := testCatalogStore(t)
	return NewHandler(s, nil, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: authed}, identities, emails)
}

func testCatalogStore(t *testing.T) *catalog.Store {
	t.Helper()
	s, err := catalog.NewStoreFromMocks()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAdminHomeUnauthorized(t *testing.T) {
	h := adminHandler(t, []string{"ada@kludge.dev"}, nil, false)
	rec := httptest.NewRecorder()
	h.Home(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/home", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAdminHomeForbiddenForNonAdmin(t *testing.T) {
	s := testCatalogStore(t)
	h := NewHandler(s, nil, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: true}, stubIdentity{
		iden: catalog.Identity{UserID: "usr_x", Email: "human@example.com", Username: "human"},
		ok:   true,
	}, []string{"admin@kludge.dev"})
	rec := httptest.NewRecorder()
	h.Home(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/home", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAdminHomeOKForMockAdmin(t *testing.T) {
	h := adminHandler(t, []string{"ada@kludge.dev"}, nil, true)
	r := chi.NewRouter()
	r.Get("/api/v1/admin/home", h.Home)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/home", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data AdminHome `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Admin || env.Data.Stats.Projects == 0 || env.Data.Stats.Users == 0 {
		t.Fatalf("unexpected home: %+v", env.Data)
	}
	foundAdmin := false
	foundCreator := false
	for _, u := range env.Data.Users {
		if u.Username == "ada" && u.Role == "admin" && u.Email == "ada@kludge.dev" {
			foundAdmin = true
		}
		if u.Username == "ken" && u.ProjectCount > 0 {
			foundCreator = true
		}
	}
	if !foundAdmin {
		t.Fatal("mock admin (ada) missing from users")
	}
	if !foundCreator {
		t.Fatal("mock creator (ken) missing or has no project count")
	}
	if env.Data.Viewer.Username != "ada" || env.Data.Viewer.GreetingName != "Ada" {
		t.Fatalf("unexpected viewer: %+v", env.Data.Viewer)
	}
}

func TestAdminUsersAndProjects(t *testing.T) {
	h := adminHandler(t, []string{"ada@kludge.dev"}, nil, true)
	r := chi.NewRouter()
	r.Get("/api/v1/admin/users", h.Users)
	r.Get("/api/v1/admin/projects", h.Projects)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("users status %d", rec.Code)
	}
	var users struct {
		Data UserList `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	if users.Data.Total == 0 || len(users.Data.Users) != users.Data.Total {
		t.Fatalf("unexpected users: %+v", users.Data)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("projects status %d", rec.Code)
	}
	var projects struct {
		Data ProjectList `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatal(err)
	}
	if projects.Data.Total == 0 {
		t.Fatal("expected projects")
	}
	for _, p := range projects.Data.Projects {
		if p.Kind != "build" && p.Kind != "api" {
			t.Fatalf("non-project kind leaked: %s", p.Kind)
		}
	}
}
