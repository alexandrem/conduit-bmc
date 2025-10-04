# Authentication Architecture

This document describes the complete authentication and authorization flow for the BMC management system, from initial login through web console access.

**Current Authentication Methods**:
- Email/Password (partial)
- API Keys (partial)
- **OIDC/OAuth2** (future - see `docs/features/002-oidc-authentication.md`)

## Architecture Overview

```
                    ┌──────────────┐
                    │   Manager    │
                    │  (Auth/JWT)  │
                    └──────────────┘
                           ▲
                           │ 1. Authenticate
                           │    (email/password)
                           │ 2. Receive JWT
┌─────────────┐            │
│     CLI     │────────────┘
│             │
│             │────────────┐
└─────────────┘            │ 3. CreateSession
                           │    (with JWT)
                           │ 4. Receive session URL
                           ▼
                    ┌──────────────┐
                    │   Gateway    │
                    │  (Session +  │
                    │   Cookie)    │
                    └──────────────┘
                           │
                           │ 5. Open URL
                           │    in browser
                           ▼
                    ┌─────────────┐
                    │   Browser   │
                    │ (Web Console)│
                    └─────────────┘
                           │
                           │ 6. Use cookie
                           │    for all ops
                           ▼
                    (Power ops, WebSocket, etc.)
```

## Complete Authentication Flow

### Step 1: Manager Authentication

**CLI → Manager**

```
CLI → Manager.Authenticate(email, password)
├─ Manager validates credentials against database
├─ Manager checks customer permissions for requested server
├─ Manager issues JWT token with encrypted server context:
│   ├─ Customer ID
│   ├─ Server ID and BMC endpoint
│   ├─ Permissions (console:access, power:control, etc.)
│   └─ Token expiration (e.g., 1 hour)
└─ Returns: {access_token: "eyJ...", refresh_token: "..."}
```

**JWT Token Structure**:
```json
{
  "customer_id": "customer@example.com",
  "server_id": "server-001",
  "bmc_endpoint": "192.168.1.100:623",
  "permissions": [
    "console:access",
    "console:write",
    "power:control",
    "vnc:access"
  ],
  "iat": 1633024800,
  "exp": 1633028400
}
```

**Manager Responsibilities**:
- Validate customer credentials:
  - Email/Password authentication (implemented)
  - API Key authentication (implemented)
  - OIDC/OAuth2 authentication (coming soon)
- Verify customer owns the requested server
- Check customer has permissions for the requested operation
- Issue JWT with encrypted server context
- Track token issuance for audit/security

### Step 2: Gateway Session Creation

**CLI → Gateway**

```
CLI → Gateway.CreateSOLSession(server_id, JWT)
├─ Gateway validates JWT signature and expiration
├─ Gateway extracts server context from JWT
├─ Gateway creates console session (sol-123)
├─ Gateway creates web session:
│   ├─ Generates secure random session ID (256-bit)
│   ├─ Stores mapping: session_id → JWT
│   ├─ Sets TokenExpiresAt from JWT claims
│   ├─ Calculates TokenRenewalAt (80% of JWT TTL)
│   └─ Associates with console session
└─ Returns: {session_id: "sol-123", console_url: "http://gateway/console/sol-123"}
```

**Web Session Storage**:
```go
WebSession {
    ID:             "secure-random-256-bit"  // Session cookie ID
    SOLSessionID:   "sol-123..."             // Associated SOL session
    VNCSessionID:   "vnc-456..."             // Associated VNC session
    CustomerJWT:    "eyJ..."                 // Manager-issued JWT token
    ExpiresAt:      time.Time                // 24 hours from creation
    TokenExpiresAt: time.Time                // JWT expiration
    TokenRenewalAt: time.Time                // When to renew (80% of TTL)
    CustomerID:     "customer@example.com"
    ServerID:       "server-001"
}
```

**Gateway Responsibilities**:
- Validate JWT signature and expiration
- Extract server context from JWT (no manager lookup needed)
- Create console/VNC sessions
- Create web sessions mapping cookies to JWTs
- Handle token renewal (future: Phase 2)

### Step 3: Browser Session Cookie

**Browser → Gateway**

```
Browser → GET /console/sol-123
├─ Gateway looks up web session by console session ID
├─ Gateway creates session cookie:
│   ├─ Name: "gateway_session"
│   ├─ Value: web_session.ID (random 256-bit string)
│   ├─ HttpOnly: true (prevents JavaScript access)
│   ├─ Secure: auto-detected from request (HTTP/HTTPS)
│   ├─ SameSite: Strict/Lax based on HTTP/HTTPS
│   └─ MaxAge: 86400 (24 hours)
├─ Browser stores cookie automatically
└─ Gateway renders HTML viewer (JWT never exposed)
```

**Cookie Configuration**:

```go
// HTTP (local development)
Cookie {
    Name:     "gateway_session"
    Secure:   false              // Allow over HTTP
    SameSite: Lax                // Permissive for localhost
    HttpOnly: true               // Always prevent XSS
    MaxAge:   86400              // 24 hours
}

// HTTPS (production)
Cookie {
    Name:     "gateway_session"
    Secure:   true               // HTTPS only
    SameSite: Strict             // Maximum CSRF protection
    HttpOnly: true               // Prevent XSS
    MaxAge:   86400              // 24 hours
}
```

**Auto-Detection**:
- Gateway detects HTTP vs HTTPS from request (`r.TLS`, `X-Forwarded-Proto`)
- Cookie security automatically adapts to environment
- No configuration needed - works in dev and production

### Step 4: Authenticated Operations

**Browser → Gateway (Power Operations)**

```
Browser → POST /api/servers/srv-001/power/on
├─ Browser automatically sends cookie: "gateway_session=abc123"
├─ Gateway extracts session ID from cookie
├─ Gateway looks up web session by ID
├─ Gateway retrieves JWT from web session
├─ Gateway validates JWT is still valid
├─ Gateway extracts permissions from JWT
├─ Gateway checks "power:control" permission
├─ Gateway uses server context to perform operation
├─ Gateway updates session activity timestamp
└─ Returns: {success: true, message: "Power on complete"}
```

**Dual Authentication Support**:

The gateway supports two authentication methods:

1. **Cookie-based (primary)** - For browser web consoles
   - Session cookie sent automatically by browser
   - JWT retrieved from server-side web session
   - No token exposure to JavaScript

2. **Header-based (fallback)** - For CLI/API direct access
   - `Authorization: Bearer <JWT>` header
   - Direct JWT validation
   - Used for programmatic access

```go
// getJWTFromRequest tries both methods
func getJWTFromRequest(r *http.Request) (string, error) {
    // Try session cookie first
    if sessionID, err := GetSessionIDFromCookie(r); err == nil {
        if webSession, err := sessionStore.Get(sessionID); err == nil {
            sessionStore.UpdateActivity(sessionID)
            return webSession.CustomerJWT, nil
        }
    }

    // Fallback to Authorization header
    if authHeader := r.Header.Get("Authorization"); authHeader != "" {
        return ExtractJWTFromAuthHeader(authHeader)
    }

    return "", errors.New("no authentication provided")
}
```

### Step 5: WebSocket Connection

**Browser → Gateway (Console Streaming)**

```
Browser → WS /console/sol-123/ws
├─ Browser automatically sends cookie with upgrade request
├─ (Phase 3) Gateway validates session via cookie
├─ Gateway retrieves JWT from web session
├─ Gateway validates permissions for console access
├─ Gateway establishes authenticated WebSocket
└─ Bidirectional streaming for console I/O
```

**Current State**: WebSocket connections don't yet validate session cookies (Phase 3 work)
**Future**: WebSocket handlers will use session middleware for cookie validation

## Security Model

### Token Isolation

**JWT Never Exposed to Browser**:
- ✅ JWT stored server-side in web session
- ✅ Browser only sees opaque session cookie
- ✅ JavaScript cannot access JWT (HttpOnly cookie)
- ✅ XSS attacks cannot steal JWT tokens

### Multi-Layer Authentication

**Defense in Depth**:

1. **Manager Layer**:
   - Validates customer credentials
   - Checks customer owns requested server
   - Verifies customer has required permissions
   - Issues JWT with encrypted server context

2. **Gateway Layer**:
   - Validates JWT signature and expiration
   - Creates session with JWT mapping
   - Sets secure session cookie
   - Validates permissions on every request

3. **Session Layer**:
   - Cookie provides session ID
   - Session maps to JWT server-side
   - Activity tracking for idle timeout
   - Automatic cleanup of expired sessions

### Cookie Security

**HttpOnly Protection**:
- Prevents JavaScript access to cookie value
- Mitigates XSS attacks attempting to steal session
- Even if attacker injects JavaScript, cannot read cookie

**Secure Flag (Production)**:
- Cookie only sent over HTTPS connections
- Prevents man-in-the-middle attacks on HTTP
- Auto-detected based on request scheme

**SameSite Protection**:
- `Strict` mode in production (HTTPS)
  - Cookie not sent on cross-site requests
  - Maximum CSRF protection
- `Lax` mode in development (HTTP)
  - More permissive for localhost testing
  - Still provides basic CSRF protection

**Short-Lived Sessions**:
- 24-hour maximum session lifetime
- JWT tokens expire sooner (e.g., 1 hour)
- Token renewal planned for Phase 2
- Activity tracking for idle timeout

### Permission Model

**Server-Specific Permissions**:

JWT tokens are scoped to specific servers with granular permissions:

```json
{
  "server_id": "server-001",
  "permissions": [
    "console:access",    // Can view console
    "console:write",     // Can send input to console
    "power:control",     // Can control power state
    "vnc:access",        // Can access VNC viewer
    "bios:access",       // Can access BIOS settings
    "media:mount"        // Can mount virtual media
  ]
}
```

**Permission Checking**:
- Gateway validates permissions on every operation
- Operations fail with 403 Forbidden if permission missing
- Audit log records all permission checks (future)

## Session Lifecycle

### Session Creation

```
1. CLI authenticates with Manager → JWT issued
2. CLI creates session with Gateway → Web session created
3. Gateway stores: session_id → JWT mapping
4. Gateway calculates token renewal time (80% of JWT TTL)
```

### Session Usage

```
1. Browser loads viewer → Session cookie set
2. Browser makes requests → Cookie sent automatically
3. Gateway validates session → Retrieves JWT
4. Gateway checks permissions → Authorizes operation
5. Gateway updates last activity timestamp
```

### Session Expiration

**Multiple Expiration Triggers**:

1. **Session Timeout** (24 hours)
   - Web session exceeds MaxAge
   - Session deleted by cleanup worker

2. **Token Expiration**
   - JWT token expires (e.g., 1 hour)
   - Operations fail with 401 Unauthorized
   - (Phase 2) Auto-renewal before expiration

3. **Idle Timeout** (future)
   - No activity for extended period
   - Session marked as expired
   - User must re-authenticate

4. **Explicit Logout** (future)
   - User clicks logout
   - Session immediately invalidated
   - Cookie deleted

### Session Cleanup

**Automatic Cleanup Worker**:
```go
// Runs every 5 minutes
func (s *InMemoryStore) cleanupWorker() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        now := time.Now()
        for id, session := range s.sessions {
            if now.After(session.ExpiresAt) {
                delete(s.sessions, id)
                log.Info().Str("session_id", id).Msg("Cleaned up expired session")
            }
        }
    }
}
```

## Token Renewal (Future: Phase 2)

### Current Limitations

- JWT tokens expire (e.g., 1 hour)
- Web sessions last 24 hours
- Long console sessions will fail when token expires
- User must re-authenticate and create new session

### Planned Solution

**Automatic Token Renewal**:

```
Gateway Background Worker:
├─ Every 1 minute, check sessions needing renewal
├─ For each session where now > TokenRenewalAt:
│   ├─ Call Manager.RefreshToken(refresh_token)
│   ├─ Receive new JWT with extended expiration
│   ├─ Update web session with new JWT
│   ├─ Recalculate TokenRenewalAt (80% of new TTL)
│   └─ Log successful renewal
└─ Sessions continue without interruption
```

**Token Renewal Timeline**:
```
JWT issued: 00:00 (expires at 01:00)
├─ 00:00 - 00:48: Normal operation
├─ 00:48: TokenRenewalAt reached (80% of 60min)
│   └─ Background worker renews token
├─ New JWT issued (expires at 01:48)
├─ 00:48 - 01:26: Normal operation continues
├─ 01:26: Next renewal (80% of new 60min)
└─ Process repeats for 24-hour session lifetime
```

**Benefits**:
- Long console sessions don't get interrupted
- Security maintained with short-lived tokens
- Automatic renewal transparent to user
- Can revoke refresh tokens to force re-authentication

See `docs/features/005-session-management.md` for complete design.

## API Endpoints

### Manager Endpoints

```
POST /manager.v1.BMCManagerService/Authenticate
  Request: {email, password}
  Response: {access_token, refresh_token, expires_at, customer}

POST /manager.v1.BMCManagerService/RefreshToken
  Request: {refresh_token, server_id}
  Response: {access_token, expires_at}

POST /manager.v1.BMCManagerService/GetServerToken
  Request: {server_id}
  Response: {token, expires_at}
  Note: Server-specific token with encrypted context
```

### Gateway Endpoints

**Session Creation**:
```
POST /gateway.v1.GatewayService/CreateSOLSession
  Headers: Authorization: Bearer <JWT>
  Request: {server_id}
  Response: {session_id, websocket_endpoint, console_url, expires_at}
  Sets: Set-Cookie: gateway_session=<session_id>

POST /gateway.v1.GatewayService/CreateVNCSession
  Headers: Authorization: Bearer <JWT>
  Request: {server_id}
  Response: {session_id, websocket_endpoint, viewer_url, expires_at}
  Sets: Set-Cookie: gateway_session=<session_id>
```

**Power Operations** (Cookie or Header Auth):
```
POST /api/servers/{serverId}/power/on
POST /api/servers/{serverId}/power/off
POST /api/servers/{serverId}/power/reset
POST /api/servers/{serverId}/power/cycle
GET  /api/servers/{serverId}/power/status
```

**Viewer Pages** (Sets Session Cookie):
```
GET /console/{sessionId}  → Renders SOL console HTML
GET /vnc/{sessionId}      → Renders VNC viewer HTML
```

**WebSocket Endpoints**:
```
WS /console/{sessionId}/ws  → SOL console data stream
WS /vnc/{sessionId}/ws      → VNC RFB protocol stream
```

## CLI Usage Examples

### Complete Flow Example

```bash
# Step 1: Authenticate with Manager (implicit - CLI does this automatically)
$ bmc-cli server console server-001

# Behind the scenes:
# CLI → Manager.Authenticate(email, password)
# Manager → {access_token: "eyJ...", refresh_token: "..."}

# Step 2: Create console session (CLI does this)
# CLI → Gateway.CreateSOLSession(server-001, JWT)
# Gateway → {session_id: "sol-123", console_url: "http://gateway/console/sol-123"}

# Step 3: Open browser (CLI does this)
# Opens: http://localhost:8081/console/sol-123
# Browser → Gateway sets cookie: gateway_session=abc123

# Step 4: Browser uses cookie for all operations
# Power on: POST /api/servers/server-001/power/on (cookie sent automatically)
# WebSocket: WS /console/sol-123/ws (cookie sent automatically)
```

### Direct API Access

```bash
# Authenticate and get token
$ curl -X POST http://localhost:8080/manager.v1.BMCManagerService/Authenticate \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}'

# Response: {"access_token": "eyJ...", "refresh_token": "..."}

# Use token for operations (no cookie needed)
$ curl -X POST http://localhost:8081/api/servers/server-001/power/on \
  -H "Authorization: Bearer eyJ..."

# Response: {"success": true, "message": "Power on complete"}
```

## Security Best Practices

### Development Environment

**Local Development** (HTTP):
- ✅ Use `Secure: false` cookies (auto-detected)
- ✅ Use `SameSite: Lax` for testing
- ✅ Still use HttpOnly for XSS protection
- ⚠️ Never use production credentials locally
- ⚠️ Use separate JWT secret for dev

### Production Environment

**Production Deployment** (HTTPS):
- ✅ Use `Secure: true` cookies (auto-detected)
- ✅ Use `SameSite: Strict` for CSRF protection
- ✅ Always use HTTPS for all connections
- ✅ Use strong JWT secret (256-bit minimum)
- ✅ Enable audit logging for all auth events
- ✅ Monitor for unusual session patterns
- ✅ Implement rate limiting on auth endpoints

### Token Management

**JWT Secrets**:
- Generate cryptographically random secrets
- Rotate secrets periodically
- Use different secrets for dev/staging/prod
- Store secrets securely (environment variables, secret manager)

**Token Expiration**:
- Short-lived access tokens (1 hour recommended)
- Longer refresh tokens (30 days recommended)
- Implement token renewal before expiration
- Revoke refresh tokens on suspicious activity

## Related Documentation

- **Web Console**: [docs/WEB.md](./WEB.md) - Web UI features and user experience
- **Session Management**: [docs/features/013-session-management.md](./features/013-session-management.md) - Detailed session architecture and token renewal
- **Architecture**: [docs/ARCHITECTURE.md](./ARCHITECTURE.md) - Overall system design and component interaction
