// internal/subscription/middleware_test.go
package subscription_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"RepoWatch/internal/subscription"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func applyMiddleware(key string, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	subscription.APIKeyMiddleware(key)(http.HandlerFunc(okHandler)).ServeHTTP(w, r)
	return w
}

func TestAPIKeyMiddleware_Returns401WhenHeaderAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := applyMiddleware("secret", req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPIKeyMiddleware_Returns401WhenKeyIsWrong(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrong")
	w := applyMiddleware("secret", req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPIKeyMiddleware_Returns200WhenKeyIsCorrect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	w := applyMiddleware("secret", req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyMiddleware_PassesAllWhenKeyIsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// no header set
	w := applyMiddleware("", req)
	assert.Equal(t, http.StatusOK, w.Code)
}
