# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

This is a **BMC (Baseboard Management Controller) access solution** for hosting
providers. The system provides secure, multi-tenant access to server management
interfaces (IPMI/Redfish) without exposing BMC ports publicly.

## Architecture

The system consists of four main components:

```
CLI Terminal ↔ Manager (auth) → Gateway (proxy) → Agent → BMC (IPMI SOL/Redfish Serial Console)
CLI Browser ↔ Manager (auth) → Gateway (web UI + proxy) → Agent → BMC (VNC/KVM Graphics Console)
External Tools ↔ Manager (auth) → Gateway (proxy) → Agent → BMC (Direct API Access)
```

### Components

1. **Manager (authentication and mapping service)**

    - Handles user authentication and authorization
    - Maps customer sessions to server IDs
    - Manages server ID to BMC endpoint mappings
    - Stores user permissions and server ownership data
    - Provides authentication tokens for Gateway access

2. **Gateway (traffic routing, web UI, and proxy)**

    - Routes authenticated traffic to correct datacenter agents
    - Handles command abstractions (IPMI/Redfish)
    - Validates authentication tokens from Manager
    - **Serves all web-based interfaces** via embedded templates
      (`internal/webui`)
    - Manages session types:
        - **VNC Sessions**: Web-based graphical console access (VNC/KVM)
        - **Console Sessions**: (Future) Web-based serial console access

3. **Local Agent (per datacenter)**

    - Runs in provider's private network
    - Maintains outbound connection to Gateway
    - Discovers and registers BMC endpoints with Manager through Gateway
    - Provides three types of BMC access:
        - **Serial Console**: IPMI SOL or Redfish serial console streaming
        - **VNC/KVM**: Graphical console access for GUI interactions

4. **Customer CLI**
    - **Pure command-line tool** following Unix philosophy
    - Authenticates with Manager, then coordinates with Gateway
    - **No embedded web servers** - delegates web UI to Gateway
    - Console access methods:
        - **`server console <id>`**: Opens Gateway's web console (default, user-friendly)
        - **`server console <id> --terminal`**: Direct SOL streaming to terminal (advanced)
        - **`server vnc <id>`**: Opens Gateway's VNC viewer in browser

## Project Structure

```
bmc-mgmt/
├── README.md                  # Primary project overview
├── CLAUDE.md                  # This guidance document
├── Makefile                   # Top-level build/test targets
├── docs/
│   ├── ARCHITECTURE.md        # High-level system topology
│   ├── DESIGN.md              # Detailed design decisions
│   ├── DEVELOPMENT.md         # Canonical development setup guide
│   └── features/              # RFDs (feature proposals)
├── manager/
│   ├── cmd/manager/           # Manager service entrypoint
│   ├── internal/              # Core handlers/business logic
│   ├── pkg/                   # Shared packages (auth, database, models)
│   ├── proto/                 # Protobuf definitions
│   └── gen/                   # Generated buf/connect code
├── gateway/                   # Gateway service (traffic routing, web UI & proxy)
│   ├── cmd/gateway/           # Gateway service entrypoint
│   ├── internal/              # Core handlers/business logic
│   ├── internal/webui/        # Embedded templates & assets
│   ├── pkg/                   # Shared packages
│   ├── proto/                 # Protobuf definitions
│   └── gen/                   # Generated buf/connect code
├── local-agent/
│   ├── cmd/agent/             # Agent service entrypoint
│   ├── internal/agent/        # Core handlers/business logic
│   ├── pkg/
│   └── config/                # Agent configuration samples
├── cli/
│   ├── cmd/                   # Cobra command tree
│   └── pkg/                   # CLI client + config helpers
├── docker/
│   ├── README.md              # Containerized IPMI/Redfish docs
│   ├── *.Dockerfile           # Service and simulator images
│   ├── configs/               # Agent/test config templates
│   ├── scripts/               # Startup helpers (VirtualBMC, etc.)
│   └── supervisor/            # Supervisord configuration
├── docker-compose.core.yml    # Core manager/gateway/agent dev stack
├── docker-compose.e2e.yml     # Ephemeral E2E infrastructure
├── docker-compose.virtualbmc.yml # Persistent VirtualBMC/Redfish dev stack
├── tests/
│   ├── e2e/                   # End-to-end test suite & clients
│   ├── smoke/                 # Functional smoke tests
│   ├── integration/           # Integration-level tests
│   ├── synthetic/             # Synthetic BMC helpers
│   └── go.mod / go.sum        # Dedicated Go module for tests
├── tooling/
│   └── make/                  # Extended make targets (docker, CI)
└── tmp/                       # Temporary tooling (generated makefiles, caches)
```

## Separation of Concerns

### Manager Responsibilities

-   **Authentication**: JWT token generation, API key validation
-   **Authorization**: User permissions, role-based access control (RBAC)
-   **Server Mapping**: Maps customer server IDs to actual BMC endpoints
-   **Ownership**: Tracks which customers own which servers
-   **User Management**: Customer accounts, permissions, API keys

### Gateway Responsibilities

-   **Traffic Routing**: Routes requests to appropriate datacenter agents
-   **Session Management**: Manages active proxy sessions and connections
-   **Token Validation**: Validates authentication tokens received from Manager
-   **Web UI Serving**: Serves all web-based interfaces via embedded templates
-   **WebSocket Proxying**: Handles real-time console/VNC data streaming
-   **Load Balancing**: Distributes traffic across available agents (TODO)

### Local Agent Responsibilities

-   **BMC Discovery**: Scans network for BMC endpoints
-   **Registration**: Reports discovered BMCs to Manager for mapping
-   **Protocol Handling**: Direct communication with BMC interfaces
-   **Connection Management**: Maintains persistent connections to Gateway

### CLI Responsibilities

-   **Pure Command-Line Interface**: Unix-style tool for scripting and
    automation
-   **Authentication Flow**: Obtains tokens from Manager
-   **Browser Launching**: Opens Gateway web UIs for graphical access
-   **SOL Terminal**: Direct SOL streaming
-   **No Web Servers**: Delegates all web functionality to Gateway

## Development Commands

**IMPORTANT: Use Local Development Environment for Testing**

When testing changes or troubleshooting issues, ALWAYS use the local development
environment (without Docker) as it provides better debugging and faster
iteration:

**Primary Development Environment** (Air hot reloading without Docker):

```bash
make local-env-up     # Start all services with Air hot reloading
make local-env-logs   # View all service logs
make local-env-status # Check service status
make local-env-down   # Stop all services
make local-env-help   # View all local development commands
```

**Docker Environment** (for container testing only):

```bash
make dev-up    # Start all services with hot reloading (Docker)
make dev-logs  # View all service logs (Docker)
make dev-down  # Stop all services (Docker)
make dev-help  # View all development commands (Docker)
```

**Testing Workflow:**

1. Start local environment: `make local-env-up`
2. Run CLI commands to test: `./bin/bmc-cli server list`
3. Check logs if issues: `make local-env-logs`
4. Make code changes (Air will auto-restart services)
5. Re-test functionality
6. Stop environment: `make local-env-down`

See `docs/DEVELOPMENT.md` for detailed setup and troubleshooting guide.

**Manager Service** (from `manager/` directory):

```bash
make gen           # Generate protobuf code
make build         # Build manager binary
make run           # Run the manager server
make test          # Run tests
```

**Gateway Service** (from `gateway/` directory):

```bash
make gen           # Generate protobuf code
make build         # Build gateway binary
make run           # Run the gateway server
make test          # Run tests
```

**Local Agent Service** (from `local-agent/` directory):

```bash
make build         # Build agent binary
make run           # Run the local agent
make test          # Run tests
```

**CLI Tool** (from `cli/` directory):

```bash
make build         # Build CLI binary
make run           # Show CLI help
./bin/bmc-cli server list                    # List servers
./bin/bmc-cli server show <server-id>        # Show server info
./bin/bmc-cli server power on <server-id>    # Power on server
./bin/bmc-cli server power status <server-id> # Get power status
./bin/bmc-cli server console <server-id>             # Open Gateway web console (default)
./bin/bmc-cli server console <server-id> --terminal # Direct SOL terminal streaming
./bin/bmc-cli server vnc <server-id>                 # Open Gateway VNC viewer
```

## Web UI Architecture

### Centralized Template System

-   **Location**: `gateway/internal/webui/`
-   **Technology**: Go `embed` filesystem with HTML templates
-   **Templates**:
    -   `base.html` - Shared layout, styling, and JavaScript utilities
    -   `vnc.html` - VNC console interface extending base template
    -   `console.html` - (Future) Serial console interface extending base
        template
-   **Benefits**:
    -   Single source of truth for all web UI
    -   Consistent styling and behavior across interfaces
    -   Templates embedded in Gateway binary for easy deployment
    -   No duplication of HTML/CSS/JS code

### CLI Architecture Principles

-   **Unix Philosophy**: CLI tools should be scriptable and composable
-   **No Embedded Web Servers**: CLI delegates web functionality to Gateway
-   **Clean Command Structure**:
    -   Default behavior: Terminal-based (SOL streaming)
    -   `--web` flag: Opens Gateway web interface in browser
-   **Browser Integration**: CLI launches system browser for web UIs

## Session Types

The system supports three distinct session types for different use cases:

### 1. Console Sessions (Serial Console)

-   **Purpose**: Serial console access to server (text-based)
-   **Protocol**: IPMI SOL (Serial-over-LAN) or Redfish serial console
-   **Default Experience**: Web-based terminal (XTerm.js) with power controls
-   **Advanced Mode**: Direct terminal streaming (`--terminal` flag)
-   **Use Cases**: OS installation, debugging, remote administration, system recovery
-   **Commands**:
    -   `bmc-cli server console <server-id>` - Web console (default, recommended)
    -   `bmc-cli server console <server-id> --terminal` - Terminal streaming (advanced)

### 2. VNC Sessions (Graphical)

-   **Purpose**: Graphical console access for GUI interactions
-   **Protocol**: VNC or BMC KVM-over-IP
-   **Experience**: Web-based graphical console with mouse/keyboard
-   **Use Cases**: GUI operations, BIOS configuration, OS desktop access
-   **Command**: `bmc-cli server vnc <server-id>`

## Security Model

-   No BMC directly exposed on public internet
-   Agent initiates outbound connections only (NAT/firewall friendly)
-   Manager handles all authentication and authorization
-   Manager enforces RBAC and server ownership verification
-   Gateway validates authentication tokens from Manager before session setup
-   Multi-tenant safe with strict separation of customer data
-   All three session types use encrypted server context tokens

## Testing Requirements

**CRITICAL: Always ensure tests pass after making changes**

Before committing any changes, you MUST run and verify that all existing tests
pass:

### Test Execution Order

1. **Unit Tests** (run these for every change):

    ```bash
    make test-all
    ```

2. **E2E Tests** (run these before major releases):

    ```bash
    # Start container environment first
    make dev-up

    # Run E2E tests
    make test-e2e

    # Clean up
    make dev-down
    ```

### Test Maintenance Guidelines

-   **When adding new features**: Write unit tests that cover the new
    functionality
-   **When modifying existing code**: Ensure existing tests still pass and
    update them if the behavior changes
-   **When fixing bugs**: Add regression tests to prevent the same issue from
    recurring
-   **When changing interfaces**: Update all affected tests to work with the new
    interface

### Test Failure Resolution

If tests fail after your changes:

1. **Understand the failure**: Read the test output carefully to understand what
   broke
2. **Assess if the change is intentional**:
    - If the test failure reflects an intentional behavior change, update the
      test
    - If the test failure indicates a regression, fix your code
3. **Update tests appropriately**: When updating tests, ensure they still
   validate the intended behavior
4. **Verify comprehensive coverage**: Make sure your changes don't reduce
   overall test coverage

### Connect RPC Testing Notes

This project uses Connect RPC (protobuf) for service communication. When writing
tests:

-   Use proper Connect RPC mock servers, not plain HTTP servers returning JSON
-   Embed `UnimplementedServiceHandler` types for mocks to ensure interface
    compliance
-   Be aware that content-type mismatches (`application/json` vs
    `application/proto`) indicate incorrect test setup

## Code Style

-   Follow Effective Go conventions
-   Follow Go Doc Comments style

## Feature Documentation

Feature documentation follows the RFD (Request for Discussion) format in
`docs/features/`. When creating new feature documents:

### RFD Format Requirements

All feature documents must include YAML frontmatter with the following fields:

```yaml
---
rfd: "XXX" # Sequential number (001, 002, etc.)
title: "Feature Name" # Descriptive title
state: "draft" # draft, under-review, approved, implemented
breaking_changes: true # true/false
testing_required: true # true/false
database_changes: true # true/false - requires migrations
api_changes: true # true/false - affects public APIs
dependencies: # External dependencies required
    - "github.com/example/lib"
database_migrations: # List of required migrations
    - "create_new_table"
areas: ["manager", "gateway"] # Affected areas (same as area)
---
```

### Document Structure

1. **Title**: `# RFD XXX - Feature Name`
2. **Summary**: Brief description of the feature
3. **Problem**: What problem this solves
4. **Solution**: How the feature addresses the problem
5. **Implementation Plan**: Phased approach if applicable
   - **IMPORTANT**: Do NOT include time estimates (weeks, hours, days)
   - Focus on deliverable phases and concrete tasks
   - Use checkboxes for task lists
6. **API Changes**: New endpoints, modified interfaces
7. **Testing Strategy**: How to validate the feature
8. **Migration Strategy**: How to deploy safely, ignore backward
   compatibility for now

### RFD Writing Guidelines

**Time Estimates:**
- ❌ **DO NOT** include time estimates like "Week 1", "2 hours", "3 days"
- ❌ **DO NOT** predict how long implementation will take
- ✅ **DO** organize work into logical phases
- ✅ **DO** use checkboxes for concrete, testable tasks
- ✅ **DO** focus on what needs to be done, not when

**Example:**
```markdown
## Implementation Plan

### Phase 1: Database Schema
- [ ] Create migration for new table
- [ ] Add indexes for common queries
- [ ] Update models to support new fields

### Phase 2: API Implementation
- [ ] Add RPC endpoint
- [ ] Implement handler logic
- [ ] Add input validation
```

### File Naming Convention

-   Files should be numbered sequentially: `001-feature-name.md`
-   Use kebab-case for feature names
-   Place all RFDs in `docs/features/` directory

### Examples

See existing RFDs for reference:

-   `docs/features/001-encrypted-server-tokens.md` - Authentication enhancement
-   `docs/features/002-oidc-authentication.md` - OIDC integration
-   `docs/features/003-server-ownership.md` - Authorization system
-   `docs/features/004-server-identity-integration.md` - Server lifecycle
    management

# important-instruction-reminders

Do what has been asked; nothing more, nothing less. NEVER create files unless
they're absolutely necessary for achieving your goal. ALWAYS prefer editing an
existing file to creating a new one. NEVER proactively create documentation
files (\*.md) or README files. Only create documentation files if explicitly
requested by the User.

      IMPORTANT: this context may or may not be relevant to your tasks. You should not respond to this context unless it is highly relevant to your task.
