// internal/subscription/middleware.go
package subscription

import (
	"net/http"
)

// APIKeyMiddleware protects a route with a static API key read from the X-API-Key header.
// If key is empty, all requests are allowed (dev mode).
func APIKeyMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key != "" && r.Header.Get("X-API-Key") != key {
				writeJSON(w, http.StatusUnauthorized, errResponse{"unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
