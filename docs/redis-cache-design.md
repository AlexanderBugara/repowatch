# Redis Cache for GitHub API — Design Spec

---

## Overview

Cache `GetLatestRelease` responses from the GitHub API in Redis with a 10-minute TTL. This reduces GitHub API calls during scan cycles and avoids rate-limit errors as the number of tracked repositories grows.

---

## Scope

Only `GetLatestRelease` is cached. `RepoExists` is called once per new subscription and does not benefit from caching.

---

## Architecture

### New file: `internal/release/cache.go`

A decorator that wraps `GitHubClient` and caches `GetLatestRelease` results in Redis.

```go
type cachedGitHubClient struct {
    inner GitHubClient
    redis *redis.Client
    ttl   time.Duration
}

func NewCachedGitHubClient(inner GitHubClient, rdb *redis.Client, ttl time.Duration) GitHubClient
```

**Cache key:** `github:release:{owner}/{repo}`  
**Value:** JSON-encoded `Release` struct (`tag_name` + `html_url`)  
**TTL:** configurable, default 10 minutes

**Behaviour:**

| Situation | Action |
|-----------|--------|
| Cache hit | Return cached `Release`, skip GitHub API call |
| Cache miss | Call `inner.GetLatestRelease`, store result in Redis, return result |
| `ErrNoRelease` from inner | Do **not** cache (repo may publish a release soon) |
| Redis unavailable (get) | Log warning, fall through to `inner` (fail-open) |
| Redis unavailable (set) | Log warning, return result from `inner` (fail-open) |

`RepoExists` is forwarded directly to `inner` with no caching.

### Changes to existing files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `RedisURL string` field, read from `REDIS_URL` env var |
| `cmd/server/main.go` | If `cfg.RedisURL != ""`, wrap `githubClient` with `NewCachedGitHubClient` |

### `main.go` wiring

```go
githubClient := release.NewGitHubClient(cfg.GitHubToken, "")
if cfg.RedisURL != "" {
    opt, err := redis.ParseURL(cfg.RedisURL)
    if err != nil {
        log.Fatalf("parse REDIS_URL: %v", err)
    }
    githubClient = release.NewCachedGitHubClient(githubClient, redis.NewClient(opt), 10*time.Minute)
}
```

---

## Dependencies

Add to `go.mod`:
```
github.com/redis/go-redis/v9
```

Test dependency:
```
github.com/alicebob/miniredis/v2
```

---

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `REDIS_URL` | `""` | Redis connection string, e.g. `redis://localhost:6379`. If empty, caching is disabled. |

Railway: add Redis plugin → `REDIS_URL` is injected automatically.

---

## Testing

File: `internal/release/cache_test.go`

Uses `miniredis` (in-process Redis) for test isolation — no real Redis required.

| Test | Verifies |
|------|----------|
| `TestCachedClient_Miss_CallsInner` | On first call, inner is called and result stored in Redis |
| `TestCachedClient_Hit_SkipsInner` | On second call, inner is NOT called; cached value returned |
| `TestCachedClient_NoRelease_NotCached` | `ErrNoRelease` is not cached; inner called every time |
| `TestCachedClient_RedisDown_FallsThrough` | Redis unavailable → inner called, no panic |
| `TestCachedClient_RepoExists_NotCached` | `RepoExists` always delegates to inner |
