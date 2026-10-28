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

	// Carry the post-login target through the provider round-trip so the
	// redirect lands back on the origin that started the login (proxy-safe).
	if next := r.URL.Query().Get("next"); h.safeNext(next) {
		if sess, err := gothic.Store.Get(r, gothic.SessionName); err == nil {
			sess.Values["kludge_next"] = next
			if err := sess.Save(r, w); err != nil {
				h.redirectAuthError(w, r, "could not start sign-in")
				return
			}
		}
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

	token, err := h.sessions.Token(u.ID)
	if err != nil {
		h.redirectAuthError(w, r, "could not create your session")
		return
	}

	next := h.loginNext(r, p)
	http.Redirect(w, r, appendQuery(next, url.Values{
		"welcome": {"1"},
		"session": {token},
	}), http.StatusFound)
}

// loginNext resolves the post-login redirect: the stashed target from the
// login origin when present, otherwise the configured frontend URL.
func (h *Handler) loginNext(r *http.Request, p *profile.Profile) string {
	target := h.cfg.FrontendURL + "/home"
	if p != nil && !p.OnboardingCompleted {
		target = h.cfg.FrontendURL + "/onboarding"
	}
	if sess, err := gothic.Store.Get(r, gothic.SessionName); err == nil {
		if v, ok := sess.Values["kludge_next"].(string); ok && h.safeNext(v) {
			target = v
		}
	}
	return target
}

func (h *Handler) safeNext(next string) bool {
	if next == "" || strings.HasPrefix(next, "//") {
		return false
	}
	if strings.HasPrefix(next, "/") {
		return true
	}
	u, err := url.Parse(next)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Host)
	if front, err := url.Parse(h.cfg.FrontendURL); err == nil && strings.EqualFold(host, front.Host) {
		return true
	}
	return host == "kludgekit.web.app" || host == "kludgekit.firebaseapp.com"
}

func appendQuery(raw string, extra url.Values) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for k, vs := range extra {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessions.UserID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if h.users == nil {
		respond.JSON(w, http.StatusOK, TestUser())
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
