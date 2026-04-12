# GitHub Release Notifier — Design Spec
**Date:** 2026-04-07  
**Status:** Approved

---

## Overview

A Go monolith that lets users subscribe their email to GitHub repository release notifications. When a new release appears, the service sends an email to all confirmed subscribers.

---

## Stack

| Concern | Choice |
|---------|--------|
| Language | Go |
| HTTP Router | Chi |
| Database | PostgreSQL |
| Migrations | golang-migrate (auto-applied at startup) |
| SMTP (dev) | MailHog (Docker) |
| GitHub client | net/http (standard library) |
| Testing | testify + interface mocks |

---

## Project Structure

```
genesis/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── subscription/
│   │   ├── handler.go        # HTTP handlers
│   │   ├── service.go        # business logic
│   │   ├── repository.go     # interface + PostgreSQL implementation
│   │   └── service_test.go
│   ├── release/
│   │   ├── scanner.go        # periodic goroutine with time.Ticker
│   │   ├── scanner_test.go
│   │   └── github.go         # GitHub API client with rate limit handling
│   └── email/
│       ├── notifier.go       # SMTP sender
│       └── notifier_test.go
├── db/
│   └── migrations/           # numbered .sql migration files
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## Database Schema

```sql
CREATE TABLE subscriptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL,
    repo          VARCHAR(255) NOT NULL,        -- format: owner/repo
    confirmed     BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_token VARCHAR(255) UNIQUE NOT NULL,
    unsub_token   VARCHAR(255) UNIQUE NOT NULL,
    last_seen_tag VARCHAR(255),
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(email, repo)
);
```

Migrations run automatically at service startup via `golang-migrate`.

---

## API Contract (Swagger 2.0, must not be changed)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/subscribe` | Subscribe email to repo releases (form params: email, repo) |
| GET | `/api/confirm/{token}` | Confirm subscription via token |
| GET | `/api/unsubscribe/{token}` | Unsubscribe via token |
| GET | `/api/subscriptions?email=` | List confirmed subscriptions for email |

**HTTP status codes:**
- `200` — success
- `400` — validation error (bad format)
- `404` — repository not found on GitHub (or token not found)
- `409` — subscription already exists

---

## Subscription Flow

1. `POST /subscribe` receives `email` + `repo` (form params)
2. Validate `repo` format matches `owner/repo` regex → 400 if invalid
3. Call GitHub API `GET /repos/{owner}/{repo}` → 404 if not found
4. Check `UNIQUE(email, repo)` constraint → 409 if already exists
5. Generate `confirm_token` and `unsub_token` via `crypto/rand`
6. Insert row with `confirmed=false`
7. Send confirmation email with link `http://{HOST}/api/confirm/{confirm_token}`

---

## Scanner Flow

1. `time.Ticker` fires every `SCAN_INTERVAL` (env, default `10m`)
2. Query all `confirmed=true` subscriptions
3. For each unique `repo`, call GitHub API `GET /repos/{owner}/{repo}/releases/latest`
4. If `tag_name != last_seen_tag` (or `last_seen_tag` is NULL) → send release email, update `last_seen_tag`
5. On GitHub `429` response → log warning, skip iteration, respect `Retry-After` header if present

---

## Email Templates

**Confirmation email:**
- Subject: `Confirm your subscription to {repo} releases`
- Body: confirmation link `http://{HOST}/api/confirm/{confirm_token}`

**Release notification email:**
- Subject: `New release: {repo} {tag_name}`
- Body: release info + GitHub link + unsubscribe link `http://{HOST}/api/unsubscribe/{unsub_token}`

---

## Configuration (env variables)

```env
PORT=8080
HOST=localhost:8080
DATABASE_URL=postgres://user:pass@db:5432/genesis
SMTP_HOST=mailhog
SMTP_PORT=1025
SMTP_FROM=noreply@genesis.app
GITHUB_TOKEN=          # optional, enables 5000 req/hr instead of 60
SCAN_INTERVAL=10m      # any Go duration string: 5m, 1h, etc.
```

---

## Docker Setup

`docker-compose.yml` starts three services:
- `app` — the Go binary
- `postgres` — PostgreSQL 16
- `mailhog` — SMTP server with web UI on port `8025`

---

## Testing Strategy

- **Unit tests** (required): service layer with mocked repository and notifier interfaces; scanner logic with mocked GitHub client
- **Integration tests** (bonus): full HTTP handler tests against a real test database
