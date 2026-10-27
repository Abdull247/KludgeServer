package admin

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	store      *catalog.Store
	pool       *pgxpool.Pool
	sessions   SessionReader
	identities catalog.IdentityLookup
	emails     []string
}

func NewHandler(store *catalog.Store, pool *pgxpool.Pool, sessions SessionReader, identities catalog.IdentityLookup, emails []string) *Handler {
	lowered := make([]string, 0, len(emails))
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			lowered = append(lowered, e)
		}
	}
	return &Handler{
		store:      store,
		pool:       pool,
		sessions:   sessions,
		identities: identities,
		emails:     lowered,
	}
}

// adminIdentity resolves the session into an identity, falling back to the
// deterministic test identity when no lookup is wired (test mode).
func (h *Handler) adminIdentity(r *http.Request) (catalog.Identity, bool) {
	uid, ok := h.sessions.UserID(r)
	if !ok {
		return catalog.Identity{}, false
	}
	if h.identities == nil {
		return catalog.TestIdentity(), true
	}
	return h.identities.ByUserID(r.Context(), uid)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (catalog.Identity, bool) {
	iden, ok := h.adminIdentity(r)
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

func (h *Handler) isAdmin(iden catalog.Identity) bool {
	email := strings.ToLower(strings.TrimSpace(iden.Email))
	for _, e := range h.emails {
		if e == email {
			return true
		}
	}
	return false
}

func viewerFor(iden catalog.Identity) catalog.Viewer {
	first := iden.FullName
	if i := strings.IndexFunc(first, func(r rune) bool { return r == ' ' }); i > 0 {
		first = first[:i]
	}
	return catalog.Viewer{
		ID:           iden.UserID,
		Username:     iden.Username,
		FullName:     iden.FullName,
		GreetingName: first,
		AvatarURL:    iden.AvatarURL,
	}
}

// Home returns the whole admin console payload: users, projects, apis.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	iden, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	users := h.loadUsers(r.Context())
	projects, apis := h.splitProjects()
	home := AdminHome{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Admin:       true,
		Viewer:      viewerFor(iden),
		Stats: AdminStats{
			Users:    len(users),
			Projects: len(projects),
			Builds:   countKind(projects, "build"),
			APIs:     len(apis),
		},
		Projects: projects,
		APIs:     apis,
		Users:    users,
	}
	respond.JSON(w, http.StatusOK, home)
}

// Users returns the full users list flattened from users+profiles.
func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	users := h.loadUsers(r.Context())
	respond.JSON(w, http.StatusOK, UserList{Users: users, Total: len(users)})
}

// Projects returns all build/api items, newest first.
func (h *Handler) Projects(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	projects, apis := h.splitProjects()
	all := append(projects, apis...)
	respond.JSON(w, http.StatusOK, ProjectList{Projects: all, Total: len(all)})
}

// splitProjects pulls items from the catalog store and separates build-kind
// from api-kind entries, each sorted newest first.
func (h *Handler) splitProjects() ([]catalog.Item, []catalog.Item) {
	items := h.store.Items()
	var builds, apis []catalog.Item
	for _, it := range items {
		switch it.Kind {
		case "build":
			builds = append(builds, it)
		case "api":
			apis = append(apis, it)
		}
	}
	byFresh := func(list []catalog.Item) {
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].PublishedAt.After(list[j].PublishedAt)
		})
	}
	byFresh(builds)
	byFresh(apis)
	return builds, apis
}

func countKind(items []catalog.Item, kind string) int {
	n := 0
	for _, it := range items {
		if it.Type != "creator" && it.Type != "collection" && it.Kind == kind {
			n++
		}
	}
	return n
}
