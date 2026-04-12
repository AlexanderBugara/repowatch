# Deploy to Railway + HTML Subscription Page — Design Spec
**Date:** 2026-04-09  
**Status:** Approved

---

## Overview

Two changes to make the service publicly usable:
1. A minimal HTML subscription page served by the Go service itself.
2. Deployment to Railway via GitHub, using the existing Dockerfile.

---

## Part 1: HTML Subscription Page

### File

`internal/subscription/static/index.html` — embedded into the binary via `//go:embed`.

### Content

- Form with two fields: `email` and `repo` (placeholder: `owner/repo`)
- Submit button
- On submit: `fetch` POST to `/api/subscribe` (no page reload)
- Response shown inline below the form: success message ("Check your email") or error text
- Style: plain CSS, no frameworks, centered card layout

### Routing

New route registered in Chi:

```
GET / → serve index.html
```

### Embedding

In `internal/subscription/handler.go` (or a dedicated file):

```go
//go:embed static/index.html
var indexHTML []byte
```

---

## Part 2: Railway Deployment

### Build

Railway auto-detects `Dockerfile` — no configuration needed.  
Every push to `main` triggers a new deployment.

### Database

Add Railway Postgres plugin. Railway automatically injects `DATABASE_URL` — the service must read it to connect.

### Environment Variables (set in Railway Dashboard)

| Variable | Description |
|----------|-------------|
| `SMTP_HOST` | SMTP server host |
| `SMTP_PORT` | SMTP server port |
| `SMTP_USER` | SMTP username |
| `SMTP_PASS` | SMTP password |
| `FROM_EMAIL` | Sender address |
| `HOST` | Public domain assigned by Railway (used in unsubscribe links) |
| `PORT` | Set automatically by Railway |

### Port Binding

Railway injects `PORT` env variable and routes traffic to it.  
`main.go` must read `PORT` from env with fallback to `8080`:

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

---

## Changes Required

| File | Change |
|------|--------|
| `internal/subscription/static/index.html` | New file — subscription form |
| `internal/subscription/handler.go` | Add `//go:embed`, register `GET /` route |
| `cmd/server/main.go` | Read `PORT` from env |

No changes to `Dockerfile` or existing routes.
