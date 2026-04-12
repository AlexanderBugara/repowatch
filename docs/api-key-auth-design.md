# API Key Authentication — Design Spec
**Date:** 2026-04-12  
**Status:** Approved

---

## Overview

Protect admin endpoints with an API key passed in the `X-API-Key` request header.

---

## Protected Endpoints

| Method | Path | Protected |
|--------|------|-----------|
| `POST` | `/api/scan` | ✓ |
| `GET` | `/api/subscriptions` | ✓ |
| `POST` | `/api/subscribe` | — |
| `GET` | `/api/confirm/{token}` | — |
| `GET` | `/api/unsubscribe/{token}` | — |
| `GET` | `/` | — |

---

## Architecture

### New file: `internal/subscription/middleware.go`

Single exported function:

```go
func APIKeyMiddleware(key string) func(http.Handler) http.Handler
```

- Reads `X-API-Key` header from the request
- If header matches `key` → passes request through
- If header is missing or wrong → returns `401 {"error": "unauthorized"}`
- If `key` is empty (env not set) → passes all requests through (dev mode)

### Config change: `internal/config/config.go`

Add field:

```go
APIKey string  // from API_KEY env var, defaults to ""
```

### Routing change: `cmd/server/main.go`

```go
r.With(subscription.APIKeyMiddleware(cfg.APIKey)).Post("/api/scan", handler.TriggerScan)
r.With(subscription.APIKeyMiddleware(cfg.APIKey)).Get("/api/subscriptions", handler.ListSubscriptions)
```

---

## Behaviour When Key Is Empty

If `API_KEY` env var is not set, middleware is a no-op. This allows local development without configuration.

---

## Error Response

```json
HTTP 401
{"error": "unauthorized"}
```

---

## Testing

Unit tests in `internal/subscription/middleware_test.go`:

- Returns 401 when `X-API-Key` header is absent
- Returns 401 when `X-API-Key` header has wrong value
- Returns 200 when `X-API-Key` header matches configured key
- Passes all requests when key is empty (dev mode)

---

## Deployment

Add to Railway Variables:

```
API_KEY=<random string>
```

---

## Changes Required

| File | Change |
|------|--------|
| `internal/subscription/middleware.go` | New file — `APIKeyMiddleware` |
| `internal/subscription/middleware_test.go` | New file — unit tests |
| `internal/config/config.go` | Add `APIKey` field |
| `cmd/server/main.go` | Wrap two routes with middleware |
