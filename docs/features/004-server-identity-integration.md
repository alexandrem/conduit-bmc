---
rfd: "004"
title: "Server Naming and Identity Management"
state: "abandoned"
priority: "medium"
status: "abandoned"
complexity: "medium"
breaking_changes: true
testing_required: true
database_changes: true
api_changes: true
dependencies: [ "RFD-003" ]
database_migrations:
    - "add_server_display_name_and_metadata"
    - "create_server_aliases_table"
areas: [ "manager", "gateway", "cli", "database" ]
---

# RFD 004 - Server Naming and Identity Management

- **Status:** ❌ Abandoned
  - **Reason:** This manual naming approach requires too many API calls and admin effort. RFD 018 replaces it with automatic identity extraction from BMC discovery metadata.
- **Dependencies:**
    - **RFD 003 (Server Ownership and Authorization)**: This RFD builds on the
      ownership model established in RFD 003. Server names and metadata are
      visible
      to all customers with access to a server, but only admins can modify
      naming
      and metadata.

## Summary

Enable administrators to assign meaningful, human-readable names to discovered
BMC endpoints, replacing synthetic IDs with business-friendly identifiers while
maintaining the discovery-based architecture.

## Problem

The current system generates synthetic server IDs from discovered BMC endpoints,
which are meaningless to users and difficult to work with in operational
contexts.

### Current Limitations

**Synthetic IDs are cryptic:**

```bash
$ bmc-cli server list
SERVER ID                                   BMC TYPE       STATUS
bmc-dc-local-01-http-//localhost-9001       ipmi           active
bmc-dc-docker-01-e2e-virtualbmc-01-623      ipmi           active
bmc-dc-east-1-http-//redfish-01-8000        redfish        active
```

**Real-world scenarios that fail:**

1. **Operations**: "Reboot the database primary server"
    - Current: Must map to `bmc-dc-prod-01-http-//10-50-1-100-623`
    - Desired: `bmc-cli server power cycle db-primary-01`

2. **Incident Response**: "Check console on web server rack A-10"
    - Current: Must find which synthetic ID corresponds to rack A-10
    - Desired: `bmc-cli server console web-a10-01`

3. **Inventory Management**: "List all production database servers"
    - Current: No way to filter by server role or environment
    - Desired: `bmc-cli server list --role=database --env=production`

4. **Multi-datacenter Operations**: "Show all servers in east datacenter"
    - Current: Parse synthetic IDs to extract datacenter
    - Desired: Built-in metadata filtering

## Solution

Add a lightweight naming and metadata layer on top of the existing discovery
system, allowing administrators to assign meaningful names and business context
to discovered servers.

**Key Design Decisions:**

- **Discovery First**: Synthetic IDs remain primary keys for stability
- **Aliases for Humans**: Display names and aliases for user-facing operations
- **Backward Compatible**: Synthetic IDs continue to work
- **Metadata Enrichment**: Store operational context (rack, role, environment)
- **CLI-friendly**: Support name/alias in all CLI commands

**Architecture:**

```
BMC Discovery → Synthetic ID (immutable) → Admin assigns name/metadata → User operations use name

Discovery Layer:        Identity Layer:          Presentation Layer:
Agent finds BMC    →    bmc-dc-01-http-//... →  db-primary-01
192.168.1.100:623       (internal ID)            (display name)
```

**Benefits:**

- **Operational Clarity**: Meaningful names in logs, alerts, CLI output
- **Business Context**: Metadata for filtering and organization
- **Backward Compatible**: Existing synthetic IDs still work
- **No External Dependencies**: Self-contained naming system
- **Flexible Metadata**: Custom tags and attributes

## Database Schema Changes

### Add Display Name to Servers Table

```sql
-- Add human-readable fields to existing servers table
ALTER TABLE servers
    ADD COLUMN display_name TEXT UNIQUE; -- Human-friendly name (optional)
ALTER TABLE servers
    ADD COLUMN description TEXT; -- Server description
ALTER TABLE servers
    ADD COLUMN tags TEXT; -- JSON array of tags
ALTER TABLE servers
    ADD COLUMN custom_metadata TEXT;
-- JSON object for custom fields

-- Create index for display name lookups
CREATE INDEX idx_servers_display_name ON servers (display_name);
```

### Server Aliases Table (Optional Multi-Name Support)

```sql
-- Allow multiple aliases per server (optional feature)
CREATE TABLE server_aliases
(
    id         TEXT PRIMARY KEY,
    server_id  TEXT        NOT NULL REFERENCES servers (id),
    alias      TEXT UNIQUE NOT NULL, -- e.g., "db-01", "prod-db-primary"
    created_by TEXT,                 -- Admin who created alias
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (server_id, alias)
);

CREATE INDEX idx_server_aliases_alias ON server_aliases (alias);
CREATE INDEX idx_server_aliases_server_id ON server_aliases (server_id);
```

### Server Model Update

```go
type Server struct {
    ID              string                 `json:"id" db:"id"`
    DisplayName     *string                `json:"display_name,omitempty"
     db:"display_name"`
    Description     *string                `json:"description,omitempty"
     db:"description"`
    Tags            []string               `json:"tags,omitempty"`
    CustomMetadata  map[string]string      `json:"custom_metadata,omitempty"`
    // ... existing fields ...
}

type ServerAlias struct {
    ID        string    `json:"id" db:"id"`
    ServerID  string    `json:"server_id" db:"server_id"`
    Alias     string    `json:"alias" db:"alias"`
    CreatedBy string    `json:"created_by" db:"created_by"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

## API Changes

### New Protobuf Messages

Add to `manager/proto/manager/v1/manager.proto`:

```protobuf
service BMCManagerService {
    // Existing methods...

    // Server naming and metadata management
    rpc SetServerName(SetServerNameRequest) returns (SetServerNameResponse);
    rpc UpdateServerMetadata(UpdateServerMetadataRequest) returns (UpdateServerMetadataResponse);
    rpc AddServerAlias(AddServerAliasRequest) returns (AddServerAliasResponse);
    rpc RemoveServerAlias(RemoveServerAliasRequest) returns (RemoveServerAliasResponse);
    rpc ResolveServerID(ResolveServerIDRequest) returns (ResolveServerIDResponse);
}

// Set human-readable display name for a server
message SetServerNameRequest {
    string server_id = 1;      // Synthetic ID or existing display name
    string display_name = 2;   // New display name (must be unique)
    string description = 3;    // Optional description
}

message SetServerNameResponse {
    bool success = 1;
    string message = 2;
}

// Update server metadata (tags, custom fields)
message UpdateServerMetadataRequest {
    string server_id = 1;                   // Synthetic ID or display name
    repeated string tags = 2;               // Tags (e.g., "production", "database")
    map<string, string> custom_metadata = 3; // Custom fields (e.g., rack, role, environment)
}

message UpdateServerMetadataResponse {
    bool success = 1;
    string message = 2;
}

// Add an alias for a server (multiple names)
message AddServerAliasRequest {
    string server_id = 1;  // Synthetic ID or display name
    string alias = 2;      // New alias (must be unique)
}

message AddServerAliasResponse {
    bool success = 1;
    string message = 2;
}

// Remove a server alias
message RemoveServerAliasRequest {
    string alias = 1;  // Alias to remove
}

message RemoveServerAliasResponse {
    bool success = 1;
    string message = 2;
}

// Resolve any server identifier to canonical ID
message ResolveServerIDRequest {
    string identifier = 1;  // Synthetic ID, display name, or alias
}

message ResolveServerIDResponse {
    string server_id = 1;         // Canonical synthetic ID
    string display_name = 2;      // Display name (if set)
    repeated string aliases = 3;  // All aliases
}
```

### Updated Server Message

```protobuf
message Server {
    string id = 1;                               // Synthetic ID (immutable)
    string display_name = 2;                     // Human-readable name (optional)
    string description = 3;                      // Server description
    repeated string tags = 4;                    // Tags for filtering
    map<string, string> custom_metadata = 5;     // Custom metadata fields
    // ... existing fields ...
}
```

## CLI Changes

### Server Naming Commands

```bash
# Set display name for a server
bmc-cli server rename bmc-dc-local-01-http-//localhost-9001 db-primary-01 \
    --description "PostgreSQL primary database server"

# Update server metadata
bmc-cli server metadata bmc-dc-local-01-http-//localhost-9001 \
    --tags production,database,postgresql \
    --rack A-10 \
    --role database \
    --environment production

# Add alias (multiple names for same server)
bmc-cli server alias add db-primary-01 prod-db-01
bmc-cli server alias add db-primary-01 postgres-primary

# Remove alias
bmc-cli server alias remove prod-db-01

# Show server details (by any identifier)
bmc-cli server show db-primary-01
bmc-cli server show prod-db-01
bmc-cli server show bmc-dc-local-01-http-//localhost-9001  # Still works
```

### Updated List Command with Filtering

```bash
# List all servers (now shows display names)
bmc-cli server list
# SERVER NAME      ID (SYNTHETIC)                          BMC TYPE       STATUS
# db-primary-01    bmc-dc-local-01-http-//localhost-9001  BMC_TYPE_IPMI  active
# web-frontend-01  bmc-dc-local-02-http-//localhost-9002  BMC_TYPE_IPMI  active
# cache-redis-01   bmc-dc-east-1-http-//10-50-1-5-623     BMC_TYPE_IPMI  active

# Filter by tags
bmc-cli server list --tags production,database

# Filter by metadata
bmc-cli server list --rack A-10
bmc-cli server list --environment production --role database

# Show synthetic IDs only (backward compatibility)
bmc-cli server list --show-ids
```

### Operations Commands with Name Support

```bash
# Power operations (use display name or alias)
bmc-cli server power on db-primary-01
bmc-cli server power off prod-db-01
bmc-cli server power status postgres-primary

# Console access
bmc-cli server console db-primary-01
bmc-cli server vnc web-frontend-01

# All commands accept any identifier (synthetic ID, display name, or alias)
```

## Implementation Plan

### Phase 1: Database Schema

- [ ] Add `display_name`, `description`, `tags`, `custom_metadata` columns to
  `servers` table
- [ ] Create `server_aliases` table
- [ ] Add database indexes
- [ ] Update `Server` model in `manager/pkg/models/models.go`
- [ ] Create `ServerAlias` model

### Phase 2: Database Methods

- [ ] Implement `SetServerDisplayName` method
- [ ] Implement `UpdateServerMetadata` method
- [ ] Implement `AddServerAlias` method
- [ ] Implement `RemoveServerAlias` method
- [ ] Implement `ResolveServerID` method (resolve any identifier to canonical
  ID)
- [ ] Update `GetServerByID` to support display names and aliases
- [ ] Update `ListServers` to support metadata filtering

### Phase 3: Manager RPC Handlers

- [ ] Define protobuf messages in `manager/proto/manager/v1/manager.proto`
- [ ] Generate protobuf code (`make proto`)
- [ ] Implement `SetServerName` RPC handler
- [ ] Implement `UpdateServerMetadata` RPC handler
- [ ] Implement `AddServerAlias` RPC handler
- [ ] Implement `RemoveServerAlias` RPC handler
- [ ] Implement `ResolveServerID` RPC handler
- [ ] Add validation (unique names, valid identifiers)

### Phase 4: CLI Commands

- [ ] Create `server rename` command
- [ ] Create `server metadata` command
- [ ] Create `server alias add` command
- [ ] Create `server alias remove` command
- [ ] Update `server list` to display names and support filtering
- [ ] Update `server show` to display full metadata
- [ ] Update all operation commands to accept any identifier type
- [ ] Add identifier resolution to CLI utility functions

### Phase 5: Backward Compatibility

- [ ] Ensure synthetic IDs continue to work in all commands
- [ ] Add migration for existing servers (optional display names)
- [ ] Update documentation with naming guidelines
- [ ] Add CLI help text for all new commands

### Phase 6: Testing & Documentation

- [ ] Add unit tests for database methods
- [ ] Add unit tests for RPC handlers
- [ ] Add integration tests for name resolution
- [ ] Add E2E tests for naming workflow
- [ ] Update user documentation with naming examples
- [ ] Create naming conventions guide

## Migration Strategy

### For New Installations

1. Database migration runs automatically
2. Servers discovered without display names (can be set later)
3. Admins can assign names as servers are commissioned

### For Existing Installations

1. Run database migration to add columns
2. Existing servers keep synthetic IDs
3. Admins gradually assign display names
4. Both synthetic IDs and display names work during transition

**Migration Steps:**

```bash
# After upgrade, set names for existing servers
bmc-cli server rename bmc-dc-local-01-http-//localhost-9001 db-primary-01
bmc-cli server rename bmc-dc-local-02-http-//localhost-9002 web-frontend-01

# Add metadata
bmc-cli server metadata db-primary-01 \
    --tags production,database \
    --rack A-10 \
    --environment production
```

### Rollback Plan

- New columns are nullable and don't affect existing operations
- Synthetic IDs remain primary keys
- Can disable display name feature without data loss
- No breaking changes to existing API calls

## Name Resolution Logic

### Resolution Priority

When a user provides a server identifier, resolve in this order:

1. **Exact synthetic ID match** (highest priority for API stability)
2. **Display name match** (primary human-readable identifier)
3. **Alias match** (secondary human-readable identifiers)

**Example:**

```go
func ResolveServerID(identifier string) (string, error) {
// 1. Check if it's a synthetic ID
if server := db.GetServerByID(identifier); server != nil {
return server.ID, nil
}

// 2. Check if it's a display name
if server := db.GetServerByDisplayName(identifier); server != nil {
return server.ID, nil
}

// 3. Check if it's an alias
if alias := db.GetServerAliasByAlias(identifier); alias != nil {
return alias.ServerID, nil
}

return "", ErrServerNotFound
}
```

## Server Metadata Schema

### Standard Tags

Recommended tags for filtering:

- **Environment**: `production`, `staging`, `development`, `test`
- **Role**: `database`, `web`, `cache`, `storage`, `compute`
- **Tier**: `frontend`, `backend`, `middleware`
- **Service**: Specific service names (`postgres`, `redis`, `nginx`)

### Custom Metadata Fields

Recommended custom fields:

```yaml
rack: "A-10"              # Physical rack location
blade: "5"                # Blade number (if blade server)
role: "database"          # Server role
environment: "production" # Environment
region: "us-east-1"       # Geographic region
cost_center: "eng-db"     # Business unit
owner: "database-team"    # Owning team
criticality: "high"       # Business criticality
```

**Example:**

```bash
bmc-cli server metadata db-primary-01 \
    --tags production,database,postgresql \
    --rack A-10 \
    --role database \
    --environment production \
    --region us-east-1 \
    --owner database-team \
    --criticality high
```

## CLI Output Examples

### Before (Current)

```bash
$ bmc-cli server list
SERVER ID                                    BMC TYPE       STATUS      DATACENTER
bmc-dc-local-01-http-//localhost-9001       BMC_TYPE_IPMI  configured  dc-local-01
bmc-dc-docker-01-e2e-virtualbmc-01-623      BMC_TYPE_IPMI  configured  dc-docker-01
bmc-dc-east-1-http-//redfish-01-8000        BMC_REDFISH    configured  dc-east-1

$ bmc-cli server power status bmc-dc-local-01-http-//localhost-9001
Power Status: ON
```

### After (Proposed)

```bash
$ bmc-cli server list
SERVER NAME      DESCRIPTION                    BMC TYPE       STATUS      TAGS
db-primary-01    PostgreSQL primary database    BMC_TYPE_IPMI  active      production,database
web-frontend-01  Nginx web server               BMC_TYPE_IPMI  active      production,web
cache-redis-01   Redis cache server             BMC_REDFISH    active      production,cache

$ bmc-cli server list --verbose
SERVER NAME      ID (SYNTHETIC)                          DESCRIPTION                    BMC TYPE       DATACENTER   RACK   ENVIRONMENT
db-primary-01    bmc-dc-local-01-http-//localhost-9001  PostgreSQL primary database    BMC_TYPE_IPMI  dc-local-01  A-10   production
web-frontend-01  bmc-dc-local-02-http-//localhost-9002  Nginx web server               BMC_TYPE_IPMI  dc-local-02  A-11   production

$ bmc-cli server show db-primary-01
Server: db-primary-01
Synthetic ID: bmc-dc-local-01-http-//localhost-9001
Description: PostgreSQL primary database
Status: active
BMC Type: IPMI
BMC Endpoint: http://localhost:9001
Datacenter: dc-local-01

Tags: production, database, postgresql
Metadata:
  rack: A-10
  role: database
  environment: production
  region: us-east-1
  owner: database-team

Aliases:
  - prod-db-01
  - postgres-primary

$ bmc-cli server power status db-primary-01
Server: db-primary-01 (bmc-dc-local-01-http-//localhost-9001)
Power Status: ON
```

## Testing Strategy

### Unit Tests

```go
func TestResolveServerID(t *testing.T) {
// Test synthetic ID resolution
// Test display name resolution
// Test alias resolution
// Test resolution priority order
}

func TestSetServerDisplayName(t *testing.T) {
// Test setting unique display name
// Test duplicate name rejection
// Test name validation
}

func TestServerMetadataFiltering(t *testing.T) {
// Test tag filtering
// Test metadata field filtering
// Test combined filters
}
```

### Integration Tests

```go
func TestServerNamingWorkflow(t *testing.T) {
// Create server with synthetic ID
// Set display name
// Add aliases
// Resolve via different identifiers
// Verify all resolutions point to same server
}

func TestBackwardCompatibility(t *testing.T) {
// Ensure synthetic IDs still work
// Test operations with both ID types
// Verify no breaking changes
}
```

### E2E Tests

1. Discover server (gets synthetic ID)
2. Assign display name via CLI
3. Add metadata and tags
4. List servers and verify display name shown
5. Perform operations using display name
6. Verify synthetic ID still works

## Security Considerations

### Naming Security

- Only authenticated admins can set server names
- Display names must be unique (prevent confusion)
- Validate display names (no special characters that could cause injection)
- Log all naming changes for audit trail

### Access Control Integration

Works with RFD 003 (Server Ownership):

- Names visible to all customers with server access
- Only admins can modify names
- Customer access control still based on synthetic IDs
- Names don't affect authorization

## Benefits

### For Operators

- **Faster Operations**: Use meaningful names instead of memorizing synthetic
  IDs
- **Fewer Errors**: Clear names reduce wrong-server operations
- **Better Organization**: Filter servers by role, environment, location

### For Incident Response

- **Quick Identification**: "Console to db-primary-01" vs looking up synthetic
  ID
- **Context in Logs**: Meaningful names in audit logs and alerts
- **Team Communication**: "Reboot web-frontend-01" clear to entire team

### For Automation

- **Script Clarity**: `power on db-primary-01` self-documenting
- **Metadata Queries**: Bulk operations on tagged servers
- **Integration**: Easier integration with monitoring and alerting

## Future Enhancements

### Phase 1: Enhanced Filtering (Future)

- Complex query language for metadata
- Saved filter presets
- Bulk operations on filtered sets

### Phase 2: Auto-naming from Discovery (Future)

- Attempt to extract hostname from BMC
- Auto-suggest names based on network info
- Integration with DNS for name hints

### Phase 3: Naming Templates (Future)

- Organization-wide naming conventions
- Template-based name validation
- Auto-generate names from templates

### Phase 4: Name History (Future)

- Track name changes over time
- Audit trail for renaming operations
- Revert to previous names

---

## Implementation Notes

**File References (No Line Numbers):**

- Synthetic ID generation: `core/identity/server.go`
- Database schema: `manager/pkg/database/database.go`
- Server models: `manager/pkg/models/models.go`
- Manager API: `manager/proto/manager/v1/manager.proto`
- CLI commands: `cli/cmd/server.go`

**Design Principles:**

- Discovery-first: Synthetic IDs remain authoritative
- Backward compatible: All existing functionality preserved
- Human-friendly: Operations use display names
- Flexible metadata: Support diverse operational contexts
- No external dependencies: Self-contained naming system
