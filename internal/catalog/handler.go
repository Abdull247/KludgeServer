package catalog

import (
	"net/http"

	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	store    *Store
	sessions SessionReader
}

func NewHandler(store *Store, sessions SessionReader) *Handler {
	return &Handler{store: store, sessions: sessions}
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
	respond.JSON(w, http.StatusOK, h.store.Home(queryFrom(r)))
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
