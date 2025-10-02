# Core Module Architecture

## Design Rationale

The `core` module is split into **separate Go modules** to minimize transitive dependencies. This architectural decision is driven by security considerations, particularly for the **Gateway** component.

## Why Split Core?

### Security Risk Profile

The **Gateway** is the HIGHEST risk component in our architecture:
- **Internet-facing** - Direct exposure to external attacks
- **Authentication boundary** - If bypassed, grants access to ALL BMCs across all datacenters
- **Centralized attack point** - Single compromise affects entire fleet
- **Web attack surface** - Vulnerable to XSS, CSRF, injection attacks
- **Proxy logic complexity** - Bugs could allow unauthorized access
- **Supply chain risk** - Any compromised dependency can affect all customers

In contrast, **Local-Agent** has lower risk:
- Runs in private datacenter networks (not internet-facing)
- Uses outbound connections only (NAT/firewall friendly)
- Limited blast radius (compromise affects only one datacenter)
- No authentication logic (just follows Gateway commands)

### Dependency Problem

In a monolithic `core` module, **every import pulls all dependencies**:

```go
// Just importing core/types would pull in:
import "core/types"
// ❌ Forces: websocket, uuid, yaml libraries
```

This means the Gateway would carry unnecessary dependencies even when only using basic types, increasing the attack surface.

### Solution: Separate Modules

By splitting `core` into focused submodules with their own `go.mod` files:

```
core/                    # Base module (types, domain) - NO external deps
├── go.mod              # Pure Go, no dependencies
├── types/
├── domain/
├── identity/
│
├── auth/               # Separate module
│   └── go.mod          # Only: github.com/google/uuid
│
├── config/             # Separate module
│   └── go.mod          # Only: gopkg.in/yaml.v3
│
└── streaming/          # Separate module
    └── go.mod          # Only: github.com/gorilla/websocket
```

### Benefits

1. **Minimal Attack Surface**: Gateway only imports what it needs
2. **Supply Chain Security**: Fewer dependencies = fewer supply chain risks
3. **Faster Builds**: Less to download and verify
4. **Clear Boundaries**: Explicit dependency graph
5. **Easier Security Audits**: Smaller dependency tree to review

## Import Guidelines

### For Gateway (HIGHEST Security Priority)
```go
// ✅ Import only what you absolutely need
import "core/types"         // No external deps
import "core/domain"        // No external deps
import "core/auth"          // Only if you need JWT/auth utilities
```

### For Manager (High Security)
```go
// ✅ Import conservatively
import "core/types"         // No external deps
import "core/auth"          // Includes uuid
import "core/config"        // Only if needed
```

### For Local-Agent (Lower Risk - Private Network)
```go
// ✅ Can import more freely
import "core/identity"      // No external deps
import "core/config"        // Includes yaml
import "core/streaming"     // Includes websocket
```

## Module Structure

| Module | External Dependencies | Purpose |
|--------|----------------------|---------|
| `core` (base) | None | Pure types, domain models, identity |
| `core/auth` | `github.com/google/uuid` | JWT claims, auth utilities |
| `core/config` | `gopkg.in/yaml.v3` | Configuration loading |
| `core/streaming` | `github.com/gorilla/websocket` | Stream proxying |

## Verification

To verify Gateway has minimal dependencies:

```bash
cd gateway
go mod graph | grep -v "^gateway" | cut -d' ' -f2 | sort -u
```

**The goal is to keep Gateway's dependency list as small as possible** since it's the primary attack surface.

## Current Implementation Status

### Workspace Limitation

The current Go workspace setup includes all submodules, which means:
- All services see all dependencies in the workspace
- `go list -m all` shows all workspace dependencies
- **However**, services still only directly depend on what they import

### What This Achieves

**Benefit**: Clear dependency boundaries
- `core` (base) has zero external dependencies
- `core/auth`, `core/config`, `core/streaming` are isolated
- Future: Can publish these as separate modules

**Limitation**: Workspace-wide visibility
- During development, all dependencies are visible
- This is a Go workspace characteristic, not a security issue
- Production builds (without workspace) will have minimal dependencies

### Future Improvements

To achieve true dependency isolation, we could:
1. **Publish submodules separately** (requires separate repos or mono-repo tooling)
2. **Remove workspace** and use `replace` directives in each service
3. **Use Git submodules** for core/* packages

For now, the split provides clear boundaries and documentation of intent.
