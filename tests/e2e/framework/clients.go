// tests/e2e/framework/clients.go
package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"
	managerv1 "manager/gen/manager/v1"
	"manager/gen/manager/v1/managerv1connect"
)

// ManagerClient wraps the Buf Connect client for manager.v1.BMCManagerService
type ManagerClient struct {
	baseURL    string
	httpClient *http.Client
	rpcClient  managerv1connect.BMCManagerServiceClient
}

// AuthSession stores tokens returned from the manager Authenticate RPC
type AuthSession struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	CustomerID   string
	Email        string
}

// ServerToken represents a delegated token scoped to a specific server
type ServerToken struct {
	Token     string
	ExpiresAt time.Time
}

func NewManagerClient(baseURL string) *ManagerClient {
	trimmed := strings.TrimRight(baseURL, "/")
	httpClient := &http.Client{Timeout: 30 * time.Second}

	return &ManagerClient{
		baseURL:    trimmed,
		httpClient: httpClient,
		rpcClient:  managerv1connect.NewBMCManagerServiceClient(httpClient, trimmed),
	}
}

func (c *ManagerClient) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (c *ManagerClient) Authenticate(ctx context.Context, email, password string) (*AuthSession, error) {
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    email,
		Password: password,
	})

	resp, err := c.rpcClient.Authenticate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("manager authenticate failed: %w", err)
	}

	session := &AuthSession{
		AccessToken:  resp.Msg.AccessToken,
		RefreshToken: resp.Msg.RefreshToken,
		ExpiresAt:    resp.Msg.ExpiresAt.AsTime(),
		Email:        resp.Msg.Customer.Email,
	}

	if resp.Msg.Customer != nil {
		session.CustomerID = resp.Msg.Customer.Id
	}

	return session, nil
}

func (c *ManagerClient) GetServerToken(ctx context.Context, session *AuthSession, serverID string) (*ServerToken, error) {
	if session == nil {
		return nil, fmt.Errorf("auth session required")
	}

	req := connect.NewRequest(&managerv1.GetServerTokenRequest{
		ServerId: serverID,
	})
	req.Header().Set("Authorization", "Bearer "+session.AccessToken)

	resp, err := c.rpcClient.GetServerToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server token: %w", err)
	}

	return &ServerToken{
		Token:     resp.Msg.Token,
		ExpiresAt: resp.Msg.ExpiresAt.AsTime(),
	}, nil
}

func (c *ManagerClient) GenerateServerToken(ctx context.Context, customer *TestCustomer, server *Server, _ []string) (string, error) {
	session, err := c.Authenticate(ctx, customer.Email, customer.Password)
	if err != nil {
		return "", err
	}

	if err := c.waitForServerRegistration(ctx, session, server.ID); err != nil {
		return "", err
	}

	serverToken, err := c.GetServerToken(ctx, session, server.ID)
	if err != nil {
		return "", err
	}

	return serverToken.Token, nil
}

func (c *ManagerClient) GenerateServerTokenWithTTL(ctx context.Context, customer *TestCustomer, server *Server, permissions []string, _ time.Duration) (string, error) {
	// The manager service currently issues fixed-duration tokens (~1h).
	// TTL parameter is ignored to keep backward compatibility in tests.
	return c.GenerateServerToken(ctx, customer, server, permissions)
}

func (c *ManagerClient) waitForServerRegistration(ctx context.Context, session *AuthSession, serverID string) error {
	lookup := func(ctx context.Context) error {
		lookupReq := connect.NewRequest(&managerv1.GetServerRequest{ServerId: serverID})
		lookupReq.Header().Set("Authorization", "Bearer "+session.AccessToken)

		_, err := c.rpcClient.GetServer(ctx, lookupReq)
		if err == nil {
			return nil
		}
		if connect.CodeOf(err) == connect.CodeNotFound {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("not ready"))
		}
		return fmt.Errorf("failed to look up server %s: %w", serverID, err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// First immediate check.
	if err := lookup(waitCtx); err == nil {
		return nil
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		return err
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for server %s registration: %w", serverID, waitCtx.Err())
		case <-ticker.C:
			err := lookup(waitCtx)
			if err == nil {
				return nil
			}
			if connect.CodeOf(err) != connect.CodeNotFound {
				return err
			}
		}
	}
}

// GatewayClient wraps the Buf Connect client for gateway.v1.GatewayService
type GatewayClient struct {
	baseURL    string
	httpClient *http.Client
	rpcClient  gatewayv1connect.GatewayServiceClient
}

// ConsoleSession contains metadata returned by CreateVNCSession
type ConsoleSession struct {
	SessionID    string
	WebsocketURL string
	ViewerURL    string
	ExpiresAt    time.Time
}

func NewGatewayClient(baseURL string) *GatewayClient {
	trimmed := strings.TrimRight(baseURL, "/")
	httpClient := &http.Client{Timeout: 30 * time.Second}

	return &GatewayClient{
		baseURL:    trimmed,
		httpClient: httpClient,
		rpcClient:  gatewayv1connect.NewGatewayServiceClient(httpClient, trimmed),
	}
}

func (c *GatewayClient) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (c *GatewayClient) ValidateToken(ctx context.Context, token string) bool {
	req := connect.NewRequest(&gatewayv1.ListServersRequest{PageSize: 1})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err := c.rpcClient.ListServers(ctx, req)
	return err == nil
}

func (c *GatewayClient) PowerOperation(ctx context.Context, token, serverID, operation string) (string, error) {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})
	req.Header().Set("Authorization", "Bearer "+token)

	var (
		resp *connect.Response[gatewayv1.PowerOperationResponse]
		err  error
	)

	switch strings.ToLower(operation) {
	case "on":
		resp, err = c.rpcClient.PowerOn(ctx, req)
	case "off":
		resp, err = c.rpcClient.PowerOff(ctx, req)
	case "cycle":
		resp, err = c.rpcClient.PowerCycle(ctx, req)
	case "reset":
		resp, err = c.rpcClient.Reset(ctx, req)
	case "status":
		status, statusErr := c.PowerStatus(ctx, token, serverID)
		return status, statusErr
	default:
		return "", fmt.Errorf("unsupported power operation: %s", operation)
	}

	if err != nil {
		return "", fmt.Errorf("power operation %s failed: %w", operation, err)
	}

	return resp.Msg.Message, nil
}

func (c *GatewayClient) PowerStatus(ctx context.Context, token, serverID string) (string, error) {
	req := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: serverID,
	})
	req.Header().Set("Authorization", "Bearer "+token)

	resp, err := c.rpcClient.GetPowerStatus(ctx, req)
	if err != nil {
		return "", fmt.Errorf("power status failed: %w", err)
	}

	return normalizePowerState(resp.Msg.State), nil
}

func (c *GatewayClient) CreateConsoleSession(ctx context.Context, token, serverID string) (*ConsoleSession, error) {
	req := connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: serverID,
	})
	req.Header().Set("Authorization", "Bearer "+token)

	resp, err := c.rpcClient.CreateVNCSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create console session failed: %w", err)
	}

	return &ConsoleSession{
		SessionID:    resp.Msg.SessionId,
		WebsocketURL: resp.Msg.WebsocketEndpoint,
		ViewerURL:    resp.Msg.ViewerUrl,
		ExpiresAt:    resp.Msg.ExpiresAt.AsTime(),
	}, nil
}

func (c *GatewayClient) CloseConsoleSession(ctx context.Context, token, sessionID string) error {
	req := connect.NewRequest(&gatewayv1.CloseVNCSessionRequest{
		SessionId: sessionID,
	})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err := c.rpcClient.CloseVNCSession(ctx, req)
	if err != nil {
		return fmt.Errorf("close console session failed: %w", err)
	}

	return nil
}

// AgentClient still uses the legacy REST API exposed by the local agent
type AgentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAgentClient(baseURL string) *AgentClient {
	return &AgentClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *AgentClient) IsHealthy() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *AgentClient) GetBMCEndpoints() ([]map[string]interface{}, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v1/bmcs")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get BMC endpoints with status: %d", resp.StatusCode)
	}

	var endpoints []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&endpoints); err != nil {
		return nil, err
	}

	return endpoints, nil
}

// RegisterServer registers a server with the BMC Manager
func (c *ManagerClient) RegisterServer(ctx context.Context, customer *TestCustomer, server *Server) error {
	// First authenticate to get a proper JWT token
	session, err := c.Authenticate(ctx, customer.Email, customer.Password)
	if err != nil {
		return fmt.Errorf("failed to authenticate customer %s: %w", customer.Email, err)
	}

	// Convert BMC type string to enum
	var bmcType managerv1.BMCType
	switch strings.ToLower(server.Type) {
	case "ipmi":
		bmcType = managerv1.BMCType_BMC_TYPE_IPMI
	case "redfish":
		bmcType = managerv1.BMCType_BMC_TYPE_REDFISH
	default:
		bmcType = managerv1.BMCType_BMC_TYPE_UNSPECIFIED
	}

	req := connect.NewRequest(&managerv1.RegisterServerRequest{
		ServerId:          server.ID,
		CustomerId:        session.CustomerID, // Use authenticated customer ID
		DatacenterId:      server.Datacenter,
		RegionalGatewayId: "gateway-docker-1", // Default gateway ID for E2E tests
		BmcType:          bmcType,
		Features:         server.Features,
		BmcEndpoint:      server.BMCEndpoint,
	})

	// Use proper JWT token for authentication
	req.Header().Set("Authorization", "Bearer "+session.AccessToken)

	_, err = c.rpcClient.RegisterServer(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register server %s: %w", server.ID, err)
	}

	return nil
}

func normalizePowerState(state gatewayv1.PowerState) string {
	s := state.String()
	s = strings.TrimPrefix(s, "POWER_STATE_")
	s = strings.ToLower(s)
	if s == "unknown" {
		return "unknown"
	}
	return s
}
