---
rfd: "023"
title: "Admin Management Dashboard"
state: "implemented"
breaking_changes: true
testing_required: true
database_changes: true
api_changes: true
dependencies: [ ]
database_migrations: [ "create_customers_table", "add_admin_indexes" ]
areas: [ "manager", "auth", "webui", "proto" ]
---

# RFD 023 - Admin Management Dashboard

**Status:** 🎉 Implemented

## Summary

Add a web-based admin dashboard hosted on the Manager service that provides
super-admins with a comprehensive overview of all managed BMCs across all
customers, gateway health monitoring, system-wide metrics, and quick access to
VNC/SOL consoles. The dashboard follows the same embedded web UI pattern as the
Gateway console, using Go templates and session-based authentication.

## Problem

Currently, system administrators have limited visibility into the multi-tenant
BMC management infrastructure:

- **No centralized view**: Admins must manually query APIs or databases to see
  all managed BMCs across customers
- **No health monitoring**: Gateway status and regional distribution require
  manual inspection or log analysis
- **No quick console access**: Accessing VNC/SOL for any BMC requires CLI
  authentication flow, even for administrative troubleshooting
- **No customer overview**: Understanding which customers manage which servers
  requires database queries
- **No system metrics**: Total BMC counts, online/offline ratios, and session
  statistics are not readily available

**Why this matters:**

- Operations teams need quick visibility into system health for incident
  response
- Admins need to troubleshoot BMC connectivity issues without customer
  involvement
- Capacity planning requires understanding BMC distribution across gateways and
  datacenters
- Customer support requires quick access to server details and console access

**Use cases affected:**

1. **Incident Response**: Admin receives alert about gateway failure, needs to
   see which BMCs are affected
2. **Customer Support**: Support needs to verify BMC status and access console
   to debug customer issue
3. **Capacity Planning**: Operations needs to understand BMC distribution across
   regions
4. **System Monitoring**: NOC team needs real-time view of system health and
   active sessions

## Solution

Create a web-based admin dashboard hosted on the Manager service (accessible at
`/admin`) that provides:

**Key Design Decisions:**

1. **Host on Manager, not Gateway**: Manager is the central coordination point
   with full visibility across all gateways and customers. Gateway instances are
   regional and have partial views.

2. **Reuse Gateway web UI patterns**: Use Go `embed` for templates, HTTP routes,
   session cookies, and Tailwind CSS. This keeps architecture consistent and
   leverages proven patterns.

3. **Super-admin scope with JWT auth**: Extend existing JWT authentication with
   `is_admin` claim rather than creating separate auth system. Admins are
   authenticated users with elevated privileges.

4. **Separate admin protobuf package**: Create `proto/manager/v1/admin.proto`
   with dedicated `AdminService` for all admin operations. This enables:
   - Clean separation of admin vs. user APIs
   - Direct Connect RPC calls from web dashboard (no HTTP JSON wrapper needed)
   - Future reuse for CLI `bmc-cli admin` commands
   - Consistent API surface across web UI and CLI

5. **Detailed default view**: Show all key information in main table (ID,
   Customer, Datacenter, Gateway, Endpoint, Status) rather than compact view,
   since admin use cases involve troubleshooting where all context is needed.

6. **Direct integration with Gateway consoles**: Admin clicks VNC/SOL → Manager
   generates server token → calls Gateway CreateSession → opens Gateway web
   console in new tab. Reuses existing console infrastructure.

**Benefits:**

- **Unified operations view**: Single pane of glass for all BMC infrastructure
- **Faster incident response**: Immediate visibility into affected systems and
  health status
- **Simplified troubleshooting**: Direct console access without customer
  credentials
- **Better capacity planning**: Clear view of BMC distribution and resource
  utilization
- **Consistent architecture**: Reuses proven Gateway web UI patterns and
  components

**Architecture Overview:**

```
Browser ──(HTTPS)──> Manager /admin Dashboard
    │                    │
    │                    ├─> JWT Auth (validate is_admin claim)
    │                    ├─> Render embedded HTML templates
    │                    ├─> Query database (servers, gateways, customers)
    │                    └─> Generate metrics
    │
    └──(Click VNC/SOL)──> Manager GetServerToken API
                             │
                             └─> Gateway CreateVNCSession API
                                   │
                                   └─> Browser opens Gateway /vnc/{session-id}
```

### Component Changes

1. **Manager - Authentication**:
    - Add `is_admin` boolean field to JWT claims
    - Add admin validation to `Authenticate()` method (check against admin users
      list)
    - Create `AdminAuthInterceptor` middleware to protect `/admin` routes
    - Validate admin claim on all dashboard API endpoints

2. **Manager - Database**:
    - Create `customers` table with `id`, `email`, `is_admin`, `created_at`
      fields (may replace existing ad-hoc customer tracking)
    - Add database indexes on `servers.customer_id`, `servers.status`,
      `server_locations.gateway_id`
    - Create repository methods: `ListAllServers()`,
      `ListCustomersWithServerCounts()`, `GetGatewayHealthMetrics()`
    - Migrate existing customer/user data if needed

3. **Manager - Web UI**:
    - Create `manager/internal/webui/` package (mirrors Gateway structure)
    - Embed HTML templates using `//go:embed` directive
    - Add HTTP routes: `/admin` (dashboard), `/admin/api/*` (JSON APIs)
    - Implement template rendering with server-side data injection
    - Serve with Tailwind CSS for consistent styling

4. **Manager - Admin API (new protobuf package)**:
    - Create `proto/manager/v1/admin.proto` with `AdminService`
    - Define admin-only RPC methods: `ListAllServers()`,
      `GetDashboardMetrics()`, `ListAllCustomers()`, `GetGatewayHealth()`,
      `GetRegions()`
    - Implement Connect RPC handlers (buf connect) for direct browser calls
    - Return paginated results for large datasets
    - All methods protected by `AdminAuthInterceptor`

5. **Frontend - Dashboard UI**:
    - System metrics header with cards (total BMCs, online/offline, active
      sessions, gateway count)
    - Gateway health table (ID, region, status, last seen, server count)
    - Customer summary table (customer ID, server count, filter action)
    - BMC server table with columns: Server ID | Customer | Datacenter |
      Gateway | Endpoint | Status | Actions
    - **Filter controls**: Customer dropdown, Region dropdown (multi-select),
      Gateway dropdown, Status dropdown, search box
    - Action buttons: Info (show details), VNC (launch console), SOL (launch
      console)
    - **Connect RPC client**: Use `@connectrpc/connect-web` to call
      `AdminService` RPCs directly from browser
    - Client-side filtering and search with vanilla JavaScript

## Implementation Plan

### Phase 1: Authentication & Database

- [ ] Create `customers` table schema (replace/migrate existing user tracking)
- [ ] Add `is_admin` field to JWT claims structure in `manager/pkg/auth/auth.go`
- [ ] Update `Authenticate()` to check admin status from config or database
- [ ] Add admin email list to Manager configuration (YAML or env var)
- [ ] Run database migration to create customers table and indexes
- [ ] Create `AdminAuthInterceptor` middleware for `/admin` routes
- [ ] Invalidate existing tokens (breaking change acceptable)

### Phase 2: Admin Protobuf Package & API

- [ ] Create `proto/manager/v1/admin.proto` with `AdminService` definition
- [ ] Define all admin RPC methods and messages (ListAllServers,
  GetDashboardMetrics, etc.)
- [ ] Generate protobuf code (`make gen-all`)
- [ ] Implement repository methods: `ListAllServers()`,
  `ListCustomersWithServerCounts()`, `GetGatewayHealthMetrics()`,
  `GetRegions()`
- [ ] Implement Connect RPC handlers for `AdminService`
- [ ] Add pagination support and multi-filter logic
- [ ] Add error handling and validation

### Phase 3: Web UI Infrastructure

- [ ] Create `manager/internal/webui/` package structure
- [ ] Create HTML templates: `admin_base.html`, `admin_dashboard.html`
- [ ] Implement `embed.go` with `//go:embed templates/*.html`
- [ ] Implement template rendering functions
- [ ] Add HTTP routes to `manager/cmd/manager/main.go`

### Phase 4: Frontend Implementation

- [ ] Build dashboard HTML structure with Tailwind CSS
- [ ] Add `@connectrpc/connect-web` library (via CDN or bundle)
- [ ] Implement Connect RPC client for `AdminService`
- [ ] Load data via Connect RPC calls (GetDashboardMetrics, ListAllServers,
  etc.)
- [ ] Add client-side filtering and search functionality
- [ ] Implement VNC/SOL launch flow (GetServerToken → CreateSession → open tab)
- [ ] Add server detail modal/expansion
- [ ] Implement live metrics updates (optional)

### Phase 5: Testing & Documentation

- [ ] Add unit tests for admin authentication interceptor
- [ ] Add unit tests for dashboard repository methods
- [ ] Add integration tests for dashboard API endpoints
- [ ] Test VNC/SOL launch workflow end-to-end
- [ ] Test access control (non-admin users blocked)
- [ ] Update `ARCHITECTURE-AI.md` with Manager web UI component
- [ ] Update `CLAUDE.md` with admin dashboard reference

## API Changes

### New Protobuf Package: `proto/manager/v1/admin.proto`

Create a dedicated admin service package for all administrative operations. This
enables:

- Direct Connect RPC calls from web dashboard
- Future CLI reuse via `bmc-cli admin` commands
- Clean separation of admin vs. customer APIs
- Consistent authorization via `AdminAuthInterceptor`

### Admin Service Definition

```protobuf
syntax = "proto3";

package manager.v1;

import "google/protobuf/timestamp.proto";

option go_package = "manager/gen/manager/v1;managerv1";

// AdminService provides administrative operations for managing the BMC infrastructure
// All methods require is_admin=true in JWT claims
service AdminService {
  // Dashboard metrics and overview
  rpc GetDashboardMetrics(GetDashboardMetricsRequest) returns (GetDashboardMetricsResponse);

  // Server management across all customers
  rpc ListAllServers(ListAllServersRequest) returns (ListAllServersResponse);

  // Customer management
  rpc ListAllCustomers(ListAllCustomersRequest) returns (ListAllCustomersResponse);

  // Gateway health and monitoring
  rpc GetGatewayHealth(GetGatewayHealthRequest) returns (GetGatewayHealthResponse);

  // Available regions for filtering
  rpc GetRegions(GetRegionsRequest) returns (GetRegionsResponse);
}
```

### New Protobuf Messages

```protobuf
// Dashboard metrics aggregation
message GetDashboardMetricsRequest {}

message GetDashboardMetricsResponse {
    int32 total_bmcs = 1;
    int32 online_bmcs = 2;
    int32 offline_bmcs = 3;
    int32 total_gateways = 4;
    int32 active_gateways = 5;
    int32 total_customers = 6;
    int32 active_sessions = 7;  // Future: requires session tracking
}

// List all servers across all customers (admin only)
message ListAllServersRequest {
    int32 page_size = 1;         // Default 100, max 500
    string page_token = 2;       // Pagination token
    string customer_filter = 3;  // Optional: filter by customer_id
    repeated string region_filter = 4;  // Optional: filter by gateway regions (multi-select)
    string gateway_filter = 5;   // Optional: filter by specific gateway_id
    string status_filter = 6;    // Optional: filter by status
}

message ListAllServersResponse {
    repeated ServerDetails servers = 1;
    string next_page_token = 2;
    int32 total_count = 3;
}

message ServerDetails {
    string server_id = 1;
    string customer_id = 2;
    string datacenter_id = 3;
    string gateway_id = 4;
    string primary_endpoint = 5;
    string primary_protocol = 6;  // "ipmi" or "redfish"
    string status = 7;            // "online", "offline", "unknown"
    bool has_vnc = 8;
    bool has_sol = 9;
    google.protobuf.Timestamp last_seen = 10;
    google.protobuf.Timestamp created_at = 11;
}

// List customers with server counts (admin only)
message ListAllCustomersRequest {
    int32 page_size = 1;
    string page_token = 2;
}

message ListAllCustomersResponse {
    repeated CustomerSummary customers = 1;
    string next_page_token = 2;
}

message CustomerSummary {
    string customer_id = 1;
    string email = 2;
    int32 server_count = 3;
    int32 online_server_count = 4;
    bool is_admin = 5;
    google.protobuf.Timestamp created_at = 6;
}

// Gateway health metrics (admin only)
message GetGatewayHealthRequest {}

message GetGatewayHealthResponse {
    repeated GatewayHealth gateways = 1;
}

message GatewayHealth {
    string gateway_id = 1;
    string region = 2;
    string endpoint = 3;
    string status = 4;  // "active", "degraded", "offline"
    google.protobuf.Timestamp last_seen = 5;
    int32 server_count = 6;
    repeated string datacenter_ids = 7;
}

// Available regions for filtering
message GetRegionsRequest {}

message GetRegionsResponse {
    repeated string regions = 1;  // e.g., ["us-east-1", "us-west-2", "eu-west-1"]
}
```

### JWT Claims Extension

```go
// manager/pkg/auth/auth.go
type Claims struct {
    CustomerID string `json:"customer_id"`
    Email      string `json:"email"`
    IsAdmin    bool   `json:"is_admin"` // NEW: indicates super-admin privileges
    jwt.RegisteredClaims
}
```

### HTTP Routes (Manager)

```
# Web UI
GET  /admin                     → Render admin dashboard HTML

# Connect RPC endpoints (buf connect over HTTP)
POST /manager.v1.AdminService/GetDashboardMetrics    → AdminService RPC
POST /manager.v1.AdminService/ListAllServers         → AdminService RPC
POST /manager.v1.AdminService/ListAllCustomers       → AdminService RPC
POST /manager.v1.AdminService/GetGatewayHealth       → AdminService RPC
POST /manager.v1.AdminService/GetRegions             → AdminService RPC

# Connect RPC uses standard HTTP POST with JSON encoding
# Browser calls via @connectrpc/connect-web library
# CLI can use same endpoints via Connect client
```

### Database Schema Changes

```sql
-- Migration: create customers table (replaces ad-hoc customer tracking)
CREATE TABLE customers (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Migration: add indexes for dashboard queries
CREATE INDEX idx_servers_customer_id ON servers(customer_id);
CREATE INDEX idx_servers_status ON servers(status);
CREATE INDEX idx_server_locations_gateway_id ON server_locations(regional_gateway_id);
CREATE INDEX idx_customers_is_admin ON customers(is_admin) WHERE is_admin = true;

-- Note: Existing customer data may need migration from current auth system
```

### Configuration Changes

Add admin user configuration to Manager:

```yaml
# config/manager.yaml
auth:
  secret_key: "your-secret-key"
  admins:
    - admin@example.com
    - ops@example.com
```

Or via environment variables:
```bash
export ADMIN_EMAILS="admin@example.com,ops@example.com"
```

Admin status is checked during authentication and baked into JWT claims.

## Testing Strategy

### Unit Tests

**Authentication:**

- `TestAdminAuthInterceptor_ValidAdminToken` - verify admin access granted
- `TestAdminAuthInterceptor_NonAdminToken` - verify 403 Forbidden for non-admin
- `TestAdminAuthInterceptor_MissingToken` - verify 401 Unauthorized
- `TestAuthenticateWithAdminUser` - verify `is_admin` claim populated correctly

**Repository Methods:**

- `TestListAllServers` - verify pagination, filtering, sorting
- `TestListCustomersWithServerCounts` - verify aggregation logic
- `TestGetGatewayHealthMetrics` - verify health calculation (last_seen
  threshold)

**API Handlers:**

- `TestGetDashboardMetrics` - verify metrics aggregation
- `TestListAllServersWithFilters` - verify customer/region/gateway/status
  filters
- `TestListAllServersMultiRegionFilter` - verify multiple region filtering
- `TestListAllCustomersUnauthorized` - verify admin-only access

### Integration Tests

**Dashboard API Flow:**

1. Authenticate as admin user → receive JWT with `is_admin=true`
2. Call `ListAllServers()` → verify returns servers from multiple customers
3. Call `GetDashboardMetrics()` → verify counts match database state
4. Call `GetGatewayHealth()` → verify gateway status based on `last_seen`

**Non-Admin Access Control:**

1. Authenticate as regular user → receive JWT with `is_admin=false`
2. Attempt to call `ListAllServers()` → verify 403 Forbidden
3. Attempt to access `/admin` → verify redirect or 403

**VNC/SOL Launch Flow:**

1. Admin authenticates and accesses dashboard
2. Click VNC button for server → JavaScript calls `GetServerToken(server_id)`
3. JavaScript calls `Gateway.CreateVNCSession(server_id, token)`
4. Verify session URL returned
5. Verify opening URL in new tab loads Gateway console

### E2E Tests

**Full Admin Workflow:**

```bash
# 1. Admin logs in
bmc-cli auth login --email admin@example.com --password secret

# 2. Access dashboard (manual browser test)
# Open browser to https://manager:8080/admin

# 3. Verify dashboard displays:
#    - System metrics cards
#    - Gateway health table
#    - Customer summary
#    - All servers from all customers

# 4. Test filtering:
#    - Filter by customer
#    - Filter by gateway
#    - Search by server ID

# 5. Test VNC launch:
#    - Click VNC button
#    - Verify new tab opens with Gateway console
#    - Verify console connects successfully
```

## Security Considerations

**Authentication & Authorization:**

- Admin access controlled by `is_admin` boolean in JWT claims (not role string
  to avoid parsing issues)
- `AdminAuthInterceptor` validates `is_admin=true` on all dashboard routes
- Non-admin users receive 403 Forbidden with no information disclosure
- Session cookies are HttpOnly, Secure (HTTPS), SameSite=Strict

**Data Exposure:**

- Dashboard shows BMC endpoints and customer associations (admin-only
  visibility)
- No BMC credentials exposed in UI or API responses
- Server tokens generated for VNC/SOL access expire in 1 hour
- Gateway session cookies expire in 24 hours

**Audit Logging:**

- Log all admin dashboard access attempts (success and failure)
- Log VNC/SOL session creation by admins with server_id and target customer
- Include admin email in all audit logs for accountability

**CSRF Protection:**

- Session cookies use SameSite=Strict attribute
- JSON API endpoints accept cookies (not vulnerable to form submission CSRF)
- Future: Add CSRF tokens for state-changing operations if needed

**Input Validation:**

- Validate pagination parameters (page_size max 500)
- Sanitize filter inputs to prevent SQL injection
- Validate server_id format before VNC/SOL session creation

## Migration Strategy

Since this is experimental software, we can introduce breaking changes for cleaner implementation.

**Deployment Steps:**

1. **Database Migration**:
   ```bash
   # Run migration to create customers table and indexes
   # This may require recreating existing auth tables
   make db-migrate
   ```

2. **Update Auth Configuration**:
   ```yaml
   # config.yaml - define admin users
   auth:
     admins:
       - admin@example.com
       - ops@example.com
   ```
   Or use environment variable:
   ```bash
   export ADMIN_EMAILS="admin@example.com,ops@example.com"
   ```

3. **Deploy Manager with Admin Dashboard**:
   ```bash
   # Deploy updated Manager binary with web UI
   # Existing sessions may be invalidated (acceptable for experimental system)
   ```

4. **Verify Dashboard Access**:
   ```bash
   # Admin user authenticates and accesses /admin
   bmc-cli auth login --email admin@example.com
   # Open browser to https://manager:8080/admin
   ```

**Breaking Changes:**

- JWT claims structure changed (added `is_admin` field) - existing tokens invalidated
- New `customers` table required - may need to migrate existing user data
- Authentication flow updated - users need to re-authenticate
- Database schema changes - requires migration and potential data reshaping

## Future Enhancements

**Real-time Updates:**

- WebSocket connection for live dashboard updates (metrics, server status
  changes)
- Push notifications for gateway health changes or critical events

**Advanced Filtering & Search:**

- Full-text search across server metadata
- Save custom filter presets
- Export filtered results to CSV/JSON

**Bulk Operations:**

- Select multiple servers and execute power operations
- Bulk VNC/SOL session creation with tabbed interface
- Bulk server metadata updates

**Enhanced Metrics:**

- Historical trends for BMC online/offline events
- Gateway performance metrics (session count, bandwidth)
- Customer usage analytics (console session duration, power operations)

**Customer Management UI:**

- Create/edit customer accounts from dashboard
- Assign server ownership
- Manage admin privileges (promote/demote admins)

**Alert Configuration:**

- Configure threshold alerts (too many offline BMCs, gateway down)
- Slack/email notifications for admin alerts
- Alert history and acknowledgment tracking

## Appendix

### Dashboard UI Wireframe

```
+-------------------------------------------------------------------------------------------+
|  BMC Management Dashboard (Admin)                                    admin@example.com ▼ |
+-------------------------------------------------------------------------------------------+
|                                                                                           |
|  [Total BMCs: 1,247]  [Online: 1,189]  [Offline: 58]  [Active Gateways: 3/3]           |
|                                                                                           |
+-------------------------------------------------------------------------------------------+
|  Gateway Health                                                                           |
+-------------------------------------------------------------------------------------------+
|  ID          | Region    | Status   | Last Seen        | Servers | Datacenters          |
|  gateway-us1 | us-east-1 | ✓ Active | 2 seconds ago    | 543     | dc1, dc2, dc3        |
|  gateway-us2 | us-west-2 | ✓ Active | 5 seconds ago    | 412     | dc4, dc5             |
|  gateway-eu1 | eu-west-1 | ✓ Active | 3 seconds ago    | 292     | dc6, dc7             |
+-------------------------------------------------------------------------------------------+
|  Customers                                                                                |
+-------------------------------------------------------------------------------------------+
|  Customer ID          | Servers | Online | Actions                                      |
|  customer1@acme.com   | 234     | 228    | [Filter Servers]                             |
|  customer2@widgets.io | 567     | 543    | [Filter Servers]                             |
|  customer3@example.com| 446     | 418    | [Filter Servers]                             |
+-------------------------------------------------------------------------------------------+
|  Servers                            [Customer ▼] [Gateway ▼] [Status ▼] [Search...    ] |
+-------------------------------------------------------------------------------------------+
|  Server ID  | Customer       | DC  | Gateway   | Endpoint         | Status  | Actions   |
|  srv-001    | customer1@...  | dc1 | gateway-us1 | 10.1.1.5      | Online  | ⓘ VNC SOL|
|  srv-002    | customer1@...  | dc1 | gateway-us1 | 10.1.1.6      | Online  | ⓘ VNC SOL|
|  srv-003    | customer2@...  | dc2 | gateway-us1 | 10.2.1.10     | Offline | ⓘ --- ---|
|  srv-004    | customer2@...  | dc4 | gateway-us2 | 10.4.1.20     | Online  | ⓘ VNC SOL|
|  ...                                                                                      |
+-------------------------------------------------------------------------------------------+
|  Showing 1-100 of 1,247                                    [◄ Previous]  [Next ►]        |
+-------------------------------------------------------------------------------------------+
```

### VNC/SOL Launch Flow Details

```
1. User clicks "VNC" button for server srv-001
   ↓
2. JavaScript: fetch('/admin/api/server-token?server_id=srv-001')
   ↓
3. Manager validates admin JWT, calls GetServerToken(srv-001)
   ↓ Returns: { server_token: "eyJ...", gateway_endpoint: "https://gateway-us1:8081" }
4. JavaScript: fetch('https://gateway-us1:8081/gateway.v1.GatewayService/CreateVNCSession', {
     server_id: 'srv-001',
     headers: { 'Authorization': 'Bearer eyJ...' }
   })
   ↓ Returns: { session_url: "https://gateway-us1:8081/vnc/abc123", session_id: "abc123" }
5. JavaScript: window.open(session_url, '_blank')
   ↓
6. Browser opens new tab → Gateway web console loads
   ↓
7. Gateway validates session → establishes VNC proxy → user sees console
```

### Frontend Technology Stack

- **Templates**: Go `html/template` with `//go:embed` directive
- **Styling**: Tailwind CSS (via CDN, consistent with Gateway console)
- **JavaScript**: Vanilla JS for filtering and UI interactions
- **RPC Client**: `@connectrpc/connect-web` for calling AdminService
- **Data Loading**: Connect RPC client (JSON over HTTP/2)
- **UI Components**: Tables, dropdowns (including multi-select for regions), metric cards, modals

### Future CLI Integration

The same `AdminService` RPCs will be reusable for future admin CLI commands:

```bash
# Future admin CLI commands using same AdminService
bmc-cli admin list-servers --region us-east-1 --status offline
bmc-cli admin dashboard-metrics
bmc-cli admin list-customers
bmc-cli admin gateway-health
```

CLI implementation will use Go Connect client calling same protobuf service as
web UI.

---

**References:**

- Gateway web console implementation: `gateway/internal/webui/`
- Manager authentication: `manager/pkg/auth/auth.go`
- Server database models: `manager/internal/database/models.go`
- Manager service definition: `proto/manager/v1/manager.proto`
- Connect RPC framework: https://connectrpc.com/
- Similar pattern in RFD 008 (Web Console Testing Strategy)
