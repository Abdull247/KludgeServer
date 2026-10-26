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
4. Server upserts the user, sets an httpOnly session cookie, then redirects to `FRONTEND_URL/`

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

`credentials: "include"` is required so the browser sends the httpOnly cookie. CORS is locked to `FRONTEND_URL`.

## Environment

See `.env.example`. Important keys:

| Variable | Purpose |
| --- | --- |
| `APP_PORT` | HTTP port (default `8080`) |
| `APP_PUBLIC_URL` | Public API URL, used to build OAuth callback URLs |
| `FRONTEND_URL` | CORS origin and post-login redirect |
| `DATABASE_URL` | Postgres URL (local now, Neon later) |
| `SESSION_SECRET` | Signs cookies (32+ chars) |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | GitHub OAuth |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth |

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
go run ./cmd/server
go test ./...
go mod tidy
```
