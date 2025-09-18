package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"service", "method", "path", "code"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)
)

// NewMetricsMiddleware Creates HTTP middleware for collecting Prometheus metrics.
func NewMetricsMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			
			defer func() {
				duration := time.Since(start)
				path := r.URL.Path
				
				httpRequestDuration.WithLabelValues(serviceName, r.Method, path).Observe(duration.Seconds())
				httpRequestsTotal.WithLabelValues(serviceName, r.Method, path, strconv.Itoa(ww.Status())).Inc()
			}()

			next.ServeHTTP(ww, r)
		})
	}
}