package admin

import (
	"context"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/catalog"
)

const selectAdminUsers = `
SELECT
    u.id::text,
    COALESCE(NULLIF(p.email, ''), u.email),
    COALESCE(NULLIF(p.full_name, ''), u.name),
    COALESCE(NULLIF(p.username, ''), 'unknown-' || substr(u.id::text, 1, 8)),
    COALESCE(NULLIF(p.avatar_url, ''), u.avatar_url),
    COALESCE(p.role, 'user'),
    COALESCE(p.onboarding_completed, false),
    (SELECT count(*) FROM catalog_items ci WHERE ci.publisher_id = u.id),
    u.created_at
FROM users u
LEFT JOIN profiles p ON p.user_id = u.id
ORDER BY u.created_at DESC
`

// loadUsers returns every account. Live mode joins users+profiles in
// Postgres; test mode (no pool) synthesizes users from the mock creators so
// the console stays demoable.
func (h *Handler) loadUsers(ctx context.Context) []AdminUser {
	if h.pool == nil {
		return h.mockUsers()
	}
	rows, err := h.pool.Query(ctx, selectAdminUsers)
	if err != nil {
		return []AdminUser{}
	}
	defer rows.Close()

	out := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		var role string
		if err := rows.Scan(
			&u.ID, &u.Email, &u.FullName, &u.Username, &u.AvatarURL,
			&role, &u.OnboardingCompleted, &u.ProjectCount, &u.CreatedAt,
		); err != nil {
			continue
		}
		if role == "" {
			role = "user"
		}
		u.Role = role
		out = append(out, u)
	}
	return out
}

// mockUsers derives accounts from the mock catalog creators (plus the test
// identity) and counts their items, giving the admin console people to show
// in test mode.
func (h *Handler) mockUsers() []AdminUser {
	test := catalog.TestIdentity()
	byID := map[string]*AdminUser{
		test.UserID: {
			ID:                  test.UserID,
			Email:               test.Email,
			FullName:            test.FullName,
			Username:            test.Username,
			AvatarURL:           test.AvatarURL,
			Role:                "admin",
			OnboardingCompleted: true,
		},
	}
	for _, c := range h.store.Creators() {
		if byID[c.ID] != nil {
			continue
		}
		byID[c.ID] = &AdminUser{
			ID:        c.ID,
			FullName:  c.DisplayName,
			Username:  c.Username,
			AvatarURL: c.AvatarURL,
		}
	}
	for _, it := range h.store.Items() {
		id := it.PublisherID
		if id == "" && it.Creator != nil {
			id = it.Creator.ID
		}
		if u := byID[id]; u != nil && it.Kind != "collection" {
			u.ProjectCount++
		}
	}

	out := make([]AdminUser, 0, len(byID))
	for _, u := range byID {
		if strings.TrimSpace(u.FullName) == "" {
			u.FullName = u.Username
		}
		out = append(out, *u)
	}
	return out
}
