package admin

import (
	"time"

	"github.com/Abdull247/KludgeServer/internal/catalog"
)

// AdminUser is the flattened users+profiles row shown in the admin console.
type AdminUser struct {
	ID                  string    `json:"id"`
	Email               string    `json:"email"`
	FullName            string    `json:"fullName"`
	Username            string    `json:"username"`
	AvatarURL           string    `json:"avatarUrl"`
	Role                string    `json:"role"`
	OnboardingCompleted bool      `json:"onboardingCompleted"`
	ProjectCount        int       `json:"projectCount"`
	CreatedAt           time.Time `json:"createdAt"`
}

// AdminStats are the headline counts for the admin home tabs.
type AdminStats struct {
	Users    int `json:"users"`
	Projects int `json:"projects"`
	Builds   int `json:"builds"`
	APIs     int `json:"apis"`
}

// AdminHome powers the admin dashboard: flat lists, no rails.
type AdminHome struct {
	GeneratedAt string         `json:"generatedAt"`
	Admin       bool           `json:"admin"`
	Viewer      catalog.Viewer `json:"viewer"`
	Stats       AdminStats     `json:"stats"`
	Projects    []catalog.Item `json:"projects"`
	APIs        []catalog.Item `json:"apis"`
	Users       []AdminUser    `json:"users"`
}

// UserList wraps the users endpoint response.
type UserList struct {
	Users []AdminUser `json:"users"`
	Total int         `json:"total"`
}

// ProjectList wraps the projects endpoint response.
type ProjectList struct {
	Projects []catalog.Item `json:"projects"`
	Total    int            `json:"total"`
}
