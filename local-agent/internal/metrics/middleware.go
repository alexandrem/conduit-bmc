package metrics

import (
	"net/http"

	coremetrics "core/metrics"
)

// HTTPMetricsMiddleware records HTTP request metrics using the shared core middleware
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return coremetrics.HTTPMetricsMiddleware(HTTPRequestsTotal, HTTPRequestDuration)(next)
}
