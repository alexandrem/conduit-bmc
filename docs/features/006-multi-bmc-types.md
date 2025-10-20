---
rfd: "006"
title: "Multi-Protocol BMC Representation"
state: "approved"
testing_required: true
database_changes: true
api_changes: true
areas: [ "manager", "gateway", "local-agent", "tests" ]
---

    # RFD 006 - Multi-Protocol BMC Representation

**Status:** ✅ Approved

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

Extend the server data model to support multiple BMC protocol endpoints.

**Note on Breaking Changes:** Since this project is still experimental and not yet in production,
we will introduce breaking changes to simplify the implementation. No backward compatibility
will be maintained for the old single-protocol format.

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

**New Structure** (breaking change):

```go
type Server struct {
    ID               string
    ControlEndpoints []*BMCControlEndpoint // Multiple endpoints (required)
    PrimaryProtocol  BMCType // Preferred protocol for operations
    // ...
}

type BMCControlEndpoint struct {
    Endpoint     string
    Type         BMCType
    Username     string
    Password     string
    Capabilities []string
    Priority     int // Lower = higher priority (0 = highest)
}
```

**Breaking Change:** The old `ControlEndpoint` field (singular) is completely removed.

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

**New Format** (breaking change):

```yaml
static:
    hosts:
        -   id: "server-42"
            customer_id: "acme-corp"
            control_endpoints: # Now required, plural only
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

**Breaking Change:** The old `control_endpoint` field (singular) is no longer supported.
All configurations must use `control_endpoints` (plural) array format.

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

### Protobuf Message Updates (Breaking Changes)

**File:** `proto/manager/v1/manager.proto`

All field numbers have been compacted (no gaps) and reordered for coherence:

```protobuf
message Server {
    string id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    repeated BMCControlEndpoint control_endpoints = 4; // Multiple protocol support (required)
    BMCType primary_protocol = 5;                      // Preferred protocol for operations
    SOLEndpoint sol_endpoint = 6;
    VNCEndpoint vnc_endpoint = 7;
    repeated string features = 8;
    string status = 9;
    google.protobuf.Timestamp created_at = 10;
    google.protobuf.Timestamp updated_at = 11;
    map<string, string> metadata = 12;
    common.v1.DiscoveryMetadata discovery_metadata = 13;
}

message BMCControlEndpoint {
    string endpoint = 1;
    BMCType type = 2;
    string username = 3;
    string password = 4;
    TLSConfig tls = 5;
    repeated string capabilities = 6;
    int32 priority = 7;  // Lower value = higher priority (0 = highest)
}

message RegisterServerRequest {
    string server_id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    string regional_gateway_id = 4;
    repeated string features = 5;
    repeated BMCControlEndpoint bmc_protocols = 6; // Multiple protocol support (required)
    BMCType primary_protocol = 7;                  // Preferred protocol for operations
}

message GetServerLocationResponse {
    string regional_gateway_id = 1;
    string regional_gateway_endpoint = 2;
    string datacenter_id = 3;
    repeated string features = 4;
    repeated BMCControlEndpoint bmc_protocols = 5; // Multiple protocol support (required)
    BMCType primary_protocol = 6;                  // Preferred protocol for operations
}

message ServerLocation {
    string server_id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    string regional_gateway_id = 4;
    repeated string features = 5;
    google.protobuf.Timestamp created_at = 6;
    google.protobuf.Timestamp updated_at = 7;
    repeated BMCControlEndpoint bmc_protocols = 8; // Multiple protocol support (required)
    BMCType primary_protocol = 9;                  // Preferred protocol for operations
}

message SystemStatusServerEntry {
    string server_id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    string regional_gateway_id = 4;
    repeated string features = 5;
    google.protobuf.Timestamp created_at = 6;
    google.protobuf.Timestamp updated_at = 7;
    repeated BMCControlEndpoint bmc_protocols = 8; // Multiple protocol support (required)
    BMCType primary_protocol = 9;                  // Preferred protocol for operations
}
```

**Breaking Changes Summary:**
- Removed `control_endpoint` (singular) from `Server` message
- Removed `bmc_type` and `bmc_endpoint` from `RegisterServerRequest`
- Removed `bmc_type` and `bmc_endpoint` from `SystemStatusServerEntry`
- Removed `bmc_type` from `GetServerLocationResponse` and `ServerLocation`
- All messages now use `control_endpoints`/`bmc_protocols` (plural) and `primary_protocol`
- **All field numbers compacted with no gaps** - old field numbers reused since project is experimental

### Database Schema Changes (Breaking Changes)

**Migration:** `migrate_to_multi_protocol_breaking_v006`

**Breaking Change Notice:** Since the project is experimental, this migration will:
1. Drop and recreate the affected tables with the new schema
2. Existing server data will be lost and must be re-registered
3. No backfill from old schema - clean slate approach

```sql
-- Drop old tables (breaking change - data loss)
DROP TABLE IF EXISTS server_bmc_protocols;

-- Create new table for multi-protocol support
CREATE TABLE IF NOT EXISTS server_bmc_protocols
(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id VARCHAR(255) NOT NULL,
    protocol_type VARCHAR(16) NOT NULL, -- 'ipmi' or 'redfish'
    endpoint VARCHAR(255) NOT NULL,
    port INTEGER,
    username VARCHAR(255),
    password VARCHAR(255),
    priority INTEGER DEFAULT 0,
    capabilities TEXT, -- JSON array
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

CREATE INDEX idx_server_protocol ON server_bmc_protocols (server_id, protocol_type);

-- Update servers table to remove old single-protocol columns
ALTER TABLE servers DROP COLUMN IF EXISTS bmc_type;
ALTER TABLE servers DROP COLUMN IF EXISTS bmc_endpoint;
ALTER TABLE servers ADD COLUMN primary_protocol VARCHAR(16);

-- No backfill - all servers must be re-registered with new format
```

**Note:** In SQLite, `ALTER TABLE DROP COLUMN` requires SQLite 3.35.0+.
For older versions, the migration will need to recreate the table.

**Model Changes:** `manager/pkg/models/models.go` (Breaking Changes)

```go
type Server struct {
    ID                string                     `json:"id"`
    CustomerID        string                     `json:"customer_id"`
    DatacenterID      string                     `json:"datacenter_id"`

    // REMOVED: ControlEndpoint (breaking change)

    // Multiple protocol support (required)
    ControlEndpoints  []*BMCControlEndpoint      `json:"control_endpoints"`
    PrimaryProtocol   BMCType                    `json:"primary_protocol"`

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
    Priority     int        `json:"priority"`  // Lower = higher priority (0 = highest)
}
```

**Breaking Change:** Removed `ControlEndpoint` field entirely. All code must use `ControlEndpoints` array.

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

**Agent Configuration:** `local-agent/config/agent.yaml` (Breaking Change)

New required field: `control_endpoints` (plural array format only)

```yaml
static:
    hosts:
        -   id: "server-42"
            customer_id: "acme-corp"

            # Multiple protocols supported (required array format)
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

**Breaking Change:** The singular `control_endpoint` field is no longer supported.
All configuration files must be updated to use the `control_endpoints` array format,
even for single-protocol servers (which would have a single-element array).

## Implementation Plan

**Note:** Since this is an experimental project, we're implementing breaking changes
without backward compatibility to simplify the codebase.

### Phase 1: Database Schema (Breaking Changes)

- [ ] Create `server_bmc_protocols` table migration (drop existing tables)
- [ ] Remove old single-protocol columns from servers table
- [ ] Update Manager models to use only `ControlEndpoints []` (remove singular field)
- [ ] Document breaking changes and migration requirements

### Phase 2: Manager API (Breaking Changes)

- [ ] Update protobuf to use only `repeated BMCControlEndpoint` (remove singular fields)
- [ ] Remove deprecated fields from `RegisterServerRequest`
- [ ] Update `SystemStatusServerEntry` to use multi-protocol format
- [ ] Update server token generation to include available protocols
- [ ] Remove all backward compatibility code

### Phase 3: Agent Support (Breaking Changes)

- [ ] Update agent config parser to require `control_endpoints` array (remove singular support)
- [ ] Add validation to reject old config format with clear error message
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
- [ ] Update deployment docs with breaking changes notice
- [ ] Document configuration migration steps (old format → new format)
- [ ] Add clear error messages for old config format attempts

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

## Breaking Changes Summary

This RFD introduces breaking changes since the project is experimental and not in production:

1. **Protobuf Changes:**
   - Removed `control_endpoint` field from `Server` message
   - Removed `bmc_type` and `bmc_endpoint` from `RegisterServerRequest`
   - Removed `bmc_type` and `bmc_endpoint` from `SystemStatusServerEntry`
   - All messages now require `control_endpoints` array format

2. **Configuration Changes:**
   - Agent configs must use `control_endpoints` (plural) - singular format no longer supported
   - Existing config files will fail with clear error message requiring update

3. **Database Changes:**
   - Old single-protocol columns removed from servers table
   - Existing server data must be re-registered (no automatic migration)
   - New `server_bmc_protocols` table with multi-protocol support

4. **Model Changes:**
   - Go models remove `ControlEndpoint` singular field entirely
   - All code must use `ControlEndpoints` array

**Migration Required:** All existing deployments must:
- Update agent configuration files to new format
- Re-register all servers with new multi-protocol format
- Update any client code relying on old field names

## Future Enhancements

- **Automatic Protocol Detection**: Agent auto-discovers both IPMI and Redfish
- **Health-based Selection**: Track latency/reliability per protocol, prefer
  healthier one
- **Protocol Metrics**: Track which protocols are used for which operations
