package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTPMetricsMiddleware creates middleware that records HTTP request metrics.
// It requires two Prometheus collectors to be provided:
//   - requestsTotal: CounterVec with labels [method, endpoint, status_code]
//   - requestDuration: HistogramVec with labels [method, endpoint]
func HTTPMetricsMiddleware(
	requestsTotal *prometheus.CounterVec,
	requestDuration *prometheus.HistogramVec,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(rw.statusCode)

			requestsTotal.WithLabelValues(r.Method, r.URL.Path, statusCode).Inc()
			requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
		})
	}
}
