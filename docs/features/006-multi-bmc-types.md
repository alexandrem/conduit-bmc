---
rfd: "006"
title: "Multi-Protocol BMC Representation"
state: "draft"
testing_required: true
database_changes: true
api_changes: true
areas: [ "manager", "gateway", "local-agent", "tests" ]
---

# RFD 006 - Multi-Protocol BMC Representation

**Status:** 🚧 Draft

## Summary

Allow a single logical server to advertise multiple BMC protocols (e.g., both
IPMI
and Redfish) instead of requiring duplicate server registrations. This enables
unified
management of hardware that exposes multiple management interfaces.

## Problem

### Current Workaround: Duplicate Server Entries

The current system requires registering the same physical server multiple times
when it
exposes multiple BMC protocols:

```
Current (Duplicated):
  - Server: "bmc-dc-prod-ipmi-server-42-623"    (IPMI endpoint)
  - Server: "bmc-dc-prod-redfish-server-42-8000" (Redfish endpoint)

Desired (Unified):
  - Server: "bmc-dc-prod-server-42"
    - IPMI: 10.0.1.42:623
    - Redfish: https://10.0.1.42:8000
```

### Specific Limitations

- **Data Model**: `models.Server` has single `ControlEndpoint` with one `Type`
  field
- **Duplicate Metadata**: Server name, location, credentials duplicated across
  entries
- **Confusing UX**: Users see two "servers" when there's only one physical
  machine
- **Token Overhead**: Each protocol requires separate server tokens
- **Ownership Complexity**: RBAC rules must be duplicated for both server
  entries
- **Agent Config**: Static config cannot express a host with multiple protocols

## Solution

Extend the server data model to support multiple BMC protocol endpoints while
maintaining
backward compatibility with existing single-protocol servers.

### Data Model Changes

**Current Structure** (`manager/pkg/models/models.go`):

```go
type Server struct {
ID              string
ControlEndpoint *BMCControlEndpoint // Single endpoint
// ...
}

type BMCControlEndpoint struct {
Endpoint     string
Type         BMCType // Single type: IPMI or Redfish
Username     string
Password     string
Capabilities []string
}
```

**Proposed Structure**:

```go
type Server struct {
ID               string
ControlEndpoints []*BMCControlEndpoint // Multiple endpoints
PrimaryProtocol  BMCType // Preferred protocol for operations
// ...
}

type BMCControlEndpoint struct {
Endpoint     string
Type         BMCType
Username     string
Password     string
Capabilities []string
Priority     int // Lower = higher priority
}
```

### Agent Configuration

Allow multiple protocols in agent config:

**Current** (`local-agent/config/agent.yaml`):

```yaml
static:
    hosts:
        -   id: "server-42"
            customer_id: "acme-corp"
            control_endpoint:
                endpoint: "10.0.1.42:623"
                type: "ipmi"
                username: "admin"
                password: "secret"
            features: [ "power", "console" ]
```

**Proposed**:

```yaml
static:
    hosts:
        -   id: "server-42"
            customer_id: "acme-corp"
            control_endpoints: # Changed to plural
                -   endpoint: "10.0.1.42:623"
                    type: "ipmi"
                    username: "admin"
                    password: "ipmi_password"
                    priority: 0  # Try IPMI first
                -   endpoint: "https://10.0.1.42:8000"
                    type: "redfish"
                    username: "admin"
                    password: "redfish_password"
                    priority: 1  # Fallback to Redfish
            features: [ "power", "console" ]
```

### Protocol Selection Strategy

**Automatic Protocol Selection** (users don't choose):

Local-agent automatically selects the best protocol for each operation based on
operation type and protocol capabilities:

**Selection Rules:**

- **Power Operations** (power on/off, reset): Prefer IPMI for speed and
  reliability
- **SOL/Console**: Use highest priority available protocol
- **Information Queries** (server info, sensors): Prefer Redfish for richer
  metadata (no fallback)
- **Default**: Use primary protocol (highest priority)

**Selective Fallback Strategy:**

Fallback only makes sense for **critical operations** where success is more
important than data quality:

- **Power/Reset/SOL**: Fallback enabled - if IPMI fails, retry with Redfish
- **Info/Sensors**: No fallback - fail hard if Redfish unavailable (users expect
  rich data)
- **VNC**: No fallback - endpoint is protocol-specific

**Rationale:** Info commands should return rich Redfish data or fail clearly,
not silently degrade to sparse IPMI output.

**Implementation:** See Appendix for reference implementation details.

**User Experience:**

```bash
# Users just specify the operation - protocol selection is completely transparent
bmc-cli server power on server-42       # Uses IPMI, falls back to Redfish if needed
bmc-cli server info server-42           # Uses Redfish only, fails if unavailable
bmc-cli server console server-42        # Uses primary protocol, falls back if needed

# Protocol selection and fallback attempts are logged on the local-agent for debugging
# Users never need to think about which protocol is being used
```

## API Changes

### Protobuf Message Updates

**File:** `proto/manager/v1/manager.proto`

```protobuf
message Server {
    string id = 1;
    string customer_id = 2;
    string datacenter_id = 3;

    // Deprecated: Use control_endpoints instead
    BMCControlEndpoint control_endpoint = 4 [deprecated = true];

    SOLEndpoint sol_endpoint = 5;
    VNCEndpoint vnc_endpoint = 6;
    repeated string features = 7;
    string status = 8;
    google.protobuf.Timestamp created_at = 9;
    google.protobuf.Timestamp updated_at = 10;
    map<string, string> metadata = 11;
    common.v1.DiscoveryMetadata discovery_metadata = 12;

    // New: Multiple protocol support
    repeated BMCControlEndpoint control_endpoints = 13;
    BMCType primary_protocol = 14;
}

message BMCControlEndpoint {
    string endpoint = 1;
    BMCType type = 2;
    string username = 3;
    string password = 4;
    TLSConfig tls = 5;
    repeated string capabilities = 6;
    int32 priority = 7;  // NEW: Lower value = higher priority
}

message RegisterServerRequest {
    string server_id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    string regional_gateway_id = 4;

    // Deprecated: Use bmc_protocols instead
    BMCType bmc_type = 5 [deprecated = true];
    string bmc_endpoint = 7 [deprecated = true];

    repeated string features = 6;

    // New: Multiple protocol support
    repeated BMCControlEndpoint bmc_protocols = 8;
    BMCType primary_protocol = 9;
}
```

### Database Schema Changes

**Migration:** `add_server_bmc_protocols_table`

```sql
-- Create new table for multi-protocol support
CREATE TABLE IF NOT EXISTS server_bmc_protocols
(
    id
    INTEGER
    PRIMARY
    KEY
    AUTOINCREMENT,
    server_id
    VARCHAR
(
    255
) NOT NULL,
    protocol_type VARCHAR
(
    16
) NOT NULL, -- 'ipmi' or 'redfish'
    endpoint VARCHAR
(
    255
) NOT NULL,
    port INTEGER,
    username VARCHAR
(
    255
),
    password VARCHAR
(
    255
),
    priority INTEGER DEFAULT 0,
    capabilities TEXT, -- JSON array
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY
(
    server_id
) REFERENCES servers
(
    id
) ON DELETE CASCADE
    );

CREATE INDEX idx_server_protocol ON server_bmc_protocols (server_id, protocol_type);

-- Backfill existing servers from current schema
-- Current schema has: bmc_type, bmc_endpoint, username columns
INSERT INTO server_bmc_protocols (server_id, protocol_type, endpoint, username,
                                  priority)
SELECT id,
       bmc_type,     -- Use dedicated column
       bmc_endpoint, -- Use dedicated column
       username,     -- Use dedicated column
       0             -- Default priority
FROM servers
WHERE bmc_type IS NOT NULL
  AND bmc_type != '';
```

**Model Changes:** `manager/pkg/models/models.go`

```go
type Server struct {
ID                string                     `json:"id"`
CustomerID        string                     `json:"customer_id"`
DatacenterID      string                     `json:"datacenter_id"`

// Deprecated: Use ControlEndpoints instead
ControlEndpoint   *BMCControlEndpoint        `json:"control_endpoint,omitempty"`

// New: Multiple protocol support
ControlEndpoints  []*BMCControlEndpoint      `json:"control_endpoints,omitempty"`
PrimaryProtocol   BMCType                    `json:"primary_protocol,omitempty"`

SOLEndpoint       *SOLEndpoint               `json:"sol_endpoint"`
VNCEndpoint       *VNCEndpoint               `json:"vnc_endpoint"`
Features          []string                   `json:"features"`
Status            string                     `json:"status"`
Metadata          map[string]string          `json:"metadata"`
DiscoveryMetadata *types.DiscoveryMetadata   `json:"discovery_metadata"`
CreatedAt         time.Time                  `json:"created_at"`
UpdatedAt         time.Time                  `json:"updated_at"`
}

type BMCControlEndpoint struct {
Endpoint     string     `json:"endpoint"`
Type         BMCType    `json:"type"`
Username     string     `json:"username"`
Password     string     `json:"password"`
TLS          *TLSConfig `json:"tls"`
Capabilities []string   `json:"capabilities"`
Priority     int        `json:"priority"`  // NEW: Lower = higher priority
}
```

### CLI Command Examples

**Server List** - Shows protocol indicators:

```bash
$ bmc-cli server list

ID               Customer        Protocols        Status
server-42        acme-corp       IPMI+Redfish     active
server-43        acme-corp       Redfish          active
server-44        acme-corp       IPMI             active
```

**Server Show** - Displays all available protocols:

```bash
$ bmc-cli server show server-42

Server ID:  server-42
Customer:   acme-corp
Datacenter: dc-prod
Status:     active

Available Protocols:
  - IPMI (10.0.1.42:623) - Primary
  - Redfish (https://10.0.1.42:8000)

Current Capabilities: power, console, vnc, sensors
Metadata:
  location: rack-1
  model: Dell PowerEdge R750
```

**Server Info** - Shows BMC information (protocol selection is transparent):

```bash
$ bmc-cli server info server-42

Server ID:  server-42

BMC Information:
  Manager ID:    BMC
  Manufacturer:  Dell
  Model:         PowerEdge R750
  Firmware:      2.85.85.85
  Serial:        ABC123XYZ
  Health:        OK
  Power State:   On
```

### Configuration File Changes

**Agent Configuration:** `local-agent/config/agent.yaml`

New field: `control_endpoints` (replaces singular `control_endpoint`)

```yaml
static:
    hosts:
        -   id: "server-42"
            customer_id: "acme-corp"

            # New: Multiple protocols supported
            control_endpoints:
                -   endpoint: "10.0.1.42:623"
                    type: "ipmi"
                    username: "admin"
                    password: "ipmi_password"
                    priority: 0  # Try IPMI first
                -   endpoint: "https://10.0.1.42:8000"
                    type: "redfish"
                    username: "admin"
                    password: "redfish_password"
                    priority: 1  # Fallback to Redfish

            features: [ "power", "console", "vnc" ]
```

**Backward Compatibility:** Singular `control_endpoint` will continue to work
and be automatically converted to a single-element `control_endpoints` array.

## Implementation Plan

### Phase 1: Database Schema

- [ ] Create `server_bmc_protocols` table migration
- [ ] Add migration to backfill existing servers into new table
- [ ] Update Manager models to include `ControlEndpoints []`
- [ ] Maintain backward compatibility with single `ControlEndpoint`

### Phase 2: Manager API

- [ ] Update protobuf to support `repeated BMCProtocol` in server messages
- [ ] Extend `RegisterServerRequest` to accept protocol arrays
- [ ] Update server token generation to include available protocols
- [ ] Keep legacy fields populated for backward compatibility

### Phase 3: Agent Support

- [ ] Update agent config parser to support `protocols` array
- [ ] Modify discovery to detect multiple protocols per host
- [ ] Update registration logic to send multiple protocols
- [ ] Add agent config validation for protocol conflicts

### Phase 4: Local-Agent Protocol Selection

- [ ] Create `protocol_selector.go` with automatic selection logic
- [ ] Implement operation-specific protocol preferences (IPMI for power, Redfish
  for info)
- [ ] Add automatic fallback mechanism if primary protocol fails
- [ ] Add logging for protocol selection and fallback events

### Phase 5: CLI Updates

- [ ] Update `server show` to display all available protocols
- [ ] Add protocol indicator in `server list` output (e.g., "IPMI+Redfish")
- [ ] **NO CLI flags for protocol selection** - completely transparent to users
- [ ] Update help documentation to explain automatic protocol selection

### Phase 6: Testing & Documentation

- [ ] Unit tests for multi-protocol data model
- [ ] Integration tests with dual-protocol servers
- [ ] E2E test with IPMI+Redfish server
- [ ] Update deployment docs
- [ ] Migration guide for existing deployments

## Benefits

### For Operators

- **Simplified Management**: One server entry instead of duplicates
- **Clear Inventory**: Accurate server count in dashboards
- **Easier RBAC**: Single set of permissions per physical server
- **Protocol Fallback**: Automatic failover to alternate protocol if primary
  unavailable

### For Users

- **Less Confusion**: `server list` shows actual physical hardware
- **Zero Protocol Knowledge Required**: System automatically picks best protocol
- **Automatic Optimization**: IPMI for fast power ops, Redfish for rich data
- **Automatic Fallback**: Operations succeed even if one protocol is down
- **Better UX**: Protocol completely transparent except in `server info` output

### For Development

- **Cleaner Tests**: E2E tests with realistic multi-protocol configurations
- **Better Metrics**: Accurate server counts and protocol usage statistics

## Protocol Selection Philosophy

**Core Principle:** Users interact with servers, not protocols.

### Why Hide Protocol Complexity?

1. **Simplicity**: Users shouldn't need to know BMC protocol details
2. **Reliability**: System can retry with alternate protocol automatically
3. **Optimization**: Different operations work better with different protocols
4. **Future-proofing**: New protocols (e.g., SSH-based) can be added
   transparently

### Protocol Visibility

**User-Facing Commands:** Protocol selection is completely transparent. CLI
commands never show which protocol was used for an operation.

**Server-Side Logging:** Protocol selection and fallback attempts are logged by
the local-agent for debugging and observability:

```
# Example local-agent logs - Power operation with fallback
INFO  Selecting protocol server_id=server-42 operation=power_on selected=IPMI reason="preferred for power ops" fallback_enabled=true
WARN  Protocol failed, attempting fallback server_id=server-42 failed_protocol=IPMI next_protocol=Redfish error="connection timeout"
INFO  Operation succeeded server_id=server-42 protocol=Redfish operation=power_on

# Example local-agent logs - Info operation without fallback
INFO  Selecting protocol server_id=server-42 operation=get_info selected=Redfish reason="rich metadata required" fallback_enabled=false
ERROR Operation failed server_id=server-42 protocol=Redfish operation=get_info error="connection refused" fallback="disabled for info operations"
```

**Server Show Command:** The `server show` command displays available protocols
as server metadata, but operations never expose which protocol was used:

```bash
$ bmc-cli server show server-42

Available Protocols:
  - IPMI (10.0.1.42:623) - Primary
  - Redfish (https://10.0.1.42:8000)
```

This informs operators what's configured without cluttering operational command
output.

## Future Enhancements

- **Automatic Protocol Detection**: Agent auto-discovers both IPMI and Redfish
- **Health-based Selection**: Track latency/reliability per protocol, prefer
  healthier one
- **Protocol Metrics**: Track which protocols are used for which operations
