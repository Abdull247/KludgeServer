# KludgeServer

Go API for a developer store: browse published resources and projects, and publish your own.

This is the backend. If you know Node.js (Fastify + better-auth), this README maps those ideas to Go.

## Mental model (Node.js -> Go)

| Node.js | This repo |
| --- | --- |
| `fastify()` + plugins | `chi` router + middleware in `internal/http` |
| `better-auth` (Google + GitHub) | `goth` (Passport-style OAuth) in `internal/auth` |
| Prisma / Drizzle | `pgx` + SQL schema in `internal/database` |
| `process.env` + dotenv | `godotenv` + `internal/config` |
| `src/index.ts` | `cmd/server/main.go` |
| npm scripts | `Makefile` (`make run`) |

Go note: `internal/` is a compiler rule, not a convention. Packages under `internal/` can only be imported by this module. That is the Go way of keeping app code private.

## Layout

```text
cmd/server/            process entrypoint (start HTTP server)
internal/
  config/              load .env and validate required values
  database/            Postgres pool + schema migrate on boot
  http/                chi router, CORS, request logger
  auth/                Google + GitHub OAuth + session cookie
  user/                users + oauth_accounts persistence
  respond/             JSON { data } / { error } helpers
.env.example           committed template
.env                   local secrets (gitignored)
```

Add new features as new packages under `internal/` (for example `internal/resource`, `internal/project`). Keep `cmd/server/main.go` thin: wire config, db, handlers, then listen.

## Requirements

- Go 1.25+
- PostgreSQL 15+ (local for development, Neon for production)

## Local Postgres

A local database is already the default so you do not burn Neon credits in development.

```bash
# Debian/Ubuntu
sudo apt-get install -y postgresql postgresql-contrib
sudo service postgresql start

sudo -u postgres psql -c "CREATE ROLE kludge LOGIN PASSWORD 'kludge_dev';"
sudo -u postgres createdb -O kludge kludge
sudo -u postgres psql -d kludge -c "GRANT ALL ON SCHEMA public TO kludge; ALTER SCHEMA public OWNER TO kludge;"
```

Connection string used in `.env`:

```text
postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable
```

Schema is applied automatically on server start (`internal/database/migrate.go`). Tables:

- `users` - one row per person (email is unique)
- `oauth_accounts` - GitHub / Google identities linked to a user
- `profiles` - public profile for each user (username, bio, onboarding)
- `catalog_items` - created/recorded catalog items (mock items are only in memory)

If the same email signs in with both providers, both accounts attach to the same user.

## Setup

```bash
git clone https://github.com/Abdull247/KludgeServer.git
cd KludgeServer

cp .env.example .env
```

Generate a session secret:

```bash
openssl rand -hex 32
```

Paste it into `SESSION_SECRET` in `.env`.

Install Go modules and start:

```bash
go mod tidy
go run ./cmd/server
```

Health check:

```bash
curl http://localhost:8080/health
```

Expected: `{"data":{"status":"ok"}}`

## OAuth (Google + GitHub)

This is the Go equivalent of better-auth social providers.

Flow:

1. Browser hits `GET /api/auth/github` or `GET /api/auth/google`
2. Server redirects to the provider
3. Provider redirects back to `/api/auth/{provider}/callback`
4. Server upserts the user and profile, sets an httpOnly session cookie, then redirects to `/onboarding` (first login) or `/home`

### GitHub

1. Open https://github.com/settings/developers
2. New OAuth App
3. Homepage URL: `http://localhost:8080`
4. Authorization callback URL: `http://localhost:8080/api/auth/github/callback`
5. Copy Client ID and Client Secret into `.env`

### Google

1. Open https://console.cloud.google.com/apis/credentials
2. Create OAuth client ID, type Web application
3. Authorized redirect URI: `http://localhost:8080/api/auth/google/callback`
4. Copy Client ID and Client Secret into `.env`

Until those values are set, `GET /api/auth/github` and `GET /api/auth/google` return `503` with `oauth is not configured` instead of crashing.

## Auth API

All JSON responses look like `{ "data": ... }` or `{ "error": "..." }`.

### Start login

```text
GET /api/auth/github
GET /api/auth/google
```

Redirects the browser to the provider. Use these as the `href` on your frontend login buttons.

### OAuth callback

```text
GET /api/auth/github/callback
GET /api/auth/google/callback
```

Provider-only. Do not call this yourself.

### Current user

```text
GET /api/auth/me
```

Cookie required. `200` with the user, or `401`.

```json
{
  "data": {
    "id": "uuid",
    "email": "you@example.com",
    "name": "You",
    "avatarUrl": "https://...",
    "createdAt": "...",
    "updatedAt": "..."
  }
}
```

### Logout

```text
POST /api/auth/logout
```

Clears the session cookie.

```json
{ "data": { "ok": true } }
```

### Current profile

```text
GET /api/profile/me
```

Cookie required. `200` with the profile, `401` if not signed in, `404` if missing.

```json
{
  "data": {
    "id": "uuid",
    "userId": "uuid",
    "fullName": "Ada Lovelace",
    "username": "ada1a2b3c",
    "email": "ada@example.com",
    "avatarUrl": "https://...",
    "bio": "New on Kludge — sharing unofficial builds, APIs, and tools.",
    "websiteUrl": null,
    "location": null,
    "role": "user",
    "accountStatus": "active",
    "emailVerified": false,
    "hasCodingBackground": null,
    "onboardingCompleted": false,
    "createdAt": "...",
    "updatedAt": "..."
  }
}
```

### Complete onboarding

```text
POST /api/profile/onboarding
```

Cookie required. Body:

```json
{ "hasCodingBackground": true }
```

### Frontend usage

```js
const api = "http://localhost:8080";

window.location.href = `${api}/api/auth/github`;

const me = await fetch(`${api}/api/auth/me`, { credentials: "include" });

await fetch(`${api}/api/auth/logout`, {
  method: "POST",
  credentials: "include",
});
```

`credentials: "include"` is required so the browser sends the httpOnly cookie. CORS is locked to `FRONTEND_URL` in production. Dev and `MODE=test` allow any origin.

## Catalog API (`/api/v1`)

Home, browse, and publish endpoints for the catalog.

Mock JSON lives in `internal/catalog/mocks/` (`viewer.json`, `sections.json`, `categories.json`, `items.json`). When `MODE=test` (or `MODE=js`) the API serves these demo files instead of Postgres, and every non-empty bearer token authenticates as the mock user.

In `MODE=live` the catalog serves only what is in the `catalog_items` table: no demo items and no demo viewer. Sections and categories are still the embedded taxonomy. The home viewer comes from the signed-in session. New rows come in through `POST /api/v1/items` (user) or `POST /api/v1/items/record` (automation).

Trending ranks items by recency (exponential decay over ~10 days) plus saves, likes, forks, and pulls.

### Authentication

Every endpoint except `/api/v1/types` requires an authenticated session: either the `kludge_session` cookie (after OAuth login) or the same value sent as a bearer token:

```bash
# Cookie session after logging in via OAuth
curl http://localhost:8080/api/v1/home \
  -H "Accept: application/json"

# Bearer token instead of cookie
curl http://localhost:8080/api/v1/home \
  -H "Accept: application/json" \
  -H "Authorization: Bearer <access-token>"
```

In `MODE=test` any non-empty token works:

```bash
curl http://localhost:8080/api/v1/home \
  -H "Authorization: Bearer any-token-here"
```

#### Getting a token manually

The "access token" is the signed session cookie value. Two supported ways to obtain one:

```bash
# Option 1: mint a token for a user id (loads .env exactly like the server)
# Find the user id first:
psql "$DATABASE_URL" -c "SELECT id, email FROM users WHERE email='ingest@kludge.dev';"

go run ./cmd/mint <userId>

# Option 2: log in via OAuth in the browser, then copy the kludge_session
# cookie from DevTools > Application > Cookies and use it as the bearer value
```

The minted value is also a valid session cookie value, so it can be pasted back into the browser for that user.

### Endpoints overview

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/types` | public | Supported item types and their categories |
| `GET` | `/api/v1/tags` | public | Chip list from `data/tags.json` |
| `GET` | `/api/v1/uploads/limits` | public | Max upload size and accepted types |
| `POST` | `/api/v1/uploads?kind=` | session | Upload artwork, banner, or source to R2 |
| `GET` | `/api/v1/home` | session | Full home feed (sections + rails) |
| `GET` | `/api/v1/sections/{id}` | session | One section with its categories + feed |
| `GET` | `/api/v1/categories/{id}` | session | One category with its items |
| `GET` | `/api/v1/trending` | session | Trending rail (builds section) |
| `GET` | `/api/v1/dropped` | session | Newest-first rail (builds section) |
| `GET` | `/api/v1/collections` | session | Collection rail |
| `GET` | `/api/v1/search?q=` | session | Cross-catalog search |
| `POST` | `/api/v1/items` | session | Publish a new item as the signed-in user |
| `POST` | `/api/v1/items/record` | automation | Ingest a new item as the automation account |
| `POST` | `/api/v1/items/{id}/save` | session | Toggle save for the viewer |
| `POST` | `/api/v1/items/{id}/like` | session | Toggle like for the viewer |
| `GET` | `/api/v1/admin/home` | admin | Admin dashboard: stats + flat project/API/user lists |
| `GET` | `/api/v1/admin/users` | admin | Every account with role and project counts |
| `GET` | `/api/v1/admin/projects` | admin | Every published build/API, newest first |

All JSON responses use the envelope `{"data": ...}` on success and `{"error": "..."}` on failure.

### Item types (the `type` field)

`GET /api/v1/types` returns all supported slugs. The `type` decides the category and section an item lands in:

| `type` slug | Label | Category | Section |
| --- | --- | --- | --- |
| `modded_apk` | Mod APK | `cat_modded_apk` / Modded APK | `sec_builds` / Builds |
| `unofficial_api` | Unofficial API | `cat_agentic_api` / Agentic API | `sec_apis` / APIs |
| `ai_jailbreak` | AI Jailbreak | `cat_ai_jailbreak` / AI Jailbreak | `sec_builds` / Builds |
| `frida` | Frida | `cat_frida` / Frida | `sec_builds` / Builds |
| `custom_rom` | Custom ROM | `cat_custom_rom` / Custom ROM | `sec_builds` / Builds |
| `ui_kit` | UI Kit | `cat_ui_kit` / UI Kit | `sec_builds` / Builds |
| `root_pack` | Root Pack | `cat_root` / Root | `sec_builds` / Builds |
| `patch` | Patch | `cat_patch` / Patch | `sec_builds` / Builds |
| `prompt_pack` | Prompt Pack | `cat_prompt` / Prompts | `sec_builds` / Builds |
| `graphql` | GraphQL | `cat_graphql` / GraphQL | `sec_apis` / APIs |
| `proxy` | Proxy | `cat_proxy` / Proxy | `sec_apis` / APIs |
| `sdk` | SDK | `cat_sdk` / SDK | `sec_apis` / APIs |

The label also matches, so `"type": "Mod APK"` works the same as `"type": "modded_apk"`.

### Home feed

```text
GET /api/v1/home?page=1&perPage=4&q=
Authorization: Bearer <access-token>
```

Query: `page`, `perPage`, `q`.

Returns sections **Builds**, **Creators**, **APIs** (stable ids `sec_builds`, `sec_creators`, `sec_apis`), each with categories, a paginated feed, and a trending slice. Also returns rails: trending, just dropped, collections.

```bash
curl http://localhost:8080/api/v1/home \
  -H "Accept: application/json" \
  -H "Authorization: Bearer <access-token>"
```

```json
{
  "data": {
    "generatedAt": "2026-10-27T12:00:00Z",
    "viewer": {
      "id": "9adb7fce-...",
      "username": "kludgeIngest",
      "greetingName": "Kludge",
      "fullName": "Kludge Ingest",
      "avatarUrl": "https://..."
    },
    "search": { "placeholder": "Search builds, APIs, kits", "chips": [ ... ] },
    "sections": [
      {
        "id": "sec_builds",
        "slug": "builds",
        "title": "Builds",
        "itemCount": 7,
        "categories": [ ... ],
        "feed": { "items": [ ... ], "pagination": { ... } },
        "trending": { "items": [ ... ], "pagination": { ... } }
      }
    ],
    "rails": {
      "trending": { "id": "rail_trending", "title": "Trending this week", "items": [ ... ] },
      "dropped": { "id": "rail_dropped", "title": "Just dropped", "items": [ ... ] },
      "collections": { "id": "rail_collections", "title": "Collections to steal", "items": [ ... ] }
    }
  }
}
```

### Section / category

```bash
# Section by id or slug
curl http://localhost:8080/api/v1/sections/sec_builds?sort=trending \
  -H "Authorization: Bearer <access-token>"

# Category by id or slug
curl http://localhost:8080/api/v1/categories/cat_modded_apk \
  -H "Authorization: Bearer <access-token>"
```

`{id}` is the section/category id or slug. Query: `page`, `perPage`, `q`, `sort=trending|fresh`.

```bash
curl "http://localhost:8080/api/v1/trending?page=1&perPage=8" \
  -H "Authorization: Bearer <access-token>"

curl "http://localhost:8080/api/v1/dropped" \
  -H "Authorization: Bearer <access-token>"

curl "http://localhost:8080/api/v1/search?q=jailbreak" \
  -H "Authorization: Bearer <access-token>"

curl "http://localhost:8080/api/v1/collections" \
  -H "Authorization: Bearer <access-token>"
```

### Save / like

```bash
curl -X POST http://localhost:8080/api/v1/items/itm_orbit/save \
  -H "Authorization: Bearer <access-token>"

curl -X POST http://localhost:8080/api/v1/items/itm_orbit/like \
  -H "Authorization: Bearer <access-token>"
```

Toggles the viewer flag and updates counts. `{id}` accepts the item id or slug.

### Creating and recording items

Both endpoints share one request body. `POST /api/v1/items` publishes as the signed-in user (kind `user`); `POST /api/v1/items/record` ingests as the automation account (kind `automation`).

> The record endpoint never fetches remote pages itself. The automation pipeline must send already-extracted, normalized JSON.

#### Request fields

| Field | Required | Rules |
| --- | --- | --- |
| `name` | yes | 2-80 characters; slug is derived from it |
| `description` | yes | 1-4000 characters; `blurb` is the first 180 chars |
| `type` | yes | One of the type slugs above (see `/api/v1/types`) |
| `artworkUrl` | yes | Absolute http(s) URL; stored as `buildArtwork` |
| `version` | no | Max 32 characters |
| `bannerUrls` | no | Array, max 8 absolute http(s) URLs |
| `tags` | no | Array, max 8; trimmed, lowercased, max 32 chars each |
| `sourceUrl` | no | Absolute http(s) URL |
| `publishedAt` | no | RFC3339 timestamp; defaults to now |

#### Full example: record a Mod APK

```bash
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Titan Mods Reborn",
    "description": "Unlocked WhatsApp build with ghost mode and anti-revoke enabled.",
    "type": "modded_apk",
    "version": "9.90.3",
    "artworkUrl": "https://images.example.com/cover.png",
    "bannerUrls": [
      "https://images.example.com/banner-wide.png",
      "https://images.example.com/banner-tall.png"
    ],
    "tags": ["whatsapp", "ghost mode", "anti-revoke"],
    "sourceUrl": "https://example.com/source-page",
    "publishedAt": "2026-10-27T12:00:00Z"
  }'
```

Response is `201` with the created item:

```json
{
  "data": {
    "id": "itm_titan-mods-rebornx7k2",
    "slug": "titan-mods-reborn",
    "name": "Titan Mods Reborn",
    "type": "modded_apk",
    "kind": "build",
    "mark": "T",
    "tone": "d",
    "blurb": "Unlocked WhatsApp build with ghost mode and anti-revoke enabled.",
    "description": "Unlocked WhatsApp build with ghost mode and anti-revoke enabled.",
    "version": "9.90.3",
    "buildArtwork": "https://images.example.com/cover.png",
    "banners": ["https://images.example.com/banner-wide.png", "https://images.example.com/banner-tall.png"],
    "categoryId": "cat_modded_apk",
    "sectionId": "sec_builds",
    "category": { "id": "cat_modded_apk", "slug": "modded-apk", "name": "Modded APK" },
    "stats": { "saves": 0, "likes": 0, "forks": 0, "pulls": 0, "runs": 0, "followers": 0, "builds": 0 },
    "metric": { "key": "saves", "label": "saves", "value": 0, "display": "0 saves" },
    "viewer": { "saved": false, "liked": false },
    "publisher": {
      "id": "9adb7fce-...",
      "username": "kludgeIngest",
      "displayName": "Kludge Ingest",
      "avatarUrl": "https://...",
      "kind": "automation",
      "verified": true
    },
    "sourceUrl": "https://example.com/source-page",
    "tags": ["whatsapp", "ghost mode", "anti-revoke"],
    "publishedAt": "2026-10-27T12:00:00Z",
    "updatedAt": "2026-10-27T12:00:00Z",
    "trendingScore": 0
  }
}
```

#### Type cases for the record endpoint

Every type uses the same body; only `name`, `description`, and `type` change per case. Minimal bodies for each:

```bash
# Mod APK -> cat_modded_apk (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Titan Mods Reborn",
    "description": "Unlocked WhatsApp build with ghost mode.",
    "type": "modded_apk",
    "artworkUrl": "https://images.example.com/cover.png",
    "sourceUrl": "https://example.com/source"
  }'
```

```bash
# Unofficial API -> cat_agentic_api (APIs)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Relay Chat Endpoint",
    "description": "Unofficial chat API that skips the usual refusals. Bring your own key.",
    "type": "unofficial_api",
    "artworkUrl": "https://images.example.com/relay.png",
    "sourceUrl": "https://example.com/docs"
  }'
```

```bash
# AI Jailbreak -> cat_ai_jailbreak (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Agent Unchained Kit",
    "description": "System prompt config that keeps agents useful past typical refusals.",
    "type": "ai_jailbreak",
    "artworkUrl": "https://images.example.com/unchained.png",
    "sourceUrl": "https://example.com/lab"
  }'
```

```bash
# Frida -> cat_frida (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "SSL Unpin Pack",
    "description": "Frida hooks for SSL pinning, root hide, and receipt checks.",
    "type": "frida",
    "artworkUrl": "https://images.example.com/hook.png"
  }'
```

```bash
# Custom ROM -> cat_custom_rom (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Night Owl A15",
    "description": "De-Googled weekly build with per-app proxy. SHA-256 on file.",
    "type": "custom_rom",
    "artworkUrl": "https://images.example.com/rom.png",
    "version": "2026.10.2"
  }'
```

```bash
# UI Kit -> cat_ui_kit (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Slate Dark UI",
    "description": "180-component dark kit with a token file that matches production.",
    "type": "ui_kit",
    "artworkUrl": "https://images.example.com/slate.png"
  }'
```

```bash
# Root Pack -> cat_root (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Magisk Stack v26",
    "description": "Zygisk, denylist presets, and a quiet boot animation bundled.",
    "type": "root_pack",
    "artworkUrl": "https://images.example.com/magisk.png"
  }'
```

```bash
# Patch -> cat_patch (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Receipt Flag Reset",
    "description": "Binary patch kit for local entitlement flags. Diffs included.",
    "type": "patch",
    "artworkUrl": "https://images.example.com/patch.png"
  }'
```

```bash
# Prompt Pack -> cat_prompt (Builds)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Lab Notes Vol. 3",
    "description": "Twelve agent prompts from the jailbreak lab. Local only.",
    "type": "prompt_pack",
    "artworkUrl": "https://images.example.com/prompts.png"
  }'
```

```bash
# GraphQL -> cat_graphql (APIs)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Shadow Vendor Graph",
    "description": "GraphQL surface over a vendor that never shipped a schema.",
    "type": "graphql",
    "artworkUrl": "https://images.example.com/graph.png",
    "sourceUrl": "https://example.com/graphiql"
  }'
```

```bash
# Proxy -> cat_proxy (APIs)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Swap SOCKS5",
    "description": "SOCKS5 hop with rotating exits and 24h log wipe.",
    "type": "proxy",
    "artworkUrl": "https://images.example.com/socks.png"
  }'
```

```bash
# SDK -> cat_sdk (APIs)
curl -X POST http://localhost:8080/api/v1/items/record \
  -H "Authorization: Bearer <automation-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Vendor Client TS",
    "description": "Typed TypeScript client for a vendor that never published one.",
    "type": "sdk",
    "artworkUrl": "https://images.example.com/sdk.png",
    "sourceUrl": "https://github.com/example/vendor-ts"
  }'
```

#### Publish as a user

```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H "Authorization: Bearer <user-access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Orbit Reborn",
    "description": "Unlocked build with hashes on file.",
    "type": "modded_apk",
    "artworkUrl": "https://images.example.com/art/cover.png",
    "tags": ["weekend drop"]
  }'
```

Identical rules as the record endpoint, but the publisher is the session user and there is no automation email gate.

#### Access rules for `/api/v1/items/record`

| Situation | Response |
| --- | --- |
| Missing or invalid token/cookie | `401` `{"error": "not authenticated"}` |
| `AUTOMATION_EMAIL` unset in live mode | `503` `{"error": "automation ingest is not configured"}` |
| Authenticated identity is not the automation account | `403` `{"error": "only the automation account may record items"}` |
| Unknown `type` | `400` `{"error": "unknown item type; see /api/v1/types"}` |
| Validation failure (name, URLs, counts) | `400` with the specific message |
| Database unreachable | `500` `{"error": "could not store item"}` |

#### Persistence

In live mode created/recorded items are written to the `catalog_items` table and reloaded on boot. In test mode they live in memory only and vanish on restart.

### Pagination shape

`feed` and `rail` responses carry a `pagination` object:

```json
{
  "page": 1,
  "perPage": 12,
  "total": 27,
  "totalPages": 3,
  "hasNext": true,
  "hasPrev": false,
  "nextPage": 2,
  "prevPage": null
}
```

### Common errors across all v1 endpoints

| Situation | Response |
| --- | --- |
| Missing or invalid token/cookie | `401` `{"error": "not authenticated"}` |
| Unknown section/category/item id or slug | `404` `{"error": "section not found"}` / `category not found` / `item not found` |
| Server misconfiguration | `503` with a descriptive message |

## Environment

See `.env.example`. Important keys:

| Variable | Purpose |
| --- | --- |
| `APP_PORT` | HTTP port (default `8080`) |
| `APP_PUBLIC_URL` | Public API URL, used to build OAuth callback URLs |
| `FRONTEND_URL` | CORS origin and post-login redirect |
| `DATABASE_URL` | Postgres URL (local now, Neon later) |
| `SESSION_SECRET` | Signs cookies (32+ chars) |
| `MODE` | `live` (default) or `test`. `test` serves catalog JSON from `internal/catalog/mocks` and skips Postgres |
| `AUTOMATION_EMAIL` | Email of the automation ingest account; the only identity allowed to `POST /api/v1/items/record`. Defaults to `ingest@kludge.dev` in test mode |
| `ADMIN_EMAILS` | Comma-separated emails granted admin (`/api/v1/admin/*` + web admin dashboard). Defaults to `ada@kludge.dev` in test mode |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | GitHub OAuth |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth |
| `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` | Cloudflare R2 credentials |
| `R2_BUCKET_NAME` | R2 bucket for artwork, banners, and source files |
| `R2_ENDPOINT` | S3 API endpoint, e.g. `https://<accountid>.r2.cloudflarestorage.com` |
| `R2_PUBLIC_URL` | Public base URL used in returned object URLs |
| `MAX_UPLOAD_BYTES` | Max size for a single upload. Bytes or `50MB`. Default `50MB` |

Production Neon example:

```text
DATABASE_URL=postgres://USER:PASSWORD@HOST/dbname?sslmode=require
APP_ENV=production
APP_PUBLIC_URL=https://api.yourdomain.com
FRONTEND_URL=https://yourdomain.com
```

Update OAuth callback URLs at GitHub and Google to match `APP_PUBLIC_URL`.

## How a request is wired

1. `cmd/server/main.go` loads config, connects to Postgres, runs migrate
2. Builds user store, session store, auth handler
3. `internal/http.NewRouter` mounts routes
4. Auth handlers live in `internal/auth/handler.go`

To add a route later: create a handler package, then register it inside `internal/http/router.go` under `/api`.

## Commands

```bash
# Start the API server
go run ./cmd/server

# Run the test suite
go test ./...

# Tidy dependencies
go mod tidy

# Mint a bearer token for a user id (loads .env like the server does)
go run ./cmd/mint <userId>
```
