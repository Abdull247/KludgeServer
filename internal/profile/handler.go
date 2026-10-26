package profile

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/google/uuid"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	profiles *Store
	sessions SessionReader
}

func NewHandler(profiles *Store, sessions SessionReader) *Handler {
	return &Handler{profiles: profiles, sessions: sessions}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := h.userID(w, r)
	if !ok {
		return
	}

	p, err := h.profiles.GetByUserID(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load profile")
		return
	}
	if p == nil {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}

	respond.JSON(w, http.StatusOK, p)
}

type onboardingBody struct {
	HasCodingBackground *bool `json:"hasCodingBackground"`
}

func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	id, ok := h.userID(w, r)
	if !ok {
		return
	}

	var body onboardingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.HasCodingBackground == nil {
		respond.Error(w, http.StatusBadRequest, "hasCodingBackground is required")
		return
	}

	p, err := h.profiles.CompleteOnboarding(r.Context(), id, *body.HasCodingBackground)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "profile not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "could not complete onboarding")
		return
	}

	respond.JSON(w, http.StatusOK, p)
}

func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := h.sessions.UserID(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return uuid.Nil, false
	}
	return id, true
}
