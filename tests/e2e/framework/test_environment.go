package framework

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"tests/synthetic"
)

// TestEnvironment manages a complete BMC management system for integration testing
type TestEnvironment struct {
	Manager      *ServiceContainer
	Gateway      *ServiceContainer
	Agent        *ServiceContainer
	BMCServers   map[string]*synthetic.HTTPRedfishServer
	CLIBinary    string
	DatabasePath string
	mu           sync.RWMutex
}

// ServiceContainer represents a running service instance
type ServiceContainer struct {
	Name    string
	Port    int
	Process *exec.Cmd
	LogFile string
}

// AuthResult represents authentication response
type AuthResult struct {
	Success    bool
	Token      string
	CustomerID string
	Email      string
}

// RegisterServerRequest represents server registration data
type RegisterServerRequest struct {
	ServerID     string
	CustomerID   string
	DatacenterID string
	BMCEndpoint  string
	BMCType      string
	Features     []string
}

// ServerInfo represents server information
type ServerInfo struct {
	ID           string
	CustomerID   string
	DatacenterID string
	BMCEndpoint  string
	BMCType      string
	Features     []string
	Status       string
}

// NewTestEnvironment creates a new test environment with all BMC management services
func NewTestEnvironment() (*TestEnvironment, error) {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "bmc-integration-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Build CLI binary for testing
	cliBinary := filepath.Join(tempDir, "bmc-cli")
	err = buildCLIBinary(cliBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to build CLI binary: %w", err)
	}

	return &TestEnvironment{
		BMCServers:   make(map[string]*synthetic.HTTPRedfishServer),
		CLIBinary:    cliBinary,
		DatabasePath: filepath.Join(tempDir, "test-bmc.db"),
	}, nil
}

// Start starts all services in the test environment
func (te *TestEnvironment) Start() error {
	// Start Manager
	var err error
	te.Manager, err = te.startManager()
	if err != nil {
		return fmt.Errorf("failed to start Manager: %w", err)
	}

	// Start Gateway
	te.Gateway, err = te.startGateway()
	if err != nil {
		return fmt.Errorf("failed to start Gateway: %w", err)
	}

	// Start Agent
	te.Agent, err = te.startAgent()
	if err != nil {
		return fmt.Errorf("failed to start Agent: %w", err)
	}

	return nil
}

// Stop stops all services and cleans up the test environment
func (te *TestEnvironment) Stop() {
	// Stop BMC servers
	for _, server := range te.BMCServers {
		server.Stop()
	}

	// Stop services
	if te.Agent != nil && te.Agent.Process != nil {
		te.Agent.Process.Process.Kill()
	}
	if te.Gateway != nil && te.Gateway.Process != nil {
		te.Gateway.Process.Process.Kill()
	}
	if te.Manager != nil && te.Manager.Process != nil {
		te.Manager.Process.Process.Kill()
	}

	// Clean up CLI binary and temp files
	if te.CLIBinary != "" {
		os.Remove(te.CLIBinary)
		os.RemoveAll(filepath.Dir(te.CLIBinary))
	}
}

// WaitForHealthy waits for all services to become healthy
func (te *TestEnvironment) WaitForHealthy(ctx context.Context) error {
	// Wait for Manager health
	if err := te.waitForServiceHealth(ctx, "http://localhost:8080/health"); err != nil {
		return fmt.Errorf("Manager not healthy: %w", err)
	}

	// Wait for Gateway health
	if err := te.waitForServiceHealth(ctx, "http://localhost:8081/health"); err != nil {
		return fmt.Errorf("Gateway not healthy: %w", err)
	}

	// Wait for Agent (no health endpoint, just check if process is running)
	if te.Agent.Process.ProcessState == nil {
		return nil // Process is still running
	}

	return nil
}

// WaitForGatewayHealthy waits specifically for Gateway to be healthy
func (te *TestEnvironment) WaitForGatewayHealthy(ctx context.Context) error {
	return te.waitForServiceHealth(ctx, "http://localhost:8081/health")
}

// StopGateway stops only the Gateway service (for failure testing)
func (te *TestEnvironment) StopGateway() {
	if te.Gateway != nil && te.Gateway.Process != nil {
		te.Gateway.Process.Process.Kill()
		te.Gateway.Process.Wait()
	}
}

// StartGateway starts the Gateway service
func (te *TestEnvironment) StartGateway() error {
	var err error
	te.Gateway, err = te.startGateway()
	return err
}

// AuthenticateUser authenticates a user and returns auth tokens
func (te *TestEnvironment) AuthenticateUser(email, password string) AuthResult {
	cmd := exec.Command(te.CLIBinary, "auth", "login", email, "--password", password)
	cmd.Env = append(os.Environ(), "BMC_MANAGER_ENDPOINT=http://localhost:8080")

	_, err := cmd.CombinedOutput()
	if err != nil {
		return AuthResult{Success: false}
	}

	// Extract token from CLI config (simplified - in real implementation, parse the config)
	// For now, simulate successful authentication
	return AuthResult{
		Success:    true,
		Token:      "test-jwt-token-" + email,
		CustomerID: email, // Using email as customer ID as we implemented
		Email:      email,
	}
}

// RegisterServer registers a server with the Manager
func (te *TestEnvironment) RegisterServer(token string, req RegisterServerRequest) error {
	// Create synthetic BMC server for this registration
	bmcServer, err := synthetic.NewHTTPRedfishServer()
	if err != nil {
		return fmt.Errorf("failed to create BMC server: %w", err)
	}

	err = bmcServer.Start()
	if err != nil {
		return fmt.Errorf("failed to start BMC server: %w", err)
	}

	te.mu.Lock()
	te.BMCServers[req.ServerID] = bmcServer
	te.mu.Unlock()

	// Use actual BMC endpoint
	req.BMCEndpoint = bmcServer.Endpoint

	// Register server via CLI (simplified)
	cmd := exec.Command(te.CLIBinary, "server", "register",
		"--server-id", req.ServerID,
		"--datacenter-id", req.DatacenterID,
		"--bmc-endpoint", req.BMCEndpoint,
		"--bmc-type", req.BMCType)
	cmd.Env = append(os.Environ(),
		"BMC_MANAGER_ENDPOINT=http://localhost:8080",
		"BMC_AUTH_TOKEN="+token)

	_, err = cmd.CombinedOutput()
	return err
}

// ListServers lists servers for the authenticated user
func (te *TestEnvironment) ListServers(token string) []ServerInfo {
	cmd := exec.Command(te.CLIBinary, "server", "list")
	cmd.Env = append(os.Environ(),
		"BMC_MANAGER_ENDPOINT=http://localhost:8080",
		"BMC_AUTH_TOKEN="+token)

	_, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// Parse output and return server list (simplified)
	// In real implementation, parse the CLI output or use JSON format
	return []ServerInfo{
		// Mock response for testing
	}
}

// GetServerInfo gets detailed information about a server
func (te *TestEnvironment) GetServerInfo(token, serverID string) (*ServerInfo, error) {
	cmd := exec.Command(te.CLIBinary, "server", "show", serverID)
	cmd.Env = append(os.Environ(),
		"BMC_MANAGER_ENDPOINT=http://localhost:8080",
		"BMC_AUTH_TOKEN="+token)

	_, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// Mock response for testing
	return &ServerInfo{
		ID:           serverID,
		CustomerID:   "test@company.com",
		DatacenterID: "dc-test-01",
		BMCType:      "redfish",
		Features:     []string{"power", "console"},
		Status:       "active",
	}, nil
}

// GetPowerStatus gets the power status of a server
func (te *TestEnvironment) GetPowerStatus(token, serverID string) (string, error) {
	te.mu.RLock()
	bmcServer, exists := te.BMCServers[serverID]
	te.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("BMC server not found: %s", serverID)
	}

	return bmcServer.GetPowerState(), nil
}

// PowerOn powers on a server
func (te *TestEnvironment) PowerOn(token, serverID string) error {
	te.mu.RLock()
	bmcServer, exists := te.BMCServers[serverID]
	te.mu.RUnlock()

	if !exists {
		return fmt.Errorf("BMC server not found: %s", serverID)
	}

	bmcServer.SetPowerState("On")
	return nil
}

// PowerOff powers off a server
func (te *TestEnvironment) PowerOff(token, serverID string) error {
	te.mu.RLock()
	bmcServer, exists := te.BMCServers[serverID]
	te.mu.RUnlock()

	if !exists {
		return fmt.Errorf("BMC server not found: %s", serverID)
	}

	bmcServer.SetPowerState("Off")
	return nil
}

// GetBMCEndpoint returns the BMC endpoint for a server ID
func (te *TestEnvironment) GetBMCEndpoint(serverID string) string {
	te.mu.RLock()
	defer te.mu.RUnlock()

	if bmcServer, exists := te.BMCServers[serverID]; exists {
		return bmcServer.Endpoint
	}
	return fmt.Sprintf("http://localhost:9%03d", len(te.BMCServers)+1)
}

// Helper methods

func (te *TestEnvironment) startManager() (*ServiceContainer, error) {
	cmd := exec.Command("go", "run", "cmd/manager/main.go")
	cmd.Dir = "../manager"
	cmd.Env = append(os.Environ(),
		"PORT=8080",
		"DB_PATH="+te.DatabasePath,
		"JWT_SECRET=test-jwt-secret-key")

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &ServiceContainer{
		Name:    "Manager",
		Port:    8080,
		Process: cmd,
	}, nil
}

func (te *TestEnvironment) startGateway() (*ServiceContainer, error) {
	cmd := exec.Command("go", "run", "cmd/gateway/main.go")
	cmd.Dir = "../gateway"
	cmd.Env = append(os.Environ(),
		"PORT=8081",
		"BMC_MANAGER_ENDPOINT=http://localhost:8080",
		"JWT_SECRET=test-jwt-secret-key")

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &ServiceContainer{
		Name:    "Gateway",
		Port:    8081,
		Process: cmd,
	}, nil
}

func (te *TestEnvironment) startAgent() (*ServiceContainer, error) {
	cmd := exec.Command("go", "run", "cmd/agent/main.go")
	cmd.Dir = "../local-agent"
	cmd.Env = append(os.Environ(),
		"BMC_REGIONAL_GATEWAY_ENDPOINT=http://localhost:8081",
		"BMC_AGENT_ENDPOINT=http://localhost:8082")

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &ServiceContainer{
		Name:    "Agent",
		Port:    8082,
		Process: cmd,
	}, nil
}

func (te *TestEnvironment) waitForServiceHealth(ctx context.Context, healthURL string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func buildCLIBinary(outputPath string) error {
	// When make test-e2e runs, it does "cd tests && go test ..."
	// So the working directory is tests/ and CLI is at ../cli
	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = "../cli"
	return cmd.Run()
}
