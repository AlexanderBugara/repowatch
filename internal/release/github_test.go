// internal/release/github_test.go
package release_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"RepoWatch/internal/release"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubClient_RepoExists_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"full_name": "owner/repo"})
	}))
	defer srv.Close()

	client := release.NewGitHubClient("", srv.URL)
	err := client.RepoExists(context.Background(), "owner", "repo")
	require.NoError(t, err)
}

func TestGitHubClient_RepoExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := release.NewGitHubClient("", srv.URL)
	err := client.RepoExists(context.Background(), "owner", "missing")
	assert.ErrorIs(t, err, release.ErrRepoNotFound)
}

func TestGitHubClient_RepoExists_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := release.NewGitHubClient("", srv.URL)
	err := client.RepoExists(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, release.ErrRateLimit)
}

func TestGitHubClient_GetLatestRelease_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/owner/repo/releases/latest", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"tag_name": "v1.2.3",
			"html_url": "https://github.com/owner/repo/releases/tag/v1.2.3",
		})
	}))
	defer srv.Close()

	client := release.NewGitHubClient("", srv.URL)
	rel, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", rel.TagName)
	assert.Equal(t, "https://github.com/owner/repo/releases/tag/v1.2.3", rel.HTMLURL)
}

func TestGitHubClient_GetLatestRelease_NoReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := release.NewGitHubClient("", srv.URL)
	_, err := client.GetLatestRelease(context.Background(), "owner", "repo")
	assert.ErrorIs(t, err, release.ErrNoRelease)
}

func TestGitHubClient_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	client := release.NewGitHubClient("mytoken", srv.URL)
	client.RepoExists(context.Background(), "owner", "repo")
	assert.Equal(t, "Bearer mytoken", gotAuth)
}
