// internal/release/scanner_test.go
package release_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"RepoWatch/internal/release"
)

// --- inline mocks ---

type mockScanRepo struct {
	subs       []release.SubscriptionEntry
	updatedID  string
	updatedTag string
}

func (m *mockScanRepo) FindAllConfirmed(ctx context.Context) ([]release.SubscriptionEntry, error) {
	return m.subs, nil
}
func (m *mockScanRepo) UpdateLastSeenTag(ctx context.Context, id, tag string) error {
	m.updatedID = id
	m.updatedTag = tag
	// Simulate updating the in-memory state so subsequent Scan calls don't re-notify.
	for i := range m.subs {
		if m.subs[i].ID == id {
			m.subs[i].LastSeenTag = &tag
		}
	}
	return nil
}

type mockScanGitHub struct {
	releases map[string]*release.Release
	err      error
}

func (m *mockScanGitHub) RepoExists(ctx context.Context, owner, repo string) error { return nil }
func (m *mockScanGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*release.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := owner + "/" + repo
	if rel, ok := m.releases[key]; ok {
		return rel, nil
	}
	return nil, release.ErrNoRelease
}

type mockScanNotifier struct {
	calls []string
}

func (m *mockScanNotifier) SendConfirmation(to, repo, confirmURL string) error { return nil }
func (m *mockScanNotifier) SendRelease(to, repo, tagName, releaseURL, unsubURL string) error {
	m.calls = append(m.calls, to+"|"+repo+"|"+tagName)
	return nil
}

// --- tests ---

func TestScanner_NotifiesOnNewRelease(t *testing.T) {
	repo := &mockScanRepo{subs: []release.SubscriptionEntry{
		{ID: "sub1", Email: "u@example.com", Repo: "owner/repo", UnsubToken: "tok1"},
	}}
	gh := &mockScanGitHub{releases: map[string]*release.Release{
		"owner/repo": {TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}}
	notifier := &mockScanNotifier{}

	scanner := release.NewScanner(repo, gh, notifier, "localhost:8080")
	scanner.Scan(context.Background())

	assert.Len(t, notifier.calls, 1)
	assert.Equal(t, "u@example.com|owner/repo|v1.0.0", notifier.calls[0])
	assert.Equal(t, "sub1", repo.updatedID)
	assert.Equal(t, "v1.0.0", repo.updatedTag)
}

func TestScanner_SkipsIfTagUnchanged(t *testing.T) {
	existing := "v1.0.0"
	repo := &mockScanRepo{subs: []release.SubscriptionEntry{
		{ID: "sub1", Email: "u@example.com", Repo: "owner/repo", UnsubToken: "tok1", LastSeenTag: &existing},
	}}
	gh := &mockScanGitHub{releases: map[string]*release.Release{
		"owner/repo": {TagName: "v1.0.0"},
	}}
	notifier := &mockScanNotifier{}

	scanner := release.NewScanner(repo, gh, notifier, "localhost:8080")
	scanner.Scan(context.Background())

	assert.Empty(t, notifier.calls)
}

func TestScanner_StopsOnRateLimit(t *testing.T) {
	repo := &mockScanRepo{subs: []release.SubscriptionEntry{
		{ID: "sub1", Email: "u@example.com", Repo: "owner/repo", UnsubToken: "tok1"},
		{ID: "sub2", Email: "u2@example.com", Repo: "owner/repo2", UnsubToken: "tok2"},
	}}
	gh := &mockScanGitHub{err: release.ErrRateLimit}
	notifier := &mockScanNotifier{}

	scanner := release.NewScanner(repo, gh, notifier, "localhost:8080")
	scanner.Scan(context.Background())

	assert.Empty(t, notifier.calls)
}

func TestScanner_NotifiesMultipleSubscribersForSameRepo(t *testing.T) {
	repo := &mockScanRepo{subs: []release.SubscriptionEntry{
		{ID: "sub1", Email: "a@example.com", Repo: "owner/repo", UnsubToken: "tok1"},
		{ID: "sub2", Email: "b@example.com", Repo: "owner/repo", UnsubToken: "tok2"},
	}}
	gh := &mockScanGitHub{releases: map[string]*release.Release{
		"owner/repo": {TagName: "v2.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v2.0.0"},
	}}
	notifier := &mockScanNotifier{}

	scanner := release.NewScanner(repo, gh, notifier, "localhost:8080")
	scanner.Scan(context.Background())

	assert.Len(t, notifier.calls, 2)
}
