package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/Abdull247/KludgeServer/internal/user"
	"github.com/go-chi/chi/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type Handler struct {
	cfg      *config.Config
	users    *user.Store
	sessions *SessionStore
}

func NewHandler(cfg *config.Config, users *user.Store, sessions *SessionStore) *Handler {
	gothic.GetProviderName = func(r *http.Request) (string, error) {
		name := chi.URLParam(r, "provider")
		if name == "" {
			return "", fmt.Errorf("provider is required")
		}
		return name, nil
	}

	return &Handler{cfg: cfg, users: users, sessions: sessions}
}

var allowedProviders = map[string]struct{}{
	"github": {},
	"google": {},
}

func (h *Handler) Begin(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		respond.Error(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	if !h.providerConfigured(provider) {
		respond.Error(w, http.StatusServiceUnavailable, provider+" oauth is not configured")
		return
	}

	gothic.BeginAuthHandler(w, r)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		respond.Error(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "oauth callback failed")
		return
	}

	email := strings.TrimSpace(strings.ToLower(gothUser.Email))
	if email == "" {
		respond.Error(w, http.StatusBadRequest, "oauth provider did not return an email")
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
		respond.Error(w, http.StatusInternalServerError, "could not persist user")
		return
	}

	if err := h.sessions.Set(w, u.ID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create session")
		return
	}

	http.Redirect(w, r, h.cfg.FrontendURL+"/", http.StatusFound)
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

func displayName(u goth.User) string {
	if u.Name != "" {
		return u.Name
	}
	if u.NickName != "" {
		return u.NickName
	}
	return u.Email
}
