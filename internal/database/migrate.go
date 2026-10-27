package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    avatar_url    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_user_id  TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TABLE IF NOT EXISTS profiles (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    full_name               TEXT NOT NULL DEFAULT '',
    username                TEXT NOT NULL UNIQUE,
    email                   TEXT NOT NULL,
    avatar_url              TEXT,
    bio                     TEXT NOT NULL DEFAULT 'New on Kludge — sharing unofficial builds, APIs, and tools.',
    website_url             TEXT,
    location                TEXT,
    role                    TEXT NOT NULL DEFAULT 'user',
    account_status          TEXT NOT NULL DEFAULT 'active',
    email_verified          BOOLEAN NOT NULL DEFAULT false,
    has_coding_background   BOOLEAN,
    onboarding_completed    BOOLEAN NOT NULL DEFAULT false,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS profiles_username_idx ON profiles (username);

CREATE TABLE IF NOT EXISTS catalog_items (
    id            TEXT PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL,
    kind          TEXT NOT NULL DEFAULT 'build',
    mark          TEXT NOT NULL DEFAULT '',
    tone          TEXT NOT NULL DEFAULT 'a',
    blurb         TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    version       TEXT NOT NULL DEFAULT '',
    build_artwork TEXT NOT NULL DEFAULT '',
    banners       JSONB NOT NULL DEFAULT '[]'::jsonb,
    category_id   TEXT NOT NULL,
    section_id    TEXT NOT NULL,
    publisher_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    source_url    TEXT NOT NULL DEFAULT '',
    tags                 JSONB NOT NULL DEFAULT '[]'::jsonb,
    stats                JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    verification_status  TEXT NOT NULL DEFAULT 'verified',
    rejection_reason     TEXT NOT NULL DEFAULT '',
    verified_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS catalog_items_category_idx ON catalog_items (category_id);
CREATE INDEX IF NOT EXISTS catalog_items_section_idx ON catalog_items (section_id);
CREATE INDEX IF NOT EXISTS catalog_items_published_idx ON catalog_items (published_at DESC);
CREATE INDEX IF NOT EXISTS catalog_items_verification_idx ON catalog_items (verification_status);

ALTER TABLE catalog_items ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'verified';
ALTER TABLE catalog_items ADD COLUMN IF NOT EXISTS rejection_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_items ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
`
