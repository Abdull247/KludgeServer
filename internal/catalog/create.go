package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PublisherKindUser       = "user"
	PublisherKindAutomation = "automation"
)

type Publisher struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	Kind        string `json:"kind"`
	Verified    bool   `json:"verified"`
}

type CreateInput struct {
	Name        string
	Description string
	Type        string
	Version     string
	ArtworkURL  string
	BannerURLs  []string
	Tags        []string
	SourceURL   string
	PublishedAt *time.Time
}

var ErrUnknownType = errors.New("unknown item type")
var ErrPersist = errors.New("persist failed")

func validateURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func normalizeURLs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" && len(v) <= 32 {
			out = append(out, v)
		}
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "\u2026"
}

func firstUpper(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "K"
	}
	return strings.ToUpper(string([]rune(s)[0]))
}

func toneFor(n int) string {
	return string(rune('a' + n%6))
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomSuffix(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			idx = big.NewInt(time.Now().UnixNano() % int64(len(alphabet)))
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b)
}

func (s *Store) uniqueSlug(name string) string {
	base := slugify(name)
	if base == "" {
		base = "item"
	}
	slug := base
	for i := 0; i < 8; i++ {
		if _, taken := s.bySlug[slug]; !taken {
			return slug
		}
		slug = fmt.Sprintf("%s-%s", base, randomSuffix(4))
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

func (s *Store) Create(ctx context.Context, in CreateInput, pub Publisher) (Item, error) {
	name := strings.TrimSpace(in.Name)
	if len(name) < 2 || len(name) > 80 {
		return Item{}, fmt.Errorf("name must be 2-80 characters")
	}
	desc := strings.TrimSpace(in.Description)
	if len(desc) < 1 || len(desc) > 4000 {
		return Item{}, fmt.Errorf("description must be 1-4000 characters")
	}
	if len(in.Version) > 32 {
		return Item{}, fmt.Errorf("version must be at most 32 characters")
	}
	if !validateURL(in.ArtworkURL) {
		return Item{}, fmt.Errorf("artworkUrl must be a valid http(s) URL")
	}
	banners := normalizeURLs(in.BannerURLs)
	if len(banners) > 8 {
		return Item{}, fmt.Errorf("bannerUrls supports at most 8 images")
	}
	for _, b := range banners {
		if !validateURL(b) {
			return Item{}, fmt.Errorf("each bannerUrl must be a valid http(s) URL")
		}
	}
	tags := normalizeTags(in.Tags)
	if len(tags) > 8 {
		return Item{}, fmt.Errorf("tags supports at most 8 entries")
	}
	if in.SourceURL != "" && !validateURL(in.SourceURL) {
		return Item{}, fmt.Errorf("sourceUrl must be a valid http(s) URL")
	}

	def, ok := LookupType(in.Type)
	if !ok {
		return Item{}, ErrUnknownType
	}
	if pub.Kind != PublisherKindUser && pub.Kind != PublisherKindAutomation {
		return Item{}, fmt.Errorf("publisher kind must be user or automation")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	published := now
	if in.PublishedAt != nil {
		published = *in.PublishedAt
	}

	it := Item{
		ID:           "itm_" + slugify(name) + randomSuffix(4),
		Slug:         s.uniqueSlug(name),
		Name:         name,
		Type:         def.Slug,
		Kind:         def.Kind,
		Mark:         firstUpper(name),
		Tone:         toneFor(len(s.items)),
		Blurb:        truncate(desc, 180),
		Description:  desc,
		Version:      strings.TrimSpace(in.Version),
		BuildArtwork: in.ArtworkURL,
		Banners:      banners,
		GalleryKind:  def.GalleryKind,
		CategoryID:   def.CategoryID,
		SectionID:    def.SectionID,
		Publisher:    &pub,
		SourceURL:    strings.TrimSpace(in.SourceURL),
		Tags:         tags,
		PublishedAt:  published,
		UpdatedAt:    now,
	}
	it.Metric = metricFor(it)

	if s.db != nil {
		var pid *uuid.UUID
		if u, err := uuid.Parse(pub.ID); err == nil {
			pid = &u
		}
		idStr, err := s.insertItem(ctx, it, pid)
		if err != nil {
			return Item{}, fmt.Errorf("%w: %v", ErrPersist, err)
		}
		it.ID = idStr
	}

	if idx, ok := s.catByID[def.CategoryID]; ok {
		c := s.categories[idx]
		it.Category = CategoryRef{ID: c.ID, Slug: c.Slug, Name: c.Name}
		s.categories[idx].ItemCount++
	}
	if idx, ok := s.secByID[def.SectionID]; ok {
		s.sections[idx].ItemCount++
	}

	s.items = append(s.items, it)
	s.byID[it.ID] = len(s.items) - 1
	s.bySlug[it.Slug] = len(s.items) - 1
	return it, nil
}
