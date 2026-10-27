package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const selectCatalogItems = `
SELECT
    ci.id, ci.slug, ci.name, ci.type, ci.kind, ci.mark, ci.tone,
    ci.blurb, ci.description, ci.version, ci.build_artwork, ci.banners,
    ci.category_id, ci.section_id, ci.stats, ci.publisher_id, ci.source_url,
    ci.tags, ci.published_at, ci.updated_at,
    p.username, p.full_name, p.avatar_url
FROM catalog_items ci
LEFT JOIN profiles p ON p.user_id = ci.publisher_id
ORDER BY ci.published_at DESC
`

// AttachDB wires the store to Postgres and loads persisted catalog rows on
// top of the mock seed. Rows are matched by slug: DB rows override mocks,
// unseen rows are appended.
func (s *Store) AttachDB(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := loadCatalogRows(ctx, pool)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = pool
	loaded := make([]Item, 0, len(rows))
	for _, r := range rows {
		loaded = append(loaded, r.toItem())
	}
	s.mergeItems(loaded)
	s.index()
	s.hydrate()
	return nil
}

// mergeItems folds DB rows into the in-memory index. Must be called with the
// write lock held.
func (s *Store) mergeItems(items []Item) {
	for _, it := range items {
		if idx, ok := s.bySlug[it.Slug]; ok {
			s.items[idx] = it
			continue
		}
		if idx, ok := s.byID[it.ID]; ok {
			s.items[idx] = it
			continue
		}
		s.items = append(s.items, it)
	}
}

type catalogItemRow struct {
	ID           string
	Slug         string
	Name         string
	Type         string
	Kind         string
	Mark         string
	Tone         string
	Blurb        string
	Description  string
	Version      string
	BuildArtwork string
	Banners      []byte
	CategoryID   string
	SectionID    string
	Stats        []byte
	PublisherID  *uuid.UUID
	SourceURL    string
	Tags         []byte
	PublishedAt  time.Time
	UpdatedAt    time.Time
	PUsername    *string
	PFullName    *string
	PAvatarURL   *string
}

func (r catalogItemRow) toItem() Item {
	var banners []string
	_ = json.Unmarshal(r.Banners, &banners)

	var tags []string
	_ = json.Unmarshal(r.Tags, &tags)

	it := Item{
		ID:           r.ID,
		Slug:         r.Slug,
		Name:         r.Name,
		Type:         r.Type,
		Kind:         r.Kind,
		Mark:         r.Mark,
		Tone:         r.Tone,
		Blurb:        r.Blurb,
		Description:  r.Description,
		Version:      r.Version,
		BuildArtwork: r.BuildArtwork,
		Banners:      banners,
		CategoryID:   r.CategoryID,
		SectionID:    r.SectionID,
		SourceURL:    r.SourceURL,
		Tags:         tags,
		PublishedAt:  r.PublishedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if err := json.Unmarshal(r.Stats, &it.Stats); err != nil {
		it.Stats = ItemStats{}
	}
	if r.PublisherID != nil {
		it.PublisherID = r.PublisherID.String()
		pub := Publisher{
			ID:       it.PublisherID,
			Kind:     PublisherKindAutomation,
			Verified: true,
		}
		if r.PUsername != nil {
			pub.Username = *r.PUsername
		}
		if r.PFullName != nil {
			pub.DisplayName = *r.PFullName
		}
		if r.PAvatarURL != nil {
			pub.AvatarURL = *r.PAvatarURL
		}
		it.Publisher = &pub
	}
	if it.Banners == nil {
		it.Banners = []string{}
	}
	if it.Tags == nil {
		it.Tags = []string{}
	}
	return it
}

func loadCatalogRows(ctx context.Context, pool *pgxpool.Pool) ([]catalogItemRow, error) {
	rows, err := pool.Query(ctx, selectCatalogItems)
	if err != nil {
		return nil, fmt.Errorf("query catalog items: %w", err)
	}
	defer rows.Close()

	out := make([]catalogItemRow, 0)
	for rows.Next() {
		var r catalogItemRow
		if err := rows.Scan(
			&r.ID, &r.Slug, &r.Name, &r.Type, &r.Kind, &r.Mark, &r.Tone,
			&r.Blurb, &r.Description, &r.Version, &r.BuildArtwork, &r.Banners,
			&r.CategoryID, &r.SectionID, &r.Stats, &r.PublisherID, &r.SourceURL,
			&r.Tags, &r.PublishedAt, &r.UpdatedAt,
			&r.PUsername, &r.PFullName, &r.PAvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan catalog item: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog items: %w", err)
	}
	return out, nil
}

// insertItem persists a newly created item and returns the stored id. Must be
// called with the write lock held.
func (s *Store) insertItem(ctx context.Context, it Item, pubID *uuid.UUID) (string, error) {
	tags, err := json.Marshal(it.Tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	banners, err := json.Marshal(it.Banners)
	if err != nil {
		return "", fmt.Errorf("marshal banners: %w", err)
	}
	stats, err := json.Marshal(it.Stats)
	if err != nil {
		return "", fmt.Errorf("marshal stats: %w", err)
	}

	var id string
	err = s.db.QueryRow(ctx, `
		INSERT INTO catalog_items (
			id, slug, name, type, kind, mark, tone, blurb, description,
			version, build_artwork, banners, category_id, section_id,
			publisher_id, source_url, tags, stats, published_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12::jsonb, $13, $14,
			$15, $16, $17::jsonb, $18::jsonb, $19, $20
		)
		RETURNING id
	`,
		it.ID, it.Slug, it.Name, it.Type, it.Kind, it.Mark, it.Tone, it.Blurb,
		it.Description, it.Version, it.BuildArtwork, string(banners),
		it.CategoryID, it.SectionID, pubID, it.SourceURL, string(tags),
		string(stats), it.PublishedAt, it.UpdatedAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert catalog item: %w", err)
	}
	return id, nil
}
