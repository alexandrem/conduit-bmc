---
rfd: "021"
title: "Prometheus Metrics Exporter"
state: "implemented"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: true
dependencies: ["github.com/prometheus/client_golang"]
database_migrations: []
areas: ["observability", "monitoring", "operations"]
---

# RFD 021 - Prometheus Metrics Exporter

**Status:** 🎉 Implemented

## Summary

Add Prometheus metrics exporter endpoints (`/metrics`) to all components (Manager, Gateway, Agent) to enable comprehensive observability of the BMC management system. This exposes standardized metrics for monitoring system health, performance, and business operations.

## Problem

**Current behavior/limitations:**
- No standardized metrics collection across components
- Gateway had a placeholder `/metrics` endpoint with TODO comment
- Limited visibility into system performance and health
- Difficult to monitor business metrics (servers, sessions, operations)
- No integration with modern monitoring stacks (Prometheus, Grafana)

**Why this matters:**
- Production deployments require observability for troubleshooting
- SRE teams need metrics for alerting and capacity planning
- Performance issues are hard to diagnose without metrics
- No visibility into request latency, error rates, or resource usage

**Use cases affected:**
- Production monitoring and alerting
- Performance analysis and optimization
- Capacity planning and scaling decisions
- Troubleshooting operational issues

## Solution

Implement Prometheus metrics exporters in all three components using the official `prometheus/client_golang` library. Metrics follow Prometheus naming conventions with component-specific prefixes (`manager_*`, `gateway_*`, `agent_*`).

**Key Design Decisions:**

- **Component-only prefixes**: Use `manager_*`, `gateway_*`, `agent_*` instead of `conduit_manager_*` for cleaner metric names
- **HTTP middleware**: Automatically collect request metrics for all endpoints
- **Histogram buckets**: Tailored to expected latencies (HTTP requests, database queries, session duration)
- **Standard Go metrics**: Include default Go runtime metrics (goroutines, memory, GC)

**Benefits:**

- Unified observability across all components
- Standard integration with Prometheus/Grafana monitoring stacks
- Automatic HTTP request tracking via middleware
- Performance insights for optimization
- Business metrics for operational visibility

**Architecture Overview:**

```
┌─────────────┐
│ Prometheus  │
└──────┬──────┘
       │ (scrape /metrics every 15s)
       │
       ├──────────────┬──────────────┬──────────────┐
       │              │              │              │
       ▼              ▼              ▼              ▼
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│ Manager  │   │ Gateway  │   │ Gateway  │   │  Agent   │
│  :8080   │   │  :8081   │   │  :8082   │   │  :8090   │
└──────────┘   └──────────┘   └──────────┘   └──────────┘
   /metrics       /metrics       /metrics       /metrics
```

### Component Changes

1. **Manager** (`manager/internal/metrics/`):
   - Metrics package with authentication, server management, database, and HTTP/RPC metrics
   - HTTP middleware for automatic request tracking
   - `/metrics` endpoint using `promhttp.Handler()`
   - Metrics: auth requests, server operations, database queries, token validations

2. **Gateway** (`gateway/internal/metrics/`):
   - Metrics package with agent, session, BMC operations, and WebSocket metrics
   - HTTP middleware for automatic request tracking
   - Updated `/metrics` endpoint (replaced TODO placeholder)
   - Metrics: agent registrations, sessions, proxy operations, WebSocket streaming

3. **Agent** (`local-agent/internal/metrics/`):
   - Metrics package with discovery, registration, BMC operations, and console session metrics
   - HTTP middleware for automatic request tracking
   - New `/metrics` endpoint added to agent routes
   - Metrics: discovery cycles, server count, BMC operations, SOL/VNC sessions

## Implementation Plan

### Phase 1: Foundation ✅

- [x] Add `prometheus/client_golang` dependency to all components
- [x] Define metric collectors for Manager component
- [x] Define metric collectors for Gateway component
- [x] Define metric collectors for Agent component

### Phase 2: Core Implementation ✅

- [x] Implement HTTP middleware for request metrics
- [x] Add `/metrics` endpoint to Manager
- [x] Update `/metrics` endpoint in Gateway (replace placeholder)
- [x] Add `/metrics` endpoint to Agent

### Phase 3: Integration ✅

- [x] Test all components build successfully
- [x] Run test suite (`make test-all`)
- [x] Update startup log messages to show metrics URLs

### Phase 4: Documentation ✅

- [x] Document metrics implementation
- [x] List all exposed metrics by component

## API Changes

### New HTTP Endpoint

All components now expose:

```http
GET /metrics HTTP/1.1
Host: localhost:8080

# Response: Prometheus text format
# TYPE manager_http_requests_total counter
manager_http_requests_total{method="GET",endpoint="/health",status_code="200"} 42
# TYPE manager_http_request_duration_seconds histogram
manager_http_request_duration_seconds_bucket{method="GET",endpoint="/health",le="0.005"} 40
...
```

### Metrics Catalog

#### Manager Metrics (`manager_*`)

**Authentication & Authorization:**
- `manager_auth_requests_total` (counter) - Authentication requests [status, method]
- `manager_auth_duration_seconds` (histogram) - Authentication duration [method]
- `manager_active_tokens` (gauge) - Active JWT tokens
- `manager_token_validations_total` (counter) - Token validations [status]

**Server Management:**
- `manager_servers_total` (gauge) - Registered BMC servers [customer_id, datacenter, status]
- `manager_server_operations_total` (counter) - Server CRUD operations [operation, status]
- `manager_server_operation_duration_seconds` (histogram) - Operation latency [operation]

**Gateway Management:**
- `manager_gateways_total` (gauge) - Registered gateways [datacenter, status]
- `manager_gateway_registrations_total` (counter) - Gateway registrations [status]

**Database Operations:**
- `manager_db_queries_total` (counter) - Database queries [operation, table, status]
- `manager_db_query_duration_seconds` (histogram) - Query latency [operation, table]
- `manager_db_connections` (gauge) - Active database connections

**HTTP/RPC:**
- `manager_http_requests_total` (counter) - HTTP requests [method, endpoint, status_code]
- `manager_http_request_duration_seconds` (histogram) - Request duration [method, endpoint]
- `manager_rpc_calls_total` (counter) - RPC calls [service, method, status]
- `manager_rpc_call_duration_seconds` (histogram) - RPC duration [service, method]

#### Gateway Metrics (`gateway_*`)

**Agent Management:**
- `gateway_agents_total` (gauge) - Connected agents [datacenter, status]
- `gateway_agent_registrations_total` (counter) - Registration attempts [datacenter, status]
- `gateway_agent_heartbeats_total` (counter) - Heartbeats received [agent_id, status]
- `gateway_agent_last_heartbeat_seconds` (gauge) - Seconds since last heartbeat [agent_id]

**Session Management:**
- `gateway_sessions_total` (gauge) - Active console sessions [type={vnc,sol}, customer_id]
- `gateway_session_operations_total` (counter) - Session operations [operation, type, status]
- `gateway_session_duration_seconds` (histogram) - Session lifetime [type]
- `gateway_session_creation_duration_seconds` (histogram) - Creation latency [type]

**BMC Operations Proxy:**
- `gateway_bmc_operations_total` (counter) - BMC operations proxied [operation, status]
- `gateway_bmc_operation_duration_seconds` (histogram) - Operation latency [operation]
- `gateway_proxy_errors_total` (counter) - Proxy errors [error_type]

**WebSocket Streaming:**
- `gateway_websocket_connections_total` (gauge) - Active WebSocket connections [type]
- `gateway_websocket_bytes_transmitted_total` (counter) - Bytes transmitted [type, direction]
- `gateway_websocket_messages_total` (counter) - Messages [type, direction]
- `gateway_websocket_errors_total` (counter) - WebSocket errors [type, error_type]

**HTTP/RPC:**
- `gateway_http_requests_total` (counter) - HTTP requests [method, endpoint, status_code]
- `gateway_http_request_duration_seconds` (histogram) - Request duration [method, endpoint]
- `gateway_rpc_calls_total` (counter) - RPC calls [service, method, status]
- `gateway_rpc_call_duration_seconds` (histogram) - RPC duration [service, method]

#### Agent Metrics (`agent_*`)

**Discovery:**
- `agent_discovery_cycles_total` (counter) - Discovery cycle runs [status]
- `agent_discovery_duration_seconds` (histogram) - Discovery cycle duration
- `agent_servers_discovered` (gauge) - BMC servers discovered [bmc_type]
- `agent_discovery_errors_total` (counter) - Discovery errors [error_type]

**Gateway Registration:**
- `agent_gateway_registrations_total` (counter) - Registration attempts [status]
- `agent_gateway_registration_duration_seconds` (histogram) - Registration duration
- `agent_heartbeats_sent_total` (counter) - Heartbeats sent [status]
- `agent_gateway_connection_status` (gauge) - Connection status (0=disconnected, 1=connected)

**BMC Operations:**
- `agent_bmc_operations_total` (counter) - BMC operations executed [bmc_type, operation, status]
- `agent_bmc_operation_duration_seconds` (histogram) - Operation latency [bmc_type, operation]
- `agent_bmc_connection_errors_total` (counter) - Connection errors [bmc_type, error_type]

**SOL/Console Sessions:**
- `agent_sol_sessions_total` (gauge) - Active SOL sessions [server_id]
- `agent_sol_bytes_total` (counter) - SOL bytes transferred [direction]
- `agent_sol_reconnections_total` (counter) - Reconnection attempts [server_id, status]
- `agent_sol_errors_total` (counter) - Session errors [error_type]

**VNC Proxy:**
- `agent_vnc_sessions_total` (gauge) - Active VNC sessions [server_id]
- `agent_vnc_bytes_total` (counter) - VNC bytes transferred [direction]
- `agent_vnc_connection_errors_total` (counter) - Connection errors [error_type]

**HTTP/RPC:**
- `agent_http_requests_total` (counter) - HTTP requests [method, endpoint, status_code]
- `agent_http_request_duration_seconds` (histogram) - Request duration [method, endpoint]
- `agent_rpc_calls_total` (counter) - RPC calls [service, method, status]
- `agent_rpc_call_duration_seconds` (histogram) - RPC duration [service, method]

### Histogram Buckets

Buckets are tailored to expected latencies:

- **HTTP/RPC requests**: `[.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]` seconds
- **Session duration**: `[60, 300, 900, 1800, 3600, 7200, 14400]` seconds (1min to 4hr)
- **Database queries**: `[.001, .005, .01, .025, .05, .1, .25, .5, 1]` seconds
- **BMC operations**: `[.1, .25, .5, 1, 2.5, 5, 10, 30]` seconds
- **Discovery cycles**: `[.5, 1, 2.5, 5, 10, 30, 60]` seconds

## Configuration

No configuration changes required. Metrics are automatically enabled on startup.

### Accessing Metrics

```bash
# Manager (default port 8080)
curl http://localhost:8080/metrics

# Gateway (default port 8081)
curl http://localhost:8081/metrics

# Agent (default port 8090)
curl http://localhost:8090/metrics
```

### Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'conduit-manager'
    static_configs:
      - targets: ['localhost:8080']

  - job_name: 'conduit-gateway'
    static_configs:
      - targets: ['localhost:8081', 'localhost:8082']

  - job_name: 'conduit-agent'
    static_configs:
      - targets: ['localhost:8090', 'localhost:8091']
```

## Testing Strategy

### Unit Tests

- Metrics packages compile successfully ✅
- All components build without errors ✅
- Test suite passes (`make test-all`) ✅

### Integration Tests

- Start each component and verify `/metrics` endpoint responds
- Verify metrics are in Prometheus text format
- Check that HTTP middleware increments request counters
- Validate histogram buckets are properly configured

### E2E Tests

- Deploy stack with Prometheus
- Verify Prometheus successfully scrapes all components
- Create Grafana dashboards for visualization
- Test metric cardinality under load

## Future Enhancements

1. **Business Logic Instrumentation**:
   - Add explicit metric calls in RPC handlers (auth, server operations, etc.)
   - Track database connection pool metrics
   - Instrument BMC operation proxying with detailed metrics

2. **RPC Interceptor Metrics**:
   - Create Connect RPC interceptor for automatic RPC call tracking
   - Track RPC error codes and types

3. **Grafana Dashboards**:
   - Pre-built dashboards for each component
   - SLI/SLO tracking dashboards
   - Capacity planning dashboards

4. **Alerting Rules**:
   - Prometheus alerting rules for common issues
   - SLO-based alerting

5. **Metric Cardinality Management**:
   - Add configuration to limit label cardinality
   - Implement metric label allowlisting

## Appendix

### Implementation Files

**Manager:**
- `manager/internal/metrics/metrics.go` - Metric definitions
- `manager/internal/metrics/middleware.go` - HTTP middleware
- `manager/cmd/manager/main.go` - Endpoint registration

**Gateway:**
- `gateway/internal/metrics/metrics.go` - Metric definitions
- `gateway/internal/metrics/middleware.go` - HTTP middleware
- `gateway/cmd/gateway/main.go` - Endpoint registration

**Agent:**
- `local-agent/internal/metrics/metrics.go` - Metric definitions
- `local-agent/internal/metrics/middleware.go` - HTTP middleware
- `local-agent/internal/agent/agent.go` - Endpoint registration

### Standard Go Metrics

All components automatically expose standard Go runtime metrics:

- `go_goroutines` - Number of goroutines
- `go_threads` - Number of OS threads
- `go_gc_duration_seconds` - GC pause duration
- `go_memstats_alloc_bytes` - Allocated memory
- `go_memstats_heap_inuse_bytes` - Heap memory in use
- `process_cpu_seconds_total` - CPU time
- `process_resident_memory_bytes` - RSS memory
- `process_open_fds` - Open file descriptors
- `process_start_time_seconds` - Process start time
