package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound   = errors.New("item not found")
	ErrNotPending = errors.New("item is not pending verification")
	ErrNeedReason = errors.New("rejection reason is required")
)

func (s *Store) GetFor(idOrSlug, userID string, admin bool) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.lookup(idOrSlug)
	if !ok {
		return Item{}, false
	}
	it := s.items[idx]
	if !canView(it, userID, admin) {
		return Item{}, false
	}
	now := s.now()
	items := decorate([]Item{it}, now)
	return applyViewer(items[0], userID, admin), true
}

func (s *Store) Pending(q query, typeSlug string) ItemPage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	page, per := pageSize(q, 24)
	items := make([]Item, 0)
	for _, it := range s.items {
		if it.VerificationStatus != VerificationPending {
			continue
		}
		if it.Type == "collection" || it.Type == "creator" {
			continue
		}
		if !matchesTypeFilter(it, typeSlug) {
			continue
		}
		items = append(items, it)
	}
	items = sortFresh(filterItems(decorate(items, now), q.Q))
	return paginate(items, page, per)
}

func (s *Store) Owned(userID, status string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	out := make([]Item, 0)
	for _, it := range s.items {
		if !isOwner(it, userID) {
			continue
		}
		if it.Type == "collection" || it.Type == "creator" {
			continue
		}
		if status != "" && it.VerificationStatus != status {
			continue
		}
		out = append(out, it)
	}
	return decorate(sortFresh(out), now)
}

func (s *Store) SavedBy(userID string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	out := make([]Item, 0)
	for _, it := range s.items {
		if !it.Viewer.Saved {
			continue
		}
		if !canView(it, userID, false) {
			continue
		}
		out = append(out, it)
	}
	return decorate(out, now)
}

func (s *Store) LikedBy(userID string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	out := make([]Item, 0)
	for _, it := range s.items {
		if !it.Viewer.Liked {
			continue
		}
		if !canView(it, userID, false) {
			continue
		}
		out = append(out, it)
	}
	return decorate(out, now)
}

func (s *Store) Verify(ctx context.Context, idOrSlug string) (Item, error) {
	return s.setVerification(ctx, idOrSlug, VerificationVerified, "")
}

func (s *Store) Reject(ctx context.Context, idOrSlug, reason string) (Item, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Item{}, ErrNeedReason
	}
	if len(reason) > 500 {
		return Item{}, fmt.Errorf("rejection reason must be at most 500 characters")
	}
	return s.setVerification(ctx, idOrSlug, VerificationRejected, reason)
}

func (s *Store) setVerification(ctx context.Context, idOrSlug, status, reason string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.lookup(idOrSlug)
	if !ok {
		return Item{}, ErrNotFound
	}
	it := s.items[idx]
	if it.VerificationStatus != VerificationPending {
		return Item{}, ErrNotPending
	}

	now := s.now()
	wasPublic := isPublic(it)
	it.VerificationStatus = status
	it.RejectionReason = reason
	it.UpdatedAt = now
	if status == VerificationVerified {
		it.VerifiedAt = &now
		it.RejectionReason = ""
	}

	if s.db != nil {
		if err := s.updateVerification(ctx, it.ID, status, it.RejectionReason, it.VerifiedAt, now); err != nil {
			return Item{}, fmt.Errorf("%w: %v", ErrPersist, err)
		}
	}

	becamePublic := isPublic(it) && !wasPublic
	if becamePublic {
		if i, ok := s.catByID[it.CategoryID]; ok {
			s.categories[i].ItemCount++
		}
		if i, ok := s.secByID[it.SectionID]; ok {
			s.sections[i].ItemCount++
		}
	}

	enrichItem(&it)
	s.items[idx] = it
	items := decorate([]Item{it}, now)
	return items[0], nil
}

func (s *Store) updateVerification(ctx context.Context, id, status, reason string, verifiedAt *time.Time, updatedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE catalog_items
		SET verification_status = $2,
		    rejection_reason = $3,
		    verified_at = $4,
		    updated_at = $5
		WHERE id = $1
	`, id, status, reason, verifiedAt, updatedAt)
	if err != nil {
		return fmt.Errorf("update verification: %w", err)
	}
	return nil
}

type ProfileStats struct {
	Projects  int `json:"projects"`
	Downloads int `json:"downloads"`
	Likes     int `json:"likes"`
	Saves     int `json:"saves"`
	Pending   int `json:"pending"`
	Rejected  int `json:"rejected"`
}

type ProfileFeed struct {
	Viewer    Viewer       `json:"viewer"`
	Stats     ProfileStats `json:"stats"`
	Pending   []Item       `json:"pending"`
	Rejected  []Item       `json:"rejected"`
	Published []Item       `json:"published"`
	Saved     []Item       `json:"saved"`
	Liked     []Item       `json:"liked"`
}

func (s *Store) Profile(iden Identity) ProfileFeed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	stats := ProfileStats{}
	pending := make([]Item, 0)
	rejected := make([]Item, 0)
	published := make([]Item, 0)
	saved := make([]Item, 0)
	liked := make([]Item, 0)

	for _, it := range s.items {
		owner := isOwner(it, iden.UserID)
		if owner && it.Type != "collection" && it.Type != "creator" {
			switch it.VerificationStatus {
			case VerificationPending:
				pending = append(pending, it)
				stats.Pending++
			case VerificationRejected:
				rejected = append(rejected, it)
				stats.Rejected++
			default:
				published = append(published, it)
				stats.Projects++
				stats.Downloads += downloadsFor(it)
				stats.Likes += it.Stats.Likes
				stats.Saves += it.Stats.Saves
			}
		}
		if it.Viewer.Saved && canView(it, iden.UserID, false) {
			saved = append(saved, it)
		}
		if it.Viewer.Liked && canView(it, iden.UserID, false) {
			liked = append(liked, it)
		}
	}

	return ProfileFeed{
		Viewer: Viewer{
			ID:           iden.UserID,
			Username:     iden.Username,
			FullName:     iden.FullName,
			GreetingName: greetingName(iden.FullName),
			AvatarURL:    iden.AvatarURL,
		},
		Stats:     stats,
		Pending:   decorate(sortFresh(pending), now),
		Rejected:  decorate(sortFresh(rejected), now),
		Published: decorate(sortFresh(published), now),
		Saved:     decorate(saved, now),
		Liked:     decorate(liked, now),
	}
}
