package profile

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var nonUser = regexp.MustCompile(`[^a-z0-9]+`)

func (s *Store) GetByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	row := s.pool.QueryRow(ctx, selectProfile+` WHERE user_id = $1`, userID)
	return scanProfile(row)
}

func (s *Store) Ensure(ctx context.Context, in EnsureInput) (*Profile, error) {
	existing, err := s.GetByUserID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		avatar := emptyToNil(in.AvatarURL)
		row := s.pool.QueryRow(ctx, `
			UPDATE profiles
			SET avatar_url = COALESCE($2, avatar_url),
			    full_name = CASE WHEN $3 <> '' THEN $3 ELSE full_name END,
			    email = CASE WHEN $4 <> '' THEN $4 ELSE email END,
			    updated_at = now()
			WHERE user_id = $1
			RETURNING `+returningCols, in.UserID, avatar, strings.TrimSpace(in.FullName), strings.ToLower(strings.TrimSpace(in.Email)))
		return scanProfile(row)
	}

	fullName := strings.TrimSpace(in.FullName)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	avatar := emptyToNil(in.AvatarURL)

	for i := 0; i < 8; i++ {
		username := generateUsername(fullName)
		row := s.pool.QueryRow(ctx, `
			INSERT INTO profiles (
				user_id, full_name, username, email, avatar_url, bio
			) VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+returningCols,
			in.UserID, fullName, username, email, avatar, DefaultBio,
		)
		p, err := scanProfile(row)
		if err == nil {
			return p, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return nil, fmt.Errorf("insert profile: %w", err)
	}

	return nil, fmt.Errorf("could not allocate a unique username")
}

func (s *Store) CompleteOnboarding(ctx context.Context, userID uuid.UUID, hasCodingBackground bool) (*Profile, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE profiles
		SET has_coding_background = $2,
		    onboarding_completed = true,
		    updated_at = now()
		WHERE user_id = $1
		RETURNING `+returningCols, userID, hasCodingBackground)
	p, err := scanProfile(row)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

const returningCols = `
	id, user_id, full_name, username, email, avatar_url, bio,
	website_url, location, role, account_status, email_verified,
	has_coding_background, onboarding_completed, created_at, updated_at
`

const selectProfile = `SELECT` + returningCols + ` FROM profiles`

func scanProfile(row pgx.Row) (*Profile, error) {
	var p Profile
	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.FullName,
		&p.Username,
		&p.Email,
		&p.AvatarURL,
		&p.Bio,
		&p.WebsiteURL,
		&p.Location,
		&p.Role,
		&p.AccountStatus,
		&p.EmailVerified,
		&p.HasCodingBackground,
		&p.OnboardingCompleted,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func generateUsername(fullName string) string {
	base := firstNameSlug(fullName)
	return base + randomSuffix(6)
}

func firstNameSlug(fullName string) string {
	fullName = strings.TrimSpace(strings.ToLower(fullName))
	if fullName == "" {
		return "user"
	}
	parts := strings.FieldsFunc(fullName, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return "user"
	}
	first := nonUser.ReplaceAllString(parts[0], "")
	if first == "" {
		return "user"
	}
	if len(first) > 16 {
		first = first[:16]
	}
	return first
}

func randomSuffix(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = alphabet[i%len(alphabet)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func emptyToNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
