package catalog

import "strings"

// SharePreview carries what share-card crawlers need for one item.
type SharePreview struct {
	Title       string
	Description string
	Image       string
}

// clampShareText keeps card copy inside what crawlers display without
// truncating mid-word more than necessary.
func clampShareText(s string, max int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, ' '); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:-") + "…"
}

// Share resolves the public preview details for an item. Only public
// (verified) items resolve so pending or rejected builds never leak.
func (s *Store) Share(idOrSlug string) (SharePreview, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.lookup(idOrSlug)
	if !ok {
		return SharePreview{}, false
	}
	it := s.items[idx]
	if !isPublic(it) {
		return SharePreview{}, false
	}

	image := it.BuildArtwork
	if image == "" && len(it.Banners) > 0 {
		image = it.Banners[0]
	}

	return SharePreview{
		Title:       clampShareText(it.Name, 80),
		Description: clampShareText(it.Blurb, 200),
		Image:       image,
	}, true
}
