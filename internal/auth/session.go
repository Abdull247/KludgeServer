package auth

import (
	"net/http"
	"time"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
)

type SessionStore struct {
	cookie   *securecookie.SecureCookie
	name     string
	secure   bool
	sameSite http.SameSite
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
	c, err := r.Cookie(s.name)
	if err != nil {
		return uuid.Nil, false
	}

	var payload sessionPayload
	if err := s.cookie.Decode(s.name, c.Value, &payload); err != nil {
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
