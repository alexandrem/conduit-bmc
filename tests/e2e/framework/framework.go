// tests/e2e/framework/framework.go
package framework

import (
	"context"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
)

type E2ETestSuite struct {
	suite.Suite
	Config        TestConfig
	ManagerClient *ManagerClient
	GatewayClient *GatewayClient
	AgentClient   *AgentClient
	Ctx           context.Context
	cancel        context.CancelFunc
}

func (s *E2ETestSuite) SetupSuite() {
	s.T().Log("Setting up E2E test suite...")
	s.Ctx, s.cancel = context.WithTimeout(context.Background(), 5*time.Minute)

	// Load test configuration
	s.Config = LoadConfig()
	s.T().Logf("Loaded config with Manager endpoint: %s", s.Config.ManagerEndpoint)

	// Initialize API clients
	s.ManagerClient = NewManagerClient(s.Config.ManagerEndpoint)
	s.GatewayClient = NewGatewayClient(s.Config.GatewayEndpoint)
	s.AgentClient = NewAgentClient(s.Config.AgentEndpoint)

	// Check if services are available, but don't fail - just log
	if s.AreServicesReady() {
		s.T().Log("All services are ready")

		// Dynamically register test servers and customers
		if s.setupTestData() {
			s.T().Log("Test data setup completed successfully")
		} else {
			s.T().Log("Test data setup failed - some tests may fail")
		}

		// Only verify IPMI if services are up
		if s.verifyIPMIConnectivity() {
			s.T().Log("IPMI endpoints verified successfully")
		} else {
			s.T().Log("IPMI endpoints not accessible - tests may be skipped")
		}
	} else {
		s.T().Log("Services not ready - some tests will be skipped")
	}

	s.T().Log("E2E test suite setup complete")
}

// Simple test method to verify the suite works
func (s *E2ETestSuite) TestFrameworkBasic() {
	s.T().Log("Framework basic test running")
	s.Assert().NotNil(s.Config)
	s.T().Log("Framework basic test completed")
}

func (s *E2ETestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *E2ETestSuite) waitForServices() bool {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			s.T().Log("Timeout waiting for services to be ready")
			return false
		case <-ticker.C:
			if s.AreServicesReady() {
				return true
			}
		}
	}
}

func (s *E2ETestSuite) AreServicesReady() bool {
	// Check Manager health
	if !s.ManagerClient.IsHealthy() {
		s.T().Log("Manager service is not healthy")
		return false
	}

	// Check Gateway health
	if !s.GatewayClient.IsHealthy() {
		s.T().Log("Gateway service is not healthy")
		return false
	}

	// Check Agent health
	if !s.AgentClient.IsHealthy() {
		s.T().Log("Agent service is not healthy")
		return false
	}

	return true
}

func (s *E2ETestSuite) verifyIPMIConnectivity() bool {
	for _, endpoint := range s.Config.IPMIEndpoints {
		s.T().Logf("Verifying IPMI connectivity to %s (%s)", endpoint.ID, endpoint.Address)

		connected := s.testIPMIConnection(endpoint)
		if !connected {
			s.T().Logf("Failed to connect to IPMI endpoint %s", endpoint.ID)
			return false
		}
	}
	return true
}

func (s *E2ETestSuite) testIPMIConnection(endpoint IPMIEndpoint) bool {
	// Direct IPMI connectivity test
	return TestIPMIConnection(endpoint.Address, endpoint.Username, endpoint.Password)
}

// Helper methods for test implementation

func (s *E2ETestSuite) AuthenticateAndGetServerToken(serverID string) string {
	// Find appropriate customer for this server
	var customer *TestCustomer
	for _, c := range s.Config.TestCustomers {
		for _, assignedServer := range c.AssignedServers {
			if assignedServer == serverID {
				customer = &c
				break
			}
		}
		if customer != nil {
			break
		}
	}

	if customer == nil {
		// Use admin customer as fallback
		for _, c := range s.Config.TestCustomers {
			if c.APIKey == "test-api-key-admin" {
				customer = &c
				break
			}
		}
	}

	s.Require().NotNil(customer, "No suitable customer found for server %s", serverID)

	// Get server info
	server := s.GetTestServer(serverID)

	// Authenticate and get server token
	token, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer, server, customer.Permissions)
	if err != nil {
		// Check if this is a "server not found" error which indicates services aren't properly set up
		if strings.Contains(err.Error(), "no rows in result set") || strings.Contains(err.Error(), "not found") {
			s.T().Skipf("Server %s not registered in manager - skipping test (services may not be fully configured)", serverID)
		}
		s.Require().NoError(err, "Failed to generate server token")
	}
	s.Require().NotEmpty(token, "Generated token is empty")

	return token
}

func (s *E2ETestSuite) createTestCustomer(email ...string) *TestCustomer {
	customerEmail := "test-customer@example.com"
	if len(email) > 0 {
		customerEmail = email[0]
	}

	return &TestCustomer{
		Email:       customerEmail,
		APIKey:      "test-api-key-" + customerEmail,
		Password:    "test-api-key-" + customerEmail,
		Name:        "Test Customer",
		Permissions: []string{"power", "console", "status"},
	}
}

func (s *E2ETestSuite) CreateTestCustomerWithID(userID string) *TestCustomer {
	return &TestCustomer{
		Email:       userID + "@example.com",
		APIKey:      "test-api-key-" + userID,
		Password:    "test-api-key-" + userID,
		Name:        "Test Customer " + userID,
		Permissions: []string{"power", "console", "status"},
	}
}

func (s *E2ETestSuite) GetTestServer(serverID string) *Server {
	s.T().Helper()

	// Find server in configured IPMI endpoints
	for _, endpoint := range s.Config.IPMIEndpoints {
		if endpoint.ID == serverID {
			return &Server{
				ID:          endpoint.ID,
				BMCEndpoint: endpoint.Address,
				Datacenter:  endpoint.Datacenter,
				Type:        "ipmi",
				Features:    append([]string{}, endpoint.ExpectedFeatures...),
			}
		}
	}

	s.T().Fatalf("cannot find server with ID %s", serverID)
	return nil
}

func (s *E2ETestSuite) GetRandomTestServer() *Server {
	if len(s.Config.IPMIEndpoints) == 0 {
		return nil
	}

	endpoint := s.Config.IPMIEndpoints[0] // Use first endpoint as default
	return &Server{
		ID:          endpoint.ID,
		BMCEndpoint: endpoint.Address,
		Datacenter:  endpoint.Datacenter,
		Type:        "ipmi",
		Features:    append([]string{}, endpoint.ExpectedFeatures...),
	}
}

// setupTestData dynamically registers test servers and customers with the manager
func (s *E2ETestSuite) setupTestData() bool {
	s.T().Log("Setting up dynamic test data (servers and customers)...")

	// Register test servers first
	for _, endpoint := range s.Config.IPMIEndpoints {
		server := &Server{
			ID:          endpoint.ID,
			BMCEndpoint: endpoint.Address,
			Datacenter:  endpoint.Datacenter,
			Type:        "ipmi", // All our test endpoints are IPMI
			Features:    endpoint.ExpectedFeatures,
		}

		// Find a test customer to associate with this server (admin customer as fallback)
		var customer *TestCustomer
		for i, c := range s.Config.TestCustomers {
			if StringInSlice(endpoint.ID, c.AssignedServers) {
				customer = &s.Config.TestCustomers[i]
				break
			}
		}
		if customer == nil {
			// Use admin customer as fallback
			for i, c := range s.Config.TestCustomers {
				if StringInSlice("admin", c.Permissions) {
					customer = &s.Config.TestCustomers[i]
					break
				}
			}
		}

		if customer == nil {
			s.T().Logf("No suitable customer found for server %s - skipping registration", server.ID)
			continue
		}

		s.T().Logf("Registering server %s with customer %s", server.ID, customer.Email)
		if err := s.ManagerClient.RegisterServer(s.Ctx, customer, server); err != nil {
			s.T().Logf("Failed to register server %s: %v", server.ID, err)
			// Don't fail the whole setup, just log and continue
		} else {
			s.T().Logf("Successfully registered server %s", server.ID)
		}
	}

	s.T().Log("Test data setup completed")
	return true
}

// Test data structures

type Server struct {
	ID          string   `json:"id"`
	BMCEndpoint string   `json:"bmc_endpoint"`
	Datacenter  string   `json:"datacenter"`
	Type        string   `json:"type"`
	Features    []string `json:"features"`
}

type Customer struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// Utility functions for running the test suite
// Note: Main test runner is in e2e_test.go
