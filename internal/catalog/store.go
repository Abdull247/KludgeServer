package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed mocks/*.json
var mocksFS embed.FS

type Store struct {
	mu         sync.RWMutex
	now        func() time.Time
	db         *pgxpool.Pool
	viewer     Viewer
	sections   []Section
	categories []Category
	items      []Item
	byID       map[string]int
	bySlug     map[string]int
	catByID    map[string]int
	secByID    map[string]int
	secBySlug  map[string]int
}

type Query struct {
	Page    int
	PerPage int
	Q       string
	Sort    string
}

type query = Query

// NewStoreFromMocks seeds the store with the bundled demo fixtures. Used in
// test mode and by tests.
func NewStoreFromMocks() (*Store, error) {
	return newStore(true)
}

// NewStoreLive builds the store without demo items or a demo viewer: items
// come from the attached database and the viewer from the session identity.
// Sections and categories are still the embedded taxonomy.
func NewStoreLive() (*Store, error) {
	return newStore(false)
}

func newStore(seedItems bool) (*Store, error) {
	s := &Store{now: time.Now}
	if err := readEmbedded("mocks/sections.json", &s.sections); err != nil {
		return nil, err
	}
	if err := readEmbedded("mocks/categories.json", &s.categories); err != nil {
		return nil, err
	}
	if seedItems {
		if err := readEmbedded("mocks/viewer.json", &s.viewer); err != nil {
			return nil, err
		}
		if err := readEmbedded("mocks/items.json", &s.items); err != nil {
			return nil, err
		}
	}
	s.index()
	s.hydrate()
	return s, nil
}

func NewStoreFromDir(dir string) (*Store, error) {
	s := &Store{now: time.Now}
	if err := readFileJSON(filepath.Join(dir, "viewer.json"), &s.viewer); err != nil {
		return nil, err
	}
	if err := readFileJSON(filepath.Join(dir, "categories.json"), &s.categories); err != nil {
		return nil, err
	}
	if err := readFileJSON(filepath.Join(dir, "sections.json"), &s.sections); err != nil {
		return nil, err
	}
	if err := readFileJSON(filepath.Join(dir, "items.json"), &s.items); err != nil {
		return nil, err
	}
	s.index()
	s.hydrate()
	return s, nil
}

func (s *Store) index() {
	s.byID = make(map[string]int, len(s.items))
	s.bySlug = make(map[string]int, len(s.items))
	s.catByID = make(map[string]int, len(s.categories))
	s.secByID = make(map[string]int, len(s.sections))
	s.secBySlug = make(map[string]int, len(s.sections))
	for i, it := range s.items {
		s.byID[it.ID] = i
		s.bySlug[it.Slug] = i
	}
	for i, c := range s.categories {
		s.catByID[c.ID] = i
		s.catByID[c.Slug] = i
	}
	for i, sec := range s.sections {
		s.secByID[sec.ID] = i
		s.secBySlug[sec.Slug] = i
	}
}

func (s *Store) hydrate() {
	counts := map[string]int{}
	secCounts := map[string]int{}
	for i := range s.items {
		it := &s.items[i]
		if idx, ok := s.catByID[it.CategoryID]; ok {
			c := s.categories[idx]
			it.Category = CategoryRef{ID: c.ID, Slug: c.Slug, Name: c.Name}
		}
		if it.Publisher == nil && it.Creator != nil {
			it.PublisherID = it.Creator.ID
			it.Publisher = &Publisher{
				ID:          it.Creator.ID,
				Username:    it.Creator.Username,
				DisplayName: it.Creator.DisplayName,
				AvatarURL:   it.Creator.AvatarURL,
				Kind:        PublisherKindUser,
				Verified:    it.Creator.Verified,
			}
		}
		if it.Description == "" && it.Blurb != "" {
			it.Description = it.Blurb
		}
		it.Metric = metricFor(*it)
		enrichItem(it)
		if isPublic(*it) {
			counts[it.CategoryID]++
			secCounts[it.SectionID]++
		}
	}
	for i := range s.categories {
		s.categories[i].ItemCount = counts[s.categories[i].ID]
	}
	for i := range s.sections {
		s.sections[i].ItemCount = secCounts[s.sections[i].ID]
	}
}

func (s *Store) Home(q query) Home {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.now()
	page, per := homePageSize(q)

	sections := make([]Section, 0, len(s.sections))
	for _, sec := range s.sections {
		items := s.itemsForSection(sec.ID)
		feed := paginate(filterItems(decorate(items, now), q.Q), page, per)
		trending := paginate(sortTrending(filterItems(cloneItems(items), q.Q), now), 1, 6)
		sections = append(sections, Section{
			ID:         sec.ID,
			Slug:       sec.Slug,
			Kind:       sec.Kind,
			Title:      sec.Title,
			Subtitle:   sec.Subtitle,
			ItemCount:  sec.ItemCount,
			Href:       sec.Href,
			Categories: s.categoriesForSection(sec.ID),
			Feed:       feed,
			Trending:   &trending,
		})
	}

	builds := s.itemsForSection("sec_builds")
	all := decorate(publicItems(cloneItems(s.items)), now)
	collections := filterKind(all, "collection")
	if len(collections) == 0 {
		collections = filterType(all, "collection")
	}

	return Home{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Viewer:      s.viewer,
		Search: SearchHints{
			Placeholder: "Search builds, APIs, kits",
			Chips:       s.searchChips(),
		},
		Sections: sections,
		Rails: HomeRails{
			Trending:    railFrom("rail_trending", "trending", "Trending this week", "Ranked by recency and saves.", "recency_saves", "/api/v1/trending", sortTrending(builds, now), 1, 8),
			Dropped:     railFrom("rail_dropped", "dropped", "Just dropped", "Newest first.", "published_at_desc", "/api/v1/dropped", sortFresh(builds), 1, 8),
			Collections: railFrom("rail_collections", "collections", "Collections to steal", "Packed shelves from people who ship.", "saves_desc", "/api/v1/collections", collections, 1, 8),
		},
	}
}

func (s *Store) Section(idOrSlug string, q query) (Section, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sec, ok := s.section(idOrSlug)
	if !ok {
		return Section{}, false
	}
	now := s.now()
	page, per := pageSize(q, 12)
	items := filterItems(decorate(s.itemsForSection(sec.ID), now), q.Q)
	if q.Sort == "trending" {
		items = sortTrending(items, now)
	} else if q.Sort == "fresh" || q.Sort == "new" {
		items = sortFresh(items)
	}
	feed := paginate(items, page, per)
	trending := paginate(sortTrending(cloneItems(items), now), 1, 8)
	sec.Categories = s.categoriesForSection(sec.ID)
	sec.Feed = feed
	sec.Trending = &trending
	return sec, true
}

func (s *Store) Category(idOrSlug string, q query) (Category, ItemPage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.catByID[idOrSlug]
	if !ok {
		return Category{}, ItemPage{}, false
	}
	cat := s.categories[idx]
	now := s.now()
	page, per := pageSize(q, 12)
	items := filterItems(decorate(s.itemsForCategory(cat.ID), now), q.Q)
	if q.Sort == "trending" {
		items = sortTrending(items, now)
	} else {
		items = sortFresh(items)
	}
	return cat, paginate(items, page, per), true
}

func (s *Store) Trending(q query) Rail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	page, per := pageSize(q, 12)
	items := sortTrending(filterItems(s.itemsForSection("sec_builds"), q.Q), now)
	return railFrom("rail_trending", "trending", "Trending this week", "Ranked by recency and saves.", "recency_saves", "/api/v1/trending", items, page, per)
}

func (s *Store) Dropped(q query) Rail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	page, per := pageSize(q, 12)
	items := sortFresh(filterItems(s.itemsForSection("sec_builds"), q.Q))
	return railFrom("rail_dropped", "dropped", "Just dropped", "Newest first.", "published_at_desc", "/api/v1/dropped", items, page, per)
}

func (s *Store) Collections(q query) Rail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	page, per := pageSize(q, 12)
	items := filterType(filterItems(decorate(publicItems(cloneItems(s.items)), now), q.Q), "collection")
	return railFrom("rail_collections", "collections", "Collections to steal", "Packed shelves from people who ship.", "saves_desc", "/api/v1/collections", items, page, per)
}

func (s *Store) Search(q query) ItemPage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	page, per := pageSize(q, 12)
	items := sortTrending(filterItems(decorate(publicItems(cloneItems(s.items)), now), q.Q), now)
	return paginate(items, page, per)
}

func (s *Store) Toggle(id, action string) (ToggleResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.byID[id]
	if !ok {
		idx, ok = s.bySlug[id]
	}
	if !ok {
		return ToggleResult{}, false
	}
	it := s.items[idx]
	switch action {
	case "save":
		it.Viewer.Saved = !it.Viewer.Saved
		if it.Viewer.Saved {
			it.Stats.Saves++
		} else if it.Stats.Saves > 0 {
			it.Stats.Saves--
		}
	case "like":
		it.Viewer.Liked = !it.Viewer.Liked
		if it.Viewer.Liked {
			it.Stats.Likes++
		} else if it.Stats.Likes > 0 {
			it.Stats.Likes--
		}
	default:
		return ToggleResult{}, false
	}
	it.Metric = metricFor(it)
	enrichItem(&it)
	s.items[idx] = it
	return ToggleResult{ID: it.ID, Viewer: it.Viewer, Stats: it.Stats, Metric: it.Metric}, true
}

// Items returns a snapshot of every catalog item, creators and collections
// included. Admin tooling slices it by kind; entry-points must not mutate the
// returned items.
func (s *Store) Items() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, len(s.items))
	copy(out, s.items)
	return out
}

// Creators returns the deduped people referenced by catalog item creators.
func (s *Store) Creators() []CreatorRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool, 8)
	out := make([]CreatorRef, 0, 8)
	for _, it := range s.items {
		if it.Creator == nil || seen[it.Creator.ID] {
			continue
		}
		seen[it.Creator.ID] = true
		out = append(out, *it.Creator)
	}
	return out
}

func (s *Store) section(idOrSlug string) (Section, bool) {
	if i, ok := s.secByID[idOrSlug]; ok {
		return s.sections[i], true
	}
	if i, ok := s.secBySlug[idOrSlug]; ok {
		return s.sections[i], true
	}
	return Section{}, false
}

func (s *Store) itemsForSection(sectionID string) []Item {
	out := make([]Item, 0)
	for _, it := range s.items {
		if it.SectionID == sectionID && it.Type != "collection" && isPublic(it) {
			out = append(out, it)
		}
	}
	return out
}

func (s *Store) itemsForCategory(categoryID string) []Item {
	out := make([]Item, 0)
	for _, it := range s.items {
		if it.CategoryID == categoryID && isPublic(it) {
			out = append(out, it)
		}
	}
	return out
}

func (s *Store) categoriesForSection(sectionID string) []Category {
	out := make([]Category, 0)
	for _, c := range s.categories {
		if c.SectionID == sectionID {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) searchChips() []CategoryRef {
	out := make([]CategoryRef, 0, 6)
	for _, c := range s.categories {
		if c.SectionID == "sec_builds" || c.SectionID == "sec_apis" {
			out = append(out, CategoryRef{ID: c.ID, Slug: c.Slug, Name: c.Name})
		}
		if len(out) == 6 {
			break
		}
	}
	return out
}

func decorate(items []Item, now time.Time) []Item {
	out := cloneItems(items)
	for i := range out {
		out[i].TrendingScore = trendingScore(out[i], now)
		out[i].Metric = metricFor(out[i])
		enrichItem(&out[i])
	}
	return out
}

func filterItems(items []Item, q string) []Item {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return items
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		blob := strings.ToLower(strings.Join([]string{
			it.Name, it.Blurb, it.Type, it.Kind, it.Category.Name, strings.Join(it.Tags, " "),
		}, " "))
		if strings.Contains(blob, q) {
			out = append(out, it)
		}
	}
	return out
}

func filterType(items []Item, t string) []Item {
	out := make([]Item, 0)
	for _, it := range items {
		if it.Type == t {
			out = append(out, it)
		}
	}
	return out
}

func filterKind(items []Item, t string) []Item {
	out := make([]Item, 0)
	for _, it := range items {
		if it.Type == t {
			out = append(out, it)
		}
	}
	return out
}

func ParseQuery(pageS, perS, q, sort string) Query {
	page, _ := strconv.Atoi(pageS)
	per, _ := strconv.Atoi(perS)
	return Query{Page: page, PerPage: per, Q: q, Sort: sort}
}

func parseQuery(pageS, perS, q, sort string) query {
	return ParseQuery(pageS, perS, q, sort)
}

func pageSize(q query, fallback int) (int, int) {
	page := q.Page
	per := q.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = fallback
	}
	return page, per
}

func homePageSize(q query) (int, int) {
	page := q.Page
	per := q.PerPage
	if page < 1 {
		page = 1
	}
	if per < 1 {
		per = 4
	}
	return page, per
}

func readEmbedded(path string, dest any) error {
	b, err := mocksFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func readFileJSON(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, dest); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
