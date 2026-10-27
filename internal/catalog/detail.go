package catalog

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "0"
	}
	const kb = 1024.0
	const mb = 1024.0 * 1024.0
	const gb = 1024.0 * 1024.0 * 1024.0
	switch {
	case float64(bytes) >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/gb)
	case float64(bytes) >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
	default:
		return fmt.Sprintf("%.0f KB", float64(bytes)/kb)
	}
}

func derivedSize(it Item) int64 {
	if it.SizeBytes > 0 {
		return it.SizeBytes
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(it.ID + "|" + it.Type))
	n := h.Sum32()
	if IsAPK(it.Type) {
		return int64(12+n%60)*1024*1024 + int64(n%512)*1024
	}
	if it.Kind == "api" {
		return int64(180+n%820) * 1024
	}
	return int64(2+n%24)*1024*1024 + int64(n%400)*1024
}

func downloadsFor(it Item) int {
	if it.Stats.Pulls > 0 {
		return it.Stats.Pulls
	}
	if it.Stats.Runs > 0 {
		return it.Stats.Runs
	}
	return it.Stats.Saves
}

func enrichItem(it *Item) {
	if it.SizeBytes <= 0 {
		it.SizeBytes = derivedSize(*it)
	}
	it.SizeDisplay = formatSize(it.SizeBytes)
	it.Downloads = downloadsFor(*it)
	if def, ok := LookupType(it.Type); ok {
		it.TypeLabel = def.Label
	}
	if strings.TrimSpace(it.Version) == "" {
		it.Version = "1.0.0"
	}
	if it.GalleryKind == "" {
		it.GalleryKind = GalleryKindFor(it.Type)
	}
	if it.Banners == nil {
		it.Banners = []string{}
	}
	if it.Tags == nil {
		it.Tags = []string{}
	}
}

func (s *Store) lookup(idOrSlug string) (int, bool) {
	if i, ok := s.byID[idOrSlug]; ok {
		return i, true
	}
	if i, ok := s.bySlug[idOrSlug]; ok {
		return i, true
	}
	return 0, false
}

func (s *Store) Get(idOrSlug string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.lookup(idOrSlug)
	if !ok {
		return Item{}, false
	}
	now := s.now()
	items := decorate([]Item{s.items[idx]}, now)
	return items[0], true
}

type RecommendResult struct {
	SeedID string `json:"seedId"`
	Items  []Item `json:"items"`
}

func sharedTags(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[strings.ToLower(t)] = struct{}{}
	}
	n := 0
	for _, t := range b {
		if _, ok := set[strings.ToLower(t)]; ok {
			n++
		}
	}
	return n
}

func relatedScore(seed, it Item) int {
	n := 0
	if it.Type == seed.Type {
		n += 6
	}
	if it.CategoryID == seed.CategoryID {
		n += 5
	}
	if it.SectionID == seed.SectionID {
		n += 2
	}
	n += sharedTags(seed.Tags, it.Tags) * 3
	return n
}

func (s *Store) Recommends(idOrSlug string, limit int) (RecommendResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.lookup(idOrSlug)
	if !ok {
		return RecommendResult{}, false
	}
	if limit < 1 {
		limit = 8
	}
	if limit > 16 {
		limit = 16
	}

	seed := s.items[idx]
	now := s.now()
	type scored struct {
		it Item
		n  int
	}
	cand := make([]scored, 0, len(s.items))
	for i, it := range s.items {
		if i == idx {
			continue
		}
		if it.Type == "collection" && seed.Type != "collection" {
			continue
		}
		n := relatedScore(seed, it)
		if n == 0 {
			continue
		}
		cand = append(cand, scored{it: it, n: n})
	}
	sort.SliceStable(cand, func(i, j int) bool {
		if cand[i].n == cand[j].n {
			return trendingScore(cand[i].it, now) > trendingScore(cand[j].it, now)
		}
		return cand[i].n > cand[j].n
	})

	out := make([]Item, 0, limit)
	seen := map[string]bool{seed.ID: true}
	for _, c := range cand {
		if seen[c.it.ID] {
			continue
		}
		seen[c.it.ID] = true
		out = append(out, c.it)
		if len(out) == limit {
			break
		}
	}

	if len(out) < limit {
		extras := sortTrending(cloneItems(s.items), now)
		for _, it := range extras {
			if seen[it.ID] || (it.Type == "collection" && seed.Type != "collection") {
				continue
			}
			seen[it.ID] = true
			out = append(out, it)
			if len(out) == limit {
				break
			}
		}
	}

	return RecommendResult{SeedID: seed.ID, Items: decorate(out, now)}, true
}
