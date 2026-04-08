// internal/email/notifier.go
package email

import (
	"fmt"
	"net/smtp"
)

// Notifier sends email notifications to subscribers.
type Notifier interface {
	SendConfirmation(to, repo, confirmURL string) error
	SendRelease(to, repo, tagName, releaseURL, unsubURL string) error
}

// SMTPNotifier sends emails via a plain SMTP server (no TLS auth — suitable for MailHog).
type SMTPNotifier struct {
	host string
	port string
	from string
}

// NewSMTPNotifier creates a new SMTP-based notifier.
func NewSMTPNotifier(host, port, from string) *SMTPNotifier {
	return &SMTPNotifier{host: host, port: port, from: from}
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
	return smtp.SendMail(addr, nil, n.from, []string{to}, msg)
}
