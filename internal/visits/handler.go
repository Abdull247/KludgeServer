package visits

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	store       *Store
	sessions    SessionReader
	identities  catalog.IdentityLookup
	adminEmails []string
	items       *catalog.Store
}

func NewHandler(store *Store, sessions SessionReader, identities catalog.IdentityLookup, adminEmails []string, items *catalog.Store) *Handler {
	lowered := make([]string, 0, len(adminEmails))
	for _, e := range adminEmails {
		if e = lower(e); e != "" {
			lowered = append(lowered, e)
		}
	}
	return &Handler{
		store:       store,
		sessions:    sessions,
		identities:  identities,
		adminEmails: lowered,
		items:       items,
	}
}

func lower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// identity resolves the person behind the session, falling back to the
// deterministic test identity when no lookup is wired (test mode).
func (h *Handler) identity(r *http.Request) (catalog.Identity, bool) {
	uid, ok := h.sessions.UserID(r)
	if !ok {
		return catalog.Identity{}, false
	}
	if h.identities == nil {
		return catalog.TestIdentity(), true
	}
	return h.identities.ByUserID(r.Context(), uid)
}

func (h *Handler) isAdmin(iden catalog.Identity) bool {
	email := lower(iden.Email)
	for _, e := range h.adminEmails {
		if email == e {
			return true
		}
	}
	return false
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (catalog.Identity, bool) {
	iden, ok := h.identity(r)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return catalog.Identity{}, false
	}
	if !h.isAdmin(iden) {
		respond.Error(w, http.StatusForbidden, "admin only")
		return catalog.Identity{}, false
	}
	return iden, true
}

// Record logs one item-page view. Signed-in visitors keep their profile on the
// row; everyone else is described by their browser alone. Fails only for
// unknown or invisible items, so bots poking dead ids get nothing logged.
func (h *Handler) Record(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	iden, authed := h.identity(r)
	admin := authed && h.isAdmin(iden)
	viewer := ""
	if authed {
		viewer = iden.UserID
	}
	it, ok := h.items.GetFor(id, viewer, admin)
	if !ok {
		respond.Error(w, http.StatusNotFound, "item not found")
		return
	}
	v := Visitor{
		ItemID:    it.ID,
		ItemName:  it.Name,
		UserAgent: Clip(r.UserAgent(), maxText),
		IPAddress: clientIP(r),
		Referer:   Clip(r.Referer(), maxText),
		Language:  Clip(r.Header.Get("Accept-Language"), 128),
	}
	if authed {
		v.Authenticated = true
		v.UserID = iden.UserID
		v.Name = iden.FullName
		v.Email = iden.Email
		v.Username = iden.Username
		v.AvatarURL = iden.AvatarURL
	}
	if _, err := h.store.Record(r.Context(), v); err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not record visit")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdminList paginates the visit log, newest first.
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	per, _ := strconv.Atoi(q.Get("perPage"))
	pg, err := h.store.List(r.Context(), page, per, q.Get("q"))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not load visits")
		return
	}
	respond.JSON(w, http.StatusOK, pg)
}

// AdminGet returns one visit row in full.
func (h *Handler) AdminGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	v, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	switch {
	case err == nil:
		respond.JSON(w, http.StatusOK, v)
	case errors.Is(err, ErrNotFound):
		respond.Error(w, http.StatusNotFound, "visit not found")
	default:
		respond.Error(w, http.StatusInternalServerError, "could not load visit")
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return Clip(r.RemoteAddr, 64)
	}
	return Clip(host, 64)
}
