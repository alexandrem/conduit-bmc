package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Agent metrics collectors
var (
	// Discovery

	DiscoveryCyclesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_discovery_cycles_total",
			Help: "Total number of discovery cycle runs",
		},
		[]string{"status"},
	)

	DiscoveryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_discovery_duration_seconds",
			Help:    "Discovery cycle duration in seconds",
			Buckets: []float64{.5, 1, 2.5, 5, 10, 30, 60},
		},
	)

	ServersDiscovered = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_servers_discovered",
			Help: "Number of BMC servers discovered",
		},
		[]string{"bmc_type"},
	)

	DiscoveryErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_discovery_errors_total",
			Help: "Total number of discovery errors",
		},
		[]string{"error_type"},
	)

	// Gateway Registration

	GatewayRegistrationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_gateway_registrations_total",
			Help: "Total number of gateway registration attempts",
		},
		[]string{"status"},
	)

	GatewayRegistrationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_gateway_registration_duration_seconds",
			Help:    "Gateway registration attempt duration in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	HeartbeatsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_heartbeats_sent_total",
			Help: "Total number of heartbeats sent to gateway",
		},
		[]string{"status"},
	)

	GatewayConnectionStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "agent_gateway_connection_status",
			Help: "Gateway connection status (0=disconnected, 1=connected)",
		},
	)

	// BMC Operations

	BMCOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_bmc_operations_total",
			Help: "Total number of BMC operations executed",
		},
		[]string{"bmc_type", "operation", "status"},
	)

	BMCOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_bmc_operation_duration_seconds",
			Help:    "BMC operation latency in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"bmc_type", "operation"},
	)

	BMCConnectionErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_bmc_connection_errors_total",
			Help: "Total number of BMC connection errors",
		},
		[]string{"bmc_type", "error_type"},
	)

	// SOL/Console Sessions

	SOLSessionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_sol_sessions_total",
			Help: "Number of active SOL sessions",
		},
		[]string{"server_id"},
	)

	SOLBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_sol_bytes_total",
			Help: "Total number of SOL bytes transferred",
		},
		[]string{"direction"},
	)

	SOLReconnectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_sol_reconnections_total",
			Help: "Total number of SOL reconnection attempts",
		},
		[]string{"server_id", "status"},
	)

	SOLErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_sol_errors_total",
			Help: "Total number of SOL session errors",
		},
		[]string{"error_type"},
	)

	// VNC Proxy

	VNCSessionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_vnc_sessions_total",
			Help: "Number of active VNC sessions",
		},
		[]string{"server_id"},
	)

	VNCBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_vnc_bytes_total",
			Help: "Total number of VNC bytes transferred",
		},
		[]string{"direction"},
	)

	VNCConnectionErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_vnc_connection_errors_total",
			Help: "Total number of VNC connection errors",
		},
		[]string{"error_type"},
	)

	// HTTP/RPC Metrics

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	RPCCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_rpc_calls_total",
			Help: "Total number of RPC calls",
		},
		[]string{"service", "method", "status"},
	)

	RPCCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_rpc_call_duration_seconds",
			Help:    "RPC call duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"service", "method"},
	)
)
