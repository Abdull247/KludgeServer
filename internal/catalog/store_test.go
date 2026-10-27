package catalog

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	s, err := NewStoreFromDir(filepath.Join(filepath.Dir(file), "mocks"))
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }
	return s
}

func TestHomeHasThreeSections(t *testing.T) {
	home := testStore(t).Home(query{Page: 1, PerPage: 4})
	if len(home.Sections) != 3 {
		t.Fatalf("sections: got %d", len(home.Sections))
	}
	if home.Sections[0].ID != "sec_builds" || home.Sections[1].ID != "sec_creators" || home.Sections[2].ID != "sec_apis" {
		t.Fatalf("unexpected section ids: %+v", home.Sections)
	}
	if home.Rails.Trending.Pagination.Total == 0 {
		t.Fatal("expected trending items")
	}
	if len(home.Rails.Trending.Items) == 0 {
		t.Fatal("expected trending rail items")
	}
	first := home.Rails.Trending.Items[0]
	if first.BuildArtwork == "" || first.Name == "" || first.Type == "" {
		t.Fatalf("item missing required fields: %+v", first)
	}
}

func TestCategoryByID(t *testing.T) {
	cat, page, ok := testStore(t).Category("cat_modded_apk", query{Page: 1, PerPage: 12})
	if !ok {
		t.Fatal("expected category")
	}
	if cat.Name != "Modded APK" {
		t.Fatalf("name: %s", cat.Name)
	}
	if page.Pagination.Total == 0 {
		t.Fatal("expected items")
	}
}

func TestGetItemEnriched(t *testing.T) {
	s := testStore(t)
	it, ok := s.Get("itm_noclip")
	if !ok {
		t.Fatal("missing item")
	}
	if it.SizeBytes <= 0 || it.SizeDisplay == "" {
		t.Fatalf("size: %+v", it)
	}
	if it.Downloads != it.Stats.Pulls {
		t.Fatalf("downloads %d", it.Downloads)
	}
	if it.TypeLabel != "Mod APK" {
		t.Fatalf("label %q", it.TypeLabel)
	}
	if it.Version == "" {
		t.Fatal("expected version")
	}
}

func TestRecommendsRelated(t *testing.T) {
	s := testStore(t)
	res, ok := s.Recommends("itm_frida", 6)
	if !ok {
		t.Fatal("missing item")
	}
	if len(res.Items) == 0 {
		t.Fatal("expected related")
	}
	if res.Items[0].Type != "frida" && res.Items[0].CategoryID != "cat_frida" {
		t.Fatalf("expected related first, got %+v", res.Items[0])
	}
	for _, it := range res.Items {
		if it.ID == "itm_frida" {
			t.Fatal("seed in recommends")
		}
	}
}

func TestToggleSave(t *testing.T) {
	s := testStore(t)
	before, ok := s.Toggle("itm_orbit", "save")
	if !ok {
		t.Fatal("missing item")
	}
	if !before.Viewer.Saved {
		t.Fatal("expected saved")
	}
	after, _ := s.Toggle("itm_orbit", "save")
	if after.Viewer.Saved {
		t.Fatal("expected unsaved")
	}
	if after.Stats.Saves != before.Stats.Saves-1 {
		t.Fatalf("saves: %d -> %d", before.Stats.Saves, after.Stats.Saves)
	}
}

func TestTrendingPrefersFreshSaves(t *testing.T) {
	s := testStore(t)
	rail := s.Trending(query{Page: 1, PerPage: 8})
	if len(rail.Items) < 2 {
		t.Fatal("need items")
	}
	if rail.Items[0].TrendingScore < rail.Items[1].TrendingScore {
		t.Fatalf("not sorted: %v %v", rail.Items[0].TrendingScore, rail.Items[1].TrendingScore)
	}
}

func TestSearchFilters(t *testing.T) {
	page := testStore(t).Search(query{Page: 1, PerPage: 20, Q: "frida"})
	if page.Pagination.Total == 0 {
		t.Fatal("expected frida hits")
	}
}
