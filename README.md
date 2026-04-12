# Live URL

https://repowatch-production.up.railway.app/

# Extra

Deploy API to hosting + HTML subscription page for
release notifications.
Redis caching of GitHub API responses with a 10-minute   
TTL.                                                     
API key authentication: endpoints protected by a token in
the header.                                             
Prometheus metrics — /metrics endpoint with basic service
indicators.
GitHub Actions CI pipeline: running linter and tests on
every push.

# GitHub Release Notifier

A Go service that lets users subscribe their email to GitHub repository release notifications.
When a new release is published, all confirmed subscribers receive an email.

## Architecture

```
cmd/server/          → entry point, wires all components
internal/config/     → env-based configuration
internal/email/      → SMTP notifier (Notifier interface + SMTPNotifier)
internal/release/    → GitHub API client + periodic release scanner
internal/subscription/ → domain: model, repository, service, HTTP handlers
db/                  → pgxpool connection + golang-migrate runner (embedded SQL)
```

**No circular dependencies:** `email` and `release/github` have no internal imports.
`subscription` imports `email` and `release` (for sentinel errors only).
`cmd/server` wires everything with a `repoAdapter` that converts between the two domains.

## Quick Start (Docker)

```bash
# 1. Clone and build
git clone <repo-url>
cd genesis

# 2. (Optional) set GitHub token for higher rate limits
cp .env.example .env
# edit .env and set GITHUB_TOKEN=ghp_...

# 3. Start everything
docker compose up --build

# 4. View emails at http://localhost:8025 (MailHog UI)
```

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/subscribe` | Subscribe (form: email, repo) |
| GET | `/api/confirm/{token}` | Confirm subscription |
| GET | `/api/unsubscribe/{token}` | Unsubscribe |
| GET | `/api/subscriptions?email=` | List confirmed subscriptions |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `HOST` | `localhost:8080` | Public hostname for email links |
| `DATABASE_URL` | `postgres://...@localhost:5432/genesis` | PostgreSQL connection string |
| `SMTP_HOST` | `localhost` | SMTP server host |
| `SMTP_PORT` | `1025` | SMTP server port |
| `SMTP_FROM` | `noreply@genesis.app` | Sender address |
| `GITHUB_TOKEN` | _(empty)_ | Optional token: 5000 req/hr vs 60 |
| `SCAN_INTERVAL` | `10m` | Release check interval (Go duration) |

## Run Tests

```bash
go test ./...
```

## Database Migrations

Migrations run automatically at startup. SQL files are embedded in the binary
at compile time (`db/migrations/*.sql`) — no filesystem access required at runtime.
