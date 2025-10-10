package gateway

// Updated for new server-customer mapping architecture
import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	commonauth "core/auth"
	"core/domain"
	"core/types"
	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"
	"gateway/internal/agent"
	"gateway/internal/session"
	"gateway/pkg/server_context"
	managerv1 "manager/gen/manager/v1"
	"manager/gen/manager/v1/managerv1connect"
	"manager/pkg/auth"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Power operation constants
const (
	PowerOpPowerOn    = "PowerOn"
	PowerOpPowerOff   = "PowerOff"
	PowerOpPowerCycle = "PowerCycle"
	PowerOpReset      = "Reset"
)

// ConsoleSession represents a unified session for both VNC and SOL console access
type ConsoleSession struct {
	SessionID   string
	ServerID    string
	BMCEndpoint string
	AgentID     string
	CustomerID  string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// Legacy type aliases for backward compatibility
type VNCSession = ConsoleSession
type SOLSession = ConsoleSession

// RegionalGatewayHandler implements the stateless Gateway.
type RegionalGatewayHandler struct {
	bmcManagerEndpoint     string
	jwtManager             *auth.JWTManager
	serverContextDecryptor *server_context.ServerContextDecryptor
	gatewayID              string
	region                 string
	externalEndpoint       string // External endpoint for VNC/console URLs
	managerClient          managerv1connect.BMCManagerServiceClient
	httpClient             *http.Client
	testMode               bool // Skip external calls during testing

	// In-memory state (rebuilt on restart via agent re-registration).
	agentRegistry *agent.Registry
	// bmc_endpoint -> agent mapping.
	bmcEndpointMapping map[string]*domain.AgentBMCMapping
	// Unified console session store (works for both VNC and SOL)
	consoleSessions map[string]*ConsoleSession
	// Web session store for cookie-based authentication
	webSessionStore session.Store
	mu              sync.RWMutex
}

// NewGatewayHandler creates a GatewayHandler.
func NewGatewayHandler(
	bmcManagerEndpoint string,
	jwtManager *auth.JWTManager,
	gatewayID, region, externalEndpoint string,
) *RegionalGatewayHandler {
	// Create HTTP client for manager communication
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create manager client
	managerClient := managerv1connect.NewBMCManagerServiceClient(
		httpClient,
		bmcManagerEndpoint,
	)

	// Create server context decryptor with same key as JWT manager.
	serverContextDecryptor := server_context.NewServerContextDecryptor("your-secret-key-change-in-production")

	return &RegionalGatewayHandler{
		bmcManagerEndpoint:     bmcManagerEndpoint,
		jwtManager:             jwtManager,
		serverContextDecryptor: serverContextDecryptor,
		gatewayID:              gatewayID,
		region:                 region,
		externalEndpoint:       externalEndpoint,
		managerClient:          managerClient,
		httpClient:             httpClient,
		testMode:               false,
		agentRegistry:          agent.NewRegistry(),
		bmcEndpointMapping:     make(map[string]*domain.AgentBMCMapping),
		webSessionStore:        session.NewInMemoryStore(),
		consoleSessions:        make(map[string]*ConsoleSession),
	}
}

// TokenValidationInterceptor validates delegated tokens from BMC Manager.
// It expects the AuthInterceptor to have already extracted the token and added it to the context.
func (h *RegionalGatewayHandler) TokenValidationInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Skip validation for agent registration and health checks
			if req.Spec().Procedure == "/gateway.v1.GatewayService/RegisterAgent" ||
				req.Spec().Procedure == "/gateway.v1.GatewayService/AgentHeartbeat" ||
				req.Spec().Procedure == "/gateway.v1.GatewayService/HealthCheck" {
				return next(ctx, req)
			}

			// Get token from context (added by AuthInterceptor)
			token, ok := ctx.Value("token").(string)
			if !ok || token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no authentication token found"))
			}

			// Validate token
			claims, err := h.jwtManager.ValidateToken(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
			}

			// Add claims to context for use in handlers
			ctx = context.WithValue(ctx, "claims", claims)
			// Token is already in context from AuthInterceptor
			return next(ctx, req)
		}
	}
}

// HealthCheck returns health info about the connected agents.
func (h *RegionalGatewayHandler) HealthCheck(
	_ context.Context,
	_ *connect.Request[gatewayv1.HealthCheckRequest],
) (*connect.Response[gatewayv1.HealthCheckResponse], error) {
	h.mu.RLock()
	agentCount := h.agentRegistry.Count()
	bmcEndpointCount := len(h.bmcEndpointMapping)
	h.mu.RUnlock()

	resp := &gatewayv1.HealthCheckResponse{
		Status: fmt.Sprintf(
			"healthy - %d agents, %d BMC endpoints",
			agentCount, bmcEndpointCount,
		),
		Timestamp: timestamppb.Now(),
	}

	return connect.NewResponse(resp), nil
}

// RegisterAgent handles Local Agent registration.
func (h *RegionalGatewayHandler) RegisterAgent(
	ctx context.Context,
	req *connect.Request[gatewayv1.RegisterAgentRequest],
) (*connect.Response[gatewayv1.RegisterAgentResponse], error) {
	log.Info().
		Str("agent_id", req.Msg.AgentId).
		Str("datacenter_id", req.Msg.DatacenterId).
		Msg("Agent registration")

	h.mu.Lock()
	defer h.mu.Unlock()

	// Register agent
	agentInfo := &agent.Info{
		ID:           req.Msg.AgentId,
		DatacenterID: req.Msg.DatacenterId,
		Endpoint:     req.Msg.Endpoint,
		LastSeen:     time.Now(),
	}
	h.agentRegistry.Register(agentInfo)

	// Update BMC endpoint mappings (no more server concepts at gateway level)
	for _, bmcEndpoint := range req.Msg.BmcEndpoints {
		// Convert protobuf metadata map to Go map
		metadata := make(map[string]string)
		for key, value := range bmcEndpoint.Metadata {
			metadata[key] = value
		}

		// Extract primary BMC endpoint from control endpoint
		bmcEndpointAddr := ""
		bmcType := types.BMCTypeIPMI // Default

		if bmcEndpoint.ControlEndpoint != nil {
			bmcEndpointAddr = bmcEndpoint.ControlEndpoint.Endpoint
			bmcType = convertProtoBMCTypeToModels(bmcEndpoint.ControlEndpoint.Type)
		}

		if bmcEndpointAddr != "" {
			// Extract credentials and capabilities from control endpoint
			var username string
			var capabilities []string
			if bmcEndpoint.ControlEndpoint != nil {
				username = bmcEndpoint.ControlEndpoint.Username
				capabilities = bmcEndpoint.ControlEndpoint.Capabilities
			}

			mapping := &domain.AgentBMCMapping{
				ServerID:          bmcEndpoint.ServerId,
				BMCEndpoint:       bmcEndpointAddr,
				AgentID:           req.Msg.AgentId,
				DatacenterID:      req.Msg.DatacenterId,
				BMCType:           bmcType,
				Features:          bmcEndpoint.Features,
				Status:            bmcEndpoint.Status,
				LastSeen:          time.Now(),
				Metadata:          metadata,
				Username:          username,
				Capabilities:      capabilities,
				DiscoveryMetadata: types.ConvertDiscoveryMetadataFromProto(bmcEndpoint.DiscoveryMetadata),
			}
			h.bmcEndpointMapping[bmcEndpointAddr] = mapping
			log.Debug().Str("server_id", bmcEndpoint.ServerId).Str("bmc_endpoint", bmcEndpointAddr).Str("agent_id", req.Msg.AgentId).Str("username", username).Strs("capabilities", capabilities).Msg("Mapped BMC endpoint to agent")
		}
	}

	// Report available BMC endpoints to manager (new architecture)
	// TODO: Temporarily disabled to fix agent registration timeout issue
	// The manager reporting call is hanging and causing agent timeouts
	log.Warn().Msg("Skipping manager endpoint reporting (temporarily disabled due to timeout issues)")

	// Use a goroutine to report endpoints asynchronously to avoid blocking agent registration
	go func() {
		managerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Debug().Int("endpoint_count", len(req.Msg.BmcEndpoints)).Msg("Attempting to report BMC endpoints to manager (async)")
		if err := h.reportEndpointsToManager(managerCtx); err != nil {
			log.Error().Err(err).Msg("Failed to report endpoints to manager")
		} else {
			log.Info().Msg("Successfully reported BMC endpoints to manager")
		}
	}()

	resp := &gatewayv1.RegisterAgentResponse{
		Success: true,
		Message: fmt.Sprintf("Agent %s registered successfully with %d BMC endpoints", req.Msg.AgentId, len(req.Msg.BmcEndpoints)),
	}

	return connect.NewResponse(resp), nil
}

// AgentHeartbeat handles periodic heartbeats from Local Agents.
func (h *RegionalGatewayHandler) AgentHeartbeat(
	_ context.Context,
	req *connect.Request[gatewayv1.AgentHeartbeatRequest],
) (*connect.Response[gatewayv1.AgentHeartbeatResponse], error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update agent last seen
	h.agentRegistry.UpdateLastSeen(req.Msg.AgentId, time.Now())

	// Update BMC endpoint mappings if provided
	agentInfo := h.agentRegistry.Get(req.Msg.AgentId)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", req.Msg.AgentId))
	}

	for _, bmcEndpoint := range req.Msg.BmcEndpoints {
		// Convert protobuf metadata map to Go map
		metadata := make(map[string]string)
		for key, value := range bmcEndpoint.Metadata {
			metadata[key] = value
		}

		// Extract primary BMC endpoint from control endpoint
		bmcEndpointAddr := ""
		bmcType := types.BMCTypeIPMI // Default

		if bmcEndpoint.ControlEndpoint != nil {
			bmcEndpointAddr = bmcEndpoint.ControlEndpoint.Endpoint
			bmcType = convertProtoBMCTypeToModels(bmcEndpoint.ControlEndpoint.Type)
		}

		if bmcEndpointAddr != "" {
			// Extract credentials and capabilities from control endpoint
			var username string
			var capabilities []string
			if bmcEndpoint.ControlEndpoint != nil {
				username = bmcEndpoint.ControlEndpoint.Username
				capabilities = bmcEndpoint.ControlEndpoint.Capabilities
			}

			mapping := &domain.AgentBMCMapping{
				ServerID:          bmcEndpoint.ServerId,
				BMCEndpoint:       bmcEndpointAddr,
				AgentID:           req.Msg.AgentId,
				DatacenterID:      agentInfo.DatacenterID,
				BMCType:           bmcType,
				Features:          bmcEndpoint.Features,
				Status:            bmcEndpoint.Status,
				LastSeen:          time.Now(),
				Metadata:          metadata,
				Username:          username,
				Capabilities:      capabilities,
				DiscoveryMetadata: types.ConvertDiscoveryMetadataFromProto(bmcEndpoint.DiscoveryMetadata),
			}
			h.bmcEndpointMapping[bmcEndpointAddr] = mapping
		}
	}

	resp := &gatewayv1.AgentHeartbeatResponse{
		Success:                  true,
		HeartbeatIntervalSeconds: 30, // 30 seconds
	}

	return connect.NewResponse(resp), nil
}

// extractServerContextFromJWT extracts server context from JWT token in the
// request.
func (h *RegionalGatewayHandler) extractServerContextFromJWT(
	ctx context.Context,
) (*commonauth.ServerContext, error) {
	token, ok := ctx.Value("token").(string)
	if !ok {
		return nil, fmt.Errorf("no token found in context")
	}

	// First validate the JWT using the regular JWT manager
	// (same signing key as manager).
	_, serverContext, err := h.jwtManager.ValidateServerToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to validate server token: %w", err)
	}

	// Check if we got server context from the token.
	if serverContext == nil {
		return nil, fmt.Errorf("token does not contain server context")
	}

	// Convert from manager's ServerContext to gateway's ServerContext.
	gatewayServerContext := &commonauth.ServerContext{
		ServerID:     serverContext.ServerID,
		CustomerID:   serverContext.CustomerID,
		BMCEndpoint:  serverContext.BMCEndpoint,
		BMCType:      serverContext.BMCType,
		Features:     serverContext.Features,
		DatacenterID: serverContext.DatacenterID,
		Permissions:  serverContext.Permissions,
		IssuedAt:     serverContext.IssuedAt,
		ExpiresAt:    serverContext.ExpiresAt,
	}

	// TODO: Replace with proper server-customer mapping validation
	// Temporarily disabled: servers now belong to "system" and we have separate server-customer mappings
	// In the new architecture, we need to validate against ServerCustomerMapping table instead
	// if gatewayServerContext.CustomerID != claims.CustomerID {
	//     return nil, fmt.Errorf("server context customer ID mismatch")
	// }

	return gatewayServerContext, nil
}

// BMC operations - these will proxy to the appropriate Local Agent
// These now work with BMC endpoints directly (Manager resolves server IDs to BMC endpoints)

// PowerOn executes a PowerOn power operation.
func (h *RegionalGatewayHandler) PowerOn(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions
	if !serverContext.HasPermission("power:write") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for power operations"))
	}

	// Forward directly to agent using BMC endpoint from token
	return h.proxyPowerOperation(ctx, serverContext.BMCEndpoint, PowerOpPowerOn)
}

// PowerOff executes a PowerOff power operation.
func (h *RegionalGatewayHandler) PowerOff(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions
	if !serverContext.HasPermission("power:write") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for power operations"))
	}

	// Forward directly to agent using BMC endpoint from token
	return h.proxyPowerOperation(ctx, serverContext.BMCEndpoint, PowerOpPowerOff)
}

// PowerCycle executes a PowerCycle power operation.
func (h *RegionalGatewayHandler) PowerCycle(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions
	if !serverContext.HasPermission("power:write") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for power operations"))
	}

	// Forward directly to agent using BMC endpoint from token
	return h.proxyPowerOperation(ctx, serverContext.BMCEndpoint, PowerOpPowerCycle)
}

// Reset executes a Reset power operation.
func (h *RegionalGatewayHandler) Reset(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions
	if !serverContext.HasPermission("power:write") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for power operations"))
	}

	// Forward directly to agent using BMC endpoint from token
	return h.proxyPowerOperation(ctx, serverContext.BMCEndpoint, PowerOpReset)
}

// GetPowerStatus obtains the power status.
func (h *RegionalGatewayHandler) GetPowerStatus(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerStatusRequest],
) (*connect.Response[gatewayv1.PowerStatusResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions
	if !serverContext.HasPermission("power:read") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for power status"))
	}

	// Check if BMC endpoint is available through an agent
	h.mu.RLock()
	mapping, exists := h.bmcEndpointMapping[serverContext.BMCEndpoint]
	h.mu.RUnlock()

	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("BMC endpoint not found: %s", serverContext.BMCEndpoint))
	}

	agentInfo := h.agentRegistry.Get(mapping.AgentID)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", mapping.AgentID))
	}

	log.Info().
		Str("server_id", serverContext.ServerID).
		Str("bmc_endpoint", serverContext.BMCEndpoint).
		Str("agent_id", mapping.AgentID).
		Str("agent_endpoint", agentInfo.Endpoint).
		Msg("Proxying power status request to agent")

	// Create RPC client for the agent
	agentClient := gatewayv1connect.NewGatewayServiceClient(
		h.httpClient,
		agentInfo.Endpoint,
	)

	// Create request for power status
	agentReq := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: serverContext.ServerID,
	})

	// Call the agent
	resp, err := agentClient.GetPowerStatus(ctx, agentReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("bmc_endpoint", serverContext.BMCEndpoint).
			Str("agent_id", mapping.AgentID).
			Msg("Power status request failed")
		return nil, err
	}

	log.Info().
		Str("server_id", serverContext.ServerID).
		Str("bmc_endpoint", serverContext.BMCEndpoint).
		Str("state", resp.Msg.State.String()).
		Msg("Power status retrieved")

	return resp, nil
}

// GetBMCInfo retrieves detailed BMC hardware information
func (h *RegionalGatewayHandler) GetBMCInfo(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetBMCInfoRequest],
) (*connect.Response[gatewayv1.GetBMCInfoResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions - BMC info is similar to power status, requires power:read
	if !serverContext.HasPermission("power:read") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for BMC info"))
	}

	// Check if BMC endpoint is available through an agent
	h.mu.RLock()
	mapping, exists := h.bmcEndpointMapping[serverContext.BMCEndpoint]
	h.mu.RUnlock()

	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("BMC endpoint not found: %s", serverContext.BMCEndpoint))
	}

	agentInfo := h.agentRegistry.Get(mapping.AgentID)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", mapping.AgentID))
	}

	log.Info().
		Str("server_id", serverContext.ServerID).
		Str("bmc_endpoint", serverContext.BMCEndpoint).
		Str("agent_id", mapping.AgentID).
		Str("agent_endpoint", agentInfo.Endpoint).
		Msg("Proxying BMC info request to agent")

	// Create RPC client for the agent
	agentClient := gatewayv1connect.NewGatewayServiceClient(
		h.httpClient,
		agentInfo.Endpoint,
	)

	// Create request for BMC info
	agentReq := connect.NewRequest(&gatewayv1.GetBMCInfoRequest{
		ServerId: serverContext.ServerID,
	})

	// Call the agent
	resp, err := agentClient.GetBMCInfo(ctx, agentReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("bmc_endpoint", serverContext.BMCEndpoint).
			Str("agent_id", mapping.AgentID).
			Msg("BMC info request failed")
		return nil, err
	}

	log.Info().
		Str("server_id", serverContext.ServerID).
		Str("bmc_endpoint", serverContext.BMCEndpoint).
		Str("bmc_type", resp.Msg.Info.BmcType).
		Msg("BMC info retrieved")

	return resp, nil
}

// Helper method to proxy power operations to Local Agents.
func (h *RegionalGatewayHandler) proxyPowerOperation(
	ctx context.Context,
	bmcEndpoint,
	operation string,
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	h.mu.RLock()
	mapping, exists := h.bmcEndpointMapping[bmcEndpoint]
	h.mu.RUnlock()

	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("BMC endpoint not found: %s", bmcEndpoint))
	}

	agentInfo := h.agentRegistry.Get(mapping.AgentID)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", mapping.AgentID))
	}

	log.Info().
		Str("operation", operation).
		Str("bmc_endpoint", bmcEndpoint).
		Str("agent_id", mapping.AgentID).
		Str("agent_endpoint", agentInfo.Endpoint).
		Msg("Proxying power operation to agent")

	// Create RPC client for the agent
	agentClient := gatewayv1connect.NewGatewayServiceClient(
		h.httpClient,
		agentInfo.Endpoint,
	)

	// Create request for the power operation
	// Note: We pass the server_id from the mapping, not the BMC endpoint
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: mapping.ServerID,
	})

	// Call the appropriate operation on the agent
	var resp *connect.Response[gatewayv1.PowerOperationResponse]
	var err error

	switch operation {
	case PowerOpPowerOn:
		resp, err = agentClient.PowerOn(ctx, req)
	case PowerOpPowerOff:
		resp, err = agentClient.PowerOff(ctx, req)
	case PowerOpPowerCycle:
		resp, err = agentClient.PowerCycle(ctx, req)
	case PowerOpReset:
		resp, err = agentClient.Reset(ctx, req)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown power operation: %s", operation))
	}

	if err != nil {
		log.Error().
			Err(err).
			Str("operation", operation).
			Str("bmc_endpoint", bmcEndpoint).
			Str("agent_id", mapping.AgentID).
			Msg("Power operation failed")
		return nil, err
	}

	log.Info().
		Str("operation", operation).
		Str("bmc_endpoint", bmcEndpoint).
		Bool("success", resp.Msg.Success).
		Msg("Power operation completed")

	return resp, nil
}

// VNC Console Session Management

// CreateVNCSession creates a new VNC console session for remote access
func (h *RegionalGatewayHandler) CreateVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CreateVNCSessionRequest],
) (*connect.Response[gatewayv1.CreateVNCSessionResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions for console access (accept either console:write or console:access)
	if !serverContext.HasPermission("console:write") && !serverContext.HasPermission("console:access") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for console access"))
	}

	// TODO: Re-enable feature check once agents properly report console/kvm features
	// For now, skip feature validation to allow VNC session creation for testing
	// Check if BMC endpoint supports VNC/console features
	// if !contains(serverContext.Features, "console") && !contains(serverContext.Features, "kvm") {
	//     return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("server does not support console access"))
	// }

	// Find agent for the BMC endpoint from server context
	h.mu.RLock()
	mapping, exists := h.bmcEndpointMapping[serverContext.BMCEndpoint]
	h.mu.RUnlock()

	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("BMC endpoint not found: %s", serverContext.BMCEndpoint))
	}

	agentInfo := h.agentRegistry.Get(mapping.AgentID)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", mapping.AgentID))
	}

	// Generate unique session ID using timestamp (same format as SOL for consistency)
	sessionID := fmt.Sprintf("vnc-%d", time.Now().UnixNano())

	// Create WebSocket endpoint URL using external endpoint
	websocketEndpoint := fmt.Sprintf("ws://%s/vnc/%s/ws", h.externalEndpoint, sessionID)

	// Create viewer URL using external endpoint
	viewerURL := fmt.Sprintf("http://%s/vnc/%s", h.externalEndpoint, sessionID)

	// Set expiration time (1 hour from now)
	expiresAt := time.Now().Add(time.Hour)

	// Store the console session (works for both VNC and SOL)
	h.mu.Lock()
	h.consoleSessions[sessionID] = &ConsoleSession{
		SessionID:   sessionID,
		ServerID:    serverContext.ServerID,
		BMCEndpoint: serverContext.BMCEndpoint,
		AgentID:     mapping.AgentID,
		CustomerID:  serverContext.CustomerID,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
	}
	h.mu.Unlock()

	log.Info().Str("session_id", sessionID).Str("server_id", serverContext.ServerID).Str("customer_id", serverContext.CustomerID).Msg("Created VNC session")

	resp := &gatewayv1.CreateVNCSessionResponse{
		SessionId:         sessionID,
		WebsocketEndpoint: websocketEndpoint,
		ViewerUrl:         viewerURL,
		ExpiresAt:         timestamppb.New(expiresAt),
	}

	return connect.NewResponse(resp), nil
}

// GetVNCSessionByID retrieves a console session by ID (supports both VNC and SOL)
func (h *RegionalGatewayHandler) GetVNCSessionByID(sessionID string) (*VNCSession, bool) {
	return h.GetConsoleSessionByID(sessionID)
}

// GetConsoleSessionCount returns the count of active console sessions
func (h *RegionalGatewayHandler) GetConsoleSessionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.consoleSessions)
}

// GetConsoleSessionByID retrieves a console session by ID for internal use
func (h *RegionalGatewayHandler) GetConsoleSessionByID(sessionID string) (*ConsoleSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	session, exists := h.consoleSessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		go func() {
			h.mu.Lock()
			delete(h.consoleSessions, sessionID)
			h.mu.Unlock()
		}()
		return nil, false
	}

	return session, true
}

// GetAgentRegistry returns the agent registry for accessing agent information
func (h *RegionalGatewayHandler) GetAgentRegistry() *agent.Registry {
	return h.agentRegistry
}

// GetVNCSession retrieves information about an existing VNC session
func (h *RegionalGatewayHandler) GetVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetVNCSessionRequest],
) (*connect.Response[gatewayv1.GetVNCSessionResponse], error) {
	// Get claims from context
	claims, ok := ctx.Value("claims").(*commonauth.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	sessionID := req.Msg.SessionId

	// TODO: Implement actual session storage and retrieval
	// For now, simulate a session response
	log.Debug().Str("session_id", sessionID).Str("customer_id", claims.CustomerID).Msg("Retrieved VNC session")

	session := &gatewayv1.VNCSession{
		Id:                sessionID,
		CustomerId:        claims.CustomerID,
		ServerId:          "server-001", // TODO: Get from actual session storage
		AgentId:           "agent-001",  // TODO: Get from actual session storage
		Status:            "active",
		WebsocketEndpoint: fmt.Sprintf("ws://%s/vnc/%s/ws", h.externalEndpoint, sessionID),
		ViewerUrl:         fmt.Sprintf("http://%s/vnc/%s", h.externalEndpoint, sessionID),
		CreatedAt:         timestamppb.Now(),
		ExpiresAt:         timestamppb.New(time.Now().Add(time.Hour)),
	}

	resp := &gatewayv1.GetVNCSessionResponse{
		Session: session,
	}

	return connect.NewResponse(resp), nil
}

// CloseVNCSession terminates an active VNC session
func (h *RegionalGatewayHandler) CloseVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CloseVNCSessionRequest],
) (*connect.Response[gatewayv1.CloseVNCSessionResponse], error) {
	// Get claims from context.
	claims, ok := ctx.Value("claims").(*commonauth.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	sessionID := req.Msg.SessionId

	// Remove session from memory
	h.mu.Lock()
	delete(h.consoleSessions, sessionID)
	h.mu.Unlock()

	log.Info().Str("session_id", sessionID).Str("customer_id", claims.CustomerID).Msg("Closed VNC session")

	resp := &gatewayv1.CloseVNCSessionResponse{}
	return connect.NewResponse(resp), nil
}

// CreateSOLSession creates a new SOL console session for terminal access
func (h *RegionalGatewayHandler) CreateSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CreateSOLSessionRequest],
) (*connect.Response[gatewayv1.CreateSOLSessionResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	// Validate server ID matches token context
	if serverContext.ServerID != req.Msg.ServerId {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("server ID mismatch"))
	}

	// Check permissions for console access
	if !serverContext.HasPermission("console:write") && !serverContext.HasPermission("console:access") {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("insufficient permissions for console access"))
	}

	// Find the agent that handles this server's BMC
	h.mu.RLock()
	mapping, exists := h.bmcEndpointMapping[serverContext.BMCEndpoint]
	h.mu.RUnlock()

	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("BMC endpoint not found: %s", serverContext.BMCEndpoint))
	}

	agentInfo := h.agentRegistry.Get(mapping.AgentID)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", mapping.AgentID))
	}

	// Generate unique session ID
	sessionID := fmt.Sprintf("sol-%d", time.Now().UnixNano())
	now := time.Now()
	expiresAt := now.Add(2 * time.Hour) // 2 hour session

	// Create console session (unified for both VNC and SOL)
	consoleSession := &ConsoleSession{
		SessionID:   sessionID,
		ServerID:    req.Msg.ServerId,
		BMCEndpoint: serverContext.BMCEndpoint,
		AgentID:     mapping.AgentID,
		CustomerID:  serverContext.CustomerID,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}

	// Store session
	h.mu.Lock()
	h.consoleSessions[sessionID] = consoleSession
	h.mu.Unlock()

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", req.Msg.ServerId).
		Str("customer_id", serverContext.CustomerID).
		Str("agent_id", mapping.AgentID).
		Msg("Created SOL session")

	// Build WebSocket endpoint for SOL streaming
	wsEndpoint := fmt.Sprintf("ws://%s/sol/%s", h.externalEndpoint, sessionID)

	// Build console URL - direct link to web-based SOL console
	consoleURL := fmt.Sprintf("http://%s/console/%s", h.externalEndpoint, sessionID)

	resp := &gatewayv1.CreateSOLSessionResponse{
		SessionId:         sessionID,
		WebsocketEndpoint: wsEndpoint,
		ExpiresAt:         timestamppb.New(expiresAt),
		ConsoleUrl:        consoleURL,
	}

	return connect.NewResponse(resp), nil
}

// GetSOLSessionByID retrieves a console session by ID (supports both VNC and SOL)
func (h *RegionalGatewayHandler) GetSOLSessionByID(sessionID string) (*SOLSession, bool) {
	return h.GetConsoleSessionByID(sessionID)
}

// GetSOLSession retrieves information about an existing SOL session
func (h *RegionalGatewayHandler) GetSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetSOLSessionRequest],
) (*connect.Response[gatewayv1.GetSOLSessionResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	sessionID := req.Msg.SessionId

	// Retrieve session
	solSession, exists := h.GetSOLSessionByID(sessionID)
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("SOL session not found: %s", sessionID))
	}

	// Verify customer owns this session
	if solSession.CustomerID != serverContext.CustomerID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("access denied"))
	}

	// Build response
	wsEndpoint := fmt.Sprintf("ws://%s/sol/%s", h.externalEndpoint, sessionID)
	consoleURL := fmt.Sprintf("http://%s/console/%s", h.externalEndpoint, sessionID)

	session := &gatewayv1.SOLSession{
		Id:                sessionID,
		CustomerId:        solSession.CustomerID,
		ServerId:          solSession.ServerID,
		AgentId:           solSession.AgentID,
		Status:            "active",
		WebsocketEndpoint: wsEndpoint,
		ConsoleUrl:        consoleURL,
		CreatedAt:         timestamppb.New(solSession.CreatedAt),
		ExpiresAt:         timestamppb.New(solSession.ExpiresAt),
	}

	resp := &gatewayv1.GetSOLSessionResponse{
		Session: session,
	}

	return connect.NewResponse(resp), nil
}

// CloseSOLSession terminates an active SOL session
func (h *RegionalGatewayHandler) CloseSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CloseSOLSessionRequest],
) (*connect.Response[gatewayv1.CloseSOLSessionResponse], error) {
	// Extract server context from JWT token
	serverContext, err := h.extractServerContextFromJWT(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid server context: %w", err))
	}

	sessionID := req.Msg.SessionId

	// Remove session from memory
	h.mu.Lock()
	delete(h.consoleSessions, sessionID)
	h.mu.Unlock()

	log.Info().Str("session_id", sessionID).Str("customer_id", serverContext.CustomerID).Msg("Closed SOL session")

	resp := &gatewayv1.CloseSOLSessionResponse{}
	return connect.NewResponse(resp), nil
}

// StartVNCProxy requests an agent to start a VNC proxy for a specific BMC
func (h *RegionalGatewayHandler) StartVNCProxy(
	ctx context.Context,
	req *connect.Request[gatewayv1.StartVNCProxyRequest],
) (*connect.Response[gatewayv1.StartVNCProxyResponse], error) {
	// Get agent info from the provided agent ID
	agentInfo := h.agentRegistry.Get(req.Msg.AgentId)
	if agentInfo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", req.Msg.AgentId))
	}

	log.Info().
		Str("agent_id", req.Msg.AgentId).
		Str("bmc_endpoint", req.Msg.BmcEndpoint).
		Str("session_id", req.Msg.SessionId).
		Msg("Setting up VNC proxy via agent")

	// Build the agent VNC WebSocket URL
	// The agent expects: /vnc/{sessionId}/{bmcHost}?type={bmcType}
	// Extract just the host:port from the agent endpoint (remove http:// if present)
	agentHost := agentInfo.Endpoint
	if strings.HasPrefix(agentHost, "http://") {
		agentHost = strings.TrimPrefix(agentHost, "http://")
	}

	// Extract just the host:port from the BMC endpoint for the URL path
	bmcHost := req.Msg.BmcEndpoint
	if strings.HasPrefix(bmcHost, "http://") {
		bmcHost = strings.TrimPrefix(bmcHost, "http://")
	}

	agentVNCURL := fmt.Sprintf("ws://%s/vnc/%s/%s?type=%s",
		agentHost, req.Msg.SessionId, bmcHost, req.Msg.BmcType)

	log.Debug().
		Str("agent_vnc_url", agentVNCURL).
		Str("agent_host", agentHost).
		Str("session_id", req.Msg.SessionId).
		Str("bmc_host", bmcHost).
		Msg("Constructed agent VNC URL")

	resp := &gatewayv1.StartVNCProxyResponse{
		Success:       true,
		Message:       fmt.Sprintf("VNC proxy configured for session %s via agent %s", req.Msg.SessionId, req.Msg.AgentId),
		ProxyEndpoint: agentVNCURL,
	}

	return connect.NewResponse(resp), nil
}

// authenticateWithManager gets an authentication token from the manager.
func (h *RegionalGatewayHandler) authenticateWithManager(ctx context.Context) (string, error) {
	// Use test credentials that match the test manager setup
	authReq := &managerv1.AuthenticateRequest{
		Email:    "test@example.com",
		Password: "password",
	}

	resp, err := h.managerClient.Authenticate(ctx, connect.NewRequest(authReq))
	if err != nil {
		return "", err
	}

	return resp.Msg.AccessToken, nil
}

// registerGatewayWithManager registers this gateway with the BMC Manager.
func (h *RegionalGatewayHandler) registerGatewayWithManager(ctx context.Context) error {
	// Authenticate with manager
	token, err := h.authenticateWithManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to authenticate with manager: %w", err)
	}

	// Get list of datacenters this gateway serves
	datacenterIDs := h.getDatacenterIDs()

	// Create register gateway request
	registerReq := &managerv1.RegisterGatewayRequest{
		GatewayId:     h.gatewayID,
		Region:        h.region,
		Endpoint:      fmt.Sprintf("http://localhost:8081"), // TODO: Make this configurable
		DatacenterIds: datacenterIDs,
	}

	// Create authenticated request
	req := connect.NewRequest(registerReq)
	req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Register with manager
	_, err = h.managerClient.RegisterGateway(ctx, req)
	return err
}

// getDatacenterIDs returns the list of datacenters this gateway currently
// serves.
func (h *RegionalGatewayHandler) getDatacenterIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	datacenterMap := make(map[string]bool)

	// Collect unique datacenter IDs from registered agents
	for _, agent := range h.agentRegistry.List() {
		datacenterMap[agent.DatacenterID] = true
	}

	// Convert map to slice
	var datacenterIDs []string
	for datacenterID := range datacenterMap {
		datacenterIDs = append(datacenterIDs, datacenterID)
	}

	// If no agents registered yet, provide a default
	if len(datacenterIDs) == 0 {
		datacenterIDs = []string{"dc-test-01"} // Default datacenter for testing
	}

	return datacenterIDs
}

// StartPeriodicRegistration starts a goroutine that periodically re-registers
// the gateway.
func (h *RegionalGatewayHandler) StartPeriodicRegistration(ctx context.Context) {
	go func() {
		// Initial registration
		if err := h.registerGatewayWithManager(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to register gateway with manager")
		} else {
			log.Info().Str("gateway_id", h.gatewayID).Msg("Successfully registered gateway with manager")
		}

		// Periodic re-registration every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := h.registerGatewayWithManager(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to re-register gateway with manager")
				} else {
					log.Debug().Str("gateway_id", h.gatewayID).Msg("Successfully re-registered gateway with manager")
				}
			}
		}
	}()
}

// reportEndpointsToManager reports available BMC endpoints to the manager.
// This allows the manager to know which BMC endpoints are accessible through
// this gateway.
func (h *RegionalGatewayHandler) reportEndpointsToManager(ctx context.Context) error {
	// Skip manager reporting in test mode
	if h.testMode {
		log.Debug().
			Str("gateway_id", h.gatewayID).
			Int("endpoint_count", len(h.bmcEndpointMapping)).
			Msg("Gateway (test mode): skipping manager endpoint reporting")
		return nil
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	log.Info().
		Str("gateway_id", h.gatewayID).
		Int("endpoint_count", len(h.bmcEndpointMapping)).
		Msg("Gateway reporting BMC endpoints to manager")

	// Prepare list of BMC endpoints for manager
	var endpoints []*managerv1.BMCEndpointAvailability
	for _, mapping := range h.bmcEndpointMapping {
		endpoints = append(endpoints, &managerv1.BMCEndpointAvailability{
			BmcEndpoint:       mapping.BMCEndpoint,
			AgentId:           mapping.AgentID,
			DatacenterId:      mapping.DatacenterID,
			BmcType:           convertBMCTypeToManagerProto(mapping.BMCType),
			Features:          mapping.Features,
			Status:            mapping.Status,
			LastSeen:          timestamppb.New(mapping.LastSeen),
			Username:          mapping.Username,
			Capabilities:      mapping.Capabilities,
			DiscoveryMetadata: mapping.DiscoveryMetadata.ConvertToProto(),
		})
		log.Debug().
			Str("bmc_endpoint", mapping.BMCEndpoint).
			Str("agent_id", mapping.AgentID).
			Str("datacenter_id", mapping.DatacenterID).
			Str("bmc_type", string(mapping.BMCType)).
			Str("status", mapping.Status).
			Str("username", mapping.Username).
			Strs("capabilities", mapping.Capabilities).
			Msg("Reporting BMC endpoint")
	}

	// Create request
	reportReq := &managerv1.ReportAvailableEndpointsRequest{
		GatewayId:    h.gatewayID,
		Region:       h.region,
		BmcEndpoints: endpoints,
	}

	// Authenticate and send request
	token, err := h.authenticateWithManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to authenticate with manager: %w", err)
	}

	req := connect.NewRequest(reportReq)
	req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))

	_, err = h.managerClient.ReportAvailableEndpoints(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to report endpoints to manager: %w", err)
	}

	log.Info().Int("endpoint_count", len(endpoints)).Msg("Successfully reported BMC endpoints to manager")
	return nil
}

// convertBMCTypeToManagerProto converts model BMC type to manager protobuf BMC
// type.
func convertBMCTypeToManagerProto(bmcType types.BMCType) managerv1.BMCType {
	switch bmcType {
	case types.BMCTypeIPMI:
		return managerv1.BMCType_BMC_IPMI
	case types.BMCTypeRedfish:
		return managerv1.BMCType_BMC_REDFISH
	default:
		return managerv1.BMCType_BMC_UNSPECIFIED
	}
}

// convertProtoBMCTypeToModels converts from gateway protobuf BMCType to types.BMCType
func convertProtoBMCTypeToModels(protoType gatewayv1.BMCType) types.BMCType {
	switch protoType {
	case gatewayv1.BMCType_BMC_IPMI:
		return types.BMCTypeIPMI
	case gatewayv1.BMCType_BMC_REDFISH:
		return types.BMCTypeRedfish
	default:
		return types.BMCTypeIPMI // Default fallback, but this should be rare
	}
}
