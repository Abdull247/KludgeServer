package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/profile"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/Abdull247/KludgeServer/internal/user"
	"github.com/go-chi/chi/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type Handler struct {
	cfg      *config.Config
	users    *user.Store
	profiles *profile.Store
	sessions *SessionStore
}

func NewHandler(cfg *config.Config, users *user.Store, profiles *profile.Store, sessions *SessionStore) *Handler {
	gothic.GetProviderName = func(r *http.Request) (string, error) {
		name := chi.URLParam(r, "provider")
		if name == "" {
			return "", fmt.Errorf("provider is required")
		}
		return name, nil
	}

	return &Handler{cfg: cfg, users: users, profiles: profiles, sessions: sessions}
}

var allowedProviders = map[string]struct{}{
	"github": {},
	"google": {},
}

func (h *Handler) Begin(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		h.redirectAuthError(w, r, "unsupported provider")
		return
	}

	if !h.providerConfigured(provider) {
		h.redirectAuthError(w, r, provider+" sign-in is not configured")
		return
	}

	gothic.BeginAuthHandler(w, r)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		h.redirectAuthError(w, r, "unsupported provider")
		return
	}

	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		h.redirectAuthError(w, r, "sign-in failed, try again")
		return
	}

	email := strings.TrimSpace(strings.ToLower(gothUser.Email))
	if email == "" {
		h.redirectAuthError(w, r, "your account did not return an email")
		return
	}

	u, err := h.users.UpsertFromOAuth(r.Context(), user.UpsertOAuthInput{
		Provider:       provider,
		ProviderUserID: gothUser.UserID,
		Email:          email,
		Name:           displayName(gothUser),
		AvatarURL:      gothUser.AvatarURL,
	})
	if err != nil {
		h.redirectAuthError(w, r, "could not save your account")
		return
	}

	p, err := h.profiles.Ensure(r.Context(), profile.EnsureInput{
		UserID:    u.ID,
		FullName:  u.Name,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
	})
	if err != nil {
		h.redirectAuthError(w, r, "could not create your profile")
		return
	}

	if err := h.sessions.Set(w, u.ID); err != nil {
		h.redirectAuthError(w, r, "could not create your session")
		return
	}

	next := h.cfg.FrontendURL + "/home?welcome=1"
	if p != nil && !p.OnboardingCompleted {
		next = h.cfg.FrontendURL + "/onboarding?welcome=1"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessions.UserID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load user")
		return
	}
	if u == nil {
		h.sessions.Clear(w)
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	respond.JSON(w, http.StatusOK, u)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = gothic.Logout(w, r)
	h.sessions.Clear(w)
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) providerConfigured(provider string) bool {
	switch provider {
	case "github":
		return h.cfg.GitHubClientID != "" && h.cfg.GitHubClientSecret != ""
	case "google":
		return h.cfg.GoogleClientID != "" && h.cfg.GoogleClientSecret != ""
	default:
		return false
	}
}

func (h *Handler) redirectAuthError(w http.ResponseWriter, r *http.Request, message string) {
	target := h.cfg.FrontendURL + "/auth?error=" + url.QueryEscape(message)
	http.Redirect(w, r, target, http.StatusFound)
}

func displayName(u goth.User) string {
	if u.Name != "" {
		return u.Name
	}
	if u.NickName != "" {
		return u.NickName
	}
	return u.Email
}
