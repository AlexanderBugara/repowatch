// internal/email/notifier_test.go
package email_test

import (
	"testing"

	"RepoWatch/internal/email"

	"github.com/stretchr/testify/assert"
)

// Compile-time check that SMTPNotifier satisfies the Notifier interface.
var _ email.Notifier = (*email.SMTPNotifier)(nil)

func TestBuildConfirmationBody_ContainsRepoAndURL(t *testing.T) {
	body := email.BuildConfirmationBody("owner/repo", "http://localhost:8080/api/confirm/abc123")
	assert.Contains(t, body, "owner/repo")
	assert.Contains(t, body, "http://localhost:8080/api/confirm/abc123")
}

func TestBuildReleaseBody_ContainsAllFields(t *testing.T) {
	body := email.BuildReleaseBody(
		"owner/repo",
		"v1.2.3",
		"https://github.com/owner/repo/releases/tag/v1.2.3",
		"http://localhost:8080/api/unsubscribe/tok123",
	)
	assert.Contains(t, body, "owner/repo")
	assert.Contains(t, body, "v1.2.3")
	assert.Contains(t, body, "https://github.com/owner/repo/releases/tag/v1.2.3")
	assert.Contains(t, body, "http://localhost:8080/api/unsubscribe/tok123")
}
