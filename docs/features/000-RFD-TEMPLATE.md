---
rfd: "XXX"
title: "Feature Name"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
dependencies: []
database_migrations: []
areas: []
---

# RFD XXX - Feature Name

**Status:** 🚧 Draft

<!-- Use status emojis: 🚧 Draft, 👀 Under Review, ✅ Approved, 🎉 Implemented -->

## Summary

2-3 sentence overview of the feature. State the motivation and the high-level outcome concisely.

## Problem

Detailed problem description:

- **Current behavior/limitations**: Describe what doesn't work or is missing
- **Why this matters**: Impact on users, operations, or system capabilities
- **Use cases affected**: Specific scenarios that are blocked or difficult
- Reference metrics, incidents, or user feedback where relevant

## Solution

High-level approach to solving the problem:

**Key Design Decisions:**
- Architectural choices and rationale
- Why this approach over alternatives
- Trade-offs considered

**Benefits:**
- What this enables or improves
- Performance/security/usability gains

**Architecture Overview:**
```
[Optional: Include ASCII diagrams showing component interactions]
User → Manager → Gateway → Agent → BMC
```

### Component Changes

Brief description of changes per component:

1. **Component A** (e.g., Manager):
   - Key change and rationale
   - API/data flow updates

2. **Component B** (e.g., Gateway):
   - Key change and rationale
   - Dependencies or impacts

3. **Component C** (e.g., CLI):
   - Key change and rationale
   - User-facing changes

**Configuration Example:**
```yaml
# Show YAML config examples where relevant
feature:
  enabled: true
  option: value
```

## Implementation Plan (Optional)

**IMPORTANT:** Do NOT include time estimates (weeks, hours, days). Focus on deliverable phases and concrete, testable tasks.

### Phase 1: Foundation/Database/Protocol
- [ ] Define data structures and types
- [ ] Create database migrations (if applicable)
- [ ] Define protobuf messages and generate code

### Phase 2: Core Implementation
- [ ] Implement [Component A] changes
- [ ] Implement [Component B] changes
- [ ] Add error handling and validation

### Phase 3: Integration & CLI
- [ ] Integrate components end-to-end
- [ ] Add CLI commands/flags
- [ ] Update configuration handling

### Phase 4: Testing & Documentation
- [ ] Add unit tests for all components
- [ ] Add integration tests
- [ ] Add E2E tests
- [ ] Update user documentation

## API Changes

**THIS IS WHERE DETAIL IS APPROPRIATE**

### New Protobuf Messages

```protobuf
// Full protobuf definitions go here
message NewFeatureRequest {
  string server_id = 1;
  string parameter = 2;
}

message NewFeatureResponse {
  bool success = 1;
  string message = 2;
}
```

### New RPC Endpoints

```protobuf
service ServiceName {
  rpc NewOperation(NewFeatureRequest) returns (NewFeatureResponse);
}
```

### CLI Commands

```bash
# New command with expected output
bmc-cli feature action <args> [--flags]

# Example output:
Feature Result:
  Status: Success
  Details: ...
```

### Configuration Changes

- New config field: `feature.enabled` (optional, default: false)
- Modified field: `existing.field` now supports additional values

## Testing Strategy

**Optional section**

### Unit Tests
- Component-specific test scope
- Key scenarios to validate
- Error handling coverage

### Integration Tests
- Cross-component test scenarios
- Test against BMC simulators (VirtualBMC, Redfish mock)
- Verify data flow end-to-end

### E2E Tests
- Full stack test scenarios
- CLI command validation
- Real-world usage patterns

## Security Considerations

**Optional section**

- Authentication/authorization impacts
- Data exposure risks and mitigations
- Credential handling (never expose actual passwords)
- Audit logging requirements
- TLS/encryption requirements

## Migration Strategy

**Optional section - include if backward compatibility is a concern**

1. **Deployment Steps**:
   - Deploy Manager changes
   - Deploy Gateway changes
   - Deploy Agent changes
   - Enable feature flag

2. **Rollback Plan**:
   - Disable feature flag
   - Revert database changes (if needed)
   - No breaking changes to existing workflows

## Future Enhancements

**Optional section for follow-up work**

- Potential improvements not in scope for initial implementation
- Features that build on this foundation
- Performance optimizations
- Additional integrations

## Appendix

**Optional section for technical deep-dives**

### Protocol Details
- Detailed protocol specifications
- Wire formats
- Authentication flows

### Reference Implementations
- Links to similar implementations
- Industry standards (RFCs, specifications)

### Test Configuration Examples
```yaml
# Detailed test setup examples
test:
  config: example
```

---

## RFD Writing Guidelines

**DO:**
- ✅ Focus on WHAT changes, not HOW to implement every detail
- ✅ Include full API specifications (protobuf, REST endpoints)
- ✅ Include configuration examples (YAML, CLI commands)
- ✅ Show expected output for CLI commands
- ✅ Use diagrams for complex architectures
- ✅ Put protocol details in Appendix
- ✅ Reference files by path only (e.g., `manager/pkg/database/database.go`)

**DO NOT:**
- ❌ Include time estimates (weeks, hours, days)
- ❌ Show complete Go/Python implementations
- ❌ Include verbose code that belongs in actual implementation
- ❌ Reference specific line numbers (e.g., `file.go:123` or `lines 45-67`)
- ❌ Use line number ranges - they become stale as code changes
- ❌ Duplicate RPC handler signatures when protobuf already defines the API
- ❌ Show function signatures for internal implementation details

**What to include in API Changes:**
- ✅ Protobuf messages and service definitions (the actual API contract)
- ✅ Database schema changes (migrations, new columns/tables)
- ✅ CLI command usage examples with expected output
- ✅ Configuration file changes
- ❌ Go handler function signatures (implementation detail)
- ❌ Internal database method signatures (implementation detail)

**Why avoid redundant signatures:**
- Protobuf already defines the API contract completely
