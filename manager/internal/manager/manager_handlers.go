package manager

import (
	"context"
	"fmt"
	"runtime"
	"time"

	managerv1 "manager/gen/manager/v1"
	"manager/internal/database"
	"manager/pkg/auth"
	"manager/pkg/models"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"core/types"
)

type BMCManagerServiceHandler struct {
	db         *database.BunDB
	jwtManager *auth.JWTManager
	startTime  time.Time
}

func NewBMCManagerServiceHandler(db *database.BunDB, jwtManager *auth.JWTManager) *BMCManagerServiceHandler {
	return &BMCManagerServiceHandler{
		db:         db,
		jwtManager: jwtManager,
		startTime:  time.Now(),
	}
}

// AuthInterceptor is authentication interceptor for Connect
func (h *BMCManagerServiceHandler) AuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Skip auth for authentication and status endpoints
			if req.Spec().Procedure == "/manager.v1.BMCManagerService/Authenticate" ||
				req.Spec().Procedure == "/manager.v1.BMCManagerService/GetSystemStatus" {
				return next(ctx, req)
			}

			// Check for JWT token in Authorization header
			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing authorization header"))
			}

			// Extract and validate JWT token
			parts := []string{}
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				parts = []string{"Bearer", authHeader[7:]}
			}

			if len(parts) != 2 || parts[0] != "Bearer" {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid authorization header format"))
			}

			claims, err := h.jwtManager.ValidateToken(parts[1])
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
			}

			// Store full claims object for new methods that need it
			ctx = context.WithValue(ctx, "claims", claims)
			// Keep individual values for backwards compatibility
			ctx = context.WithValue(ctx, "customer_id", claims.CustomerID)
			ctx = context.WithValue(ctx, "customer_email", claims.Email)
			return next(ctx, req)
		}
	}
}

// Authenticate verifies customer credentials and issues access tokens
func (h *BMCManagerServiceHandler) Authenticate(
	ctx context.Context,
	req *connect.Request[managerv1.AuthenticateRequest],
) (*connect.Response[managerv1.AuthenticateResponse], error) {
	// TODO: Implement actual authentication logic
	// For now, return a placeholder response for demo purposes

	// Use email address as customer ID - this aligns with OIDC where email is a stable identifier
	customerID := req.Msg.Email
	customer := &models.Customer{
		ID:    customerID,
		Email: req.Msg.Email,
	}
	accessToken, err := h.jwtManager.GenerateToken(customer)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	response := &managerv1.AuthenticateResponse{
		AccessToken:  accessToken,
		RefreshToken: "refresh_" + uuid.New().String(),
		ExpiresAt:    timestamppb.New(time.Now().Add(24 * time.Hour)),
		Customer: &managerv1.Customer{
			Id:        customerID,
			Email:     req.Msg.Email,
			CreatedAt: timestamppb.Now(),
		},
	}

	return connect.NewResponse(response), nil
}

// RefreshToken issues new access tokens using refresh tokens
func (h *BMCManagerServiceHandler) RefreshToken(
	ctx context.Context,
	req *connect.Request[managerv1.RefreshTokenRequest],
) (*connect.Response[managerv1.RefreshTokenResponse], error) {
	// TODO: Implement refresh token logic
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("refresh token not implemented"))
}

// GetServerToken generates a server-specific token with encrypted BMC context
func (h *BMCManagerServiceHandler) GetServerToken(
	ctx context.Context,
	req *connect.Request[managerv1.GetServerTokenRequest],
) (*connect.Response[managerv1.GetServerTokenResponse], error) {
	// Get customer ID from JWT claims (set by auth interceptor)
	claims, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// TEMPORARY IMPLEMENTATION: Get server by ID (allowing all customers access to all servers)
	// TODO: Replace with proper server-customer mapping check using ServerCustomerMapping table
	// TODO: Implement: 1) Query ServerCustomerMapping to verify customer has access to server
	// TODO: Implement: 2) Only allow access if mapping exists or customer is admin
	// TODO: Implement: 3) Add proper error handling for permission denied cases
	server, err := h.db.Servers.Get(ctx, req.Msg.ServerId)
	if err != nil {
		if err.Error() == "server not found" {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get server: %w", err))
	}

	// Create customer object for token generation
	customer := &models.Customer{
		ID:    claims.CustomerID,
		Email: claims.Email,
	}

	// Define permissions for this server token
	// In production, these would be determined by customer role/subscription
	permissions := []string{"power:read", "power:write", "console:read", "console:write"}

	// Generate server-specific token with encrypted BMC context
	serverToken, err := h.jwtManager.GenerateServerToken(customer, server, permissions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate server token: %w", err))
	}

	response := &managerv1.GetServerTokenResponse{
		Token:     serverToken,
		ExpiresAt: timestamppb.New(time.Now().Add(1 * time.Hour)), // Server tokens expire in 1 hour
	}

	bmcEndpoint := ""
	if server.ControlEndpoint != nil {
		bmcEndpoint = server.ControlEndpoint.Endpoint
	}
	log.Debug().
		Str("customer_id", claims.CustomerID).
		Str("server_id", server.ID).
		Str("bmc_endpoint", bmcEndpoint).
		Msg("Generated server token")

	return connect.NewResponse(response), nil
}

// RegisterServer registers a server and maps it to a regional gateway
func (h *BMCManagerServiceHandler) RegisterServer(
	ctx context.Context,
	req *connect.Request[managerv1.RegisterServerRequest],
) (*connect.Response[managerv1.RegisterServerResponse], error) {
	customerID, ok := ctx.Value("customer_id").(string)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("customer not authenticated"))
	}

	// Convert BMC type from protobuf to models
	var bmcType models.BMCType
	switch req.Msg.BmcType {
	case managerv1.BMCType_BMC_TYPE_IPMI:
		bmcType = models.BMCTypeIPMI
	case managerv1.BMCType_BMC_TYPE_REDFISH:
		bmcType = models.BMCTypeRedfish
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid BMC type"))
	}

	// Create server record with BMC endpoint information
	server := &models.Server{
		ID:           req.Msg.ServerId,
		CustomerID:   customerID,
		DatacenterID: req.Msg.DatacenterId,
		ControlEndpoint: &models.BMCControlEndpoint{
			Endpoint: req.Msg.BmcEndpoint,
			Type:     bmcType,
			Username: "", // Will be filled later
			Password: "", // Will be filled later
		},
		Features:  req.Msg.Features,
		Status:    "active",
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Populate SOL/Console endpoint if feature is present
	log.Debug().
		Str("server_id", req.Msg.ServerId).
		Strs("features", req.Msg.Features).
		Msg("Processing features for endpoint population")

	for _, feature := range req.Msg.Features {
		if feature == types.FeatureConsole.String() {
			// Determine SOL type based on BMC type
			solType := models.SOLTypeIPMI
			if bmcType == models.BMCTypeRedfish {
				solType = models.SOLTypeRedfishSerial
			}
			server.SOLEndpoint = &models.SOLEndpoint{
				Type:     solType,
				Endpoint: req.Msg.BmcEndpoint,
				Username: "", // Will be filled later
				Password: "", // Will be filled later
			}
			log.Debug().
				Str("server_id", req.Msg.ServerId).
				Str("sol_type", string(solType)).
				Msg("Created SOL endpoint")
			break
		}
	}

	// Populate VNC endpoint if feature is present
	for _, feature := range req.Msg.Features {
		if feature == types.FeatureVNC.String() {
			server.VNCEndpoint = &models.VNCEndpoint{
				Type:     models.VNCTypeNative, // Default to native VNC
				Endpoint: req.Msg.BmcEndpoint,
				Username: "", // Will be filled later
				Password: "", // Will be filled later
			}
			log.Debug().
				Str("server_id", req.Msg.ServerId).
				Msg("Created VNC endpoint")
			break
		}
	}

	log.Info().
		Str("server_id", server.ID).
		Str("bmc_endpoint", server.ControlEndpoint.Endpoint).
		Bool("has_sol", server.SOLEndpoint != nil).
		Bool("has_vnc", server.VNCEndpoint != nil).
		Msg("Creating server record")
	err := h.db.Servers.Create(ctx, server)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create server record: %w", err))
	}
	log.Info().Str("server_id", server.ID).Msg("Successfully created server record")

	// Create server location record for gateway routing
	location := &models.ServerLocation{
		ServerID:          req.Msg.ServerId,
		CustomerID:        customerID,
		DatacenterID:      req.Msg.DatacenterId,
		RegionalGatewayID: req.Msg.RegionalGatewayId,
		BMCType:           bmcType,
		Features:          req.Msg.Features,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	err = h.db.Locations.Create(ctx, location)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to register server location: %w", err))
	}

	response := &managerv1.RegisterServerResponse{
		Success: true,
		Message: fmt.Sprintf("Server %s registered successfully", req.Msg.ServerId),
	}
	return connect.NewResponse(response), nil
}

// GetServerLocation resolves which gateway handles a specific server
func (h *BMCManagerServiceHandler) GetServerLocation(
	ctx context.Context,
	req *connect.Request[managerv1.GetServerLocationRequest],
) (*connect.Response[managerv1.GetServerLocationResponse], error) {
	// Get customer ID from JWT claims (set by auth interceptor)
	_, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// Get server location from database
	location, err := h.db.Locations.Get(ctx, req.Msg.ServerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %w", err))
	}

	// TODO: Replace with proper server-customer mapping check using ServerCustomerMapping table
	// For now, allowing all authenticated customers to access all servers

	// Get gateway information
	gateway, err := h.db.Gateways.Get(ctx, location.RegionalGatewayID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get gateway info: %w", err))
	}

	// Convert BMC type to protobuf
	var bmcType managerv1.BMCType
	switch location.BMCType {
	case models.BMCTypeIPMI:
		bmcType = managerv1.BMCType_BMC_TYPE_IPMI
	case models.BMCTypeRedfish:
		bmcType = managerv1.BMCType_BMC_TYPE_REDFISH
	default:
		bmcType = managerv1.BMCType_BMC_TYPE_UNSPECIFIED
	}

	response := &managerv1.GetServerLocationResponse{
		RegionalGatewayId:       gateway.ID,
		RegionalGatewayEndpoint: gateway.Endpoint,
		DatacenterId:            location.DatacenterID,
		BmcType:                 bmcType,
		Features:                location.Features,
	}

	return connect.NewResponse(response), nil
}

// RegisterGateway allows gateways to register and announce their capabilities
func (h *BMCManagerServiceHandler) RegisterGateway(
	ctx context.Context,
	req *connect.Request[managerv1.RegisterGatewayRequest],
) (*connect.Response[managerv1.RegisterGatewayResponse], error) {
	// Create or update gateway record (using upsert for re-registration support)
	gateway := &models.RegionalGateway{
		ID:            req.Msg.GatewayId,
		Region:        req.Msg.Region,
		Endpoint:      req.Msg.Endpoint,
		DatacenterIDs: req.Msg.DatacenterIds,
		Status:        "active",
		LastSeen:      time.Now(),
		CreatedAt:     time.Now(),
	}

	err := h.db.Gateways.Upsert(ctx, gateway)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to register gateway: %w", err))
	}

	response := &managerv1.RegisterGatewayResponse{
		Success: true,
		Message: fmt.Sprintf("Gateway %s registered successfully", req.Msg.GatewayId),
	}
	return connect.NewResponse(response), nil
}

// ListGateways returns available gateways, optionally filtered by region
func (h *BMCManagerServiceHandler) ListGateways(
	ctx context.Context,
	req *connect.Request[managerv1.ListGatewaysRequest],
) (*connect.Response[managerv1.ListGatewaysResponse], error) {
	// Get gateways from database
	gateways, err := h.db.Gateways.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list gateways: %w", err))
	}

	// Filter by region if specified
	if req.Msg.Region != "" {
		filtered := make([]*models.RegionalGateway, 0)
		for _, g := range gateways {
			if g.Region == req.Msg.Region {
				filtered = append(filtered, g)
			}
		}
		gateways = filtered
	}

	// Get customer ID from context for delegated token generation
	customerID, ok := ctx.Value("customer_id").(string)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("customer not authenticated"))
	}

	// Convert to protobuf format
	var protoGateways []*managerv1.RegionalGateway
	for _, gateway := range gateways {
		// Generate a delegated token for this customer to access this gateway
		customer := &models.Customer{
			ID: customerID,
		}
		delegatedToken, err := h.jwtManager.GenerateToken(customer)
		if err != nil {
			log.Warn().Err(err).Str("gateway_id", gateway.ID).Msg("Failed to generate delegated token")
			delegatedToken = "" // Continue without token rather than failing completely
		}

		protoGateway := &managerv1.RegionalGateway{
			Id:             gateway.ID,
			Region:         gateway.Region,
			Endpoint:       gateway.Endpoint,
			DatacenterIds:  gateway.DatacenterIDs,
			Status:         gateway.Status,
			LastSeen:       timestamppb.New(gateway.LastSeen),
			CreatedAt:      timestamppb.New(gateway.CreatedAt),
			DelegatedToken: delegatedToken, // Include delegated token for gateway access
		}
		protoGateways = append(protoGateways, protoGateway)
	}

	response := &managerv1.ListGatewaysResponse{
		Gateways: protoGateways,
	}
	return connect.NewResponse(response), nil
}

// GetSystemStatus returns comprehensive system status for admin monitoring
func (h *BMCManagerServiceHandler) GetSystemStatus(
	ctx context.Context,
	req *connect.Request[managerv1.GetSystemStatusRequest],
) (*connect.Response[managerv1.GetSystemStatusResponse], error) {
	// Note: For simplicity, not requiring authentication for this endpoint
	// In production, this should require admin authentication

	// Get all gateways from database
	gateways, err := h.db.Gateways.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list gateways: %w", err))
	}

	// Get all servers with BMC endpoints from database
	locations, err := h.db.Locations.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list locations: %w", err))
	}

	servers, err := h.db.Servers.ListAll(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list servers: %w", err))
	}

	// Create a map of servers by ID for quick lookup
	serverMap := make(map[string]*models.Server)
	for _, s := range servers {
		serverMap[s.ID] = s
	}

	// Merge location and server data
	type serverWithBMC struct {
		ServerID          string
		CustomerID        string
		DatacenterID      string
		RegionalGatewayID string
		BMCType           models.BMCType
		BMCEndpoint       string
		Features          []string
		CreatedAt         time.Time
		UpdatedAt         time.Time
	}

	allServers := make([]serverWithBMC, 0, len(locations))
	for _, loc := range locations {
		server, exists := serverMap[loc.ServerID]

		bmcEndpoint := ""
		bmcType := loc.BMCType

		if exists && server.ControlEndpoint != nil {
			bmcEndpoint = server.ControlEndpoint.Endpoint
			bmcType = server.ControlEndpoint.Type
		}

		allServers = append(allServers, serverWithBMC{
			ServerID:          loc.ServerID,
			CustomerID:        loc.CustomerID,
			DatacenterID:      loc.DatacenterID,
			RegionalGatewayID: loc.RegionalGatewayID,
			BMCType:           bmcType,
			BMCEndpoint:       bmcEndpoint,
			Features:          loc.Features,
			CreatedAt:         loc.CreatedAt,
			UpdatedAt:         loc.UpdatedAt,
		})
	}

	// Debug: log what we retrieved
	log.Debug().Int("count", len(allServers)).Msg("Found servers")
	for i, server := range allServers {
		log.Debug().
			Int("index", i).
			Str("server_id", server.ServerID).
			Str("bmc_endpoint", server.BMCEndpoint).
			Str("customer_id", server.CustomerID).
			Msg("Server record")
	}

	// Build gateway status information
	var gatewayStatuses []*managerv1.GatewayStatus
	activeGateways := 0
	cutoffTime := time.Now().Add(-2 * time.Minute) // Consider gateways active if seen within 2 minutes

	for _, gateway := range gateways {
		// Check if gateway is active
		isActive := gateway.LastSeen.After(cutoffTime)
		if isActive {
			activeGateways++
		}

		// Find servers for this gateway
		var gatewayServers []*managerv1.SystemStatusServerEntry
		for _, server := range allServers {
			if server.RegionalGatewayID == gateway.ID {
				gatewayServers = append(gatewayServers, &managerv1.SystemStatusServerEntry{
					ServerId:          server.ServerID,
					CustomerId:        server.CustomerID,
					DatacenterId:      server.DatacenterID,
					RegionalGatewayId: server.RegionalGatewayID,
					BmcType:           convertBMCTypeToProto(server.BMCType),
					Features:          server.Features,
					CreatedAt:         timestamppb.New(server.CreatedAt),
					UpdatedAt:         timestamppb.New(server.UpdatedAt),
					BmcEndpoint:       server.BMCEndpoint,
				})
			}
		}

		gatewayStatus := &managerv1.GatewayStatus{
			Id:            gateway.ID,
			Region:        gateway.Region,
			Endpoint:      gateway.Endpoint,
			DatacenterIds: gateway.DatacenterIDs,
			Status:        gateway.Status,
			LastSeen:      timestamppb.New(gateway.LastSeen),
			CreatedAt:     timestamppb.New(gateway.CreatedAt),
			ServerCount:   int32(len(gatewayServers)),
			Servers:       gatewayServers,
		}
		gatewayStatuses = append(gatewayStatuses, gatewayStatus)
	}

	// Build overall server list
	var allServerEntries []*managerv1.SystemStatusServerEntry
	for _, server := range allServers {
		allServerEntries = append(allServerEntries, &managerv1.SystemStatusServerEntry{
			ServerId:          server.ServerID,
			CustomerId:        server.CustomerID,
			DatacenterId:      server.DatacenterID,
			RegionalGatewayId: server.RegionalGatewayID,
			BmcType:           convertBMCTypeToProto(server.BMCType),
			Features:          server.Features,
			CreatedAt:         timestamppb.New(server.CreatedAt),
			UpdatedAt:         timestamppb.New(server.UpdatedAt),
			BmcEndpoint:       server.BMCEndpoint,
		})
	}

	// Build system status
	systemStatus := &managerv1.SystemStatus{
		Version:        getServiceVersion(),
		StartedAt:      timestamppb.New(h.startTime),
		StatusTime:     timestamppb.New(time.Now()),
		TotalGateways:  int32(len(gateways)),
		ActiveGateways: int32(activeGateways),
		TotalServers:   int32(len(allServers)),
		Gateways:       gatewayStatuses,
		Servers:        allServerEntries,
	}

	response := &managerv1.GetSystemStatusResponse{
		Status: systemStatus,
	}
	return connect.NewResponse(response), nil
}

// Helper function to convert BMCType from models to protobuf
func convertBMCTypeToProto(bmcType models.BMCType) managerv1.BMCType {
	switch bmcType {
	case models.BMCTypeIPMI:
		return managerv1.BMCType_BMC_TYPE_IPMI
	case models.BMCTypeRedfish:
		return managerv1.BMCType_BMC_TYPE_REDFISH
	default:
		return managerv1.BMCType_BMC_TYPE_UNSPECIFIED
	}
}

// Helper function to convert SOLType from models to protobuf
func convertSOLTypeToProto(solType models.SOLType) managerv1.SOLType {
	switch solType {
	case models.SOLTypeIPMI:
		return managerv1.SOLType_SOL_TYPE_IPMI
	case models.SOLTypeRedfishSerial:
		return managerv1.SOLType_SOL_TYPE_REDFISH_SERIAL
	default:
		return managerv1.SOLType_SOL_TYPE_UNSPECIFIED
	}
}

// Helper function to convert VNCType from models to protobuf
func convertVNCTypeToProto(vncType models.VNCType) managerv1.VNCType {
	switch vncType {
	case models.VNCTypeNative:
		return managerv1.VNCType_VNC_TYPE_NATIVE
	case models.VNCTypeWebSocket:
		return managerv1.VNCType_VNC_TYPE_WEBSOCKET
	default:
		return managerv1.VNCType_VNC_TYPE_UNSPECIFIED
	}
}

// Helper function to get service version
func getServiceVersion() string {
	return fmt.Sprintf("BMC Manager v1.0.0 (Go %s)", runtime.Version())
}

// GetServer retrieves detailed information about a specific server
// Moved from gateway in BMC-centric architecture
func (h *BMCManagerServiceHandler) GetServer(
	ctx context.Context,
	req *connect.Request[managerv1.GetServerRequest],
) (*connect.Response[managerv1.GetServerResponse], error) {
	// Get customer ID from JWT claims (set by auth interceptor)
	_, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// TEMPORARY IMPLEMENTATION: Get server by ID (allowing all customers access to all servers)
	// TODO: Replace with proper server-customer mapping check using ServerCustomerMapping table
	// TODO: Implement proper ownership validation for GetServer operation
	server, err := h.db.Servers.Get(ctx, req.Msg.ServerId)
	if err != nil {
		if err.Error() == "server not found" {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get server: %w", err))
	}

	// Convert to protobuf format
	protoServer := &managerv1.Server{
		Id:           server.ID,
		CustomerId:   server.CustomerID,
		DatacenterId: server.DatacenterID,
		Features:     server.Features,
		Status:       server.Status,
		CreatedAt:    timestamppb.New(server.CreatedAt),
		UpdatedAt:    timestamppb.New(server.UpdatedAt),
		Metadata:     server.Metadata,
	}

	// Convert control endpoint
	if server.ControlEndpoint != nil {
		protoServer.ControlEndpoint = &managerv1.BMCControlEndpoint{
			Endpoint:     server.ControlEndpoint.Endpoint,
			Type:         convertBMCTypeToProto(server.ControlEndpoint.Type),
			Username:     server.ControlEndpoint.Username,
			Password:     server.ControlEndpoint.Password,
			Capabilities: server.ControlEndpoint.Capabilities,
		}
		if server.ControlEndpoint.TLS != nil {
			protoServer.ControlEndpoint.Tls = &managerv1.TLSConfig{
				Enabled:            server.ControlEndpoint.TLS.Enabled,
				InsecureSkipVerify: server.ControlEndpoint.TLS.InsecureSkipVerify,
				CaCert:             server.ControlEndpoint.TLS.CACert,
			}
		}
	}

	// Convert SOL endpoint
	if server.SOLEndpoint != nil {
		protoServer.SolEndpoint = &managerv1.SOLEndpoint{
			Type:     convertSOLTypeToProto(server.SOLEndpoint.Type),
			Endpoint: server.SOLEndpoint.Endpoint,
			Username: server.SOLEndpoint.Username,
			Password: server.SOLEndpoint.Password,
		}
		if server.SOLEndpoint.Config != nil {
			protoServer.SolEndpoint.Config = &managerv1.SOLConfig{
				BaudRate:       int32(server.SOLEndpoint.Config.BaudRate),
				FlowControl:    server.SOLEndpoint.Config.FlowControl,
				TimeoutSeconds: int32(server.SOLEndpoint.Config.TimeoutSeconds),
			}
		}
	}

	// Convert VNC endpoint
	if server.VNCEndpoint != nil {
		protoServer.VncEndpoint = &managerv1.VNCEndpoint{
			Type:     convertVNCTypeToProto(server.VNCEndpoint.Type),
			Endpoint: server.VNCEndpoint.Endpoint,
			Username: server.VNCEndpoint.Username,
			Password: server.VNCEndpoint.Password,
		}
		if server.VNCEndpoint.Config != nil {
			protoServer.VncEndpoint.Config = &managerv1.VNCConfig{
				Protocol: server.VNCEndpoint.Config.Protocol,
				Path:     server.VNCEndpoint.Config.Path,
				Display:  int32(server.VNCEndpoint.Config.Display),
				ReadOnly: server.VNCEndpoint.Config.ReadOnly,
			}
		}
	}

	resp := &managerv1.GetServerResponse{
		Server: protoServer,
	}

	return connect.NewResponse(resp), nil
}

// ListServers returns all servers accessible by the authenticated customer
// Moved from gateway in BMC-centric architecture
func (h *BMCManagerServiceHandler) ListServers(
	ctx context.Context,
	req *connect.Request[managerv1.ListServersRequest],
) (*connect.Response[managerv1.ListServersResponse], error) {
	// Get customer ID from JWT claims (set by auth interceptor)
	_, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// Set pagination defaults
	pageSize := req.Msg.PageSize
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 50 // Default page size
	}

	// For now, show all servers to any authenticated customer
	// TODO: Replace with proper server-customer mapping logic
	servers, err := h.db.Servers.ListAll(ctx)
	nextPageToken := "" // Disable pagination for simplicity
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list servers: %w", err))
	}

	// Convert to protobuf format
	var protoServers []*managerv1.Server
	for _, server := range servers {
		protoServer := &managerv1.Server{
			Id:           server.ID,
			CustomerId:   server.CustomerID,
			DatacenterId: server.DatacenterID,
			Features:     server.Features,
			Status:       server.Status,
			CreatedAt:    timestamppb.New(server.CreatedAt),
			UpdatedAt:    timestamppb.New(server.UpdatedAt),
			Metadata:     server.Metadata,
		}

		// Convert control endpoint
		if server.ControlEndpoint != nil {
			protoServer.ControlEndpoint = &managerv1.BMCControlEndpoint{
				Endpoint:     server.ControlEndpoint.Endpoint,
				Type:         convertBMCTypeToProto(server.ControlEndpoint.Type),
				Username:     server.ControlEndpoint.Username,
				Password:     server.ControlEndpoint.Password,
				Capabilities: server.ControlEndpoint.Capabilities,
			}
			if server.ControlEndpoint.TLS != nil {
				protoServer.ControlEndpoint.Tls = &managerv1.TLSConfig{
					Enabled:            server.ControlEndpoint.TLS.Enabled,
					InsecureSkipVerify: server.ControlEndpoint.TLS.InsecureSkipVerify,
					CaCert:             server.ControlEndpoint.TLS.CACert,
				}
			}
		}

		// Convert SOL endpoint
		if server.SOLEndpoint != nil {
			protoServer.SolEndpoint = &managerv1.SOLEndpoint{
				Type:     convertSOLTypeToProto(server.SOLEndpoint.Type),
				Endpoint: server.SOLEndpoint.Endpoint,
				Username: server.SOLEndpoint.Username,
				Password: server.SOLEndpoint.Password,
			}
			if server.SOLEndpoint.Config != nil {
				protoServer.SolEndpoint.Config = &managerv1.SOLConfig{
					BaudRate:       int32(server.SOLEndpoint.Config.BaudRate),
					FlowControl:    server.SOLEndpoint.Config.FlowControl,
					TimeoutSeconds: int32(server.SOLEndpoint.Config.TimeoutSeconds),
				}
			}
		}

		// Convert VNC endpoint
		if server.VNCEndpoint != nil {
			protoServer.VncEndpoint = &managerv1.VNCEndpoint{
				Type:     convertVNCTypeToProto(server.VNCEndpoint.Type),
				Endpoint: server.VNCEndpoint.Endpoint,
				Username: server.VNCEndpoint.Username,
				Password: server.VNCEndpoint.Password,
			}
			if server.VNCEndpoint.Config != nil {
				protoServer.VncEndpoint.Config = &managerv1.VNCConfig{
					Protocol: server.VNCEndpoint.Config.Protocol,
					Path:     server.VNCEndpoint.Config.Path,
					Display:  int32(server.VNCEndpoint.Config.Display),
					ReadOnly: server.VNCEndpoint.Config.ReadOnly,
				}
			}
		}

		protoServers = append(protoServers, protoServer)
	}

	resp := &managerv1.ListServersResponse{
		Servers:       protoServers,
		NextPageToken: nextPageToken,
	}

	return connect.NewResponse(resp), nil
}

// ReportAvailableEndpoints allows gateways to report BMC endpoints they can proxy
// This establishes the BMC endpoint to gateway mapping for routing decisions
func (h *BMCManagerServiceHandler) ReportAvailableEndpoints(
	ctx context.Context,
	req *connect.Request[managerv1.ReportAvailableEndpointsRequest],
) (*connect.Response[managerv1.ReportAvailableEndpointsResponse], error) {
	log.Info().
		Str("gateway_id", req.Msg.GatewayId).
		Int("endpoint_count", len(req.Msg.BmcEndpoints)).
		Str("region", req.Msg.Region).
		Msg("Gateway reporting BMC endpoints")

	// Store BMC endpoint availability in database
	for _, endpoint := range req.Msg.BmcEndpoints {
		log.Debug().
			Str("bmc_endpoint", endpoint.BmcEndpoint).
			Str("agent_id", endpoint.AgentId).
			Str("datacenter_id", endpoint.DatacenterId).
			Str("bmc_type", endpoint.BmcType.String()).
			Str("status", endpoint.Status).
			Msg("BMC endpoint reported")

		// Check if there's an existing server location for this BMC endpoint
		// We need to find any server that matches this BMC endpoint and update it
		if err := h.updateServerWithBMCEndpoint(ctx, endpoint, req.Msg.GatewayId); err != nil {
			log.Warn().Err(err).Str("bmc_endpoint", endpoint.BmcEndpoint).Msg("Failed to update server with BMC endpoint")
			// Continue processing other endpoints even if one fails
		}
	}

	resp := &managerv1.ReportAvailableEndpointsResponse{
		Success: true,
		Message: fmt.Sprintf("Recorded %d BMC endpoints from gateway %s", len(req.Msg.BmcEndpoints), req.Msg.GatewayId),
	}

	return connect.NewResponse(resp), nil
}

// updateServerWithBMCEndpoint creates or updates server records with BMC endpoint information
// from gateway endpoint reports
func (h *BMCManagerServiceHandler) updateServerWithBMCEndpoint(ctx context.Context, endpoint *managerv1.BMCEndpointAvailability, gatewayID string) error {
	// Convert BMC type from protobuf to models
	var bmcType models.BMCType
	switch endpoint.BmcType {
	case managerv1.BMCType_BMC_TYPE_IPMI:
		bmcType = models.BMCTypeIPMI
	case managerv1.BMCType_BMC_TYPE_REDFISH:
		bmcType = models.BMCTypeRedfish
	default:
		bmcType = models.BMCTypeIPMI // Default fallback
	}

	// For servers reported by gateways, we need to create a synthetic server ID
	// based on the BMC endpoint since gateways don't have server concepts
	serverID := models.GenerateServerIDFromBMCEndpoint(endpoint.DatacenterId, endpoint.BmcEndpoint)

	// Create or update server record
	controlEndpoint := &models.BMCControlEndpoint{
		Endpoint:     endpoint.BmcEndpoint,
		Type:         bmcType,
		Username:     endpoint.Username,
		Capabilities: endpoint.Capabilities,
	}

	server := &models.Server{
		ID:              serverID,
		CustomerID:      "system", // System-managed servers from gateway reports
		DatacenterID:    endpoint.DatacenterId,
		ControlEndpoint: controlEndpoint,
		Features:        endpoint.Features,
		Status:          endpoint.Status,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Populate SOL/Console endpoint if feature is present
	log.Debug().
		Str("server_id", serverID).
		Strs("features", endpoint.Features).
		Msg("Processing features for endpoint population (from gateway)")

	for _, feature := range endpoint.Features {
		if feature == types.FeatureConsole.String() {
			// Determine SOL type based on BMC type
			solType := models.SOLTypeIPMI
			if bmcType == models.BMCTypeRedfish {
				solType = models.SOLTypeRedfishSerial
			}
			server.SOLEndpoint = &models.SOLEndpoint{
				Type:     solType,
				Endpoint: endpoint.BmcEndpoint,
				Username: endpoint.Username,
				Password: "", // Will be filled later
			}
			log.Debug().
				Str("server_id", serverID).
				Str("sol_type", string(solType)).
				Msg("Created SOL endpoint (from gateway)")
			break
		}
	}

	// Populate VNC endpoint if feature is present
	for _, feature := range endpoint.Features {
		if feature == types.FeatureVNC.String() {
			server.VNCEndpoint = &models.VNCEndpoint{
				Type:     models.VNCTypeNative, // Default to native VNC
				Endpoint: endpoint.BmcEndpoint,
				Username: endpoint.Username,
				Password: "", // Will be filled later
			}
			log.Debug().
				Str("server_id", serverID).
				Msg("Created VNC endpoint (from gateway)")
			break
		}
	}

	// Check if server already exists
	existing, err := h.db.Servers.Get(ctx, serverID)
	if err != nil && err.Error() != "server not found" {
		return fmt.Errorf("failed to check existing server: %w", err)
	}

	if existing != nil {
		// Server exists, update it
		if err := h.db.Servers.Update(ctx, server); err != nil {
			return fmt.Errorf("failed to update server record: %w", err)
		}
	} else {
		// Server doesn't exist, create it
		if err := h.db.Servers.Create(ctx, server); err != nil {
			return fmt.Errorf("failed to create server record: %w", err)
		}
	}

	// Also create/update server location mapping
	location := &models.ServerLocation{
		ServerID:          serverID,
		CustomerID:        "system", // System-managed servers from gateway reports
		DatacenterID:      endpoint.DatacenterId,
		RegionalGatewayID: gatewayID,
		BMCType:           bmcType,
		Features:          endpoint.Features,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Use Upsert for location since it has that method
	if err := h.db.Locations.Upsert(ctx, location); err != nil {
		return fmt.Errorf("failed to create/update server location: %w", err)
	}

	log.Info().
		Str("server_id", serverID).
		Str("bmc_endpoint", endpoint.BmcEndpoint).
		Msg("Created/updated server location")
	return nil
}
