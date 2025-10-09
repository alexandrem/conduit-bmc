---
rfd: "002"
title: "OIDC Authentication Backend Support"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: true
api_changes: true
dependencies:
	- "github.com/coreos/go-oidc/v3"
	- "golang.org/x/oauth2"
database_migrations:
	- "add_oidc_columns_to_customers"
	- "create_oidc_sessions_table"
	- "create_oidc_states_table"
	- "create_oidc_providers_table"
areas: [ "manager", "cli" ]
---

# RFD 002 - OIDC Authentication Backend Support

**Status:** 🚧 Draft

## Summary

Add OpenID Connect (OIDC) backend support to the BMC Management System, enabling
authentication through popular identity providers like Google, GitHub, and Azure
AD while maintaining existing API key authentication for service-to-service
communication.

## Problem

The current authentication system has several limitations that impact enterprise
adoption and user experience:

### Current Authentication Issues

1. **Limited Enterprise Integration**: No support for corporate SSO systems
2. **Manual User Management**: Users must be manually provisioned in the system
3. **Password Fatigue**: Additional credentials for users to manage
4. **Limited MFA Support**: No integration with corporate MFA policies
5. **Audit Limitations**: Insufficient authentication audit trails
6. **Compliance Gaps**: Doesn't leverage existing identity compliance frameworks

### Current Flow

```
CLI → Manager (API key/password) → JWT token → Gateway → Agent → BMC
```

**Problems:**

- No integration with enterprise identity providers
- Manual user lifecycle management
- Limited security policy enforcement
- Poor user experience for human operators

## Solution

Implement a hybrid authentication model supporting both OIDC for human users and
API keys for service accounts, providing enterprise-grade authentication while
maintaining backward compatibility.

### Proposed Architecture

```
Human Users (Interactive): Browser → Manager OIDC page → Provider → Manager → JWT → CLI
Human Users (Headless):    CLI → Device flow → Provider → Manager → JWT → CLI
Service Accounts:          CLI → Manager (API key) → JWT → Gateway → Agent → BMC
```

**Key Design Decisions:**

- **Manager handles OIDC**: All OAuth callbacks and web flows handled by Manager
  service
- **CLI stays pure**: No embedded HTTP servers, follows Unix philosophy
- **Device flow for headless**: SSH sessions, automation scripts use OAuth
  device flow
- **Gateway unaware**: Gateway only validates JWTs, doesn't know about OIDC

**Benefits:**

- **Enterprise Integration**: Works with existing corporate identity systems
- **Enhanced Security**: Leverages provider MFA and security policies
- **Better UX**: Familiar login flows, no additional passwords
- **Centralized Management**: User management in existing identity systems
- **Compliance**: Better audit trails and policy enforcement
- **Scalability**: Automated user provisioning and deprovisioning
- **Architecture Compliance**: Maintains CLI as pure command-line tool

## Details

### Authentication Strategy Matrix

| User Type                 | Method      | Use Case              | Implementation                    |
|---------------------------|-------------|-----------------------|-----------------------------------|
| Human Users (Interactive) | OIDC Web    | Browser-based login   | Authorization Code Flow (Manager) |
| Human Users (Headless)    | OIDC Device | SSH sessions, CI/CD   | Device Authorization Grant        |
| Service Accounts          | API Keys    | Automation, scripts   | Existing system                   |
| Direct API Access         | API Keys    | External integrations | Existing system                   |

### High-Level Architecture

**Component Responsibilities:**

1. **Manager Service**
	- Hosts OIDC login web pages
	- Handles OAuth callbacks from providers
	- Manages OIDC provider configurations
	- Issues JWTs after successful authentication
	- Provides RPC endpoints for CLI polling

2. **CLI**
	- Opens browser to Manager OIDC login page
	- Polls Manager for authentication completion
	- Stores tokens securely (encrypted, file-based)
	- Supports device flow for headless environments

3. **Gateway**
	- Remains OIDC-unaware
	- Validates JWTs from Manager (existing functionality)
	- No changes required

### Database Schema

**New Tables:**

- `oidc_sessions` - Tracks OIDC authentication sessions
- `oidc_states` - CSRF protection for OAuth flows
- `oidc_providers` - Provider configurations

**Modified Tables:**

- `customers` - Add OIDC provider, subject, email verification fields

### Configuration

**Provider Configuration Example:**

```yaml
oidc:
	enabled: true
	redirect_url: "http://localhost:8080/auth/oidc/callback"
	providers:
		-   name: "google"
			display_name: "Google"
			client_id: "${GOOGLE_CLIENT_ID}"
			client_secret: "${GOOGLE_CLIENT_SECRET}"
			issuer_url: "https://accounts.google.com"
			scopes: [ "openid", "email", "profile" ]
			role_mapping:
				"@company.com": [ "user" ]
```

**Role Mapping:**

- Email domain-based (e.g., `@company.com` → `user` role)
- Group/organization-based (e.g., Azure AD groups, GitHub orgs)
- Provider-specific claims supported (Azure: `groups`, GitHub: `organizations`)

### CLI Commands

```bash
# Interactive authentication (opens Manager web page in browser)
bmc-cli auth login                           # Opens Manager, lists available providers
bmc-cli auth login --provider google         # Opens Manager with Google login

# Device flow (headless environments - SSH, automation)
bmc-cli auth login --device                  # Starts device flow, shows code
bmc-cli auth login --device --provider google

# Status and management
bmc-cli auth status                          # Show current authentication status
bmc-cli auth refresh                         # Refresh current tokens
bmc-cli auth logout                          # Logout and clear tokens
```

**Flow:**

1. CLI requests OIDC session from Manager
2. CLI opens browser to Manager's login page
3. User authenticates with provider
4. CLI polls Manager for completion
5. CLI stores encrypted tokens locally

## API Changes

### Manager HTTP Endpoints (Web UI)

**Authentication Pages:**

- `GET /auth/oidc/login?session={id}` - Provider selection page
- `GET /auth/oidc/{provider}/login?session={id}` - Initiate OAuth flow
- `GET /auth/oidc/{provider}/callback?code={code}&state={state}` - OAuth
  callback
- `GET /auth/oidc/success?session={id}` - Success page

### Manager RPC Endpoints (Connect)

**ListOIDCProviders**

```protobuf
rpc ListOIDCProviders(ListOIDCProvidersRequest) returns (ListOIDCProvidersResponse)

message ListOIDCProvidersResponse {
	repeated OIDCProvider providers = 1;
}

message OIDCProvider {
	string name = 1;
	string display_name = 2;
	bool enabled = 3;
}
```

**InitiateOIDCAuth** (Web Flow)

```protobuf
rpc InitiateOIDCAuth(InitiateOIDCAuthRequest) returns (InitiateOIDCAuthResponse)

message InitiateOIDCAuthRequest {
	string provider = 1;
}

message InitiateOIDCAuthResponse {
	string session_id = 1;
	string login_url = 2;  // Manager OIDC login page URL
}
```

**CheckOIDCAuthStatus** (Polling)

```protobuf
rpc CheckOIDCAuthStatus(CheckOIDCAuthStatusRequest) returns (CheckOIDCAuthStatusResponse)

message CheckOIDCAuthStatusRequest {
	string session_id = 1;
}

message CheckOIDCAuthStatusResponse {
	string status = 1;  // "pending", "completed", "expired", "failed"
	string access_token = 2;
	string refresh_token = 3;
	google.protobuf.Timestamp expires_at = 4;
	string message = 5;
}
```

**InitiateDeviceAuth** (Device Flow)

```protobuf
rpc InitiateDeviceAuth(InitiateDeviceAuthRequest) returns (InitiateDeviceAuthResponse)

message InitiateDeviceAuthRequest {
	string provider = 1;
}

message InitiateDeviceAuthResponse {
	string device_code = 1;
	string user_code = 2;
	string verification_uri = 3;
	int32 interval = 4;  // Polling interval in seconds
}
```

**PollDeviceAuth** (Device Flow Polling)

```protobuf
rpc PollDeviceAuth(PollDeviceAuthRequest) returns (PollDeviceAuthResponse)

message PollDeviceAuthRequest {
	string provider = 1;
	string device_code = 2;
}

message PollDeviceAuthResponse {
	string status = 1;  // "pending", "completed", "slow_down", "expired", "access_denied"
	string access_token = 2;
	string refresh_token = 3;
	google.protobuf.Timestamp expires_at = 4;
	string message = 5;
}
```

### Security Considerations

- **CSRF Protection**: State parameter validation prevents cross-site request
  forgery
- **Replay Protection**: Nonce validation prevents token replay attacks
- **Token Storage**: Refresh tokens encrypted at rest (CLI and Manager)
- **Token Lifetimes**: Short-lived ID tokens (1 hour), longer refresh tokens (30
  days)
- **HTTPS Only**: Production deployments enforce HTTPS for all OIDC flows

## Implementation Plan

### Phase 1: Database and Core Infrastructure

- [ ] Create database migrations for OIDC tables
	- [ ] Add OIDC columns to customers table
	- [ ] Create oidc_sessions table
	- [ ] Create oidc_states table (for CSRF protection)
	- [ ] Create oidc_providers table
- [ ] Implement OIDCConfig structure and YAML parsing
- [ ] Create OIDCService with provider management
	- [ ] Provider initialization and discovery
	- [ ] OAuth2 config setup per provider
	- [ ] ID token verifier setup
- [ ] Add database methods for OIDC state management
	- [ ] StoreOIDCState/ValidateOIDCState
	- [ ] StoreAuthResult/GetAuthResult (for CLI polling)

### Phase 2: Manager OIDC Implementation

- [ ] Create Manager web templates for OIDC login
	- [ ] oidc_login.html (provider selection page)
	- [ ] oidc_success.html (post-auth success page)
- [ ] Implement Manager HTTP handlers
	- [ ] handleOIDCLoginPage (GET /auth/oidc/login)
	- [ ] handleProviderLogin (GET /auth/oidc/{provider}/login)
	- [ ] handleProviderCallback (GET /auth/oidc/{provider}/callback)
	- [ ] handleOIDCSuccess (GET /auth/oidc/success)
- [ ] Implement Connect RPC endpoints
	- [ ] ListOIDCProviders
	- [ ] InitiateOIDCAuth (returns login URL for CLI)
	- [ ] CheckOIDCAuthStatus (polling endpoint for CLI)
	- [ ] InitiateDeviceAuth
	- [ ] PollDeviceAuth
- [ ] Add user claims extraction and role mapping
- [ ] Implement createOrUpdateCustomer for OIDC users

### Phase 3: CLI OIDC Integration

- [ ] Update CLI auth command structure
	- [ ] Add --provider flag
	- [ ] Add --device flag for headless flow
- [ ] Implement CLI OIDCAuthenticator
	- [ ] InteractiveLogin (browser-based flow)
	- [ ] DeviceFlowLogin (headless flow)
	- [ ] Browser opening utility
	- [ ] Status polling logic
- [ ] Implement FileTokenStore for secure token storage
	- [ ] Token encryption/decryption
	- [ ] File permissions (0600)
	- [ ] Per-provider token files
- [ ] Update auth status command to show OIDC provider
- [ ] Update auth refresh command for OIDC tokens

### Phase 4: Testing and Validation

- [ ] Unit tests
	- [ ] OIDCService.HandleCallback
	- [ ] Role mapping logic
	- [ ] Token encryption/decryption
	- [ ] State validation
- [ ] Integration tests
	- [ ] Mock OIDC provider setup
	- [ ] End-to-end web flow test
	- [ ] End-to-end device flow test
	- [ ] CLI polling test
- [ ] Security tests
	- [ ] CSRF protection (state parameter)
	- [ ] Token tampering detection
	- [ ] Role escalation prevention
	- [ ] Token expiration handling

### Phase 5: Provider Configuration and Documentation

- [ ] Create provider configuration templates
	- [ ] Google OIDC config example
	- [ ] GitHub OIDC config example
	- [ ] Azure AD config example
- [ ] Add OIDC configuration to development environment
- [ ] Create deployment documentation
	- [ ] Provider registration instructions
	- [ ] Role mapping examples
	- [ ] Troubleshooting guide
- [ ] Update user-facing documentation
	- [ ] CLI authentication guide
	- [ ] OIDC provider setup guide

### Phase 6: Deployment

- [ ] Add feature flag for OIDC (disabled by default)
- [ ] Deploy to development environment
- [ ] Configure mock OIDC provider for testing
- [ ] Gradual enablement strategy
	- [ ] Enable for pilot users
	- [ ] Monitor authentication metrics
	- [ ] Expand to broader user base

## Migration Strategy

The system will support multiple authentication methods simultaneously:

```go
// Manager supports hybrid authentication
type AuthHandler struct {
oidcService  *OIDCService
apiKeyAuth   *APIKeyAuth
passwordAuth *PasswordAuth
}

// Authenticate tries authentication methods in order: OIDC JWT, API key, password
func (h *AuthHandler) Authenticate(ctx context.Context, req *AuthRequest) (*AuthResponse, error)
```

**Authentication Priority:**

1. OIDC JWT validation (if token present)
2. API key authentication (if API key present)
3. Password authentication (if email/password present)

**Migration Path:**

1. Deploy with OIDC disabled (feature flag off)
2. Enable OIDC for pilot users
3. All authentication methods remain functional
4. No breaking changes to existing workflows

## Testing

### Test Strategy

1. **Unit Tests**
   ```go
   func TestOIDCService_HandleCallback(t *testing.T)
   func TestRoleMapping(t *testing.T)
   func TestTokenEncryption(t *testing.T)
   ```

2. **Integration Tests**
   ```go
   func TestE2EOIDCFlow(t *testing.T)
   func TestCLIDeviceFlow(t *testing.T)
   func TestProviderSpecificClaims(t *testing.T)
   ```

3. **Security Tests**
   ```go
   func TestStateValidation(t *testing.T)
   func TestTokenTampering(t *testing.T)
   func TestRoleEscalation(t *testing.T)
   ```

### Mock Providers

```go
// Mock provider for testing
type MockOIDCProvider struct {
claims map[string]interface{}
}
```

## Deployment

### Configuration Management

**Development:**

```yaml
oidc:
	enabled: true
	redirect_url: "http://localhost:8080/auth/oidc/callback"
	providers:
		-   name: "mock"
			display_name: "Mock Provider (Development)"
			# Mock provider configuration for testing
```

**Production:**

```yaml
oidc:
	enabled: true
	redirect_url: "https://bmc.company.com/auth/oidc/callback"
	session_store:
		type: "redis"
		redis_url: "${REDIS_URL}"
	providers:
		-   name: "company-azure"
			display_name: "Company SSO"
			client_id: "${AZURE_CLIENT_ID}"
			client_secret: "${AZURE_CLIENT_SECRET}"
			issuer_url: "https://login.microsoftonline.com/${COMPANY_TENANT_ID}/v2.0"
```

### Monitoring

```yaml
metrics:
	-   name: oidc_auth_attempts_total
		type: counter
		labels: [ provider, status ]
		description: Total OIDC authentication attempts

	-   name: oidc_token_refresh_total
		type: counter
		labels: [ provider, status ]
		description: Total OIDC token refreshes

	-   name: oidc_session_duration_seconds
		type: histogram
		labels: [ provider ]
		description: OIDC session duration
```

## Security

### Threat Model

- **Authorization Code Interception**: Mitigated by PKCE and short-lived codes
- **State Parameter Attacks**: Mitigated by cryptographically secure state
  generation
- **Token Leakage**: Mitigated by encryption and secure storage
- **Session Hijacking**: Mitigated by secure session management and HTTPS

### Compliance

- **SOC 2**: Enhanced audit logging and access controls
- **GDPR**: User consent and data minimization
- **HIPAA**: Role-based access and audit trails

## Alternatives Considered

### Alternative 1: SAML 2.0 Authentication

- **Pros:** Enterprise standard, mature protocol
- **Cons:** More complex than OIDC, XML-based, limited mobile support

### Alternative 2: Direct LDAP Integration

- **Pros:** Simple integration, widely supported
- **Cons:** Less secure, no modern features like MFA, requires VPN

### Alternative 3: Custom Authentication Service

- **Pros:** Full control, tailored features
- **Cons:** Development overhead, security risks, maintenance burden

## References

- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [OAuth 2.0 Security Best Practices](https://tools.ietf.org/html/draft-ietf-oauth-security-topics)
- [Device Authorization Grant](https://tools.ietf.org/html/rfc8628)
- [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636)
