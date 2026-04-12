# Prometheus Metrics — Design Spec

---

## Overview

Expose a `/metrics` endpoint in Prometheus format with Go runtime metrics, HTTP request metrics, and business-level counters. Endpoint is protected by `X-API-Key` header.

---

## Metrics

| Metric | Type | Labels | Source |
|--------|------|--------|--------|
| `http_requests_total` | Counter | `method`, `path`, `status` | HTTP middleware |
| `http_request_duration_seconds` | Histogram | `method`, `path` | HTTP middleware |
| `subscriptions_created_total` | Counter | — | `service.go` |
| `subscriptions_confirmed_total` | Counter | — | `service.go` |
| `subscriptions_unsubscribed_total` | Counter | — | `service.go` |
| `scan_cycles_total` | Counter | — | `scanner.go` |
| `emails_sent_total` | Counter | `type` (confirmation\|release) | `notifier.go` |
| Go runtime (memory, goroutines, GC) | Various | — | `promhttp` built-in |

### High-Cardinality Prevention

HTTP middleware uses Chi route pattern (`/api/confirm/{token}`) instead of the actual request path. This prevents one label value per unique token.

---

## Architecture

### New package: `internal/metrics/`

**`metrics.go`** — registers and exports all metric variables:

```go
var (
    HTTPRequestsTotal    *prometheus.CounterVec
    HTTPRequestDuration  *prometheus.HistogramVec
    SubscriptionsCreated prometheus.Counter
    SubscriptionsConfirmed prometheus.Counter
    SubscriptionsUnsubscribed prometheus.Counter
    ScanCyclesTotal      prometheus.Counter
    EmailsSentTotal      *prometheus.CounterVec
)

func Init() // registers all metrics with default registry
```

**`middleware.go`** — HTTP middleware that records request count and duration:

```go
func HTTPMetrics(next http.Handler) http.Handler
```

Uses `chi.RouteContext(r.Context()).RoutePattern()` for the path label.

### Changes to existing code

| File | Change |
|------|--------|
| `internal/subscription/service.go` | Increment `SubscriptionsCreated`, `SubscriptionsConfirmed`, `SubscriptionsUnsubscribed` |
| `internal/release/scanner.go` | Increment `ScanCyclesTotal` at start of each `Scan()` call |
| `internal/email/notifier.go` | Increment `EmailsSentTotal` with appropriate `type` label |
| `cmd/server/main.go` | Call `metrics.Init()`, register middleware and `GET /metrics` route |

### Routing

```go
r.Use(metrics.HTTPMetrics)
r.With(subscription.APIKeyMiddleware(cfg.APIKey)).Get("/metrics", promhttp.Handler())
```

---

## Dependencies

Add to `go.mod`:
```
github.com/prometheus/client_golang
```

---

## Testing

Unit test in `internal/metrics/middleware_test.go`:
- After a request, `http_requests_total` counter incremented with correct labels
- After a request, `http_request_duration_seconds` histogram has one observation
