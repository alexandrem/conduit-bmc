---
rfd: "003"
title: "Server Ownership and Authorization"
state: "partially-implemented"
priority: "high"
status: "partially-implemented"
breaking_changes: false
testing_required: true
database_changes: true
api_changes: true
dependencies:
    - "github.com/golang-jwt/jwt/v5"
database_migrations:
    - "alter_server_customer_mappings_add_columns"
    - "create_indexes_for_ownership"
areas: [ "manager", "gateway", "authorization", "database" ]
---

# RFD 003 - Server Ownership and Authorization

**Status:** 🚧 Partially Implemented

## Summary

Implement multi-tenant server ownership and authorization for the BMC management
platform by connecting the existing `server_customer_mappings` database table to
the token generation system. This enables dynamic permission management while
preserving the stateless gateway architecture.

## Problem

The current system has the infrastructure for multi-tenant authorization but
isn't using it, creating a disconnect between the database schema and actual
authorization logic.

### Current Architecture Issues

```
Token Generation Flow (Broken):
CLI → Manager (generates token with HARDCODED permissions) → JWT token → Gateway (validates permissions) → Agent → BMC

Database Flow (Unused):
server_customer_mappings table exists but is never queried for permissions
```

**Problems:**

1. **Hardcoded Permissions**: All tokens get same permissions regardless of
   customer
    - Location: `manager/internal/manager/manager_handlers.go` in
      `GetServerToken` handler
    - Current:
      `permissions := []string{"power:read", "power:write", "console:read", "console:write"}`
    - Impact: Cannot restrict customer access to read-only
    - TODO comments exist acknowledging missing ServerCustomerMapping check

2. **Database Table Exists But Unused**: `ServerCustomerMapping` model exists
   but table not queried
    - Model location: `manager/pkg/models/models.go`
    - Current fields: `ID`, `ServerID`, `CustomerID`, `CreatedAt`, `UpdatedAt`
    - Missing fields: `access_level`, `permissions`, `granted_by`, `expires_at`
    - No database methods to query/grant/revoke access

3. **No Access Management API**: Cannot grant server access to customers
   dynamically
    - No RPC endpoints for access management
    - No CLI commands for access control
    - Requires code changes to grant access

4. **Multi-Tenancy Incomplete**:
    - JWT tokens work correctly ✅
    - Permission checking works ✅
    - But all customers get same permissions ❌

### Current Flow Limitations

```go
// Current: hardcoded permissions in GetServerToken handler
permissions := []string{"power:read", "power:write", "console:read", "console:write"}
// PROBLEM: Same permissions for everyone, regardless of customer access level
// TODO comment exists acknowledging missing ServerCustomerMapping check
```

## Solution

Connect the existing database infrastructure to token generation, enabling
dynamic permission management without architectural changes.

**Key Design Decisions:**

- Query `server_customer_mappings` during token generation instead of hardcoding
  permissions
- Store permissions in JWT token (no gateway changes needed)
- Use access levels (read/write/admin) with optional custom permission overrides
- Fallback to read-only permissions on database lookup failure

**Why This Approach:**

- Leverages existing JWT infrastructure and token validation
- Maintains stateless gateway architecture
- Single-line change in token generation minimizes risk
- Database-first design enables future extensibility (RBAC, delegation)

**Trade-offs:**

- Cannot revoke token mid-session (1-hour expiry window acceptable for BMC
  access)
- Requires database query on every token generation (acceptable overhead)

**Benefits:**

- Dynamic permission management without code changes
- Full audit trail of access grants/revocations
- Time-bound access with optional expiration
- Backward compatible with existing deployments

**Architecture Overview:**

```
Fixed Flow:
CLI → Manager (queries DB for permissions) → JWT token with DB permissions → Gateway (validates) → Agent → BMC

Database Integration:
server_customer_mappings table queried during token generation
Permissions stored in DB, embedded in token
Gateway validates permissions (no changes needed)
```

### Component Changes

1. **Manager**:
    - Update token generation to query database for permissions (one-line
      change)
    - Add RPC endpoints for access management (grant/revoke/list)
    - Implement handlers for new RPC methods with admin authorization

2. **Database**:
    - Add permission columns to `server_customer_mappings` table
    - Add methods: `GetCustomerPermissionsForServer`, `GrantServerAccess`,
      `RevokeServerAccess`, `ListCustomerServers`
    - Create indexes for customer_id and server_id lookups

3. **CLI**:
    - Add `server access` command tree with grant/revoke/list/show subcommands
    - Support access level flags (--level=read/write/admin)
    - Support expiration flags (--expires-in=7d)

4. **Gateway**:
    - No changes required (already validates permissions from token)

## API Changes

### Database Schema Changes

**Update Model** (`manager/pkg/models/models.go`):

```go
type ServerCustomerMapping struct {
ID          string     `json:"id" db:"id"`
ServerID    string     `json:"server_id" db:"server_id"`
CustomerID  string     `json:"customer_id" db:"customer_id"`
AccessLevel string     `json:"access_level" db:"access_level"` // NEW: "read", "write", "admin"
Permissions []string   `json:"permissions" db:"permissions"` // NEW: Custom permission overrides
GrantedBy   string     `json:"granted_by" db:"granted_by"`   // NEW: Audit trail
ExpiresAt   *time.Time `json:"expires_at" db:"expires_at"` // NEW: Optional expiration
CreatedAt   time.Time  `json:"created_at" db:"created_at"`
UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}
```

**Database Migration**:

```sql
-- Add missing columns to existing table
ALTER TABLE server_customer_mappings
    ADD COLUMN access_level TEXT DEFAULT 'read';
ALTER TABLE server_customer_mappings
    ADD COLUMN permissions TEXT; -- JSON array of permissions
ALTER TABLE server_customer_mappings
    ADD COLUMN granted_by TEXT;
ALTER TABLE server_customer_mappings
    ADD COLUMN expires_at DATETIME;

-- Create indexes for performance
CREATE INDEX idx_server_customer_mappings_customer ON server_customer_mappings (customer_id);
CREATE INDEX idx_server_customer_mappings_server ON server_customer_mappings (server_id);
```

**Access Levels:**

- `read` - View power status, read-only operations (default)
- `write` - Power control + console access
- `admin` - Full control including server management

**Permission String Format:**

- `power:read` - View power status
- `power:write` - Power operations (on/off/cycle/reset)
- `console:read` - View console info
- `console:write` - Access console (VNC/SOL)
- `sensors:read` - Read sensor data (future)
- `admin:all` - Administrative access (future)

### Implementation Notes

**Database Methods:**
Add permission management methods to `manager/pkg/database/database.go`:

- `GetCustomerPermissionsForServer` - queries permissions for customer-server
  pair, checks expiration
- `GrantServerAccess` - creates/updates access with upsert logic
- `RevokeServerAccess` - removes access mapping
- `ListCustomerServers` - returns accessible servers (filters expired)

**Token Generation:**
Update `GetServerToken` handler in
`manager/internal/manager/manager_handlers.go` to query database for permissions
instead of hardcoding. Fallback to read-only permissions on lookup failure.

### New Protobuf Messages

Add to `manager/proto/manager/v1/manager.proto`:

```protobuf
service BMCManagerService {
    // Existing methods...

    // Server ownership management
    rpc GrantServerAccess(GrantServerAccessRequest) returns (GrantServerAccessResponse);
    rpc RevokeServerAccess(RevokeServerAccessRequest) returns (RevokeServerAccessResponse);
    rpc ListAccessibleServers(ListAccessibleServersRequest) returns (ListAccessibleServersResponse);
    rpc GetServerAccess(GetServerAccessRequest) returns (GetServerAccessResponse);
}

message GrantServerAccessRequest {
    string server_id = 1;
    string customer_id = 2;
    string access_level = 3;  // "read", "write", "admin"
    repeated string custom_permissions = 4;  // Optional override
    google.protobuf.Timestamp expires_at = 5;  // Optional expiration
}

message GrantServerAccessResponse {
    bool success = 1;
    string message = 2;
}

message RevokeServerAccessRequest {
    string server_id = 1;
    string customer_id = 2;
}

message RevokeServerAccessResponse {
    bool success = 1;
    string message = 2;
}

message ListAccessibleServersRequest {
    string customer_id = 1;  // Implicit from auth token
}

message ListAccessibleServersResponse {
    repeated ServerAccessInfo servers = 1;
}

message GetServerAccessRequest {
    string server_id = 1;
    string customer_id = 2;
}

message GetServerAccessResponse {
    ServerAccessInfo access = 1;
}

message ServerAccessInfo {
    string server_id = 1;
    string customer_id = 2;
    string access_level = 3;
    repeated string permissions = 4;
    google.protobuf.Timestamp granted_at = 5;
    google.protobuf.Timestamp expires_at = 6;
    string granted_by = 7;
}
```

### CLI Commands

**Usage Examples:**

```bash
# Grant read-only access
bmc-cli server access grant server-001 user@company.com --level=read

# Grant write access with expiration
bmc-cli server access grant server-002 user@company.com --level=write --expires-in=7d

# Grant admin access
bmc-cli server access grant server-003 admin@company.com --level=admin

# Revoke access
bmc-cli server access revoke server-001 user@company.com

# List accessible servers
bmc-cli server access list --customer=user@company.com

# Check specific access
bmc-cli server access show server-001 user@company.com
```

## Implementation Plan

### Phase 1: Database Schema

- [ ] Add columns to `server_customer_mappings`: access_level, permissions,
  granted_by, expires_at
- [ ] Create database migration script
- [ ] Add indexes for customer_id and server_id
- [ ] Update `ServerCustomerMapping` model in `manager/pkg/models/models.go`

### Phase 2: Database Methods

- [ ] Implement `GetCustomerPermissionsForServer` method
- [ ] Implement `GrantServerAccess` method with upsert logic
- [ ] Implement `RevokeServerAccess` method
- [ ] Implement `ListCustomerServers` method
- [ ] Add `defaultPermissionsForAccessLevel` helper

### Phase 3: Token Generation Integration

- [ ] Update token generation in `manager_handlers.go` to query database
- [ ] Add error handling with read-only fallback
- [ ] Add logging for permission lookup failures

### Phase 4: RPC Endpoints

- [ ] Define protobuf messages in `manager/proto/manager/v1/manager.proto`
- [ ] Generate protobuf code (`make proto`)
- [ ] Implement `GrantServerAccess` RPC handler
- [ ] Implement `RevokeServerAccess` RPC handler
- [ ] Implement `ListAccessibleServers` RPC handler
- [ ] Implement `GetServerAccess` RPC handler
- [ ] Add admin permission checks to all handlers

### Phase 5: CLI Commands

- [ ] Create `cli/cmd/server_access.go` with command tree
- [ ] Implement `server access grant` command
- [ ] Implement `server access revoke` command
- [ ] Implement `server access list` command
- [ ] Implement `server access show` command

### Phase 6: Testing & Documentation

- [ ] Add unit tests for database methods
- [ ] Add unit tests for RPC handlers
- [ ] Add integration tests for token generation flow
- [ ] Add E2E tests for full access control workflow
- [ ] Update user documentation

## Migration Strategy

### For New Installations

1. Database migration runs automatically on first start
2. Create admin customer and grant access to all servers
3. Use CLI to grant access to additional customers

### For Existing Installations

1. Run database migration to add columns
2. Create default mappings for existing customers:
   ```sql
   INSERT INTO server_customer_mappings (id, server_id, customer_id, access_level, granted_by, created_at, updated_at)
   SELECT uuid_generate_v4(), s.id, s.customer_id, 'admin', 'migration', NOW(), NOW()
   FROM servers s;
   ```
3. Verify token generation uses database
4. Test access grant/revoke functionality

### Rollback Plan

If issues arise, revert the one-line change in token generation to use hardcoded
permissions:

```go
// Temporary rollback
permissions := []string{"power:read", "power:write", "console:read", "console:write"}
```

Database changes are additive (new columns) and don't affect existing
functionality.

## Testing Strategy

### Unit Tests

```go
func TestGetCustomerPermissionsForServer(t *testing.T) {
// Test permission lookup
// Test expiration handling
// Test access level defaults
}

func TestGrantServerAccess(t *testing.T) {
// Test creating new access
// Test updating existing access
// Test custom permissions
}

func TestRevokeServerAccess(t *testing.T) {
// Test access removal
// Test non-existent mapping
}
```

### Integration Tests

```go
func TestTokenGenerationWithDatabasePermissions(t *testing.T) {
// Grant read-only access
// Generate token
// Verify token has read-only permissions

// Upgrade to write access
// Generate new token
// Verify new permissions
}

func TestPermissionEnforcement(t *testing.T) {
// Grant read-only access
// Attempt power operation (should fail)
// Grant write access
// Attempt power operation (should succeed)
}
```

### E2E Tests

1. Admin grants server access to customer
2. Customer generates server token
3. Token has correct permissions from database
4. Gateway allows/denies operations based on permissions
5. Admin revokes access
6. New token generation fails (or gets empty permissions)

## Security Considerations

### Token Security

- ✅ ServerContext encrypted with AES-256-GCM
- ✅ 1-hour token expiry limits revocation window
- ✅ Tokens tamper-proof (signature validation)
- ⚠️ Cannot revoke token mid-session (acceptable for 1-hour window)

### Access Control

- ✅ Only admins can grant/revoke access (enforced in RPC handlers)
- ✅ Customer cannot grant access to own servers (future: delegation)
- ✅ Access expiration enforced at token generation time
- ✅ Default to read-only on permission lookup failure

### Audit Trail

Add logging for security events:

- Permission grants/revokes (who, what, when)
- Token generation with permission details
- Access denied events in gateway
- Failed permission lookups

## Future Enhancements

### Phase 1: Enhanced Features (Future)

1. **Delegation Support**
    - Customers can delegate access to other customers
    - Time-bounded delegations
    - Audit trail for delegated access

2. **Permission Templates**
    - Predefined permission sets (operator, viewer, admin)
    - Organization-wide defaults
    - Role-based permission mapping

3. **Advanced Expiration**
    - Scheduled access (valid only during business hours)
    - One-time access tokens
    - Access request approval workflows

### Phase 2: Pluggable Backends (Future)

Only implement if needed for specific customer requirements:

- RBAC backend for complex role hierarchies
- External authorization service integration
- OIDC claim-based permissions
- Integration with existing access control systems
