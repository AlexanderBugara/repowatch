// internal/release/cache.go
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// cachedGitHubClient wraps a GitHubClient and caches GetLatestRelease results in Redis.
// RepoExists is always forwarded to the inner client without caching.
// All Redis errors are logged and the inner client is used as fallback (fail-open).
type cachedGitHubClient struct {
	inner GitHubClient
	redis *redis.Client
	ttl   time.Duration
}

// NewCachedGitHubClient returns a GitHubClient that caches GetLatestRelease in Redis.
func NewCachedGitHubClient(inner GitHubClient, rdb *redis.Client, ttl time.Duration) GitHubClient {
	return &cachedGitHubClient{inner: inner, redis: rdb, ttl: ttl}
}

// RepoExists delegates directly to the inner client — not cached.
func (c *cachedGitHubClient) RepoExists(ctx context.Context, owner, repo string) error {
	return c.inner.RepoExists(ctx, owner, repo)
}

// GetLatestRelease returns a cached release if available, otherwise calls the inner
// client and stores the result. Errors (including ErrNoRelease) are never cached.
func (c *cachedGitHubClient) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	key := fmt.Sprintf("github:release:%s/%s", owner, repo)

	// Try cache hit.
	val, err := c.redis.Get(ctx, key).Result()
	if err == nil {
		var rel Release
		if jsonErr := json.Unmarshal([]byte(val), &rel); jsonErr == nil {
			return &rel, nil
		}
	} else if err != redis.Nil {
		// Redis is down or returned an unexpected error — log and fall through.
		log.Printf("cache: redis get %s: %v", key, err)
	}

	// Cache miss or Redis error — call the real GitHub API.
	rel, err := c.inner.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, err // never cache errors
	}

	// Store in Redis. Ignore storage failures — the data was returned successfully.
	if b, jsonErr := json.Marshal(rel); jsonErr == nil {
		if setErr := c.redis.Set(ctx, key, b, c.ttl).Err(); setErr != nil {
			log.Printf("cache: redis set %s: %v", key, setErr)
		}
	}

	return rel, nil
}
