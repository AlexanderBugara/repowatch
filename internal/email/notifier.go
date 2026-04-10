// internal/email/notifier.go
package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
)

// Notifier sends email notifications to subscribers.
type Notifier interface {
	SendConfirmation(to, repo, confirmURL string) error
	SendRelease(to, repo, tagName, releaseURL, unsubURL string) error
}

// SMTPNotifier sends emails via SMTP. When user and pass are empty,
// connects without authentication (suitable for MailHog in development).
type SMTPNotifier struct {
	host string
	port string
	from string
	user string
	pass string
}

// NewSMTPNotifier creates a new SMTP-based notifier.
// Pass empty user and pass for unauthenticated servers (e.g. MailHog).
func NewSMTPNotifier(host, port, from, user, pass string) *SMTPNotifier {
	return &SMTPNotifier{host: host, port: port, from: from, user: user, pass: pass}
}

// SendConfirmation sends an email asking the user to confirm their subscription.
func (n *SMTPNotifier) SendConfirmation(to, repo, confirmURL string) error {
	subject := fmt.Sprintf("Confirm your subscription to %s releases", repo)
	return n.send(to, subject, BuildConfirmationBody(repo, confirmURL))
}

// SendRelease sends a new-release notification email.
func (n *SMTPNotifier) SendRelease(to, repo, tagName, releaseURL, unsubURL string) error {
	subject := fmt.Sprintf("New release: %s %s", repo, tagName)
	return n.send(to, subject, BuildReleaseBody(repo, tagName, releaseURL, unsubURL))
}

func (n *SMTPNotifier) send(to, subject, body string) error {
	addr := n.host + ":" + n.port
	msg := []byte(
		"From: " + n.from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)
	var auth smtp.Auth
	if n.user != "" {
		auth = smtp.PlainAuth("", n.user, n.pass, n.host)
	}
	return smtp.SendMail(addr, auth, n.from, []string{to}, msg)
}

// BrevoNotifier sends emails via Brevo HTTP API.
type BrevoNotifier struct {
	apiKey string
	from   string
}

// NewBrevoNotifier creates a Brevo HTTP API notifier.
func NewBrevoNotifier(apiKey, from string) *BrevoNotifier {
	return &BrevoNotifier{apiKey: apiKey, from: from}
}

// SendConfirmation sends an email asking the user to confirm their subscription.
func (n *BrevoNotifier) SendConfirmation(to, repo, confirmURL string) error {
	subject := fmt.Sprintf("Confirm your subscription to %s releases", repo)
	return n.send(to, subject, BuildConfirmationBody(repo, confirmURL))
}

// SendRelease sends a new-release notification email.
func (n *BrevoNotifier) SendRelease(to, repo, tagName, releaseURL, unsubURL string) error {
	subject := fmt.Sprintf("New release: %s %s", repo, tagName)
	return n.send(to, subject, BuildReleaseBody(repo, tagName, releaseURL, unsubURL))
}

func (n *BrevoNotifier) send(to, subject, body string) error {
	payload := map[string]any{
		"sender":      map[string]string{"email": n.from},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"textContent": body,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo api returned status %d", resp.StatusCode)
	}
	return nil
}

// BuildConfirmationBody constructs the plain-text body for a confirmation email.
func BuildConfirmationBody(repo, confirmURL string) string {
	return fmt.Sprintf(
		"You requested to subscribe to release notifications for %s.\n\n"+
			"Please confirm your email address by visiting:\n%s\n\n"+
			"If you did not make this request, you can safely ignore this email.",
		repo, confirmURL,
	)
}

// BuildReleaseBody constructs the plain-text body for a release notification email.
func BuildReleaseBody(repo, tagName, releaseURL, unsubURL string) string {
	return fmt.Sprintf(
		"A new release is available for %s!\n\n"+
			"Version: %s\n"+
			"Release page: %s\n\n"+
			"To unsubscribe from these notifications:\n%s",
		repo, tagName, releaseURL, unsubURL,
	)
}
