package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id)
	return scanUser(row)
}

func (s *Store) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

// EnsureByEmail returns the user with the given email, creating a minimal
// account when missing. Backs the automation ingest account.
func (s *Store) EnsureByEmail(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	existing, err := s.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET updated_at = now()
		RETURNING id, email, name, avatar_url, created_at, updated_at
	`, email, "Kludge Ingest")
	return scanUser(row)
}

type UpsertOAuthInput struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

func (s *Store) UpsertFromOAuth(ctx context.Context, in UpsertOAuthInput) (*User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2
	`, in.Provider, in.ProviderUserID).Scan(&userID)

	if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE users
			SET name = CASE WHEN $2 <> '' THEN $2 ELSE name END,
			    avatar_url = CASE WHEN $3 <> '' THEN $3 ELSE avatar_url END,
			    updated_at = now()
			WHERE id = $1
		`, userID, in.Name, in.AvatarURL)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, in.Email).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				INSERT INTO users (email, name, avatar_url)
				VALUES ($1, $2, $3)
				RETURNING id
			`, in.Email, in.Name, in.AvatarURL).Scan(&userID)
			if err != nil {
				return nil, fmt.Errorf("insert user: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("lookup email: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO oauth_accounts (user_id, provider, provider_user_id)
			VALUES ($1, $2, $3)
		`, userID, in.Provider, in.ProviderUserID)
		if err != nil {
			return nil, fmt.Errorf("insert oauth account: %w", err)
		}
	} else {
		return nil, fmt.Errorf("lookup oauth account: %w", err)
	}

	row := tx.QueryRow(ctx, `
		SELECT id, email, name, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID)
	u, err := scanUser(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
