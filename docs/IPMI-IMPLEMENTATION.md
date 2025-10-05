# IPMI Implementation Strategy

## Overview

This project uses **subprocess-based IPMI tools** (`ipmitool` and `ipmiconsole`)
rather than native Go libraries for all IPMI/BMC operations. This document
explains our rationale and implementation approach.

## Why Subprocess Tools Over Go Libraries

### The Problem with Go IPMI Libraries

We evaluated `github.com/bougou/go-ipmi` and other Go IPMI libraries but found
critical issues:

- **Crashes**: Nil pointer dereferences, incomplete session initialization,
  panics instead of errors
- **Limited functionality**: No SOL console support, incomplete IPMI command
  sets
- **Poor compatibility**: Hardcoded protocols (no lanplus→lan fallback),
  vendor-specific quirks
- **High maintenance**: Limited maintainers, would require deep IPMI expertise

### The Subprocess Approach

Instead, we use battle-tested command-line tools:

**For Power Operations:** `ipmitool`

- Industry standard
- Proven across millions of servers worldwide
- Handles all BMC vendor quirks
- Comprehensive error messages

**For Serial Console:** `ipmiconsole` (FreeIPMI)

- Purpose-built for SOL sessions
- Robust connection handling
- Automatic reconnection logic
- Wide protocol support

## Architecture

### Current Implementation

```
┌─────────────────────────────────────────────────────────┐
│                    Local Agent                          │
│                                                         │
│  ┌────────────────────────────────────────────────┐     │
│  │           IPMI Client (Go)                     │     │
│  │                                                │     │
│  │  ┌──────────────────────────────────────┐      │     │
│  │  │      SubprocessClient                │      │     │
│  │  │  - Executes ipmitool commands        │      │     │
│  │  │  - Parses output                     │      │     │
│  │  │  - Auto-fallback lanplus → lan       │      │     │
│  │  └──────────────────────────────────────┘      │     │
│  │                                                │     │
│  │  ┌──────────────────────────────────────┐      │     │
│  │  │      IPMISOLSession                  │      │     │
│  │  │  - Manages ipmiconsole subprocess    │      │     │
│  │  │  - Bidirectional streaming           │      │     │
│  │  │  - Handles reconnections             │      │     │
│  │  └──────────────────────────────────────┘      │     │
│  └────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │  Binaries            │
              │  - ipmitool          │
              │  - ipmiconsole       │
              └──────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │    IPMI BMC          │
              │  - IPMI v1.5/v2.0    │
              │  - Vendor-specific   │
              └──────────────────────┘
```

## Implementation Details

### Power Operations (`local-agent/pkg/ipmi/subprocess.go`)

**Supported Operations:**

- `PowerOn` → `ipmitool chassis power on`
- `PowerOff` → `ipmitool chassis power off`
- `PowerCycle` → `ipmitool chassis power cycle`
- `Reset` → `ipmitool chassis power reset`
- `GetPowerState` → `ipmitool chassis power status`
- `GetBMCInfo` → `ipmitool bmc info`

**Features:**

- Auto-fallback from IPMI v2.0 (lanplus) to v1.5 (lan)
- Timeout handling (10 second default)
- Context cancellation support
- Structured error messages

### Serial Console (`local-agent/pkg/sol/ipmi_sol.go`)

**Features:**

- Subprocess lifecycle management
- Stdin/Stdout/Stderr pipes for bidirectional I/O
- Automatic reconnection with exponential backoff
- Session replay buffer
- Metrics tracking (bytes transferred, uptime, errors)

## Benefits

### 1. Reliability

- ✅ **No crashes** - Subprocess failures return errors, never panic
- ✅ **Proven stability** - Tools used in production for 10-20+ years
- ✅ **Handles edge cases** - Vendor quirks, protocol variations, network issues

### 2. Compatibility

- ✅ **Wide BMC support** - Works with all major vendors (Dell, HP, Supermicro,
  etc.)
- ✅ **Protocol flexibility** - Auto-detects and falls back between IPMI versions
- ✅ **Real hardware tested** - Tools validated against thousands of server
  models

### 3. Maintainability

- ✅ **Less code** - ~200 lines vs ~1000+ for native implementation
- ✅ **Clear debugging** - Can run same commands manually for troubleshooting
- ✅ **Standard tools** - System administrators already familiar with these tools

### 4. Feature Completeness

- ✅ **Full IPMI support** - All commands available, not limited subset
- ✅ **SOL console** - Native support in ipmiconsole
- ✅ **Advanced features** - Sensors, SEL, FRU, SDR accessible via ipmitool

## Trade-offs

- **Subprocess overhead**: ~10-50ms per operation (negligible compared to 200-1000ms total BMC latency)
- **Binary dependencies**: Requires `ipmitool` and `freeipmi` installed (included in Dockerfile, standard on Linux)
- **Process isolation**: Slightly more resources, but prevents crashes from affecting agent

## Alternative Approaches Considered

### 1. Pure Go IPMI Implementation

**Pros:** No external dependencies, type-safe
**Cons:** Complex protocol, maintenance burden, would recreate known bugs
**Decision:** ❌ Not worth the effort

### 2. CGO Bindings to C Libraries

**Pros:** Native performance, library features
**Cons:** CGO complexity, cross-compilation issues, still need C libraries
**Decision:** ❌ Adds complexity without solving core issues

### 3. Different Go IPMI Library

**Pros:** Stay in Go ecosystem
**Cons:** All Go libraries have similar limitations and bugs
**Decision:** ❌ Same fundamental problems

### 4. Hybrid Approach (Library + Subprocess Fallback)

**Pros:** Use library when possible, fallback to subprocess
**Cons:** Two code paths to maintain, complexity
**Decision:** ❌ Subprocess-only is simpler and more reliable

## Future Considerations

### When to Reconsider Native Go

We would reconsider a Go library if:

1. A mature, actively-maintained library emerges with proven stability
2. The library has comprehensive test coverage against real hardware
3. Performance requirements make subprocess overhead unacceptable (unlikely)
4. Cross-platform requirements prevent using system binaries

**Current assessment:** None of these conditions are met. Subprocess approach
remains optimal.

### Potential Enhancements

**1. Command Caching**
Cache BMC info, capabilities to reduce redundant calls.

**2. Performance Monitoring**
Track subprocess execution times, identify slow BMCs.

## References

- [ipmitool Documentation](https://github.com/ipmitool/ipmitool)
- [FreeIPMI Project](https://www.gnu.org/software/freeipmi/)
- [IPMI v2.0 Specification](https://www.intel.com/content/www/us/en/products/docs/servers/ipmi/ipmi-second-gen-interface-spec-v2-rev1-1.html)
- [RFD 011 - FreeIPMI SOL Streaming](./features/011-freeipmi-sol-streaming.md)
- [RFD 014 - ipmitool Power Operations](./features/014-ipmitool-power-operations.md)

## Related Documentation

- **Architecture**: See `docs/ARCHITECTURE.md` for overall system design
- **Development**: See `docs/DEVELOPMENT.md` for local setup with IPMI
  simulators
- **Docker Setup**: See `docker/README.md` for VirtualBMC and Redfish containers
