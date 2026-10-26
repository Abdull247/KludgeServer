package profile

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("profile not found")

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
