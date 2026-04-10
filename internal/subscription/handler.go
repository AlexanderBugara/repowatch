// internal/subscription/handler.go
package subscription

import (
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed static/index.html
var indexHTML []byte

// Handler exposes the subscription service over HTTP.
type Handler struct {
	svc         *Service
	scanTrigger func()
}

// NewHandler creates a new HTTP handler wrapping the subscription service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SetScanTrigger registers a function called by POST /api/scan.
func (h *Handler) SetScanTrigger(fn func()) {
	h.scanTrigger = fn
}

type errResponse struct {
	Error string `json:"error"`
}

type msgResponse struct {
	Message string `json:"message"`
}

// subscriptionView is the JSON shape returned to clients (matches Swagger Subscription schema).
type subscriptionView struct {
	Email       string  `json:"email"`
	Repo        string  `json:"repo"`
	Confirmed   bool    `json:"confirmed"`
	LastSeenTag *string `json:"last_seen_tag,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// Subscribe handles POST /api/subscribe.
// Expects form fields: email, repo.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	emailAddr := strings.TrimSpace(r.FormValue("email"))    
  	repo := strings.TrimSpace(r.FormValue("repo"))   

	if emailAddr == "" || repo == "" {
		writeJSON(w, http.StatusBadRequest, errResponse{"email and repo are required"})
		return
	}

	err := h.svc.Subscribe(r.Context(), emailAddr, repo)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRepo):
			writeJSON(w, http.StatusBadRequest, errResponse{err.Error()})
		case errors.Is(err, ErrRepoNotFound):
			writeJSON(w, http.StatusNotFound, errResponse{err.Error()})
		case errors.Is(err, ErrAlreadySubscribed):
			writeJSON(w, http.StatusConflict, errResponse{err.Error()})
		default:
			log.Printf("subscribe error: %v", err)
			writeJSON(w, http.StatusInternalServerError, errResponse{"internal server error"})
		}
		return
	}
	writeJSON(w, http.StatusOK, msgResponse{"Subscription created. Please confirm your email."})
}

// Confirm handles GET /api/confirm/{token}.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errResponse{"token is required"})
		return
	}
	if err := h.svc.Confirm(r.Context(), token); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errResponse{"token not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errResponse{"internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, msgResponse{"Subscription confirmed."})
}

// Unsubscribe handles GET /api/unsubscribe/{token}.
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errResponse{"token is required"})
		return
	}
	if err := h.svc.Unsubscribe(r.Context(), token); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errResponse{"token not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errResponse{"internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, msgResponse{"Unsubscribed successfully."})
}

// TriggerScan handles POST /api/scan — runs an immediate scan cycle.
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if h.scanTrigger != nil {
		go h.scanTrigger()
	}
	writeJSON(w, http.StatusOK, msgResponse{"Scan started."})
}

// ServeIndex handles GET / — serves the HTML subscription page.
func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(indexHTML)
}

// ListSubscriptions handles GET /api/subscriptions?email=...
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	emailAddr := r.URL.Query().Get("email")
	subs, err := h.svc.ListByEmail(r.Context(), emailAddr)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			writeJSON(w, http.StatusBadRequest, errResponse{err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errResponse{"internal server error"})
		return
	}

	views := make([]subscriptionView, 0, len(subs))
	for _, s := range subs {
		views = append(views, subscriptionView{
			Email:       s.Email,
			Repo:        s.Repo,
			Confirmed:   s.Confirmed,
			LastSeenTag: s.LastSeenTag,
		})
	}
	writeJSON(w, http.StatusOK, views)
}
