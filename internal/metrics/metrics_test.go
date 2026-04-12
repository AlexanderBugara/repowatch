// internal/metrics/metrics_test.go
package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"RepoWatch/internal/metrics"
)

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.Counter.GetValue()
}

func counterVecValue(t *testing.T, cv *prometheus.CounterVec, labels prometheus.Labels) float64 {
	t.Helper()
	c, err := cv.GetMetricWith(labels)
	require.NoError(t, err)
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.Counter.GetValue()
}

func TestHTTPMiddleware_IncrementsRequestCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	r := chi.NewRouter()
	r.Use(m.HTTPMiddleware)
	r.Get("/api/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	val := counterVecValue(t, m.HTTPRequestsTotal, prometheus.Labels{
		"method": "GET",
		"path":   "/api/subscriptions",
		"status": "200",
	})
	assert.Equal(t, 1.0, val)
}

func TestHTTPMiddleware_RecordsDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	r := chi.NewRouter()
	r.Use(m.HTTPMiddleware)
	r.Get("/api/subscribe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/subscribe", nil)
	httptest.NewRecorder()
	r.ServeHTTP(httptest.NewRecorder(), req)

	// Verify histogram has at least one observation.
	mfs, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "http_request_duration_seconds" {
			for _, m := range mf.GetMetric() {
				if m.Histogram.GetSampleCount() > 0 {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "http_request_duration_seconds should have observations")
}

func TestMetrics_BusinessCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	m.SubscriptionsCreated.Inc()
	m.SubscriptionsCreated.Inc()
	m.SubscriptionsConfirmed.Inc()
	m.ScanCyclesTotal.Inc()
	m.EmailsSentTotal.WithLabelValues("confirmation").Inc()
	m.EmailsSentTotal.WithLabelValues("release").Inc()
	m.EmailsSentTotal.WithLabelValues("release").Inc()

	assert.Equal(t, 2.0, counterValue(t, m.SubscriptionsCreated))
	assert.Equal(t, 1.0, counterValue(t, m.SubscriptionsConfirmed))
	assert.Equal(t, 1.0, counterValue(t, m.ScanCyclesTotal))
	assert.Equal(t, 1.0, counterVecValue(t, m.EmailsSentTotal, prometheus.Labels{"type": "confirmation"}))
	assert.Equal(t, 2.0, counterVecValue(t, m.EmailsSentTotal, prometheus.Labels{"type": "release"}))
}
