package auth

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/profile"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/Abdull247/KludgeServer/internal/user"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type Handler struct {
	cfg      *config.Config
	users    *user.Store
	profiles *profile.Store
	sessions *SessionStore
	handoffs *HandoffStore
}

func NewHandler(cfg *config.Config, users *user.Store, profiles *profile.Store, sessions *SessionStore, handoffs *HandoffStore) *Handler {
	gothic.GetProviderName = func(r *http.Request) (string, error) {
		name := chi.URLParam(r, "provider")
		if name == "" {
			return "", fmt.Errorf("provider is required")
		}
		return name, nil
	}

	return &Handler{cfg: cfg, users: users, profiles: profiles, sessions: sessions, handoffs: handoffs}
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

	if isAppClient(detectClient(r)) {
		h.beginApp(w, r, provider, false)
		return
	}

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

func (h *Handler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		respond.Error(w, http.StatusBadRequest, "unsupported provider")
		return
	}
	if !h.providerConfigured(provider) {
		respond.Error(w, http.StatusServiceUnavailable, provider+" sign-in is not configured")
		return
	}
	h.beginApp(w, r, provider, true)
}

func (h *Handler) beginApp(w http.ResponseWriter, r *http.Request, provider string, asJSON bool) {
	if h.handoffs == nil {
		h.failAppStart(w, r, asJSON, "app sign-in is not available")
		return
	}

	state, err := randomToken(32)
	if err != nil {
		h.failAppStart(w, r, asJSON, "could not start sign-in")
		return
	}

	gothProvider, err := goth.GetProvider(provider)
	if err != nil {
		h.failAppStart(w, r, asJSON, "unsupported provider")
		return
	}

	sess, err := gothProvider.BeginAuth(state)
	if err != nil {
		h.failAppStart(w, r, asJSON, "could not start sign-in")
		return
	}

	authURL, err := sess.GetAuthURL()
	if err != nil {
		h.failAppStart(w, r, asJSON, "could not start sign-in")
		return
	}

	if err := h.handoffs.PutTransaction(r.Context(), state, provider, clientApp, sess.Marshal()); err != nil {
		h.failAppStart(w, r, asJSON, "could not start sign-in")
		return
	}

	if asJSON {
		respond.JSON(w, http.StatusOK, map[string]any{
			"authorizationUrl": authURL,
			"expiresIn":        h.cfg.OAuthTxnTTL,
		})
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *Handler) failAppStart(w http.ResponseWriter, r *http.Request, asJSON bool, message string) {
	if asJSON {
		respond.Error(w, http.StatusInternalServerError, message)
		return
	}
	h.redirectAuthError(w, r, message)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if _, ok := allowedProviders[provider]; !ok {
		h.redirectAuthError(w, r, "unsupported provider")
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if h.handoffs != nil && state != "" {
		client, sessionData, ok, err := h.handoffs.TakeTransaction(r.Context(), state, provider)
		if err != nil {
			h.appAuthError(w, "could not complete sign-in")
			return
		}
		if ok && isAppClient(client) {
			h.completeApp(w, r, provider, sessionData)
			return
		}
	}

	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		h.redirectAuthError(w, r, "sign-in failed, try again")
		return
	}

	u, p, err := h.finishUser(r, provider, gothUser)
	if err != nil {
		h.redirectAuthError(w, r, err.Error())
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

func (h *Handler) completeApp(w http.ResponseWriter, r *http.Request, provider, sessionData string) {
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		h.appAuthError(w, "OAuth sign-in was cancelled or denied")
		return
	}

	gothProvider, err := goth.GetProvider(provider)
	if err != nil {
		h.appAuthError(w, "unsupported provider")
		return
	}

	sess, err := gothProvider.UnmarshalSession(sessionData)
	if err != nil {
		h.appAuthError(w, "sign-in failed, try again")
		return
	}

	if _, err := sess.Authorize(gothProvider, r.URL.Query()); err != nil {
		h.appAuthError(w, "sign-in failed, try again")
		return
	}

	gothUser, err := gothProvider.FetchUser(sess)
	if err != nil {
		h.appAuthError(w, "sign-in failed, try again")
		return
	}

	u, _, err := h.finishUser(r, provider, gothUser)
	if err != nil {
		h.appAuthError(w, err.Error())
		return
	}

	ticket, err := h.handoffs.PutHandoff(r.Context(), u.ID)
	if err != nil {
		h.appAuthError(w, "could not create your sign-in ticket")
		return
	}

	writeHandoffPage(w, appDeepLink(h.cfg.AppLink, ticket), appIntentLink(h.cfg.AppPackage, ticket))
}

func (h *Handler) finishUser(r *http.Request, provider string, gothUser goth.User) (*user.User, *profile.Profile, error) {
	email := strings.TrimSpace(strings.ToLower(gothUser.Email))
	if email == "" {
		return nil, nil, fmt.Errorf("your account did not return an email")
	}

	if h.users == nil {
		u := TestUser()
		return u, nil, nil
	}

	u, err := h.users.UpsertFromOAuth(r.Context(), user.UpsertOAuthInput{
		Provider:       provider,
		ProviderUserID: gothUser.UserID,
		Email:          email,
		Name:           displayName(gothUser),
		AvatarURL:      gothUser.AvatarURL,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not save your account")
	}

	if h.profiles == nil {
		return u, nil, nil
	}

	p, err := h.profiles.Ensure(r.Context(), profile.EnsureInput{
		UserID:    u.ID,
		FullName:  u.Name,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not create your profile")
	}

	return u, p, nil
}

func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "ticket is required")
		return
	}
	ticket := strings.TrimSpace(body.Ticket)
	if len(ticket) < 32 {
		respond.Error(w, http.StatusBadRequest, "ticket is required")
		return
	}
	if h.handoffs == nil {
		respond.Error(w, http.StatusUnauthorized, "the sign-in ticket is invalid, expired, or already used")
		return
	}

	userID, ok, err := h.handoffs.ConsumeHandoff(r.Context(), ticket)
	if err != nil || !ok {
		respond.Error(w, http.StatusUnauthorized, "the sign-in ticket is invalid, expired, or already used")
		return
	}

	token, err := h.sessions.Token(userID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not create your session")
		return
	}

	u, err := h.publicUser(r, userID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load user")
		return
	}
	if u == nil {
		respond.Error(w, http.StatusUnauthorized, "the sign-in ticket is invalid, expired, or already used")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"user":      u,
		"session":   token,
		"tokenType": "Bearer",
	})
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	writeHandoffPage(w, appDeepLink(h.cfg.AppLink, ticket), appIntentLink(h.cfg.AppPackage, ticket))
}

func (h *Handler) publicUser(r *http.Request, id uuid.UUID) (*user.User, error) {
	if h.users == nil {
		if id == TestUserID {
			return TestUser(), nil
		}
		return TestUser(), nil
	}
	return h.users.GetByID(r.Context(), id)
}

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

func (h *Handler) appAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>Kludge</title></head>
<body style="margin:0;background:#0b0b0b;color:#f3f3f3;font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh">
  <p>%s</p>
</body>
</html>`, html.EscapeString(message))
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
