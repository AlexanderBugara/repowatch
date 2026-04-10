// internal/release/scanner.go
package release

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"RepoWatch/internal/email"
)

// SubscriptionEntry is the scanner's minimal view of a subscription row.
// Defined here to avoid importing the subscription package (circular dependency prevention).
type SubscriptionEntry struct {
	ID          string
	Email       string
	Repo        string
	UnsubToken  string
	LastSeenTag *string
}

// SubscriptionSource is the interface the scanner needs to read and update subscriptions.
type SubscriptionSource interface {
	FindAllConfirmed(ctx context.Context) ([]SubscriptionEntry, error)
	UpdateLastSeenTag(ctx context.Context, id, tag string) error
}

// Scanner periodically checks GitHub for new releases and emails confirmed subscribers.
type Scanner struct {
	source   SubscriptionSource
	github   GitHubClient
	notifier email.Notifier
	host     string
}

// NewScanner creates a release scanner.
func NewScanner(source SubscriptionSource, github GitHubClient, notifier email.Notifier, host string) *Scanner {
	return &Scanner{source: source, github: github, notifier: notifier, host: host}
}

// Start launches the periodic scan loop. Blocks until ctx is cancelled.
func (s *Scanner) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Scan(ctx)
		}
	}
}

// Scan performs one full scan cycle. Exported so tests can call it directly without a ticker.
func (s *Scanner) Scan(ctx context.Context) {
	entries, err := s.source.FindAllConfirmed(ctx)
	if err != nil {
		log.Printf("scanner: fetch subscriptions: %v", err)
		return
	}

	// Group entries by repo to make one GitHub API call per unique repo.
	byRepo := make(map[string][]SubscriptionEntry)
	for _, e := range entries {
		byRepo[e.Repo] = append(byRepo[e.Repo], e)
	}

	for repo, subscribers := range byRepo {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			continue
		}

		rel, err := s.github.GetLatestRelease(ctx, parts[0], parts[1])
		if err != nil {
			if errors.Is(err, ErrRateLimit) {
				log.Printf("scanner: GitHub rate limit reached, aborting scan cycle")
				return
			}
			if errors.Is(err, ErrNoRelease) {
				continue
			}
			log.Printf("scanner: get latest release for %s: %v", repo, err)
			continue
		}

		for _, sub := range subscribers {
			if sub.LastSeenTag != nil && *sub.LastSeenTag == rel.TagName {
				continue
			}
			unsubURL := fmt.Sprintf("https://%s/api/unsubscribe/%s", s.host, sub.UnsubToken)
			if err := s.notifier.SendRelease(sub.Email, repo, rel.TagName, rel.HTMLURL, unsubURL); err != nil {
				log.Printf("scanner: send release email to %s: %v", sub.Email, err)
				continue
			}
			if err := s.source.UpdateLastSeenTag(ctx, sub.ID, rel.TagName); err != nil {
				log.Printf("scanner: update last_seen_tag for %s: %v", sub.ID, err)
			}
		}
	}
}
