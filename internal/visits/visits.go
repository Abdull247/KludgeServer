package visits

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxText caps stored header-derived strings so hostile clients cannot bloat rows.
const maxText = 512

var ErrNotFound = errors.New("visit not found")

// Visitor is one item-page view. For signed-in people it carries the resolved
// profile; for guests it carries whatever the browser volunteered, plus the
// connection address. Item name is denormalised so the admin log reads without
// joins.
type Visitor struct {
	ID            string    `json:"id"`
	ItemID        string    `json:"itemId"`
	ItemName      string    `json:"itemName"`
	Authenticated bool      `json:"authenticated"`
	UserID        string    `json:"userId,omitempty"`
	Name          string    `json:"name,omitempty"`
	Email         string    `json:"email,omitempty"`
	Username      string    `json:"username,omitempty"`
	AvatarURL     string    `json:"avatarUrl,omitempty"`
	UserAgent     string    `json:"userAgent"`
	IPAddress     string    `json:"ipAddress"`
	Referer       string    `json:"referer"`
	Language      string    `json:"language"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Page is a parsed window over the visit log, newest first.
type Page struct {
	Visitors   []Visitor          `json:"visitors"`
	Pagination catalog.Pagination `json:"pagination"`
	Total      int                `json:"total"`
}

// Store keeps visits in Postgres. Without a pool (test mode or a database
// outage at boot) rows live in memory so the endpoints stay exercisable.
type Store struct {
	mu  sync.RWMutex
	db  *pgxpool.Pool
	mem []Visitor
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Clip trim/limits a text value before it is stored.
func Clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (s *Store) Record(ctx context.Context, v Visitor) (Visitor, error) {
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		v.ID = uuid.NewString()
		v.CreatedAt = time.Now().UTC()
		s.mem = append([]Visitor{v}, s.mem...)
		return v, nil
	}
	var userID *uuid.UUID
	if v.UserID != "" {
		if uid, err := uuid.Parse(v.UserID); err == nil {
			userID = &uid
		}
	}
	var id string
	var createdAt time.Time
	err := s.db.QueryRow(ctx, `
		INSERT INTO item_visits (
			item_id, item_name, authenticated, user_id, name, email, username,
			avatar_url, user_agent, ip_address, referer, language
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`,
		v.ItemID, v.ItemName, v.Authenticated, userID, v.Name, v.Email,
		v.Username, v.AvatarURL, v.UserAgent, v.IPAddress, v.Referer, v.Language,
	).Scan(&id, &createdAt)
	if err != nil {
		return Visitor{}, fmt.Errorf("insert visit: %w", err)
	}
	v.ID = id
	v.CreatedAt = createdAt.UTC()
	return v, nil
}

func (s *Store) List(ctx context.Context, page, perPage int, q string) (Page, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 30
	}
	if perPage > 100 {
		perPage = 100
	}
	if s.db == nil {
		return listMemory(s.snapshot(), page, perPage, q), nil
	}

	where, args := listWhere(q)
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM item_visits"+where, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count visits: %w", err)
	}
	totalPages := 1
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	if page > totalPages {
		page = totalPages
	}
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.Query(ctx, `
		SELECT `+visitColumns+`
		FROM item_visits
		`+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args))+`
	`, args...)
	if err != nil {
		return Page{}, fmt.Errorf("query visits: %w", err)
	}
	defer rows.Close()

	visitors, err := collectRows(rows)
	if err != nil {
		return Page{}, err
	}
	return window(visitors, page, perPage, total), nil
}

func (s *Store) Get(ctx context.Context, id string) (Visitor, error) {
	uid, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return Visitor{}, ErrNotFound
	}
	if s.db == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, v := range s.mem {
			if v.ID == uid.String() {
				return v, nil
			}
		}
		return Visitor{}, ErrNotFound
	}
	row := s.db.QueryRow(ctx, "SELECT "+visitColumns+" FROM item_visits WHERE id = $1", uid)
	return scanVisitor(row)
}

func (s *Store) snapshot() []Visitor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Visitor, len(s.mem))
	copy(out, s.mem)
	return out
}

func collectRows(rows pgx.Rows) ([]Visitor, error) {
	out := make([]Visitor, 0)
	for rows.Next() {
		v, err := scanVisitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visits: %w", err)
	}
	return out, nil
}

func scanVisitor(row pgx.Row) (Visitor, error) {
	var v Visitor
	var authed bool
	err := row.Scan(
		&v.ID, &v.ItemID, &v.ItemName, &authed, &v.UserID,
		&v.Name, &v.Email, &v.Username, &v.AvatarURL,
		&v.UserAgent, &v.IPAddress, &v.Referer, &v.Language, &v.CreatedAt,
	)
	if err != nil {
		return Visitor{}, fmt.Errorf("scan visit: %w", err)
	}
	v.Authenticated = authed
	v.CreatedAt = v.CreatedAt.UTC()
	return v, nil
}

const visitColumns = `
	id, item_id, item_name, authenticated, COALESCE(user_id::text, ''),
	name, email, username, avatar_url, user_agent, ip_address, referer,
	language, created_at`

func listWhere(q string) (string, []any) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", nil
	}
	like := "%" + strings.ToLower(q) + "%"
	return ` WHERE lower(name) LIKE $1 OR lower(email) LIKE $1 OR lower(username) LIKE $1 OR lower(item_name) LIKE $1 OR lower(ip_address) LIKE $1 OR lower(user_agent) LIKE $1 OR lower(language) LIKE $1`, []any{like}
}

func listMemory(all []Visitor, page, perPage int, q string) Page {
	q = strings.TrimSpace(strings.ToLower(q))
	visitors := make([]Visitor, 0, len(all))
	for _, v := range all {
		if q == "" {
			visitors = append(visitors, v)
			continue
		}
		blob := strings.ToLower(strings.Join([]string{v.Name, v.Email, v.Username, v.ItemName, v.IPAddress, v.UserAgent, v.Language}, " "))
		if strings.Contains(blob, q) {
			visitors = append(visitors, v)
		}
	}
	sort.SliceStable(visitors, func(i, j int) bool {
		return visitors[i].CreatedAt.After(visitors[j].CreatedAt)
	})
	var total int
	if q == "" {
		total = len(all)
	} else {
		total = len(visitors)
	}
	return window(visitors, page, perPage, total)
}

func window(visitors []Visitor, page, perPage, total int) Page {
	totalPages := 1
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	if start > len(visitors) {
		start = len(visitors)
	}
	end := start + perPage
	if end > len(visitors) {
		end = len(visitors)
	}
	var next, prev *int
	if page < totalPages && total > 0 {
		n := page + 1
		next = &n
	}
	if page > 1 {
		p := page - 1
		prev = &p
	}
	if visitors == nil {
		visitors = []Visitor{}
	}
	return Page{
		Visitors: visitors[start:end],
		Total:    total,
		Pagination: catalog.Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    next != nil,
			HasPrev:    prev != nil,
			NextPage:   next,
			PrevPage:   prev,
		},
	}
}
