package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"core/domain"
	"core/types"
	managerv1 "manager/gen/manager/v1"
	"manager/gen/manager/v1/managerv1connect"

	"connectrpc.com/connect"

	"cli/pkg/config"
)

// BMCManagerClient handles authentication and server location resolution
type BMCManagerClient struct {
	client     managerv1connect.BMCManagerServiceClient
	config     *config.Config
	httpClient *http.Client
}

func NewBMCManagerClient(cfg *config.Config) *BMCManagerClient {
	httpClient := &http.Client{}
	client := managerv1connect.NewBMCManagerServiceClient(httpClient, cfg.Manager.Endpoint)

	return &BMCManagerClient{
		client:     client,
		config:     cfg,
		httpClient: httpClient,
	}
}

// Authenticate performs initial authentication with BMC Manager
func (c *BMCManagerClient) Authenticate(ctx context.Context, email, password string) (*AuthResult, error) {
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    email,
		Password: password,
	})

	resp, err := c.client.Authenticate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Update config with tokens
	c.config.Auth.AccessToken = resp.Msg.AccessToken
	c.config.Auth.RefreshToken = resp.Msg.RefreshToken
	c.config.Auth.Email = resp.Msg.Customer.Email
	c.config.Auth.TokenExpiresAt = resp.Msg.ExpiresAt.AsTime()

	return &AuthResult{
		AccessToken:  resp.Msg.AccessToken,
		RefreshToken: resp.Msg.RefreshToken,
		ExpiresAt:    resp.Msg.ExpiresAt.AsTime(),
		Customer: Customer{
			ID:    resp.Msg.Customer.Id,
			Email: resp.Msg.Customer.Email,
		},
	}, nil
}

// RefreshToken refreshes the access token using the refresh token
func (c *BMCManagerClient) RefreshToken(ctx context.Context) (*AuthResult, error) {
	if c.config.Auth.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	req := connect.NewRequest(&managerv1.RefreshTokenRequest{
		RefreshToken: c.config.Auth.RefreshToken,
	})

	resp, err := c.client.RefreshToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	// Update config with new token
	c.config.Auth.AccessToken = resp.Msg.AccessToken
	c.config.Auth.TokenExpiresAt = resp.Msg.ExpiresAt.AsTime()

	return &AuthResult{
		AccessToken: resp.Msg.AccessToken,
		ExpiresAt:   resp.Msg.ExpiresAt.AsTime(),
	}, nil
}

// GetServerLocation resolves which regional gateway handles a server
func (c *BMCManagerClient) GetServerLocation(ctx context.Context, serverID string) (*ServerLocation, error) {
	req := connect.NewRequest(&managerv1.GetServerLocationRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.GetServerLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server location: %w", err)
	}

	return &ServerLocation{
		ServerID:                serverID,
		RegionalGatewayID:       resp.Msg.RegionalGatewayId,
		RegionalGatewayEndpoint: resp.Msg.RegionalGatewayEndpoint,
		DatacenterID:            resp.Msg.DatacenterId,
		PrimaryProtocol:         resp.Msg.PrimaryProtocol.String(),
		Features:                resp.Msg.Features,
	}, nil
}

// ListGateways returns all available regional gateways
func (c *BMCManagerClient) ListGateways(ctx context.Context) ([]RegionalGateway, error) {
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{})

	// Explicitly add authorization header
	if c.config.Auth.AccessToken != "" {
		req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", c.config.Auth.AccessToken))
	}

	resp, err := c.client.ListGateways(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}

	var gateways []RegionalGateway
	for _, gateway := range resp.Msg.Gateways {
		gateways = append(gateways, RegionalGateway{
			ID:             gateway.Id,
			Region:         gateway.Region,
			Endpoint:       gateway.Endpoint,
			DatacenterIDs:  gateway.DatacenterIds,
			Status:         gateway.Status,
			DelegatedToken: gateway.DelegatedToken,
		})
	}

	return gateways, nil
}

// ListServers returns all servers accessible to the authenticated customer
func (c *BMCManagerClient) ListServers(ctx context.Context) ([]domain.Server, error) {
	req := connect.NewRequest(&managerv1.ListServersRequest{})
	c.addAuthHeaders(req)

	resp, err := c.client.ListServers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	var servers []domain.Server
	for _, server := range resp.Msg.Servers {
		clientServer := domain.Server{
			ID:           server.Id,
			CustomerID:   server.CustomerId,
			DatacenterID: server.DatacenterId,
			Features:     server.Features,
			Status:       server.Status,
			Metadata:     server.Metadata,
		}

		// Convert control endpoints
		clientServer.ControlEndpoints = make([]*types.BMCControlEndpoint, 0, len(server.ControlEndpoints))
		for _, endpoint := range server.ControlEndpoints {
			bmcEndpoint := &types.BMCControlEndpoint{
				Endpoint:     endpoint.Endpoint,
				Type:         types.BMCType(endpoint.Type),
				Username:     endpoint.Username,
				Password:     endpoint.Password,
				Capabilities: endpoint.Capabilities,
			}
			if endpoint.Tls != nil {
				bmcEndpoint.TLS = &types.TLSConfig{
					Enabled:            endpoint.Tls.Enabled,
					InsecureSkipVerify: endpoint.Tls.InsecureSkipVerify,
					CACert:             endpoint.Tls.CaCert,
				}
			}
			clientServer.ControlEndpoints = append(clientServer.ControlEndpoints, bmcEndpoint)
		}
		clientServer.PrimaryProtocol = types.BMCType(server.PrimaryProtocol)

		// Convert SOL endpoint
		if server.SolEndpoint != nil {
			clientServer.SOLEndpoint = &types.SOLEndpoint{
				Type:     types.SOLType(server.SolEndpoint.Type),
				Endpoint: server.SolEndpoint.Endpoint,
				Username: server.SolEndpoint.Username,
				Password: server.SolEndpoint.Password,
			}
			if server.SolEndpoint.Config != nil {
				clientServer.SOLEndpoint.Config = &types.SOLConfig{
					BaudRate:       int(server.SolEndpoint.Config.BaudRate),
					FlowControl:    server.SolEndpoint.Config.FlowControl,
					TimeoutSeconds: int(server.SolEndpoint.Config.TimeoutSeconds),
				}
			}
		}

		// Convert VNC endpoint
		if server.VncEndpoint != nil {
			clientServer.VNCEndpoint = &types.VNCEndpoint{
				Type:     types.VNCType(server.VncEndpoint.Type),
				Endpoint: server.VncEndpoint.Endpoint,
				Username: server.VncEndpoint.Username,
				Password: server.VncEndpoint.Password,
			}
			if server.VncEndpoint.Config != nil {
				clientServer.VNCEndpoint.Config = &types.VNCConfig{
					Protocol: server.VncEndpoint.Config.Protocol,
					Path:     server.VncEndpoint.Config.Path,
					Display:  int(server.VncEndpoint.Config.Display),
					ReadOnly: server.VncEndpoint.Config.ReadOnly,
				}
			}
		}

		servers = append(servers, clientServer)
	}

	return servers, nil
}

// GetServer returns detailed information about a specific server
func (c *BMCManagerClient) GetServer(ctx context.Context, serverID string) (*domain.Server, error) {
	req := connect.NewRequest(&managerv1.GetServerRequest{
		ServerId: serverID,
	})
	c.addAuthHeaders(req)

	resp, err := c.client.GetServer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	server := resp.Msg.Server
	clientServer := &domain.Server{
		ID:           server.Id,
		CustomerID:   server.CustomerId,
		DatacenterID: server.DatacenterId,
		Features:     server.Features,
		Status:       server.Status,
		Metadata:     server.Metadata,
	}

	// Convert control endpoints
	clientServer.ControlEndpoints = make([]*types.BMCControlEndpoint, 0, len(server.ControlEndpoints))
	for _, endpoint := range server.ControlEndpoints {
		bmcEndpoint := &types.BMCControlEndpoint{
			Endpoint:     endpoint.Endpoint,
			Type:         convertProtoBMCTypeToTypes(endpoint.Type),
			Username:     endpoint.Username,
			Password:     endpoint.Password,
			Capabilities: endpoint.Capabilities,
		}
		if endpoint.Tls != nil {
			bmcEndpoint.TLS = &types.TLSConfig{
				Enabled:            endpoint.Tls.Enabled,
				InsecureSkipVerify: endpoint.Tls.InsecureSkipVerify,
				CACert:             endpoint.Tls.CaCert,
			}
		}
		clientServer.ControlEndpoints = append(clientServer.ControlEndpoints, bmcEndpoint)
	}
	clientServer.PrimaryProtocol = convertProtoBMCTypeToTypes(server.PrimaryProtocol)

	// Convert SOL endpoint
	if server.SolEndpoint != nil {
		clientServer.SOLEndpoint = &types.SOLEndpoint{
			Type:     convertProtoSOLTypeToTypes(server.SolEndpoint.Type),
			Endpoint: server.SolEndpoint.Endpoint,
			Username: server.SolEndpoint.Username,
			Password: server.SolEndpoint.Password,
		}
		if server.SolEndpoint.Config != nil {
			clientServer.SOLEndpoint.Config = &types.SOLConfig{
				BaudRate:       int(server.SolEndpoint.Config.BaudRate),
				FlowControl:    server.SolEndpoint.Config.FlowControl,
				TimeoutSeconds: int(server.SolEndpoint.Config.TimeoutSeconds),
			}
		}
	}

	// Convert VNC endpoint
	if server.VncEndpoint != nil {
		clientServer.VNCEndpoint = &types.VNCEndpoint{
			Type:     convertProtoVNCTypeToTypes(server.VncEndpoint.Type),
			Endpoint: server.VncEndpoint.Endpoint,
			Username: server.VncEndpoint.Username,
			Password: server.VncEndpoint.Password,
		}
		if server.VncEndpoint.Config != nil {
			clientServer.VNCEndpoint.Config = &types.VNCConfig{
				Protocol: server.VncEndpoint.Config.Protocol,
				Path:     server.VncEndpoint.Config.Path,
				Display:  int(server.VncEndpoint.Config.Display),
				ReadOnly: server.VncEndpoint.Config.ReadOnly,
			}
		}
	}

	// Convert discovery metadata
	if server.DiscoveryMetadata != nil {
		clientServer.DiscoveryMetadata = types.ConvertDiscoveryMetadataFromProto(server.DiscoveryMetadata)
	}

	return clientServer, nil
}

// EnsureValidToken checks if token is valid and refreshes if needed
func (c *BMCManagerClient) EnsureValidToken(ctx context.Context) error {
	// Check if we have an access token
	if c.config.Auth.AccessToken == "" {
		return fmt.Errorf("no access token available - please run 'bmc-cli auth login' to authenticate")
	}

	// Check if token is expired (using UTC for consistency)
	if time.Now().UTC().After(c.config.Auth.TokenExpiresAt.UTC()) {
		return fmt.Errorf("access token expired at %v - please run 'bmc-cli auth login' to re-authenticate", c.config.Auth.TokenExpiresAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// GetServerToken generates a server-specific token with encrypted BMC context
func (c *BMCManagerClient) GetServerToken(ctx context.Context, serverID string) (*ServerTokenResult, error) {
	req := connect.NewRequest(&managerv1.GetServerTokenRequest{
		ServerId: serverID,
	})
	c.addAuthHeaders(req)

	resp, err := c.client.GetServerToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server token: %w", err)
	}

	return &ServerTokenResult{
		Token:     resp.Msg.Token,
		ExpiresAt: resp.Msg.ExpiresAt.AsTime(),
	}, nil
}

func addAuthHeadersManager[T any](req *connect.Request[T], token string) {
	if token != "" {
		req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
}

func (c *BMCManagerClient) addAuthHeaders(req interface{}) {
	switch r := req.(type) {
	case *connect.Request[managerv1.GetServerLocationRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	case *connect.Request[managerv1.ListGatewaysRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	case *connect.Request[managerv1.RefreshTokenRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	case *connect.Request[managerv1.ListServersRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	case *connect.Request[managerv1.GetServerRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	case *connect.Request[managerv1.GetServerTokenRequest]:
		addAuthHeadersManager(r, c.config.Auth.AccessToken)
	}
}

// Data types
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Customer     Customer
}

type Customer struct {
	ID    string
	Email string
}

type ServerLocation struct {
	ServerID                string
	RegionalGatewayID       string
	RegionalGatewayEndpoint string
	DatacenterID            string
	PrimaryProtocol         string
	Features                []string
}

type RegionalGateway struct {
	ID             string
	Region         string
	Endpoint       string
	DatacenterIDs  []string
	Status         string
	DelegatedToken string
}

// Server is now imported from core/models

type ServerTokenResult struct {
	Token     string
	ExpiresAt time.Time
}

// Helper functions to convert protobuf enums to core types

func convertProtoBMCTypeToTypes(protoType managerv1.BMCType) types.BMCType {
	switch protoType {
	case managerv1.BMCType_BMC_IPMI:
		return types.BMCTypeIPMI
	case managerv1.BMCType_BMC_REDFISH:
		return types.BMCTypeRedfish
	default:
		return types.BMCTypeNone
	}
}

func convertProtoSOLTypeToTypes(protoType managerv1.SOLType) types.SOLType {
	switch protoType {
	case managerv1.SOLType_SOL_IPMI:
		return types.SOLTypeIPMI
	case managerv1.SOLType_SOL_REDFISH_SERIAL:
		return types.SOLTypeRedfishSerial
	default:
		return types.SOLTypeNone
	}
}

func convertProtoVNCTypeToTypes(protoType managerv1.VNCType) types.VNCType {
	switch protoType {
	case managerv1.VNCType_VNC_NATIVE:
		return types.VNCTypeNative
	case managerv1.VNCType_VNC_WEBSOCKET:
		return types.VNCTypeWebSocket
	default:
		return types.VNCTypeNone
	}
}
