---
rfd: "019"
title: "BMC Discovery Metadata Display"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: true
api_changes: true
dependencies: [ ]
database_migrations:
    - "add_discovery_metadata_to_servers"
areas: [ "local-agent", "manager", "gateway", "cli" ]
---

# RFD 019 - BMC Discovery Metadata Display

## Summary

Enhance the BMC discovery process to capture and persist detailed metadata about
discovered BMC servers, including vendor information, protocol configuration,
endpoint types, TLS settings, and discovery method. This metadata will be
displayed through the `server show` CLI command to aid troubleshooting and
provide visibility into BMC configuration details.

## Problem

The local agent currently discovers and configures various BMC properties during
server registration, but this valuable information is not persisted or exposed
to users:

- **VNC Configuration**: Endpoint password handling, TLS usage, transport type (
  native vs WebSocket)
- **Serial Console Configuration**: Vendor-specific endpoint paths (Redfish
  vendors have different SerialConsole implementations)
- **Protocol Details**: Whether IPMI fallback is being used, why certain
  protocols were chosen
- **Discovery Method**: How the server was found (static config, network scan,
  manual registration)
- **Vendor Information**: BMC manufacturer and model information from Redfish
  API
- **Connectivity Details**: Port numbers, protocol versions, authentication
  methods

This information is critical for:

- **Troubleshooting connectivity issues**: Understanding which endpoints and
  protocols are in use
- **Debugging authentication failures**: Knowing what credentials and auth
  methods are configured
- **Vendor compatibility**: Identifying BMC vendor/model for known issues or
  limitations
- **Configuration validation**: Verifying TLS settings, timeouts, and other
  parameters
- **Operational visibility**: Understanding the discovery source and
  configuration rationale

Currently, users running `bmc-cli server show` see basic server information but
not the detailed discovery metadata that would help diagnose issues.

## Solution

Extend the server data model to include a comprehensive `DiscoveryMetadata`
structure that captures:

1. **Discovery Information**
    - Discovery method (static config, network scan, API registration)
    - Discovery timestamp
    - Discovery source (agent ID that found the server)
    - Configuration source (file path, API endpoint)

2. **BMC Vendor Information**
    - Manufacturer name (e.g., "Dell", "HPE", "Supermicro", "Lenovo")
    - Model identifier
    - Firmware version
    - BMC version/revision

3. **Protocol Configuration**
    - Active protocol (IPMI, Redfish)
    - Fallback configuration (e.g., "IPMI fallback enabled for console")
    - Protocol version (IPMI 2.0, Redfish 1.6.0)
    - API paths (Redfish SerialConsole path, Systems path)

4. **Endpoint Configuration**
    - Control endpoint details (host, port, scheme)
    - SOL endpoint details (type, port, special configuration)
    - VNC endpoint details (transport type, display number)

5. **Security Configuration**
    - TLS enabled/disabled status
    - TLS verification settings (InsecureSkipVerify)
    - Authentication method (basic auth, session-based, VNC password)
    - Cipher suites (IPMI)

6. **Network Information**
    - IP address
    - MAC address (if available)
    - Network segment/VLAN
    - Reachability status from agent

7. **Capability Discovery**
    - Supported features discovered via API
    - Unsupported/unavailable features
    - Feature discovery errors or warnings

### Component Changes

1. **Core Types** (`core/types`)
    - Add `DiscoveryMetadata` struct with all metadata fields
    - Add `DiscoveryMethod` enum (static, network_scan, api_registration,
      manual)
    - Add `ProtocolFallback` struct for fallback configuration tracking

2. **Local Agent Discovery** (`local-agent/internal/discovery`)
    - Capture discovery metadata during BMC discovery
    - Extract vendor information from Redfish API responses
    - Record configuration decisions (why IPMI fallback was chosen, etc.)
    - Track discovery method and source

3. **Manager Proto & Database** (`manager/proto/manager/v1`)
    - Add `DiscoveryMetadata` protobuf message
    - Extend `Server` message to include discovery_metadata field
    - Add database migration to create discovery_metadata JSONB column
    - Store and retrieve metadata with server records

4. **Manager Service** (`manager/internal`)
    - Persist discovery metadata when servers are registered
    - Return metadata in `GetServer` and `ListServers` responses
    - Support filtering/searching by metadata fields (future enhancement)

5. **CLI Display** (`cli/cmd/server`)
    - Enhance `server show` command to display discovery metadata
    - Group metadata into logical sections (Discovery, Protocol, Security,
      Network)
    - Add `--format json` flag for machine-readable output
    - Add `--metadata-only` flag to show only discovery metadata

### Example Output

```bash
$ bmc-cli server show bmc-dc-local-dev-http-//localhost-9001

Server Information
==================
ID:          bmc-dc-local-dev-http-//localhost-9001
Customer:    customer-1
Datacenter:  dc-local-dev
Status:      active
Features:    power, console, vnc, sensors

BMC Discovery Metadata
======================

Discovery Information:
  Method:         Static Configuration
  Source:         local-agent-01
  Discovered:     2025-10-09 10:23:45 UTC
  Config File:    /etc/bmc-agent/config.yaml

Vendor Information:
  Manufacturer:   Contoso
  Model:          Redfish Mockup Server
  Firmware:       1.0.0
  BMC Type:       Redfish

Protocol Configuration:
  Primary:        Redfish 1.6.0
  Fallback:       IPMI SOL (console only)
  Console Type:   redfish_serial
  Console Path:   /redfish/v1/Systems/1/SerialConsole
  VNC Type:       websocket

Endpoint Details:
  Control:        https://localhost:8000 (Redfish)
  Console:        https://localhost:8000/redfish/v1/Systems/1/SerialConsole
  VNC:            ws://novnc:6080/websockify

Security Configuration:
  TLS Enabled:    Yes (control endpoint)
  TLS Verify:     No (InsecureSkipVerify)
  Auth Method:    Basic Authentication
  VNC Auth:       Password (8 chars)

Network Information:
  IP Address:     localhost / 127.0.0.1
  Control Port:   8000 (HTTPS)
  IPMI Port:      623 (fallback)
  VNC Port:       6080 (WebSocket)
  Reachable:      Yes

Capabilities:
  Supported:      Power control, Serial console, VNC/KVM, Sensors, Boot control
  Limitations:    IPMI fallback required for console (vendor: Contoso)
```

## Implementation Plan

### Phase 1: Data Model & Core Types

- [ ] Define `DiscoveryMetadata` struct in `core/types`
- [ ] Add `DiscoveryMethod` and related enums
- [ ] Add protobuf definitions for `DiscoveryMetadata`
- [ ] Generate protobuf code

### Phase 2: Database Schema

- [ ] Create migration to add `discovery_metadata` JSONB column to servers table
- [ ] Add indexes for common metadata queries (vendor, discovery_method)
- [ ] Update ORM models to include discovery metadata

### Phase 3: Discovery Enhancement

- [ ] Update `local-agent/internal/discovery` to capture metadata during
  discovery
- [ ] Extract vendor information from Redfish API responses
- [ ] Record protocol decisions and fallback configuration
- [ ] Capture network and security information
- [ ] Pass metadata through registration flow

### Phase 4: Manager Integration

- [ ] Update `RegisterServer` RPC to accept discovery metadata
- [ ] Update `GetServer` RPC to return discovery metadata
- [ ] Update `ListServers` RPC to include metadata (optional field)
- [ ] Implement database persistence for metadata

### Phase 5: CLI Enhancement

- [ ] Update `server show` command to display discovery metadata
- [ ] Implement structured output formatting
- [ ] Add `--format json` flag for machine-readable output
- [ ] Add `--metadata-only` flag to show only discovery metadata

### Phase 6: Testing & Documentation

- [ ] Add unit tests for metadata capture logic
- [ ] Add integration tests for end-to-end metadata flow
- [ ] Update CLI help documentation
- [ ] Add troubleshooting guide using metadata

## API Changes

### New Protobuf Messages

```protobuf
// DiscoveryMetadata contains detailed information about how a BMC was discovered and configured
message DiscoveryMetadata {
    // Discovery information
    DiscoveryMethod discovery_method = 1;
    google.protobuf.Timestamp discovered_at = 2;
    string discovery_source = 3;        // Agent ID that discovered this BMC
    string config_source = 4;           // Config file path or API endpoint

    // Vendor information
    VendorInfo vendor = 5;

    // Protocol configuration
    ProtocolConfig protocol = 6;

    // Endpoint details
    EndpointDetails endpoints = 7;

    // Security configuration
    SecurityConfig security = 8;

    // Network information
    NetworkInfo network = 9;

    // Capability discovery
    CapabilityInfo capabilities = 10;

    // Additional metadata
    map<string, string> additional_info = 11;
}

enum DiscoveryMethod {
    DISCOVERY_METHOD_UNSPECIFIED = 0;
    DISCOVERY_METHOD_STATIC_CONFIG = 1;  // From agent config file
    DISCOVERY_METHOD_NETWORK_SCAN = 2;   // Auto-discovered via network scan
    DISCOVERY_METHOD_API_REGISTRATION = 3; // Registered via API
    DISCOVERY_METHOD_MANUAL = 4;         // Manually added by admin
}

message VendorInfo {
    string manufacturer = 1;
    string model = 2;
    string firmware_version = 3;
    string bmc_version = 4;
}

message ProtocolConfig {
    string primary_protocol = 1;        // "ipmi" or "redfish"
    string primary_version = 2;         // "2.0", "1.6.0"
    string fallback_protocol = 3;       // "ipmi" or empty
    string fallback_reason = 4;         // Why fallback is needed
    string console_type = 5;            // "ipmi", "redfish_serial"
    string console_path = 6;            // Redfish path to SerialConsole
    string vnc_transport = 7;           // "native", "websocket"
}

message EndpointDetails {
    string control_endpoint = 1;
    string control_scheme = 2;          // "https", "http", "ipmi"
    int32 control_port = 3;
    string console_endpoint = 4;
    string vnc_endpoint = 5;
    int32 vnc_display = 6;              // VNC display number
}

message SecurityConfig {
    bool tls_enabled = 1;
    bool tls_verify = 2;
    string auth_method = 3;             // "basic", "session", "digest"
    string vnc_auth_type = 4;           // "password", "vencrypt", "none"
    int32 vnc_password_length = 5;
    string ipmi_cipher_suite = 6;
}

message NetworkInfo {
    string ip_address = 1;
    string mac_address = 2;
    string network_segment = 3;
    string vlan_id = 4;
    bool reachable = 5;
    int32 latency_ms = 6;               // Ping latency from agent
}

message CapabilityInfo {
    repeated string supported_features = 1;
    repeated string unsupported_features = 2;
    repeated string discovery_errors = 3;
    repeated string discovery_warnings = 4;
}
```

### Modified Messages

```protobuf
// Server message updated to include discovery metadata
message Server {
    // ... existing fields ...
    DiscoveryMetadata discovery_metadata = 12;  // New field
}
```

### Modified RPCs

No new RPCs are added. Existing RPCs are enhanced:

- `RegisterServer`: Accepts optional `DiscoveryMetadata` in request
- `GetServer`: Returns `DiscoveryMetadata` in response
- `ListServers`: Optionally includes `DiscoveryMetadata` (controlled by request
  flag)

### CLI Changes

```bash
# Show full server information including discovery metadata
bmc-cli server show <server-id>

# Show only discovery metadata
bmc-cli server show <server-id> --metadata-only

# Output as JSON for programmatic use
bmc-cli server show <server-id> --format json

# Show metadata for all servers
bmc-cli server list --include-metadata

# Filter by discovery method
bmc-cli server list --discovery-method static_config

# Filter by vendor
bmc-cli server list --vendor supermicro
```

## Migration Strategy

**No backward compatibility required** - we will recreate the database from
scratch during deployment.

1. **Database Schema**:
    - Create `discovery_metadata` JSONB column as part of initial servers table
      schema
    - Add indexes for common metadata queries (vendor, discovery_method)
    - All servers will have metadata from initial discovery

## Testing Strategy

### Unit Tests

1. **Core Types** (`core/types/discovery_metadata_test.go`):
    - Test metadata struct serialization/deserialization
    - Test enum validation
    - Test default values

2. **Discovery Logic** (`local-agent/internal/discovery/metadata_test.go`):
    - Test metadata extraction from Redfish responses
    - Test vendor information parsing
    - Test protocol decision recording
    - Test network information collection

3. **Database** (`manager/pkg/database/discovery_metadata_test.go`):
    - Test JSONB storage and retrieval
    - Test metadata indexing
    - Test partial metadata updates

4. **CLI Formatting** (`cli/cmd/server/show_test.go`):
    - Test metadata display formatting
    - Test JSON output
    - Test handling of missing metadata

### Integration Tests

1. **Redfish Discovery** (`tests/integration/discovery_redfish_test.go`):
    - Test metadata capture from real Redfish simulator
    - Verify vendor information extraction
    - Verify protocol configuration recording

2. **IPMI Discovery** (`tests/integration/discovery_ipmi_test.go`):
    - Test metadata capture from VirtualBMC
    - Verify IPMI-specific metadata
    - Verify fallback scenarios

3. **End-to-End Flow** (`tests/integration/metadata_e2e_test.go`):
    - Register server with metadata via agent
    - Retrieve server via Manager API
    - Display metadata via CLI
    - Verify metadata consistency

### E2E Tests

1. **Static Configuration** (`tests/e2e/static_config_metadata_test.go`):
    - Start system with static BMC configuration
    - Verify metadata is populated correctly
    - Verify CLI displays metadata

2. **Network Discovery** (`tests/e2e/network_discovery_metadata_test.go`):
    - Run network scan discovery
    - Verify discovered servers have metadata
    - Verify metadata includes discovery method

3. **CLI Display** (`tests/e2e/cli_metadata_display_test.go`):
    - Test `server show` with metadata
    - Test `--metadata-only` flag
    - Test `--format json` output

## Security Considerations

- **Credential Exposure**: Discovery metadata MUST NOT include actual
  credentials
    - Store only credential metadata (username, auth method, password length)
    - Never log or display actual passwords

- **Network Information**: IP addresses and network topology are sensitive
    - Require authentication to view metadata
    - Log metadata access for audit purposes

- **Vendor Information**: BMC firmware versions can reveal vulnerabilities
    - Restrict metadata access to server owners
    - Consider vulnerability scanning based on firmware version

- **Configuration Details**: TLS and security settings should be protected
    - Audit log when insecure configurations are detected
    - Alert on TLS verification disabled

## Future Enhancements

1. **Metadata Search & Filtering**:
    - Query servers by vendor, firmware version, or discovery method
    - Alert on servers with outdated firmware
    - Group servers by network segment for bulk operations

2. **Metadata History**:
    - Track metadata changes over time
    - Show when configuration changed and why
    - Alert on unexpected changes (security)

3. **Automated Diagnostics**:
    - Use metadata to suggest troubleshooting steps
    - Detect common misconfigurations automatically
    - Provide vendor-specific guidance

4. **Metadata Validation**:
    - Validate metadata against known BMC models
    - Warn about unusual configurations
    - Suggest optimal settings based on vendor

5. **Export & Reporting**:
    - Export metadata as CSV for asset management
    - Generate reports on BMC inventory
    - Track firmware version compliance

6. **Metadata Updates**:
    - Allow updating metadata without re-discovery
    - Support manual metadata correction
    - Sync metadata from external sources (CMDB, asset management)

## Troubleshooting Use Cases

With discovery metadata, users can troubleshoot common issues:

1. **VNC Connection Fails**:
    - Check VNC transport type (native vs websocket)
    - Verify VNC endpoint and port
    - Check VNC authentication type

2. **Console Authentication Errors**:
    - Verify console type (IPMI vs Redfish)
    - Check if fallback is being used
    - See why fallback was chosen (vendor limitation)

3. **TLS Errors**:
    - Check if TLS is enabled
    - Verify InsecureSkipVerify setting
    - Review security configuration

4. **Vendor-Specific Issues**:
    - Identify BMC manufacturer and model
    - Check firmware version for known issues
    - Reference vendor-specific documentation

5. **Performance Issues**:
    - Check network latency from agent
    - Verify reachability status
    - Review discovery errors/warnings
