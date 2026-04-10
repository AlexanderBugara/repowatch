// internal/subscription/service.go
package subscription

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"RepoWatch/internal/release"
)

// repoPattern validates the owner/repo format.
var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// Service errors that handlers map to HTTP status codes.
var (
	ErrInvalidRepo      = errors.New("invalid repository format, expected owner/repo")
	ErrRepoNotFound     = errors.New("repository not found on GitHub")
	ErrAlreadySubscribed = errors.New("subscription already exists for this email and repository")
	ErrInvalidEmail     = errors.New("email query parameter is required")
)

// GitHubChecker is the subset of GitHub capabilities needed by the subscription service.
type GitHubChecker interface {
	RepoExists(ctx context.Context, owner, repo string) error
}

// EmailNotifier is the subset of notification capabilities needed by the subscription service.
type EmailNotifier interface {
	SendConfirmation(to, repo, confirmURL string) error
}

// Service implements subscription business logic.
type Service struct {
	repo      Repository
	github    GitHubChecker
	notifier  EmailNotifier
	host      string
	onConfirm func()
}

// NewService creates a new subscription service.
func NewService(repo Repository, github GitHubChecker, notifier EmailNotifier, host string) *Service {
	return &Service{repo: repo, github: github, notifier: notifier, host: host}
}

// SetOnConfirm registers a callback invoked after a subscription is confirmed.
func (s *Service) SetOnConfirm(fn func()) {
	s.onConfirm = fn
}

// Subscribe validates the repo, checks GitHub, creates the subscription, and sends a confirmation email.
func (s *Service) Subscribe(ctx context.Context, emailAddr, repo string) error {
	if !repoPattern.MatchString(repo) {
		return ErrInvalidRepo
	}

	parts := strings.SplitN(repo, "/", 2)
	if err := s.github.RepoExists(ctx, parts[0], parts[1]); err != nil {
		if errors.Is(err, release.ErrRepoNotFound) {
			return ErrRepoNotFound
		}
		return fmt.Errorf("check repo existence: %w", err)
	}

	confirmToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate confirm token: %w", err)
	}
	unsubToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate unsub token: %w", err)
	}

	sub := &Subscription{
		Email:        emailAddr,
		Repo:         repo,
		ConfirmToken: confirmToken,
		UnsubToken:   unsubToken,
	}
	if err := s.repo.Create(ctx, sub); err != nil {
		if errors.Is(err, ErrDuplicate) {
			return ErrAlreadySubscribed
		}
		return err
	}

	confirmURL := fmt.Sprintf("https://%s/api/confirm/%s", s.host, confirmToken)
	return s.notifier.SendConfirmation(emailAddr, repo, confirmURL)
}

// Confirm marks a subscription as confirmed by its confirmation token.
func (s *Service) Confirm(ctx context.Context, token string) error {
	sub, err := s.repo.FindByConfirmToken(ctx, token)
	if err != nil {
		return err
	}
	if err := s.repo.Confirm(ctx, sub.ID); err != nil {
		return err
	}
	if s.onConfirm != nil {
		go s.onConfirm()
	}
	return nil
}

// Unsubscribe removes a subscription by its unsubscribe token.
func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	sub, err := s.repo.FindByUnsubToken(ctx, token)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, sub.ID)
}

// ListByEmail returns all confirmed subscriptions for a given email address.
func (s *Service) ListByEmail(ctx context.Context, emailAddr string) ([]*Subscription, error) {
	if emailAddr == "" {
		return nil, ErrInvalidEmail
	}
	return s.repo.FindConfirmedByEmail(ctx, emailAddr)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
