// internal/release/github.go
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for GitHub API responses.
var (
	ErrRepoNotFound = errors.New("repository not found")
	ErrRateLimit    = errors.New("github rate limit exceeded")
	ErrNoRelease    = errors.New("no releases found for repository")
)

// Release holds the fields of a GitHub release that the service cares about.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// GitHubClient fetches data from the GitHub REST API.
type GitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) error
	GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error)
}

type httpGitHubClient struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewGitHubClient creates a GitHub API client.
// Pass token="" for unauthenticated access (60 req/hr).
// Pass baseURL="" to use the real GitHub API; pass a test server URL in tests.
func NewGitHubClient(token, baseURL string) GitHubClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &httpGitHubClient{
		token:   token,
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// RepoExists returns nil if the repo exists, ErrRepoNotFound if it doesn't, ErrRateLimit on 429.
func (c *httpGitHubClient) RepoExists(ctx context.Context, owner, repo string) error {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	resp, err := c.do(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrRepoNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		return fmt.Errorf("github: unexpected status %d for repo check", resp.StatusCode)
	}
}

// GetLatestRelease returns the latest release for a repo, or ErrNoRelease / ErrRateLimit.
func (c *httpGitHubClient) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL, owner, repo)
	resp, err := c.do(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		var rel Release
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return nil, fmt.Errorf("github: decode release: %w", err)
		}
		return &rel, nil
	case http.StatusNotFound:
		return nil, ErrNoRelease
	case http.StatusTooManyRequests:
		return nil, ErrRateLimit
	default:
		return nil, fmt.Errorf("github: unexpected status %d for latest release", resp.StatusCode)
	}
}

func (c *httpGitHubClient) do(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}
