// internal/release/cache_test.go
package release_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"RepoWatch/internal/release"
)

// countingClient tracks how many times GetLatestRelease is called.
type countingClient struct {
	calls  int
	result *release.Release
	err    error
}

func (c *countingClient) RepoExists(_ context.Context, _, _ string) error { return nil }
func (c *countingClient) GetLatestRelease(_ context.Context, _, _ string) (*release.Release, error) {
	c.calls++
	return c.result, c.err
}

// newTestRedis starts an in-process Redis and returns a connected client.
// The server is automatically stopped when the test ends.
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

func TestCachedClient_Miss_CallsInner(t *testing.T) {
	_, rdb := newTestRedis(t)
	inner := &countingClient{result: &release.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}}
	client := release.NewCachedGitHubClient(inner, rdb, time.Minute)

	rel, err := client.GetLatestRelease(context.Background(), "owner", "repo")

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", rel.TagName)
	assert.Equal(t, 1, inner.calls)
}

func TestCachedClient_Hit_SkipsInner(t *testing.T) {
	_, rdb := newTestRedis(t)
	inner := &countingClient{result: &release.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}}
	client := release.NewCachedGitHubClient(inner, rdb, time.Minute)

	// First call — populates the cache
	_, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	require.NoError(t, err)

	// Second call — must be served from cache
	rel, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", rel.TagName)
	assert.Equal(t, 1, inner.calls, "inner should be called only once")
}

func TestCachedClient_NoRelease_NotCached(t *testing.T) {
	_, rdb := newTestRedis(t)
	inner := &countingClient{err: release.ErrNoRelease}
	client := release.NewCachedGitHubClient(inner, rdb, time.Minute)

	_, err1 := client.GetLatestRelease(context.Background(), "owner", "repo")
	_, err2 := client.GetLatestRelease(context.Background(), "owner", "repo")

	assert.ErrorIs(t, err1, release.ErrNoRelease)
	assert.ErrorIs(t, err2, release.ErrNoRelease)
	assert.Equal(t, 2, inner.calls, "ErrNoRelease must not be cached — inner called every time")
}

func TestCachedClient_RedisDown_FallsThrough(t *testing.T) {
	mr, rdb := newTestRedis(t)
	mr.Close() // kill Redis before the call
	inner := &countingClient{result: &release.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}}
	client := release.NewCachedGitHubClient(inner, rdb, time.Minute)

	rel, err := client.GetLatestRelease(context.Background(), "owner", "repo")

	require.NoError(t, err, "Redis being down must not surface as an error")
	assert.Equal(t, "v1.0.0", rel.TagName)
	assert.Equal(t, 1, inner.calls)
}

func TestCachedClient_RepoExists_DelegatesDirectly(t *testing.T) {
	_, rdb := newTestRedis(t)
	inner := &countingClient{}
	client := release.NewCachedGitHubClient(inner, rdb, time.Minute)

	err := client.RepoExists(context.Background(), "owner", "repo")

	assert.NoError(t, err)
	assert.Equal(t, 0, inner.calls, "RepoExists must not go through the release cache")
}
