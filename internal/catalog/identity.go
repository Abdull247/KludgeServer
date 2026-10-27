package catalog

import (
	"context"

	"github.com/google/uuid"
)

// Identity is the resolved person behind a session: user id, contact email
// and the profile fields used to render a publisher card.
type Identity struct {
	UserID    string
	Email     string
	Username  string
	FullName  string
	AvatarURL string
}

// IdentityLookup resolves a session user into an Identity. Implemented by an
// adapter over the profile store so catalog does not import profile directly.
type IdentityLookup interface {
	ByUserID(ctx context.Context, userID uuid.UUID) (Identity, bool)
}

// TestIdentity is used when no identity lookup is wired (test mode without a
// database). Its email matches the test-mode default admin email so admin and
// record paths stay exercisable.
func TestIdentity() Identity {
	return Identity{
		UserID:    "usr_ada",
		Email:     "ada@kludge.dev",
		Username:  "ada",
		FullName:  "Ada Lovelace",
		AvatarURL: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=256&h=256&q=80",
	}
}
