---
rfd: "022"
title: "OpenTelemetry Distributed Tracing and Observability"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
dependencies: [
  "go.opentelemetry.io/otel",
  "go.opentelemetry.io/otel/sdk",
  "go.opentelemetry.io/contrib/instrumentation",
  "signoz (containerized)"
]
database_migrations: []
areas: ["observability", "monitoring", "distributed-tracing", "performance"]
---

# RFD 022 - OpenTelemetry Distributed Tracing and Observability

**Status:** 🚧 Draft

## Summary

Integrate OpenTelemetry SDK across all components (Manager, Gateway, Agent) to enable distributed tracing and unified observability with SigNoz backend. Replace existing Prometheus metrics with OTLP (OpenTelemetry Protocol) exporters while maintaining backward compatibility via Prometheus bridge. Enable full request flow visibility from CLI → Manager → Gateway → Agent → BMC with automatic context propagation through HTTP, gRPC, and WebSocket connections.

## Problem

**Current behavior/limitations:**

- **No distributed tracing**: Cannot track requests across service boundaries (Manager → Gateway → Agent)
- **Limited visibility**: Prometheus metrics show individual component health but not request flows
- **No trace correlation**: Impossible to correlate a VNC session creation with its constituent operations (auth → session setup → agent connection → BMC protocol)
- **Missing context propagation**: No way to trace a single user request through all three components
- **Difficult debugging**: When sessions fail, must manually correlate logs from Manager, Gateway, and Agent using timestamps
- **No business context tracking**: Cannot filter traces by customer_id, server_id, datacenter, or session_id

**Why this matters:**

- **Production troubleshooting**: Debugging cross-service issues requires manual log correlation, increasing MTTR (Mean Time To Recovery)
- **Performance optimization**: Cannot identify bottlenecks in multi-hop request paths (e.g., "which component is slowing down VNC session creation?")
- **SLO monitoring**: No ability to measure end-to-end latency for business operations (auth → session creation)
- **Capacity planning**: Missing visibility into request fan-out patterns (one gateway request → multiple agent operations)
- **User experience**: Cannot track individual user journeys through the system

**Use cases affected:**

- Debugging slow VNC/SOL session establishment
- Identifying which component caused an authentication failure
- Tracking BMC operation retries across Agent → BMC connections
- Measuring end-to-end latency for power operations
- Correlating WebSocket streaming performance with backend operations
- Understanding impact of database queries on overall request latency

## Solution

Implement comprehensive distributed tracing using OpenTelemetry SDK with SigNoz as the observability backend. Replace Prometheus metrics SDK with OpenTelemetry metrics while maintaining backward compatibility via OTLP-to-Prometheus bridge.

**Key Design Decisions:**

- **Unified OTLP for traces + metrics**: Single exporter for both telemetry types instead of dual Prometheus + tracing
  - **Rationale**: Simplifies infrastructure, enables trace-metric correlation in SigNoz
  - **Trade-off**: Requires Prometheus bridge for existing Grafana dashboards (temporary)

- **SigNoz as backend (containerized)**: Deploy SigNoz stack in Docker Compose for development
  - **Rationale**: Open-source, self-hosted, excellent UI, ClickHouse-based for performance
  - **Alternative considered**: Jaeger (rejected: no native metrics support, less polished UI)
  - **Alternative considered**: Cloud services (Honeycomb, Grafana Cloud) - rejected for dev simplicity

- **Full component instrumentation**: Manager, Gateway, and Agent all instrumented
  - **Rationale**: Partial instrumentation would leave gaps in trace visibility
  - **Trade-off**: More upfront implementation work, but comprehensive observability

- **Telemetry disabled for local Air development**: `OTEL_SDK_DISABLED=true` for `make local-env-up`
  - **Rationale**: Local development doesn't need observability overhead, faster iteration
  - **Enabled for**: Docker Compose development (`make dev-up`)

- **W3C Trace Context + Baggage propagation**: Standard context propagation across all transports
  - **HTTP headers**: Automatic via `otelhttp` middleware for REST/Connect RPC
  - **gRPC metadata**: Via `otelgrpc` interceptors for Agent ↔ Gateway persistent connections
  - **WebSocket headers**: Custom middleware to extract/inject trace context during upgrade
  - **Baggage**: Propagate business context (customer_id, server_id, session_id, datacenter, agent_id)

**Benefits:**

- **End-to-end request tracking**: See complete flow from CLI auth → Manager → Gateway → Agent → BMC in single trace
- **Performance insights**: Identify bottlenecks by comparing span durations across components
- **Simplified debugging**: Click on error in SigNoz UI, see all related spans and logs in one view
- **Business metrics**: Filter traces by customer, server, datacenter using baggage propagation
- **Single pane of glass**: Traces, metrics, and infrastructure in unified SigNoz UI
- **Production-ready**: OpenTelemetry is CNCF standard with wide industry adoption

**Architecture Overview:**

```
┌─────────────────────────────────────────────────────────────────┐
│                        SigNoz Stack                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  ClickHouse  │  │ Query Service│  │   Frontend   │         │
│  │  (storage)   │  │  (backend)   │  │   (UI:3301)  │         │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘         │
│         │                  │                                     │
│  ┌──────┴──────────────────┴───────────────────────────┐       │
│  │         OpenTelemetry Collector (OTLP)              │       │
│  │         gRPC:4317  HTTP:4318                        │       │
│  └──────────────────────┬──────────────────────────────┘       │
└─────────────────────────┼────────────────────────────────────┬─┘
                          │ OTLP Export                        │
         ┌────────────────┼────────────────┬──────────────────┬┘
         │ (traces+metrics)│                │                  │
    ┌────▼─────┐      ┌───▼──────┐    ┌───▼──────┐      ┌────▼─────┐
    │ Manager  │      │ Gateway  │    │ Gateway  │      │  Agent   │
    │  :8080   │      │  :8081   │    │  :8082   │      │  :8090   │
    └────┬─────┘      └────┬─────┘    └────┬─────┘      └────┬─────┘
         │                 │               │                  │
         │ W3C Trace       │ W3C Trace     │ gRPC             │ IPMI/
         │ Context         │ Context       │ Metadata         │ Redfish
         ▼                 ▼               ▼                  ▼
    ┌─────────┐       ┌──────────┐    ┌──────────┐      ┌──────────┐
    │ SQLite  │       │ Sessions │    │ WebSocket│      │   BMC    │
    │   DB    │       │ (memory) │    │ Streams  │      │ Hardware │
    └─────────┘       └──────────┘    └──────────┘      └──────────┘

Trace Flow Example (VNC Session Creation):
┌──────────────────────────────────────────────────────────────────┐
│ Trace ID: 1a2b3c4d5e6f7g8h9i0j                                   │
├──────────────────────────────────────────────────────────────────┤
│ ┌─ Span: POST /auth (Manager)                  [150ms]          │
│ │  └─ Span: db.query.users (Manager)           [45ms]           │
│ │  └─ Span: jwt.sign (Manager)                 [5ms]            │
│ │                                                                │
│ └─ Span: POST /CreateVNCSession (Gateway)      [850ms]          │
│    ├─ Span: jwt.validate (Gateway)             [10ms]           │
│    ├─ Span: session.create (Gateway)           [15ms]           │
│    └─ Span: agent.rpc.StartVNC (Agent)         [800ms]          │
│       ├─ Span: vnc.connect (Agent)             [750ms]          │
│       │  └─ Span: tcp.dial bmc:5900 (Agent)    [700ms] ⚠️       │
│       └─ Span: websocket.upgrade (Gateway)     [25ms]           │
└──────────────────────────────────────────────────────────────────┘
                   ↑ Bottleneck identified in trace UI
```

### Component Changes

1. **Core** (`core/telemetry/`):
   - New telemetry package with tracer/meter provider initialization
   - Configuration struct for OTLP endpoint, service name, sampling rate
   - W3C Trace Context + Baggage propagators
   - Shared initialization logic for all components
   - Graceful shutdown handling for flushing telemetry

2. **Manager** (`manager/`):
   - Initialize OpenTelemetry on startup in `cmd/manager/main.go`
   - Instrument RPC handlers: `AuthenticateUser`, `ValidateToken`, `CreateServer`, etc.
   - Trace database queries via bun ORM hooks (`db.AddQueryHook`)
   - Convert Prometheus metrics to OTLP meters (keep metric names unchanged)
   - Add baggage: `customer_id` from auth context
   - Maintain `/metrics` endpoint via Prometheus bridge exporter

3. **Gateway** (`gateway/`):
   - Initialize OpenTelemetry on startup in `cmd/gateway/main.go`
   - Instrument RPC handlers: `CreateVNCSession`, `CreateSOLSession`, proxy operations
   - Trace session lifecycle: creation, active streams, cleanup
   - Custom WebSocket middleware for trace context extraction from upgrade headers
   - HTTP middleware via `otelhttp` for automatic span creation
   - gRPC interceptor via `otelgrpc` for Agent ↔ Gateway streams
   - Add baggage: `session_id`, `server_id`, `datacenter`
   - Convert Prometheus metrics to OTLP meters

4. **Agent** (`local-agent/`):
   - Initialize OpenTelemetry on startup in `cmd/agent/main.go`
   - Instrument RPC handlers: BMC operations, discovery, SOL/VNC proxying
   - Trace BMC protocol interactions (IPMI commands, Redfish API calls)
   - Trace VNC/SOL session establishment and data transfer
   - gRPC interceptor for outbound Gateway connection
   - Add baggage: `agent_id`, `bmc_type` (ipmi/redfish)
   - Convert Prometheus metrics to OTLP meters

**Configuration Example:**

```yaml
# Docker Compose environment variables
services:
  manager:
    environment:
      OTEL_SDK_DISABLED: "false"
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://signoz-otel-collector:4317"
      OTEL_SERVICE_NAME: "bmc-manager"
      OTEL_RESOURCE_ATTRIBUTES: "environment=development,datacenter=docker-dev"
      OTEL_TRACES_SAMPLER: "parentbased_traceidratio"
      OTEL_TRACES_SAMPLER_ARG: "1.0"  # 100% sampling in dev

# Local development (Air)
$ export OTEL_SDK_DISABLED=true  # Disable for local-env-up
$ make local-env-up

# Docker Compose development (with SigNoz)
$ make dev-full-up  # Starts core services + SigNoz stack
```

## Implementation Plan

### Phase 1: Infrastructure Setup

- [ ] Create `docker-compose.observability.yml` with SigNoz stack
  - ClickHouse database for trace/metric storage
  - SigNoz query-service backend
  - SigNoz frontend UI (port 3301)
  - OpenTelemetry Collector (OTLP receiver on 4317/4318)
- [ ] Create `docker/configs/otel-collector-config.yaml`
- [ ] Add Makefile targets to `tooling/make/Makefile.dev`:
  - `dev-observability-up` - Start SigNoz stack
  - `dev-observability-down` - Stop SigNoz stack
  - `dev-full-up` - Start core + observability
- [ ] Update `docker-compose.core.yml` with OTEL environment variables
- [ ] Test SigNoz UI accessible at `http://localhost:3301`

### Phase 2: Core Telemetry Package

- [ ] Create `core/telemetry/` package
- [ ] Implement `config.go` - Telemetry configuration struct
- [ ] Implement `tracer.go` - Global tracer provider with OTLP exporter
- [ ] Implement `metrics.go` - Global meter provider with OTLP exporter
- [ ] Implement `propagation.go` - W3C + Baggage propagators
- [ ] Implement `shutdown.go` - Graceful shutdown with flush
- [ ] Add OpenTelemetry dependencies to `core/go.mod`
- [ ] Write unit tests for configuration parsing

### Phase 3: Manager Instrumentation

- [ ] Add OpenTelemetry dependencies to `manager/go.mod`
- [ ] Initialize telemetry in `manager/cmd/manager/main.go`
- [ ] Add HTTP middleware via `otelhttp` to mux
- [ ] Add Connect RPC interceptor for automatic span creation
- [ ] Instrument `AuthenticateUser` - add span + baggage (customer_id)
- [ ] Instrument `ValidateToken` - add span
- [ ] Instrument server CRUD operations - add spans
- [ ] Trace database queries via bun hooks
- [ ] Migrate Prometheus metrics to OTLP meters
- [ ] Add Prometheus bridge exporter for `/metrics` endpoint
- [ ] Test traces visible in SigNoz for auth flow

### Phase 4: Gateway Instrumentation

- [ ] Add OpenTelemetry dependencies to `gateway/go.mod`
- [ ] Initialize telemetry in `gateway/cmd/gateway/main.go`
- [ ] Add HTTP middleware via `otelhttp` to router
- [ ] Add Connect RPC interceptor for automatic span creation
- [ ] Add gRPC interceptor via `otelgrpc` for Agent streams
- [ ] Implement custom WebSocket middleware for trace context extraction
- [ ] Instrument `CreateVNCSession` - add span + baggage (session_id, server_id)
- [ ] Instrument `CreateSOLSession` - add span + baggage
- [ ] Instrument BMC proxy operations - add spans
- [ ] Trace session lifecycle (create → active → close)
- [ ] Migrate Prometheus metrics to OTLP meters
- [ ] Add Prometheus bridge exporter for `/metrics` endpoint
- [ ] Test end-to-end trace: Manager → Gateway

### Phase 5: Agent Instrumentation

- [ ] Add OpenTelemetry dependencies to `local-agent/go.mod`
- [ ] Initialize telemetry in `local-agent/cmd/agent/main.go`
- [ ] Add gRPC interceptor via `otelgrpc` for outbound Gateway connection
- [ ] Instrument BMC discovery cycle - add spans
- [ ] Instrument power operations (IPMI/Redfish) - add spans + baggage (bmc_type)
- [ ] Instrument SOL session handling - add spans
- [ ] Instrument VNC proxying - add spans
- [ ] Trace BMC protocol interactions (IPMI commands, HTTP calls)
- [ ] Migrate Prometheus metrics to OTLP meters
- [ ] Test end-to-end trace: Manager → Gateway → Agent → BMC

### Phase 6: Advanced Features

- [ ] Implement baggage propagation across all components
  - `customer_id` (Manager)
  - `server_id`, `session_id`, `datacenter` (Gateway)
  - `agent_id`, `bmc_type` (Agent)
- [ ] Add span attributes for filtering (status codes, error types)
- [ ] Add span events for key state transitions (session created, BMC connected)
- [ ] Implement structured logging correlation (trace_id in logs via zerolog)
- [ ] Configure trace sampling strategies (100% dev, lower in prod)

### Phase 7: Testing & Validation

- [ ] Verify all unit tests pass with telemetry enabled
- [ ] Run `make test-all` - ensure no regressions
- [ ] Create E2E test: verify trace completeness for VNC session creation
- [ ] Measure performance overhead (<5% latency impact)
- [ ] Test telemetry disabled (`OTEL_SDK_DISABLED=true`) - verify zero overhead
- [ ] Validate `/metrics` endpoint still works (Prometheus bridge)
- [ ] Test local development (`make local-env-up`) works without SigNoz

### Phase 8: Documentation

- [ ] Update `docker/README.md` with SigNoz setup instructions
- [ ] Update `docs/.ai/DEVELOPMENT-AI.md` with observability commands
- [ ] Create developer guide: "How to add custom spans"
- [ ] Create operator guide: "Viewing traces in SigNoz"
- [ ] Add troubleshooting section for common telemetry issues
- [ ] Update main `README.md` with observability section

## Configuration Changes

**Environment Variables (Standard OpenTelemetry):**

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `OTEL_SDK_DISABLED` | Disable telemetry completely | `false` | `true` (local dev) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | - | `http://signoz-otel-collector:4317` |
| `OTEL_SERVICE_NAME` | Service identifier in traces | - | `bmc-manager`, `bmc-gateway`, `bmc-agent` |
| `OTEL_RESOURCE_ATTRIBUTES` | Additional resource attributes | - | `environment=dev,datacenter=us-west` |
| `OTEL_TRACES_SAMPLER` | Trace sampling strategy | `parentbased_always_on` | `parentbased_traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | Sampler argument (e.g., ratio) | - | `1.0` (100%), `0.1` (10%) |

**Docker Compose Files:**

- `docker-compose.observability.yml` - New file for SigNoz stack
- `docker-compose.core.yml` - Updated with OTEL environment variables

**Makefile Targets:**

```bash
# Start core services with observability
make dev-full-up

# Start only SigNoz stack
make dev-observability-up

# Stop SigNoz stack
make dev-observability-down

# Local development (telemetry disabled)
make local-env-up
```

## Testing Strategy

### Unit Tests

- Telemetry configuration parsing (valid/invalid endpoints)
- Span creation with correct attributes
- Baggage propagation between functions
- Metrics registration and collection
- Graceful shutdown and flush

### Integration Tests

- End-to-end trace propagation: Manager → Gateway → Agent
- Context propagation through HTTP headers (W3C Trace Context)
- Context propagation through gRPC metadata
- Context propagation through WebSocket upgrade headers
- Baggage attributes present in all child spans
- Metrics exported to OTLP collector
- Prometheus bridge endpoint serving metrics

### E2E Tests

- Full VNC session creation trace visible in SigNoz
- Full SOL session creation trace visible in SigNoz
- Power operation trace spans BMC interaction
- Error traces include error attributes and events
- Performance overhead <5% for traced requests
- Trace sampling works correctly (configurable via env vars)

### Performance Validation

**Baseline (without telemetry):**
- Measure VNC session creation latency
- Measure auth request latency
- Measure power operation latency

**With telemetry (OTEL_SDK_DISABLED=false):**
- Ensure <5% latency increase
- Monitor CPU/memory overhead (<2% increase)
- Validate no goroutine leaks

**Telemetry disabled (OTEL_SDK_DISABLED=true):**
- Ensure zero overhead (no-op implementation)

## Migration Strategy

### Deployment Steps

1. **Deploy infrastructure** (SigNoz stack via `docker-compose.observability.yml`)
2. **Deploy Manager** with OpenTelemetry SDK (backward compatible)
3. **Deploy Gateway** with OpenTelemetry SDK (backward compatible)
4. **Deploy Agent** with OpenTelemetry SDK (backward compatible)
5. **Verify traces** in SigNoz UI
6. **Monitor performance** for 24-48 hours
7. **Migrate Grafana dashboards** from Prometheus to SigNoz (optional)

### Rollback Plan

1. Set `OTEL_SDK_DISABLED=true` in all component configurations
2. Restart services (telemetry becomes no-op)
3. Verify `/metrics` endpoint still works (Prometheus bridge maintains compatibility)
4. No code rollback needed - telemetry disabled via configuration

### Backward Compatibility

- **Prometheus `/metrics` endpoint**: Maintained via OTLP-to-Prometheus bridge exporter
- **Existing Grafana dashboards**: Continue working without changes
- **Metric names**: Unchanged (same names as RFD 021)
- **No API changes**: Telemetry is transparent to clients

### Breaking Changes

**None** - This feature is additive only. All existing functionality remains unchanged.

## Security Considerations

- **No sensitive data in spans**: Never include passwords, JWT tokens, or credentials in span attributes
- **PII handling**: Customer email addresses should be hashed if included in baggage
- **OTLP endpoint authentication**: Production deployments should use authenticated OTLP endpoints
- **Trace data retention**: Configure SigNoz retention policies (default: 7 days traces, 30 days metrics)
- **Access control**: SigNoz UI should be behind authentication in production

## Future Enhancements

1. **Distributed context for WebSocket messages**:
   - Inject trace context into individual WebSocket frames
   - Enable tracing of long-lived streaming sessions with per-message granularity

2. **Exemplars integration**:
   - Link metrics to traces via OpenTelemetry exemplars
   - Click on metric spike in SigNoz → see example traces

3. **Alerting based on traces**:
   - Define alerts on trace attributes (e.g., "alert when VNC session creation >5s")
   - SLO-based alerting using trace data

4. **Continuous profiling**:
   - Integrate with Pyroscope or SigNoz profiling (when available)
   - Correlate CPU/memory profiles with traces

5. **Log correlation**:
   - Inject trace_id/span_id into structured logs (zerolog)
   - Link from trace span to related log entries in SigNoz

6. **Production deployment guide**:
   - Kubernetes deployment for SigNoz
   - Production sampling strategies
   - Multi-region OTLP collector setup

## Appendix

### OpenTelemetry SDK Dependencies

**Core packages:**
```go
go.opentelemetry.io/otel v1.24.0
go.opentelemetry.io/otel/sdk v1.24.0
go.opentelemetry.io/otel/sdk/metric v1.24.0
```

**Exporters:**
```go
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.24.0
go.opentelemetry.io/otel/exporters/prometheus v0.46.0  // Bridge
```

**Instrumentation libraries:**
```go
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0
```

### SigNoz Stack Components

**Docker Compose services:**

- `signoz-clickhouse` - ClickHouse database for time-series storage
- `signoz-query-service` - Backend API for trace/metric queries
- `signoz-frontend` - React UI (port 3301)
- `signoz-otel-collector` - OpenTelemetry Collector (OTLP receiver)
  - gRPC endpoint: `4317`
  - HTTP endpoint: `4318`

**Ports mapping:**

| Service    | Port | Purpose                       |
|------------|------|-------------------------------|
| SigNoz UI  | 3301 | Web interface                 |
| OTLP gRPC  | 4317 | Trace/metric ingestion (gRPC) |
| OTLP HTTP  | 4318 | Trace/metric ingestion (HTTP) |
| ClickHouse | 9000 | Database (internal)           |

### Trace Context Propagation Examples

**HTTP (W3C Trace Context):**
```http
GET /v1/servers HTTP/1.1
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
tracestate: vendor1=value1,vendor2=value2
baggage: customer_id=cust123,datacenter=us-west
```

**gRPC (Metadata):**
```go
md := metadata.Pairs(
  "traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
  "baggage", "agent_id=agent-01,bmc_type=ipmi",
)
ctx := metadata.NewOutgoingContext(ctx, md)
```

**WebSocket (Upgrade Headers):**
```http
GET /vnc/session-123 HTTP/1.1
Upgrade: websocket
Connection: Upgrade
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
baggage: session_id=session-123,server_id=srv-456
```

### Sample Trace Queries in SigNoz

**Find slow VNC sessions (>2 seconds):**
```
serviceName=bmc-gateway AND name=CreateVNCSession AND duration > 2s
```

**Filter by customer:**
```
baggage.customer_id=customer-123
```

**Find errors in BMC operations:**
```
serviceName=bmc-agent AND status=error AND name=*BMC*
```

**Trace specific session:**
```
baggage.session_id=session-abc123
```

### Reference Documentation

- **OpenTelemetry Go SDK**: https://opentelemetry.io/docs/instrumentation/go/
- **SigNoz Documentation**: https://signoz.io/docs/
- **W3C Trace Context**: https://www.w3.org/TR/trace-context/
- **OTLP Specification**: https://opentelemetry.io/docs/specs/otlp/
- **OpenTelemetry Semantic Conventions**: https://opentelemetry.io/docs/specs/semconv/

---

**Implementation Checklist Status:**

Phase 1: ⬜ Not Started
Phase 2: ⬜ Not Started
Phase 3: ⬜ Not Started
Phase 4: ⬜ Not Started
Phase 5: ⬜ Not Started
Phase 6: ⬜ Not Started
Phase 7: ⬜ Not Started
Phase 8: ⬜ Not Started
