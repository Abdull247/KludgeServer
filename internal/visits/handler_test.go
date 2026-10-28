package visits

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type stubSessions struct {
	uid uuid.UUID
	ok  bool
}

func (s stubSessions) UserID(*http.Request) (uuid.UUID, bool) {
	return s.uid, s.ok
}

type fakeLookup struct {
	iden catalog.Identity
	ok   bool
}

func (f fakeLookup) ByUserID(context.Context, uuid.UUID) (catalog.Identity, bool) {
	return f.iden, f.ok
}

var guestIdentity = catalog.Identity{
	UserID:   "usr_guest",
	Email:    "guest@example.com",
	Username: "weekday",
	FullName: "Weekday Guest",
}

func testRouter(lookup catalog.IdentityLookup, sessions stubSessions) (chi.Router, *Store) {
	store := NewStore(nil)
	items, _ := catalog.NewStoreFromMocks()
	h := NewHandler(store, sessions, lookup, []string{"ada@kludge.dev"}, items)
	r := chi.NewRouter()
	r.Post("/items/{id}/visit", h.Record)
	r.Get("/admin/visitors", h.AdminList)
	r.Get("/admin/visitors/{id}", h.AdminGet)
	return r, store
}

var adminSession = stubSessions{uid: uuid.MustParse("00000000-0000-0000-0000-00000000000a"), ok: true}

func TestRecordGuestVisit(t *testing.T) {
	r, store := testRouter(nil, stubSessions{})
	req := httptest.NewRequest("POST", "/items/itm_noclip/visit", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14)")
	req.Header.Set("Referer", "https://t.me/kludge")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("record guest: status %d", rec.Code)
	}
	page, err := store.List(context.Background(), 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Visitors) != 1 {
		t.Fatalf("record guest: total %d rows %d", page.Total, len(page.Visitors))
	}
	v := page.Visitors[0]
	if v.Authenticated || v.Email != "" || v.Name != "" {
		t.Fatalf("guest row carries identity: %+v", v)
	}
	if v.ItemID != "itm_noclip" || v.ItemName != "Noclip" {
		t.Fatalf("guest row item: %s", v.ItemID)
	}
	if v.UserAgent != "Mozilla/5.0 (Linux; Android 14)" || v.Referer != "https://t.me/kludge" || v.Language != "en-US,en;q=0.9" {
		t.Fatalf("guest row browser info: %+v", v)
	}
	if v.IPAddress == "" || v.ID == "" || v.CreatedAt.IsZero() {
		t.Fatalf("guest row metadata: %+v", v)
	}
}

func TestRecordSignedInVisit(t *testing.T) {
	r, store := testRouter(nil, adminSession)
	req := httptest.NewRequest("POST", "/items/itm_noclip/visit", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("record signed in: status %d", rec.Code)
	}
	page, _ := store.List(context.Background(), 1, 10, "")
	v := page.Visitors[0]
	if !v.Authenticated {
		t.Fatal("signed in row marked as guest")
	}
	iden := catalog.TestIdentity()
	if v.Email != iden.Email || v.Name != iden.FullName || v.Username != iden.Username {
		t.Fatalf("signed in row profile: %+v", v)
	}
	if v.UserID != iden.UserID {
		t.Fatalf("signed in row user id: %q", v.UserID)
	}
	if v.UserAgent != "Mozilla/5.0" {
		t.Fatalf("signed in row should still keep browser info: %+v", v)
	}
}

func TestRecordUnknownItem(t *testing.T) {
	r, store := testRouter(nil, adminSession)
	req := httptest.NewRequest("POST", "/items/itm_zzz/visit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown item: status %d", rec.Code)
	}
	page, _ := store.List(context.Background(), 1, 10, "")
	if page.Total != 0 {
		t.Fatalf("unknown item should not log: %d rows", page.Total)
	}
}

func TestHeaderValuesAreClipped(t *testing.T) {
	r, store := testRouter(nil, adminSession)
	req := httptest.NewRequest("POST", "/items/itm_noclip/visit", nil)
	req.Header.Set("User-Agent", strings.Repeat("x", 900))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("record capped: status %d", rec.Code)
	}
	page, _ := store.List(context.Background(), 1, 10, "")
	if got := len(page.Visitors[0].UserAgent); got != maxText {
		t.Fatalf("user agent len %d, want %d", got, maxText)
	}
}

func TestAdminGate(t *testing.T) {
	lookup := fakeLookup{iden: guestIdentity, ok: true}
	r, _ := testRouter(lookup, adminSession)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/visitors", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list: status %d", rec.Code)
	}

	anon := NewHandler(NewStore(nil), stubSessions{}, nil, []string{"ada@kludge.dev"}, nil)
	anonRouter := chi.NewRouter()
	anonRouter.Get("/admin/visitors", anon.AdminList)
	rec = httptest.NewRecorder()
	anonRouter.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/visitors", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list: status %d", rec.Code)
	}
}

func TestAdminListAndGet(t *testing.T) {
	r, store := testRouter(nil, adminSession)
	for range 2 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/items/itm_noclip/visit", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("seed visit: status %d", rec.Code)
		}
	}
	// A second item so the q filter has something to exclude.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/items/itm_noclip/visit", nil)
	req.Header.Set("User-Agent", "curl/8")
	req.Header.Set("Accept-Language", "de-DE")
	r.ServeHTTP(rec, req)

	page, _ := store.List(context.Background(), 1, 10, "")
	if page.Total != 3 || page.Pagination.TotalPages != 1 {
		t.Fatalf("list totals: %+v", page.Pagination)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/visitors", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list: status %d", rec.Code)
	}
	var listed struct {
		Data Page `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	if listed.Data.Total != 3 || len(listed.Data.Visitors) != 3 {
		t.Fatalf("admin list body: %+v", listed.Data)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/visitors?q=de-de", nil))
	_ = json.Unmarshal(rec.Body.Bytes(), &listed)
	if listed.Data.Total != 1 || listed.Data.Visitors[0].Language != "de-DE" {
		t.Fatalf("admin list filter: %+v", listed.Data)
	}

	seeded, _ := store.List(context.Background(), 1, 10, "")
	v, err := store.Get(context.Background(), seeded.Visitors[0].ID)
	if err != nil || v.ItemID != "itm_noclip" {
		t.Fatalf("admin get: %+v err %v", v, err)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/visitors/not-a-uuid", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin get invalid id: status %d", rec.Code)
	}
}
