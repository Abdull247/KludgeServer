package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	store           *Store
	sessions        SessionReader
	identities      IdentityLookup
	automationEmail string
}

func NewHandler(store *Store, sessions SessionReader, identities IdentityLookup, automationEmail string) *Handler {
	return &Handler{
		store:           store,
		sessions:        sessions,
		identities:      identities,
		automationEmail: strings.ToLower(strings.TrimSpace(automationEmail)),
	}
}

// identity resolves the person behind the session into a catalog Identity.
// When no lookup is wired (test mode without a database) it falls back to the
// deterministic test identity.
func (h *Handler) identity(r *http.Request) (Identity, bool) {
	uid, ok := h.sessions.UserID(r)
	if !ok {
		return Identity{}, false
	}
	if h.identities == nil {
		return TestIdentity(), true
	}
	return h.identities.ByUserID(r.Context(), uid)
}

func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := h.sessions.UserID(r); !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return false
	}
	return true
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	home := h.store.Home(queryFrom(r))
	if iden, ok := h.identity(r); ok {
		home.Viewer = Viewer{
			ID:           iden.UserID,
			Username:     iden.Username,
			FullName:     iden.FullName,
			GreetingName: greetingName(iden.FullName),
			AvatarURL:    iden.AvatarURL,
		}
	}
	respond.JSON(w, http.StatusOK, home)
}

func greetingName(fullName string) string {
	fields := strings.Fields(strings.TrimSpace(fullName))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (h *Handler) Section(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	sec, ok := h.store.Section(chi.URLParam(r, "id"), queryFrom(r))
	if !ok {
		respond.Error(w, http.StatusNotFound, "section not found")
		return
	}
	respond.JSON(w, http.StatusOK, sec)
}

func (h *Handler) Category(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	cat, page, ok := h.store.Category(chi.URLParam(r, "id"), queryFrom(r))
	if !ok {
		respond.Error(w, http.StatusNotFound, "category not found")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"category":   cat,
		"items":      page.Items,
		"pagination": page.Pagination,
	})
}

func (h *Handler) Trending(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	respond.JSON(w, http.StatusOK, h.store.Trending(queryFrom(r)))
}

func (h *Handler) Dropped(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	respond.JSON(w, http.StatusOK, h.store.Dropped(queryFrom(r)))
}

func (h *Handler) Collections(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	respond.JSON(w, http.StatusOK, h.store.Collections(queryFrom(r)))
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	respond.JSON(w, http.StatusOK, h.store.Search(queryFrom(r)))
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, "save")
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, "like")
}

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request, action string) {
	if !h.requireUser(w, r) {
		return
	}
	res, ok := h.store.Toggle(chi.URLParam(r, "id"), action)
	if !ok {
		respond.Error(w, http.StatusNotFound, "item not found")
		return
	}
	respond.JSON(w, http.StatusOK, res)
}

func queryFrom(r *http.Request) query {
	q := r.URL.Query()
	return parseQuery(q.Get("page"), q.Get("perPage"), q.Get("q"), q.Get("sort"))
}

type createBody struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Version     string   `json:"version"`
	ArtworkURL  string   `json:"artworkUrl"`
	BannerURLs  []string `json:"bannerUrls"`
	Tags        []string `json:"tags"`
	SourceURL   string   `json:"sourceUrl"`
	PublishedAt string   `json:"publishedAt"`
}

func decodeCreate(r *http.Request) (CreateInput, error) {
	var b createBody
	dec := json.NewDecoder(io.LimitReader(r.Body, 512<<10))
	if err := dec.Decode(&b); err != nil {
		return CreateInput{}, fmt.Errorf("invalid JSON body")
	}
	in := CreateInput{
		Name:        b.Name,
		Description: b.Description,
		Type:        b.Type,
		Version:     b.Version,
		ArtworkURL:  b.ArtworkURL,
		BannerURLs:  b.BannerURLs,
		Tags:        b.Tags,
		SourceURL:   b.SourceURL,
	}
	if s := strings.TrimSpace(b.PublishedAt); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return CreateInput{}, fmt.Errorf("publishedAt must be an RFC3339 timestamp")
		}
		in.PublishedAt = &t
	}
	return in, nil
}

func identityPublisher(iden Identity, kind string) Publisher {
	return Publisher{
		ID:          iden.UserID,
		Username:    iden.Username,
		DisplayName: iden.FullName,
		AvatarURL:   iden.AvatarURL,
		Kind:        kind,
		Verified:    kind == PublisherKindAutomation,
	}
}

func (h *Handler) respondCreate(w http.ResponseWriter, item Item, err error) {
	switch {
	case err == nil:
		respond.JSON(w, http.StatusCreated, item)
	case errors.Is(err, ErrUnknownType):
		respond.Error(w, http.StatusBadRequest, "unknown item type; see /api/v1/types")
	case errors.Is(err, ErrPersist):
		respond.Error(w, http.StatusInternalServerError, "could not store item")
	default:
		respond.Error(w, http.StatusBadRequest, err.Error())
	}
}

// CreateItem registers a new catalog item published by the session user.
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	in, err := decodeCreate(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	iden, ok := h.identity(r)
	if !ok {
		respond.Error(w, http.StatusForbidden, "profile not found for session user")
		return
	}
	item, err := h.store.Create(r.Context(), in, identityPublisher(iden, PublisherKindUser))
	h.respondCreate(w, item, err)
}

// RecordItem ingests an item extracted by the automation account. Unlike
// CreateItem the publisher is the automation identity, and the endpoint is
// gated to the configured automation email. Without a wired identity lookup
// (test mode) the session is treated as the automation account directly.
func (h *Handler) RecordItem(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	if h.automationEmail == "" {
		respond.Error(w, http.StatusServiceUnavailable, "automation ingest is not configured")
		return
	}
	iden, ok := h.identity(r)
	if !ok {
		respond.Error(w, http.StatusForbidden, "profile not found for session user")
		return
	}
	if h.identities != nil && !strings.EqualFold(iden.Email, h.automationEmail) {
		respond.Error(w, http.StatusForbidden, "only the automation account may record items")
		return
	}
	in, err := decodeCreate(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.store.Create(r.Context(), in, identityPublisher(iden, PublisherKindAutomation))
	h.respondCreate(w, item, err)
}

// Types lists the supported item type slugs and the category each maps to.
func (h *Handler) Types(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, TypeDefs())
}

// Tags returns the editable chip list from data/tags.json.
func (h *Handler) Tags(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, loadTags())
}
