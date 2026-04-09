// internal/subscription/service_test.go
package subscription_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"RepoWatch/internal/release"
	"RepoWatch/internal/subscription"
)

// --- inline mocks ---

type mockRepo struct {
	subs      []*subscription.Subscription
	createErr error
}

func (m *mockRepo) Create(ctx context.Context, s *subscription.Subscription) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.subs = append(m.subs, s)
	return nil
}
func (m *mockRepo) FindByConfirmToken(ctx context.Context, token string) (*subscription.Subscription, error) {
	for _, s := range m.subs {
		if s.ConfirmToken == token {
			return s, nil
		}
	}
	return nil, subscription.ErrNotFound
}
func (m *mockRepo) FindByUnsubToken(ctx context.Context, token string) (*subscription.Subscription, error) {
	for _, s := range m.subs {
		if s.UnsubToken == token {
			return s, nil
		}
	}
	return nil, subscription.ErrNotFound
}
func (m *mockRepo) FindConfirmedByEmail(ctx context.Context, email string) ([]*subscription.Subscription, error) {
	var out []*subscription.Subscription
	for _, s := range m.subs {
		if s.Email == email && s.Confirmed {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *mockRepo) FindAllConfirmed(ctx context.Context) ([]*subscription.Subscription, error) {
	return m.subs, nil
}
func (m *mockRepo) Confirm(ctx context.Context, id string) error {
	for _, s := range m.subs {
		if s.ID == id {
			s.Confirmed = true
			return nil
		}
	}
	return subscription.ErrNotFound
}
func (m *mockRepo) Delete(ctx context.Context, id string) error {
	for i, s := range m.subs {
		if s.ID == id {
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			return nil
		}
	}
	return subscription.ErrNotFound
}
func (m *mockRepo) UpdateLastSeenTag(ctx context.Context, id, tag string) error { return nil }

type mockGitHub struct{ err error }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) error { return m.err }

type mockNotifier struct{ called bool }

func (m *mockNotifier) SendConfirmation(to, repo, confirmURL string) error {
	m.called = true
	return nil
}
func (m *mockNotifier) SendRelease(to, repo, tagName, releaseURL, unsubURL string) error { return nil }

func newSvc(r subscription.Repository, gh subscription.GitHubChecker, n subscription.EmailNotifier) *subscription.Service {
	return subscription.NewService(r, gh, n, "localhost:8080")
}

// --- tests ---

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	svc := newSvc(&mockRepo{}, &mockGitHub{}, &mockNotifier{})
	err := svc.Subscribe(context.Background(), "user@example.com", "noslash")
	assert.ErrorIs(t, err, subscription.ErrInvalidRepo)
}

func TestSubscribe_RepoNotFound(t *testing.T) {
	svc := newSvc(&mockRepo{}, &mockGitHub{err: release.ErrRepoNotFound}, &mockNotifier{})
	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	assert.ErrorIs(t, err, subscription.ErrRepoNotFound)
}

func TestSubscribe_AlreadyExists(t *testing.T) {
	svc := newSvc(&mockRepo{createErr: subscription.ErrDuplicate}, &mockGitHub{}, &mockNotifier{})
	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	assert.ErrorIs(t, err, subscription.ErrAlreadySubscribed)
}

func TestSubscribe_Success_SendsConfirmation(t *testing.T) {
	repo := &mockRepo{}
	notifier := &mockNotifier{}
	svc := newSvc(repo, &mockGitHub{}, notifier)

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	require.NoError(t, err)
	assert.True(t, notifier.called, "confirmation email should be sent")
	require.Len(t, repo.subs, 1)
	assert.Equal(t, "owner/repo", repo.subs[0].Repo)
	assert.NotEmpty(t, repo.subs[0].ConfirmToken)
	assert.NotEmpty(t, repo.subs[0].UnsubToken)
}

func TestConfirm_TokenNotFound(t *testing.T) {
	svc := newSvc(&mockRepo{}, &mockGitHub{}, &mockNotifier{})
	err := svc.Confirm(context.Background(), "no-such-token")
	assert.ErrorIs(t, err, subscription.ErrNotFound)
}

func TestUnsubscribe_TokenNotFound(t *testing.T) {
	svc := newSvc(&mockRepo{}, &mockGitHub{}, &mockNotifier{})
	err := svc.Unsubscribe(context.Background(), "no-such-token")
	assert.ErrorIs(t, err, subscription.ErrNotFound)
}

func TestListByEmail_EmptyEmail(t *testing.T) {
	svc := newSvc(&mockRepo{}, &mockGitHub{}, &mockNotifier{})
	_, err := svc.ListByEmail(context.Background(), "")
	assert.ErrorIs(t, err, subscription.ErrInvalidEmail)
}

func TestListByEmail_ReturnsOnlyConfirmed(t *testing.T) {
	repo := &mockRepo{subs: []*subscription.Subscription{
		{ID: "1", Email: "u@example.com", Repo: "owner/a", Confirmed: true},
		{ID: "2", Email: "u@example.com", Repo: "owner/b", Confirmed: false},
	}}
	svc := newSvc(repo, &mockGitHub{}, &mockNotifier{})
	subs, err := svc.ListByEmail(context.Background(), "u@example.com")
	require.NoError(t, err)
	assert.Len(t, subs, 1)
	assert.Equal(t, "owner/a", subs[0].Repo)
}
