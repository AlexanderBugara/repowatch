// internal/subscription/model.go
package subscription

import "time"

// Subscription is the full domain model for an email subscription to a repo's releases.
type Subscription struct {
	ID           string
	Email        string
	Repo         string
	Confirmed    bool
	ConfirmToken string
	UnsubToken   string
	LastSeenTag  *string
	CreatedAt    time.Time
}
