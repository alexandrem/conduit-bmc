package manager

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"

	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"
	managerv1 "manager/gen/manager/v1"
	"manager/internal/database"
	"manager/pkg/auth"
	"manager/pkg/models"
)

// AdminServiceHandler handles admin dashboard operations
type AdminServiceHandler struct {
	db         *database.BunDB
	jwtManager *auth.JWTManager
}

// NewAdminServiceHandler creates a new admin service handler
func NewAdminServiceHandler(db *database.BunDB, jwtManager *auth.JWTManager) *AdminServiceHandler {
	return &AdminServiceHandler{
		db:         db,
		jwtManager: jwtManager,
	}
}

// GetDashboardMetrics returns aggregated metrics for the admin dashboard
func (h *AdminServiceHandler) GetDashboardMetrics(
	ctx context.Context,
	req *connect.Request[managerv1.GetDashboardMetricsRequest],
) (*connect.Response[managerv1.GetDashboardMetricsResponse], error) {
	log.Info().Msg("GetDashboardMetrics called")

	metrics, err := h.db.Admin.GetDashboardMetrics(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get dashboard metrics")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get metrics: %w", err))
	}

	return connect.NewResponse(metrics), nil
}

// ListAllServers returns all servers across all customers with filtering
func (h *AdminServiceHandler) ListAllServers(
	ctx context.Context,
	req *connect.Request[managerv1.ListAllServersRequest],
) (*connect.Response[managerv1.ListAllServersResponse], error) {
	log.Info().
		Str("customer_filter", req.Msg.CustomerFilter).
		Strs("region_filter", req.Msg.RegionFilter).
		Str("gateway_filter", req.Msg.GatewayFilter).
		Str("status_filter", req.Msg.StatusFilter).
		Int32("page_size", req.Msg.PageSize).
		Msg("ListAllServers called")

	// Apply default page size
	pageSize := req.Msg.PageSize
	if pageSize == 0 {
		pageSize = 100
	} else if pageSize > 500 {
		pageSize = 500
	}

	// Calculate offset from page token (simplified - in production use proper pagination)
	offset := int32(0)
	// TODO: Implement proper page token parsing

	filters := &database.AdminServerFilters{
		CustomerFilter: req.Msg.CustomerFilter,
		RegionFilter:   req.Msg.RegionFilter,
		GatewayFilter:  req.Msg.GatewayFilter,
		StatusFilter:   req.Msg.StatusFilter,
		PageSize:       pageSize,
		Offset:         offset,
	}

	servers, err := h.db.Admin.ListAllServersWithFilters(ctx, filters)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list all servers")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list servers: %w", err))
	}

	totalCount, err := h.db.Admin.GetTotalServersCount(ctx, filters)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total servers count")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get total count: %w", err))
	}

	response := &managerv1.ListAllServersResponse{
		Servers:       servers,
		NextPageToken: "", // TODO: Implement proper pagination tokens
		TotalCount:    totalCount,
	}

	return connect.NewResponse(response), nil
}

// ListAllCustomers returns all customers with server counts
func (h *AdminServiceHandler) ListAllCustomers(
	ctx context.Context,
	req *connect.Request[managerv1.ListAllCustomersRequest],
) (*connect.Response[managerv1.ListAllCustomersResponse], error) {
	log.Info().Int32("page_size", req.Msg.PageSize).Msg("ListAllCustomers called")

	// Apply default page size
	pageSize := req.Msg.PageSize
	if pageSize == 0 {
		pageSize = 100
	} else if pageSize > 500 {
		pageSize = 500
	}

	// Calculate offset from page token (simplified)
	offset := int32(0)
	// TODO: Implement proper page token parsing

	customers, err := h.db.Admin.ListAllCustomersWithCounts(ctx, pageSize, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list all customers")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list customers: %w", err))
	}

	response := &managerv1.ListAllCustomersResponse{
		Customers:     customers,
		NextPageToken: "", // TODO: Implement proper pagination tokens
	}

	return connect.NewResponse(response), nil
}

// GetGatewayHealth returns health information for all gateways
func (h *AdminServiceHandler) GetGatewayHealth(
	ctx context.Context,
	req *connect.Request[managerv1.GetGatewayHealthRequest],
) (*connect.Response[managerv1.GetGatewayHealthResponse], error) {
	log.Info().Msg("GetGatewayHealth called")

	gateways, err := h.db.Admin.GetGatewayHealth(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get gateway health")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get gateway health: %w", err))
	}

	response := &managerv1.GetGatewayHealthResponse{
		Gateways: gateways,
	}

	return connect.NewResponse(response), nil
}

// GetRegions returns available regions for filtering
func (h *AdminServiceHandler) GetRegions(
	ctx context.Context,
	req *connect.Request[managerv1.GetRegionsRequest],
) (*connect.Response[managerv1.GetRegionsResponse], error) {
	log.Info().Msg("GetRegions called")

	regions, err := h.db.Admin.GetRegions(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get regions")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get regions: %w", err))
	}

	response := &managerv1.GetRegionsResponse{
		Regions: regions,
	}

	return connect.NewResponse(response), nil
}

// LaunchVNCSession creates a VNC session for admin console access
func (h *AdminServiceHandler) LaunchVNCSession(
	ctx context.Context,
	req *connect.Request[managerv1.LaunchSessionRequest],
) (*connect.Response[managerv1.LaunchSessionResponse], error) {
	log.Info().Str("server_id", req.Msg.ServerId).Msg("LaunchVNCSession called")

	// Get server location to find the gateway
	serverLocation, err := h.db.Locations.Get(ctx, req.Msg.ServerId)
	if err != nil {
		log.Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Failed to get server location")
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %w", err))
	}

	// Get gateway information
	gateway, err := h.db.Gateways.Get(ctx, serverLocation.RegionalGatewayID)
	if err != nil {
		log.Error().Err(err).Str("gateway_id", serverLocation.RegionalGatewayID).Msg("Failed to get gateway")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("gateway not found: %w", err))
	}

	// Get server-scoped token for gateway authentication
	server, err := h.db.Servers.Get(ctx, req.Msg.ServerId)
	if err != nil {
		log.Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Failed to get server")
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %w", err))
	}

	// Get admin claims from context
	claims, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// Create customer object for token generation
	customer := &models.Customer{
		ID:    claims.CustomerID,
		Email: claims.Email,
	}

	// Define permissions for VNC session
	permissions := []string{"console:access", "vnc", "read"}

	// Generate server token for gateway authentication
	log.Debug().
		Str("customer_id", customer.ID).
		Str("server_id", server.ID).
		Int("permissions_count", len(permissions)).
		Msg("Generating server token for VNC session")

	tokenString, err := h.jwtManager.GenerateServerToken(customer, server, permissions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate server token")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	tokenPrefix := tokenString
	if len(tokenString) > 20 {
		tokenPrefix = tokenString[:20] + "..."
	}
	log.Debug().
		Str("token_prefix", tokenPrefix).
		Int("token_length", len(tokenString)).
		Str("gateway_endpoint", gateway.Endpoint).
		Msg("Server token generated, calling gateway")

	// Create VNC session on gateway via direct HTTP call
	// In production, this would use a proper gateway client
	sessionResp, err := h.createGatewayVNCSession(ctx, gateway.Endpoint, req.Msg.ServerId, tokenString)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create VNC session on gateway")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create VNC session: %w", err))
	}

	response := &managerv1.LaunchSessionResponse{
		SessionId:         sessionResp.SessionId,
		WebsocketEndpoint: sessionResp.WebsocketEndpoint,
		ViewerUrl:         sessionResp.ViewerUrl,
		ExpiresAt:         sessionResp.ExpiresAt,
	}

	return connect.NewResponse(response), nil
}

// LaunchSOLSession creates a SOL session for admin console access
func (h *AdminServiceHandler) LaunchSOLSession(
	ctx context.Context,
	req *connect.Request[managerv1.LaunchSessionRequest],
) (*connect.Response[managerv1.LaunchSessionResponse], error) {
	log.Info().Str("server_id", req.Msg.ServerId).Msg("LaunchSOLSession called")

	// Get server location to find the gateway
	serverLocation, err := h.db.Locations.Get(ctx, req.Msg.ServerId)
	if err != nil {
		log.Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Failed to get server location")
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %w", err))
	}

	// Get gateway information
	gateway, err := h.db.Gateways.Get(ctx, serverLocation.RegionalGatewayID)
	if err != nil {
		log.Error().Err(err).Str("gateway_id", serverLocation.RegionalGatewayID).Msg("Failed to get gateway")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("gateway not found: %w", err))
	}

	// Get server-scoped token for gateway authentication
	server, err := h.db.Servers.Get(ctx, req.Msg.ServerId)
	if err != nil {
		log.Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Failed to get server")
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %w", err))
	}

	// Get admin claims from context
	claims, ok := ctx.Value("claims").(*models.AuthClaims)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get auth claims"))
	}

	// Create customer object for token generation
	customer := &models.Customer{
		ID:    claims.CustomerID,
		Email: claims.Email,
	}

	// Define permissions for SOL session
	permissions := []string{"console:access", "sol", "read"}

	// Generate server token for gateway authentication
	tokenString, err := h.jwtManager.GenerateServerToken(customer, server, permissions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate server token")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	// Create SOL session on gateway via direct HTTP call
	sessionResp, err := h.createGatewaySOLSession(ctx, gateway.Endpoint, req.Msg.ServerId, tokenString)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create SOL session on gateway")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create SOL session: %w", err))
	}

	response := &managerv1.LaunchSessionResponse{
		SessionId:         sessionResp.SessionId,
		WebsocketEndpoint: sessionResp.WebsocketEndpoint,
		ViewerUrl:         sessionResp.ConsoleUrl,
		ExpiresAt:         sessionResp.ExpiresAt,
	}

	return connect.NewResponse(response), nil
}

// createGatewayVNCSession creates a VNC session on the gateway
func (h *AdminServiceHandler) createGatewayVNCSession(
	ctx context.Context,
	gatewayEndpoint string,
	serverID string,
	token string,
) (*gatewayv1.CreateVNCSessionResponse, error) {
	// Create gateway client with authentication
	client := gatewayv1connect.NewGatewayServiceClient(
		http.DefaultClient,
		gatewayEndpoint,
		connect.WithInterceptors(newAuthInterceptor(token)),
	)

	// Create VNC session
	resp, err := client.CreateVNCSession(ctx, connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: serverID,
	}))
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// createGatewaySOLSession creates a SOL session on the gateway
func (h *AdminServiceHandler) createGatewaySOLSession(
	ctx context.Context,
	gatewayEndpoint string,
	serverID string,
	token string,
) (*gatewayv1.CreateSOLSessionResponse, error) {
	// Create gateway client with authentication
	client := gatewayv1connect.NewGatewayServiceClient(
		http.DefaultClient,
		gatewayEndpoint,
		connect.WithInterceptors(newAuthInterceptor(token)),
	)

	// Create SOL session
	resp, err := client.CreateSOLSession(ctx, connect.NewRequest(&gatewayv1.CreateSOLSessionRequest{
		ServerId: serverID,
	}))
	if err != nil {
		return nil, err
	}

	return resp.Msg, nil
}

// newAuthInterceptor creates an interceptor that adds Bearer token to requests
func newAuthInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}
}
