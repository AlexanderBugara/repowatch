// internal/subscription/integration_test.go
package subscription_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"RepoWatch/db"
	"RepoWatch/internal/subscription"
)

// integrationDB connects to TEST_DATABASE_URL and skips if it is not set.
func integrationDB(t *testing.T) *subscription.PostgresRepository {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration tests")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err, "connect to test database")
	require.NoError(t, db.Migrate(dsn), "run migrations")

	t.Cleanup(func() {
		pool.Exec(ctx, "TRUNCATE TABLE subscriptions RESTART IDENTITY CASCADE")
		pool.Close()
	})
	pool.Exec(ctx, "TRUNCATE TABLE subscriptions RESTART IDENTITY CASCADE")
	return subscription.NewPostgresRepository(pool)
}

// integrationHandler builds a chi router wired to a real repo and inline mocks.
func integrationHandler(repo subscription.Repository) (*chi.Mux, *mockNotifier) {
	notifier := &mockNotifier{}
	svc := subscription.NewService(repo, &mockGitHub{}, notifier, "localhost:8080")
	h := subscription.NewHandler(svc)

	r := chi.NewRouter()
	r.Post("/api/subscribe", h.Subscribe)
	r.Get("/api/confirm/{token}", h.Confirm)
	r.Get("/api/unsubscribe/{token}", h.Unsubscribe)
	r.Get("/api/subscriptions", h.ListSubscriptions)
	r.Get("/", h.ServeIndex)
	return r, notifier
}

// postForm sends a POST with form-encoded body.
func postForm(router http.Handler, path string, fields map[string]string) *httptest.ResponseRecorder {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// getPath sends a GET request.
func getPath(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- integration tests ---

func TestIntegration_Subscribe_CreatesUnconfirmedSubscription(t *testing.T) {
	repo := integrationDB(t)
	router, notifier := integrationHandler(repo)

	w := postForm(router, "/api/subscribe", map[string]string{
		"email": "alice@example.com",
		"repo":  "owner/repo",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, notifier.called, "confirmation email should be sent")

	// Subscription must NOT appear in confirmed list yet.
	subs, err := repo.FindConfirmedByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Empty(t, subs, "subscription must be unconfirmed after subscribe")
}

func TestIntegration_Confirm_ActivatesSubscription(t *testing.T) {
	repo := integrationDB(t)
	router, notifier := integrationHandler(repo)

	// Subscribe.
	w := postForm(router, "/api/subscribe", map[string]string{
		"email": "bob@example.com",
		"repo":  "owner/repo",
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, notifier.called)

	// Extract confirm token from the DB.
	// (We can't read it from the email in integration tests, so we query directly.)
	ctx := context.Background()
	all, err := repo.FindAllConfirmed(ctx) // returns 0 since not confirmed yet
	require.NoError(t, err)
	assert.Empty(t, all)

	// Find the token via the subscriptions table by querying unconfirmed.
	// We use FindByConfirmToken indirectly via GET /api/confirm/:token.
	// To get the token we subscribe again and capture the notifier call.
	// Easier: reset and subscribe via a notifier that records the confirm URL.

	// Reset and use a notifier that captures the confirmURL.
	pool2DSN := os.Getenv("TEST_DATABASE_URL")
	pool2, err2 := db.Connect(ctx, pool2DSN)
	require.NoError(t, err2)
	defer pool2.Close()
	repo2 := subscription.NewPostgresRepository(pool2)

	capturingNotifier := &capturingMockNotifier{}
	svc2 := subscription.NewService(repo2, &mockGitHub{}, capturingNotifier, "localhost")
	h2 := subscription.NewHandler(svc2)
	r2 := chi.NewRouter()
	r2.Post("/api/subscribe", h2.Subscribe)
	r2.Get("/api/confirm/{token}", h2.Confirm)
	r2.Get("/api/subscriptions", h2.ListSubscriptions)

	w2 := postForm(r2, "/api/subscribe", map[string]string{
		"email": "carol@example.com",
		"repo":  "owner/repo",
	})
	require.Equal(t, http.StatusOK, w2.Code)
	require.NotEmpty(t, capturingNotifier.confirmToken)

	// Confirm the subscription.
	wConfirm := getPath(r2, "/api/confirm/"+capturingNotifier.confirmToken)
	assert.Equal(t, http.StatusOK, wConfirm.Code)

	// Now the subscription must appear as confirmed.
	confirmed, err := repo2.FindConfirmedByEmail(ctx, "carol@example.com")
	require.NoError(t, err)
	require.Len(t, confirmed, 1)
	assert.Equal(t, "owner/repo", confirmed[0].Repo)
	assert.True(t, confirmed[0].Confirmed)
}

func TestIntegration_Unsubscribe_RemovesSubscription(t *testing.T) {
	repo := integrationDB(t)
	ctx := context.Background()

	capturingNotifier := &capturingMockNotifier{}
	svc := subscription.NewService(repo, &mockGitHub{}, capturingNotifier, "localhost")
	h := subscription.NewHandler(svc)
	r := chi.NewRouter()
	r.Post("/api/subscribe", h.Subscribe)
	r.Get("/api/confirm/{token}", h.Confirm)
	r.Get("/api/unsubscribe/{token}", h.Unsubscribe)

	// Subscribe and confirm.
	w := postForm(r, "/api/subscribe", map[string]string{
		"email": "dave@example.com",
		"repo":  "owner/repo",
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, capturingNotifier.confirmToken)

	// unsubToken is stored in the DB but not sent in the confirmation email — fetch it directly.
	sub, err := repo.FindByConfirmToken(ctx, capturingNotifier.confirmToken)
	require.NoError(t, err)
	require.NotEmpty(t, sub.UnsubToken)

	getPath(r, "/api/confirm/"+capturingNotifier.confirmToken)

	// Unsubscribe.
	wUnsub := getPath(r, "/api/unsubscribe/"+sub.UnsubToken)
	assert.Equal(t, http.StatusOK, wUnsub.Code)

	// Subscription must be gone.
	confirmed, err := repo.FindConfirmedByEmail(ctx, "dave@example.com")
	require.NoError(t, err)
	assert.Empty(t, confirmed)
}

func TestIntegration_ListSubscriptions_ReturnsConfirmedOnly(t *testing.T) {
	repo := integrationDB(t)

	capturingNotifier := &capturingMockNotifier{}
	svc := subscription.NewService(repo, &mockGitHub{}, capturingNotifier, "localhost")
	h := subscription.NewHandler(svc)
	r := chi.NewRouter()
	r.Post("/api/subscribe", h.Subscribe)
	r.Get("/api/confirm/{token}", h.Confirm)
	r.Get("/api/subscriptions", h.ListSubscriptions)

	// Subscribe to two repos, confirm only one.
	postForm(r, "/api/subscribe", map[string]string{"email": "eve@example.com", "repo": "owner/repo-a"})
	tokenA := capturingNotifier.confirmToken

	postForm(r, "/api/subscribe", map[string]string{"email": "eve@example.com", "repo": "owner/repo-b"})
	// don't confirm repo-b

	getPath(r, "/api/confirm/"+tokenA)

	// GET /api/subscriptions?email=eve@example.com
	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions?email=eve@example.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var subs []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&subs))
	assert.Len(t, subs, 1)
	assert.Equal(t, "owner/repo-a", subs[0]["repo"])
}

func TestIntegration_Subscribe_DuplicateReturns409(t *testing.T) {
	repo := integrationDB(t)
	router, _ := integrationHandler(repo)

	fields := map[string]string{"email": "frank@example.com", "repo": "owner/repo"}
	w1 := postForm(router, "/api/subscribe", fields)
	require.Equal(t, http.StatusOK, w1.Code)

	w2 := postForm(router, "/api/subscribe", fields)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

// capturingMockNotifier records confirmToken and unsubToken so tests can use them.
type capturingMockNotifier struct {
	confirmToken string
	unsubToken   string
}

func (n *capturingMockNotifier) SendConfirmation(to, repo, confirmURL string) error {
	// Extract token from URL: http://localhost/api/confirm/<token>
	parts := strings.Split(confirmURL, "/api/confirm/")
	if len(parts) == 2 {
		n.confirmToken = parts[1]
	}
	return nil
}

func (n *capturingMockNotifier) SendRelease(to, repo, tagName, releaseURL, unsubURL string) error {
	// Extract unsub token from URL: http://localhost/api/unsubscribe/<token>
	parts := strings.Split(unsubURL, "/api/unsubscribe/")
	if len(parts) == 2 {
		n.unsubToken = parts[1]
	}
	return nil
}
