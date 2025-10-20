package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Manager metrics collectors
var (
	// Authentication & Authorization

	AuthRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_auth_requests_total",
			Help: "Total number of authentication requests",
		},
		[]string{"status", "method"},
	)

	AuthDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "manager_auth_duration_seconds",
			Help:    "Authentication request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method"},
	)

	ActiveTokens = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "manager_active_tokens",
			Help: "Current number of active JWT tokens",
		},
	)

	TokenValidationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_token_validations_total",
			Help: "Total number of token validation attempts",
		},
		[]string{"status"},
	)

	// Server Management

	ServersTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "manager_servers_total",
			Help: "Total number of BMC servers registered",
		},
		[]string{"customer_id", "datacenter", "status"},
	)

	ServerOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_server_operations_total",
			Help: "Total number of server CRUD operations",
		},
		[]string{"operation", "status"},
	)

	ServerOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "manager_server_operation_duration_seconds",
			Help:    "Server operation latency in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)

	// Gateway Management

	GatewaysTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "manager_gateways_total",
			Help: "Total number of gateways registered",
		},
		[]string{"datacenter", "status"},
	)

	GatewayRegistrationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_gateway_registrations_total",
			Help: "Total number of gateway registration attempts",
		},
		[]string{"status"},
	)

	// Database Operations

	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table", "status"},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "manager_db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "table"},
	)

	DBConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "manager_db_connections",
			Help: "Number of active database connections",
		},
	)

	// HTTP/RPC Metrics

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "manager_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	RPCCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "manager_rpc_calls_total",
			Help: "Total number of RPC calls",
		},
		[]string{"service", "method", "status"},
	)

	RPCCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "manager_rpc_call_duration_seconds",
			Help:    "RPC call duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"service", "method"},
	)
)
