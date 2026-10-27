package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

type stubIdentity struct {
	iden Identity
	ok   bool
}

func (s stubIdentity) ByUserID(context.Context, uuid.UUID) (Identity, bool) {
	return s.iden, s.ok
}

func testHandler(t *testing.T, authed bool) *Handler {
	t.Helper()
	s := testStore(t)
	return NewHandler(s, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: authed}, nil, "ada@kludge.dev")
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

func TestTypesRoute(t *testing.T) {
	h := testHandler(t, false)
	r := chi.NewRouter()
	r.Get("/api/v1/types", h.Types)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/types", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []ItemTypeDef `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 12 {
		t.Fatalf("types %d", len(env.Data))
	}
	var apk *ItemTypeDef
	for i := range env.Data {
		if env.Data[i].Slug == "modded_apk" {
			apk = &env.Data[i]
			break
		}
	}
	if apk == nil || apk.GalleryKind != GalleryScreenshots {
		t.Fatalf("apk type gallery: %+v", apk)
	}
}

func TestTagsRoute(t *testing.T) {
	h := testHandler(t, false)
	r := chi.NewRouter()
	r.Get("/api/v1/tags", h.Tags)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []Tag `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) < 8 {
		t.Fatalf("tags %d", len(env.Data))
	}
	if env.Data[0].Slug == "" || env.Data[0].Label == "" {
		t.Fatalf("empty tag: %+v", env.Data[0])
	}
}

func createRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/items", h.CreateItem)
	r.Post("/items/record", h.RecordItem)
	return r
}

func newCreateBody(name, itemType string) *strings.Reader {
	return strings.NewReader(`{
		"name": ` + quote(name) + `,
		"description": "A freshly recorded build with hashes on file.",
		"type": ` + quote(itemType) + `,
		"artworkUrl": "https://images.example.com/art/cover.png",
		"bannerUrls": ["https://images.example.com/art/wide.png"],
		"tags": ["Weekend Drop", " beta "]
	}`)
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestCreateItemUnauthorized(t *testing.T) {
	h := testHandler(t, false)
	req := httptest.NewRequest(http.MethodPost, "/items", newCreateBody("Orbit Reborn", "modded_apk"))
	rec := httptest.NewRecorder()
	h.CreateItem(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCreateItemOK(t *testing.T) {
	h := testHandler(t, true)
	r := createRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/items", newCreateBody("Orbit Reborn", "modded_apk"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data Item `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.ID == "" || env.Data.Slug == "" || env.Data.Slug != "orbit-reborn" {
		t.Fatalf("unexpected item: %+v", env.Data)
	}
	if env.Data.CategoryID != "cat_modded_apk" || env.Data.SectionID != "sec_builds" {
		t.Fatalf("taxonomy miss: %+v", env.Data)
	}
	if env.Data.GalleryKind != GalleryScreenshots {
		t.Fatalf("apk gallery kind: %q", env.Data.GalleryKind)
	}
	if env.Data.Publisher == nil || env.Data.Publisher.Kind != PublisherKindUser || env.Data.Publisher.Username != "ada" {
		t.Fatalf("unexpected publisher: %+v", env.Data.Publisher)
	}
	if len(env.Data.Tags) != 2 || env.Data.Tags[0] != "weekend drop" {
		t.Fatalf("unexpected tags: %+v", env.Data.Tags)
	}

	home := h.store.Home(query{Page: 1, PerPage: 50})
	found := false
	for _, sec := range home.Sections {
		for _, it := range sec.Feed.Items {
			if it.ID == env.Data.ID {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("created item missing from home feed")
	}
}

func TestCreateItemUnknownType(t *testing.T) {
	h := testHandler(t, true)
	req := httptest.NewRequest(http.MethodPost, "/items", newCreateBody("Orbit Reborn", "totally_bogus"))
	rec := httptest.NewRecorder()
	h.CreateItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateItemBadPayload(t *testing.T) {
	h := testHandler(t, true)
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name": "x"}`))
	rec := httptest.NewRecorder()
	h.CreateItem(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRecordItemNotConfigured(t *testing.T) {
	s := testStore(t)
	h := NewHandler(s, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: true}, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/items/record", newCreateBody("Ingest Drop", "unofficial_api"))
	rec := httptest.NewRecorder()
	h.RecordItem(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRecordItemForbiddenForNonAutomation(t *testing.T) {
	s := testStore(t)
	h := NewHandler(s, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: true}, stubIdentity{
		iden: Identity{UserID: "usr_x", Email: "human@example.com", Username: "human"},
		ok:   true,
	}, "ingest@kludge.dev")
	req := httptest.NewRequest(http.MethodPost, "/items/record", newCreateBody("Ingest Drop", "unofficial_api"))
	rec := httptest.NewRecorder()
	h.RecordItem(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRecordItemOKForAutomation(t *testing.T) {
	s := testStore(t)
	h := NewHandler(s, stubSession{id: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ok: true}, stubIdentity{
		iden: Identity{UserID: "usr_bot", Email: "ingest@kludge.dev", Username: "kludge-ingest", FullName: "Kludge Ingest"},
		ok:   true,
	}, "Ingest@Kludge.dev")
	r := createRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/items/record", newCreateBody("Ingest Drop", "unofficial_api"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data Item `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Type != "unofficial_api" || env.Data.CategoryID != "cat_agentic_api" || env.Data.SectionID != "sec_apis" {
		t.Fatalf("taxonomy miss: %+v", env.Data)
	}
	if env.Data.GalleryKind != GalleryBanners {
		t.Fatalf("api gallery kind: %q", env.Data.GalleryKind)
	}
	if env.Data.Publisher == nil || env.Data.Publisher.Kind != PublisherKindAutomation {
		t.Fatalf("unexpected publisher: %+v", env.Data.Publisher)
	}
}
