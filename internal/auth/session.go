package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/user"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
)

var TestUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

func TestUser() *user.User {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return &user.User{
		ID:        TestUserID,
		Email:     "ada@kludge.dev",
		Name:      "Ada Lovelace",
		AvatarURL: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=256&h=256&q=80",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

type SessionStore struct {
	cookie   *securecookie.SecureCookie
	name     string
	secure   bool
	sameSite http.SameSite
	testMode bool
}

type sessionPayload struct {
	UserID string `json:"userId"`
}

func NewSessionStore(cfg *config.Config) *SessionStore {
	hashKey := []byte(cfg.SessionSecret)
	blockKey := hashKey[:32]

	secure := !cfg.IsDev()
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}

	oauthStore := sessions.NewCookieStore(hashKey)
	oauthStore.MaxAge(int((10 * time.Minute).Seconds()))
	oauthStore.Options.Path = "/"
	oauthStore.Options.HttpOnly = true
	oauthStore.Options.Secure = secure
	oauthStore.Options.SameSite = sameSite
	gothic.Store = oauthStore

	return &SessionStore{
		cookie:   securecookie.New(hashKey, blockKey),
		name:     cfg.SessionCookie,
		secure:   secure,
		sameSite: sameSite,
		testMode: cfg.IsTest(),
	}
}

func (s *SessionStore) Set(w http.ResponseWriter, userID uuid.UUID) error {
	encoded, err := s.cookie.Encode(s.name, sessionPayload{UserID: userID.String()})
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    encoded,
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})
	return nil
}

func (s *SessionStore) UserID(r *http.Request) (uuid.UUID, bool) {
	if id, ok := s.fromBearer(r); ok {
		return id, true
	}

	c, err := r.Cookie(s.name)
	if err != nil {
		return uuid.Nil, false
	}

	return s.decode(c.Value)
}

func (s *SessionStore) fromBearer(r *http.Request) (uuid.UUID, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return uuid.Nil, false
	}
	typ, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(typ, "Bearer") {
		return uuid.Nil, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return uuid.Nil, false
	}
	if s.testMode {
		return TestUserID, true
	}
	return s.decode(token)
}

// Token encodes a bearer token for the given user id using the same secret,
// name, and serializer as the session cookie. Useful for minting API tokens
// for machine accounts (see cmd/mint).
func (s *SessionStore) Token(userID uuid.UUID) (string, error) {
	return s.cookie.Encode(s.name, sessionPayload{UserID: userID.String()})
}

func (s *SessionStore) decode(value string) (uuid.UUID, bool) {
	var payload sessionPayload
	if err := s.cookie.Decode(s.name, value, &payload); err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(payload.UserID)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func (s *SessionStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})
}
