package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Gateway metrics collectors
var (
	// Agent Management

	AgentsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_agents_total",
			Help: "Total number of agents connected",
		},
		[]string{"datacenter", "status"},
	)

	AgentRegistrationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_agent_registrations_total",
			Help: "Total number of agent registration attempts",
		},
		[]string{"datacenter", "status"},
	)

	AgentHeartbeatsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_agent_heartbeats_total",
			Help: "Total number of agent heartbeats received",
		},
		[]string{"agent_id", "status"},
	)

	AgentLastHeartbeat = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_agent_last_heartbeat_seconds",
			Help: "Seconds since last heartbeat from agent",
		},
		[]string{"agent_id"},
	)

	// Session Management

	SessionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_sessions_total",
			Help: "Number of active console sessions",
		},
		[]string{"type", "customer_id"},
	)

	SessionOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_session_operations_total",
			Help: "Total number of session operations",
		},
		[]string{"operation", "type", "status"},
	)

	SessionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_session_duration_seconds",
			Help:    "Session lifetime duration in seconds",
			Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400},
		},
		[]string{"type"},
	)

	SessionCreationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_session_creation_duration_seconds",
			Help:    "Session creation latency in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"type"},
	)

	// BMC Operations Proxy

	BMCOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_bmc_operations_total",
			Help: "Total number of BMC operations proxied",
		},
		[]string{"operation", "status"},
	)

	BMCOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_bmc_operation_duration_seconds",
			Help:    "BMC operation latency in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"operation"},
	)

	ProxyErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_proxy_errors_total",
			Help: "Total number of proxy errors",
		},
		[]string{"error_type"},
	)

	// WebSocket Streaming

	WebSocketConnectionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_websocket_connections_total",
			Help: "Number of active WebSocket connections",
		},
		[]string{"type"},
	)

	WebSocketBytesTransmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_websocket_bytes_transmitted_total",
			Help: "Total number of WebSocket bytes transmitted",
		},
		[]string{"type", "direction"},
	)

	WebSocketMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_websocket_messages_total",
			Help: "Total number of WebSocket messages",
		},
		[]string{"type", "direction"},
	)

	WebSocketErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_websocket_errors_total",
			Help: "Total number of WebSocket errors",
		},
		[]string{"type", "error_type"},
	)

	// HTTP/RPC Metrics

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	RPCCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_rpc_calls_total",
			Help: "Total number of RPC calls",
		},
		[]string{"service", "method", "status"},
	)

	RPCCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_rpc_call_duration_seconds",
			Help:    "RPC call duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"service", "method"},
	)
)
