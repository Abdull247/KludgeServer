package share

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/go-chi/chi/v5"
)

type stubSource map[string]catalog.SharePreview

func (s stubSource) Share(id string) (catalog.SharePreview, bool) {
	p, ok := s[id]
	return p, ok
}

func testHandler(source Source) chi.Router {
	h := NewHandler(&config.Config{FrontendURL: "https://kludgekit.web.app"}, source)
	r := chi.NewRouter()
	r.Get("/i/{id}", h.Item)
	return r
}

func TestShareCardWithImage(t *testing.T) {
	r := testHandler(stubSource{
		"itm_x": {Title: "<script>alert(1)</script>", Description: "A & b", Image: "https://cdn.example/a.png"},
	})
	req := httptest.NewRequest(http.MethodGet, "/i/itm_x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content type %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`og:image" content="https://cdn.example/a.png`,
		`twitter:card" content="summary_large_image`,
		`url=https://kludgekit.web.app/item?id=itm_x`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Fatal("title not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("expected escaped title")
	}
}

func TestShareCardWithoutImage(t *testing.T) {
	r := testHandler(stubSource{"itm_x": {Title: "Noclip", Description: "desc"}})
	req := httptest.NewRequest(http.MethodGet, "/i/itm_x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `twitter:card" content="summary"`) {
		t.Fatalf("missing summary card: %s", body)
	}
	if strings.Contains(body, "og:image") {
		t.Fatal("og:image should be absent")
	}
}

func TestShareUnknownRedirectsHome(t *testing.T) {
	r := testHandler(stubSource{})
	req := httptest.NewRequest(http.MethodGet, "/i/nope", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://kludgekit.web.app" {
		t.Fatalf("location %q", loc)
	}
}
