---
rfd: "007"
title: "Persistent Session Management"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: true
api_changes: true
dependencies:
    - "github.com/redis/go-redis/v9"
database_migrations: []
areas: [ "gateway" ]
---

# RFD 007 - Persistent Session Management

**Status:** 🚧 Draft

## Summary

Add database-backed persistence to console sessions to enable recovery across
gateway
restarts, historical tracking, and better operational visibility.

## Problem

**Current behavior/limitations:**

- Gateway restart = all active sessions lost
- Users must manually recreate sessions after restarts
- No recovery from network interruptions
- Cannot query past session usage
- No audit trail of console access
- Cannot list active sessions across gateway instances
- No automatic cleanup of stale sessions

**Why this matters:**

- Poor user experience during maintenance windows or crashes
- Compliance and security auditing requirements unmet
- Missing capacity planning data
- Manual operational overhead for session cleanup

**Use cases affected:**

- Maintenance restarts lose all active console sessions
- Security audits cannot track console access history
- Operators cannot see which servers have active sessions
- Stale sessions consume resources indefinitely

## Solution

**Key Design Decisions:**

- **Single backend approach**: Use one session backend (Redis or in-memory),
  configurable per environment
- **Redis/DragonflyDB for production**: In-memory data store with persistence,
  native TTL support, multi-gateway HA ready
- **DragonflyDB preferred**: Drop-in Redis replacement with 25x better
  performance, 30% lower memory usage, and snapshot persistence
- **In-memory backend for development**: Local sessions without Redis dependency
  for dev/test environments
- **Pluggable architecture**: SessionBackend interface allows easy backend
  swapping
- **Automatic expiration**: Leverage Redis TTL for automatic session cleanup (no
  background jobs)
- **Single source of truth**: No dual writes, no sync complexity, stronger
  consistency

**Benefits:**

- **Session Recovery**: Users don't lose sessions on gateway restart
- **Native HA Support**: Redis cluster enables multi-gateway session sharing
  out-of-the-box
- **Operational Visibility**: List and manage sessions across instances using
  Redis queries
- **Automatic Cleanup**: Redis TTL automatically removes expired sessions
- **Performance**: Sub-millisecond latency for session operations
- **Audit Trail**: Redis Streams for session event logging

**Architecture Overview:**

```
Production (Redis Backend):
User → Gateway → [Redis/DragonflyDB] → SET session:{id} EX {ttl}
                       ↓
                  Active Session (in Redis)
                       ↓
Gateway Restart → [Redis/DragonflyDB] → Validate → Sessions Recovered
                        ↓
                  (TTL auto-cleanup)

Development (Memory Backend):
User → Gateway → [In-Memory Map] → Active Session
                       ↓
Gateway Restart → (sessions lost, no persistence)
```

### Component Changes

**1. Gateway (gateway/internal/gateway/handler.go):**

- Replace in-memory map with pluggable SessionBackend interface
- Backend choice based on configuration (redis or memory)
- Startup recovery: If Redis backend, scan for active sessions and validate
- No cleanup needed: Redis TTL handles expiration, memory backend uses expiration
  checks

**2. Gateway Session Backend (gateway/pkg/session/backend.go):**

- SessionBackend interface for pluggable storage
- RedisBackend implementation: CRUD with secondary indexes, TTL, audit streams
- MemoryBackend implementation: Simple map with mutex for dev/test
- Factory pattern: Create backend based on config
- Connection pooling and automatic failover (Redis only)

**Configuration Example:**

```yaml
# gateway/config.yaml
session:
    backend: redis  # or "memory" for development/testing
    ttl: 4h
    closed_ttl: 720h  # Keep closed sessions for 30 days (audit)

# Redis config (only used when backend=redis)
redis:
    # DragonflyDB is Redis-compatible, use same config
    endpoints:
        - "dragonfly-01.example.com:6379"
        - "dragonfly-02.example.com:6379"
    password: "${REDIS_PASSWORD}"
    db: 0
    pool_size: 20
    enable_streams: true  # Enable audit trail logging
```

## Implementation Plan

### Phase 1: Backend Interface

- [ ] Define SessionBackend interface with CRUD operations
- [ ] Implement MemoryBackend (simple map-based, for dev/test)
- [ ] Implement RedisBackend with DragonflyDB support
- [ ] Add backend factory based on configuration
- [ ] Add unit tests for both backends with mocks

### Phase 2: Gateway Integration

- [ ] Replace in-memory session map with SessionBackend
- [ ] Update session creation to use backend directly
- [ ] Update session closure to use backend
- [ ] Implement session recovery on startup (Redis backend only)
- [ ] Add graceful shutdown for backend connections

### Phase 3: Operational Features

- [ ] Add Redis Streams for session audit trail (RedisBackend only)
- [ ] Add session validation during recovery
- [ ] Add backend health metrics and monitoring
- [ ] Add secondary indexes for customer/server queries (RedisBackend)
- [ ] Add configuration validation and backend selection logic
- [ ] Update deployment documentation with backend options

### Phase 4: Testing & Documentation

- [ ] Unit tests: MemoryBackend and RedisBackend with mocks (miniredis)
- [ ] Integration tests: Session recovery with RedisBackend
- [ ] Integration tests: Concurrent operations on both backends
- [ ] Integration tests: Multi-gateway session sharing with Redis cluster
- [ ] E2E tests: Full session lifecycle with both backends
- [ ] Load tests: 10,000+ concurrent sessions (Redis), 1,000+ (Memory)
- [ ] Update operational runbooks with backend configuration guide

## API Changes

### Redis Data Model

**Session Storage (Redis Hash):**

```
Key: session:{session_id}
TTL: {session_ttl} (e.g., 4 hours for active, 30 days for closed)

Fields:
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "server_id": "server-001",
  "customer_id": "acme-corp",
  "agent_id": "agent-dc1-rack1",
  "bmc_endpoint": "http://192.168.1.100",
  "session_type": "vnc",
  "websocket_endpoint": "ws://gateway-01:8080/ws/vnc/a1b2c3d4...",
  "created_at": "2025-10-09T14:30:00Z",
  "last_activity": "2025-10-09T16:42:15Z",
  "expires_at": "2025-10-09T18:30:00Z",
  "closed_at": null
}
```

**Secondary Indexes (Redis Sets):**

```
# Customer index
Key: sessions:customer:{customer_id}
Members: [session_id1, session_id2, ...]
TTL: None (cleaned when session expires)

# Server index
Key: sessions:server:{server_id}
Members: [session_id1, session_id2, ...]
TTL: None (cleaned when session expires)

# Active sessions
Key: sessions:active
Members: [session_id1, session_id2, ...]
TTL: None (cleaned when session expires)
```

**Audit Trail (Redis Streams - Optional):**

```
Key: sessions:audit
Entries:
{
  "event": "session_created",
  "session_id": "a1b2c3d4...",
  "customer_id": "acme-corp",
  "timestamp": "2025-10-09T14:30:00Z"
}
{
  "event": "session_closed",
  "session_id": "a1b2c3d4...",
  "reason": "user_disconnect",
  "timestamp": "2025-10-09T16:45:00Z"
}
```

### Configuration Changes

New configuration section in `gateway/config.yaml`:

- `session.backend`: Backend type - "redis" (production) or "memory" (dev/test)
- `session.ttl`: Active session TTL (default: 4h)
- `session.closed_ttl`: Closed session retention for audit (default: 720h/30
  days)
- `redis.endpoints`: List of Redis/DragonflyDB endpoints for HA (when
  backend=redis)
- `redis.password`: Redis password (use env var for security)
- `redis.db`: Redis database number (default: 0)
- `redis.pool_size`: Connection pool size (default: 20)
- `redis.enable_streams`: Enable audit trail logging (default: false)

## Migration Strategy

**Deployment Steps:**

1. **Choose backend**: Set `session.backend=redis` for production, `memory` for
   dev
2. **For Redis backend**: Deploy DragonflyDB or Redis instance (local or cluster)
3. Deploy new gateway version with backend configuration
4. Gateway automatically connects to configured backend on startup
5. **No breaking changes**: Existing deployments default to memory backend (same
   as before)
6. Sessions created after upgrade use configured backend
7. On restart: Redis backend recovers sessions, memory backend loses them (
   expected)

**Redis/DragonflyDB Setup:**

Development (Docker Compose):

```yaml
services:
  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly:latest
    ports:
      - "6379:6379"
    volumes:
      - dragonfly-data:/data
    command: >
      --snapshot_cron "*/30 * * * *"
      --dbfilename sessions.dfs
```

Production (Kubernetes):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dragonfly
spec:
  serviceName: dragonfly
  replicas: 2
  template:
    spec:
      containers:
        - name: dragonfly
          image: docker.dragonflydb.io/dragonflydb/dragonfly:latest
          args:
            - --replication=true
            - --snapshot_cron=*/30 * * * *
          volumeMounts:
            - name: data
              mountPath: /data
```

**Rollback Plan:**

1. Change `session.backend=memory` in config (instant rollback, no deploy)
2. Or revert to previous gateway version
3. Redis data remains intact for re-enabling Redis backend
4. No data migration needed - backends are independent

**Operational Considerations:**

- DragonflyDB recommended over Redis for better performance and lower memory
- Enable snapshotting for persistence across Redis restarts
- Redis cluster for multi-gateway HA support
- Monitor Redis memory usage (automatic eviction with TTL)
- No manual cleanup needed - Redis TTL handles expiration
- Backup Redis snapshots for audit trail retention

## Testing Strategy

### Unit Tests

- MemoryBackend CRUD operations with concurrent access
- RedisBackend CRUD operations with miniredis mock
- TTL expiration validation for both backends
- Redis connection error handling and retry logic
- Backend factory and configuration parsing
- Secondary index updates (RedisBackend)

### Integration Tests

- RedisBackend: Create session → restart gateway → verify recovery
- RedisBackend: Wait for TTL expiration → verify auto-cleanup
- MemoryBackend: Restart gateway → verify sessions not recovered (expected)
- Concurrent session creation: 10,000+ (Redis), 1,000+ (Memory)
- Backend switch: Memory to Redis → existing sessions migrate
- Multi-gateway session sharing via Redis cluster

### E2E Tests

- RedisBackend: Full VNC session lifecycle with gateway restart and recovery
- RedisBackend: Full SOL session lifecycle with gateway restart and recovery
- MemoryBackend: Full session lifecycle without persistence (dev workflow)
- Session expiration: create → wait past TTL → verify cleanup (both backends)
- Multi-gateway (RedisBackend): create on gateway-1 → list from gateway-2
- Backend switching: Start with memory → switch to Redis → verify persistence

## Security Considerations

- **Redis Authentication**: Require password authentication for Redis access
- **TLS Encryption**: Enable TLS for Redis connections in production
- **Network Isolation**: Redis accessible only from gateway instances (network
  policy)
- **Audit Trail**: Redis Streams provide immutable append-only log of session
  events
- **Data Retention**: TTL-based retention balances audit requirements with
  storage
- **No Credential Storage**: Redis contains session metadata only, no BMC
  credentials
- **ACLs**: Use Redis ACLs to restrict gateway to session namespace only

## Future Enhancements

- **Admin API for cross-gateway session management**: Manager aggregates sessions
  from all Gateways, provides unified admin API for listing/closing sessions
  across the entire system (requires separate RFD for Manager-Gateway query
  protocol)
- Session recording and playback using Redis Streams
- Per-customer session limits using Redis counters with INCR/DECR
- Real-time session metrics dashboard with Redis pub/sub
- Session transfer between gateway instances (already supported via shared Redis)
- Automatic session termination on user logout
- Geo-distributed Redis with conflict-free replication (CRDT support in
  DragonflyDB)

## Appendix

### Session Backend Interface Reference

```go
// gateway/pkg/session/backend.go
type SessionBackend interface {
    // Core CRUD operations
    Save(ctx context.Context, session *ConsoleSession, ttl time.Duration) error
    Get(ctx context.Context, sessionID string) (*ConsoleSession, error)
    Update(ctx context.Context, session *ConsoleSession) error
    Delete(ctx context.Context, sessionID string) error

    // Query operations
    ListActive(ctx context.Context) ([]*ConsoleSession, error)
    ListByCustomer(ctx context.Context, customerID string) ([]*ConsoleSession, error)
    ListByServer(ctx context.Context, serverID string) ([]*ConsoleSession, error)

    // TTL operations
    RefreshTTL(ctx context.Context, sessionID string, ttl time.Duration) error
    GetTTL(ctx context.Context, sessionID string) (time.Duration, error)

    // Audit operations (optional, implementation-specific)
    LogEvent(ctx context.Context, event SessionEvent) error
    GetAuditLog(ctx context.Context, sessionID string) ([]SessionEvent, error)

    // Lifecycle
    Close() error
}

// Implementations:
// - RedisBackend: Full persistence, TTL, audit streams, multi-gateway
// - MemoryBackend: No persistence, TTL via expiration checks, no audit
```

### Redis Commands Used

**Session Operations:**

```bash
# Create session
HSET session:{id} field1 value1 field2 value2 ...
EXPIRE session:{id} 14400  # 4 hours
SADD sessions:active {id}
SADD sessions:customer:{customer_id} {id}
SADD sessions:server:{server_id} {id}

# Get session
HGETALL session:{id}

# Update session
HSET session:{id} last_activity {timestamp}

# Close session (mark closed, keep for audit)
HSET session:{id} closed_at {timestamp}
EXPIRE session:{id} 2592000  # 30 days
SREM sessions:active {id}

# List by customer
SMEMBERS sessions:customer:{customer_id}
# Then HGETALL for each session

# Scan all active
SSCAN sessions:active 0
```

**DragonflyDB Benefits:**

- **Performance**: 25x faster than Redis for some workloads
- **Memory**: 30% less memory usage vs Redis
- **Vertical Scaling**: Better multi-core utilization
- **Snapshot Persistence**: Built-in point-in-time snapshots
- **Redis Compatible**: Drop-in replacement, no code changes
