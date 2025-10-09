---
rfd: "001"
title: "Encrypted Server Information in JWT Tokens"
state: "implemented"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: true
areas: ["gateway", "manager", "authentication"]
---

# RFD 001 - Encrypted Server Information in JWT Tokens

**Status:** 🎉 Implemented

## Summary

Improve the current server ID handling in the gateway by embedding encrypted BMC
host endpoint information directly in delegated JWT tokens created by the
manager. This eliminates the need for the gateway to perform server ID lookups
and makes the system more stateless and performant.

## Problem

The current server ID handling system has architectural limitations that create
a disconnect between manager server concepts and gateway BMC operations:

### Current Architecture Issues

```
CLI → Manager (auth + server-to-BMC resolution) → JWT token with server ID → Gateway (TODO: resolve server ID) → Agent → BMC
```

**Problems:**

1. **Incomplete Resolution**: Gateway receives server IDs but has no mechanism
   to resolve them to BMC endpoints
2. **Manager Coupling**: Manager must resolve server IDs to BMC endpoints, but
   this information isn't passed to gateway
3. **Placeholder Implementation**: Gateway currently uses server ID as BMC
   endpoint placeholder
4. **Architecture Mismatch**: Manager works with server concepts while gateway
   works with BMC endpoints
5. **Missing Information**: Gateway needs BMC endpoint details (type, features,
   datacenter) for proper routing

### Current Flow Limitations

```
Gateway receives server IDs but has no resolution mechanism:
- Manager resolves server ID → BMC endpoint
- This information isn't passed to Gateway
- Gateway uses server ID as placeholder for BMC endpoint
```

## Solution

Implement encrypted server context tokens that embed BMC endpoint information
directly in JWT tokens, making the gateway stateless and eliminating server
lookup overhead.

### Proposed Architecture

```
CLI → Manager (auth + server-to-BMC resolution + encryption) → Self-contained JWT with BMC context → Gateway (decrypt + direct BMC operations) → Agent → BMC
```

**Benefits:**

-   **Complete Resolution**: Gateway receives BMC endpoint information directly
    in JWT token
-   **Proper Separation**: Manager handles server concepts, gateway handles BMC
    operations with embedded context
-   **Enhanced Routing**: Gateway gets BMC type, features, and datacenter
    information for optimal routing
-   **Stateless Operations**: Gateway doesn't need to maintain server-to-BMC
    mapping state
-   **Enhanced Security**: BMC endpoint information is encrypted and
    tamper-proof

## Details

### Server Context Token Structure

**Embedded Server Context:**
```go
type ServerContext struct {
    ServerID      string    `json:"server_id"`
    CustomerID    string    `json:"customer_id"`
    BMCEndpoint   string    `json:"bmc_endpoint"`    // Actual BMC address
    BMCType       string    `json:"bmc_type"`        // "ipmi", "redfish"
    Features      []string  `json:"features"`        // Supported features
    DatacenterID  string    `json:"datacenter_id"`   // For routing
    Permissions   []string  `json:"permissions"`     // Server-specific perms
    ExpiresAt     time.Time `json:"exp"`
}
```

**JWT Structure:**
```go
type EncryptedJWT struct {
    jwt.StandardClaims
    CustomerID    string `json:"customer_id"`
    Email         string `json:"email"`
    ServerContext string `json:"server_context,omitempty"`  // AES-256-GCM encrypted
}
```

### High-Level Flow

**Token Generation (Manager):**
1. Customer authenticates with Manager
2. Manager validates customer owns requested server
3. Manager looks up server → BMC endpoint mapping
4. Manager creates ServerContext with BMC details
5. Manager encrypts ServerContext using AES-256-GCM
6. Manager embeds encrypted context in JWT
7. Returns JWT to client

**Token Usage (Gateway):**
1. Gateway receives JWT from client request
2. Gateway validates JWT signature
3. Gateway decrypts ServerContext from JWT
4. Gateway validates ServerContext expiration
5. Gateway extracts BMC endpoint from context
6. Gateway routes request to Agent using BMC endpoint

### CLI Token Caching

**Flow:**
- CLI caches server-specific tokens (per server ID)
- Reuses cached tokens until expiration
- Requests new tokens from Manager when expired
- Eliminates repeated Manager calls for same server

## API Changes

### Manager RPC Endpoint

```protobuf
rpc GetServerToken(GetServerTokenRequest) returns (GetServerTokenResponse)

message GetServerTokenRequest {
  string server_id = 1;
}

message GetServerTokenResponse {
  string token = 1;
  google.protobuf.Timestamp expires_at = 2;
}
```

### Security Considerations

**Encryption:**
- AES-256-GCM for server context encryption
- Unique nonce per encryption
- Token expiration: 1 hour (configurable)

**Permissions:**
- Fine-grained per-server permissions
- Permission validation at Gateway
- Support for read/write/admin scopes

## Security

### Threat Model

- **Token Interception**: Mitigated by encryption and short expiration
- **Key Compromise**: Mitigated by key rotation and monitoring
- **Permission Escalation**: Mitigated by fine-grained permission model
- **Replay Attacks**: Mitigated by nonce and expiration

### Key Management

- Hardware security modules (HSMs) for production key storage
- Automated key rotation (30-day intervals)
- Multiple active keys during rotation periods
- Audit logging for all key operations

## Alternatives Considered

### Alternative 1: Server ID Caching

- **Pros:** Simple to implement, backward compatible
- **Cons:** Still requires initial lookups, cache invalidation complexity

### Alternative 2: Gateway Database Replication

- **Pros:** Eliminates Manager calls, maintains current API
- **Cons:** Database complexity, consistency challenges, higher resource usage

### Alternative 3: Service Mesh with Caching

- **Pros:** Transparent caching, service discovery benefits
- **Cons:** Additional infrastructure complexity, vendor lock-in

## References

-   [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
-   [JSON Web Encryption (JWE)](https://tools.ietf.org/html/rfc7516)
-   [AES-GCM Security Guidelines](https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf)
