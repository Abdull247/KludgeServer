package profile

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("profile not found")

var TestUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

func TestProfile() *Profile {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	avatar := "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=256&h=256&q=80"
	return &Profile{
		ID:                  uuid.MustParse("00000000-0000-4000-8000-000000000011"),
		UserID:              TestUserID,
		FullName:            "Ada Lovelace",
		Username:            "ada",
		Email:               "ada@kludge.dev",
		AvatarURL:           &avatar,
		Bio:                 DefaultBio,
		Role:                "user",
		AccountStatus:       "active",
		EmailVerified:       true,
		OnboardingCompleted: true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

const DefaultBio = "New on Kludge — sharing unofficial builds, APIs, and tools."

type Profile struct {
	ID                  uuid.UUID `json:"id"`
	UserID              uuid.UUID `json:"userId"`
	FullName            string    `json:"fullName"`
	Username            string    `json:"username"`
	Email               string    `json:"email"`
	AvatarURL           *string   `json:"avatarUrl"`
	Bio                 string    `json:"bio"`
	WebsiteURL          *string   `json:"websiteUrl"`
	Location            *string   `json:"location"`
	Role                string    `json:"role"`
	AccountStatus       string    `json:"accountStatus"`
	EmailVerified       bool      `json:"emailVerified"`
	HasCodingBackground *bool     `json:"hasCodingBackground"`
	OnboardingCompleted bool      `json:"onboardingCompleted"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type EnsureInput struct {
	UserID    uuid.UUID
	FullName  string
	Email     string
	AvatarURL string
}
