// internal/metrics/metrics.go
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds all Prometheus metric variables for the service.
type Metrics struct {
	HTTPRequestsTotal         *prometheus.CounterVec
	HTTPRequestDuration       *prometheus.HistogramVec
	SubscriptionsCreated      prometheus.Counter
	SubscriptionsConfirmed    prometheus.Counter
	SubscriptionsUnsubscribed prometheus.Counter
	ScanCyclesTotal           prometheus.Counter
	EmailsSentTotal           *prometheus.CounterVec
}

// NewMetrics creates and registers all metrics with the given Registerer.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		SubscriptionsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "subscriptions_created_total",
			Help: "Total number of subscriptions created.",
		}),

		SubscriptionsConfirmed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "subscriptions_confirmed_total",
			Help: "Total number of subscriptions confirmed.",
		}),

		SubscriptionsUnsubscribed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "subscriptions_unsubscribed_total",
			Help: "Total number of unsubscribes.",
		}),

		ScanCyclesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scan_cycles_total",
			Help: "Total number of GitHub scan cycles run.",
		}),

		EmailsSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "emails_sent_total",
			Help: "Total number of emails sent.",
		}, []string{"type"}),
	}

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.SubscriptionsCreated,
		m.SubscriptionsConfirmed,
		m.SubscriptionsUnsubscribed,
		m.ScanCyclesTotal,
		m.EmailsSentTotal,
	)

	return m
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// HTTPMiddleware records request count and duration for every HTTP request.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		path := chi.RouteContext(r.Context()).RoutePattern()
		if path == "" {
			path = r.URL.Path
		}
		status := strconv.Itoa(rw.status)
		elapsed := time.Since(start).Seconds()

		m.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(elapsed)
	})
}
