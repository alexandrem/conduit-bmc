package metrics

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// responseWriter wraps http.ResponseWriter to capture status code
// and implements http.Hijacker for WebSocket support
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface for WebSocket upgrades
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

// Flush implements http.Flusher interface for streaming responses
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
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
