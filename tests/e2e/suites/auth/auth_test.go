// tests/e2e/suites/auth/auth_test.go
package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"tests/e2e/framework"
)

const (
	server01 = "bmc-dc-docker-01-e2e-virtualbmc-01-623"
	server02 = "bmc-dc-docker-01-e2e-virtualbmc-02-623"
)

// AuthTestSuite embeds the framework E2E test suite
type AuthTestSuite struct {
	framework.E2ETestSuite
}

func (s *AuthTestSuite) TestAuthenticationFlow() {
	if !s.Config.TestScenarios.Authentication.Enabled {
		s.T().Skip("Authentication testing is disabled")
	}

	// Test manager authentication via Buf Connect client
	s.Run("ManagerAuthentication", func() {
		if !s.ManagerClient.IsHealthy() {
			s.T().Skip("Manager service not ready for authentication test")
		}

		customer := s.Config.TestCustomers[0]
		session, err := s.ManagerClient.Authenticate(s.Ctx, customer.Email, customer.Password)
		s.Require().NoError(err, "Manager authentication should succeed")
		s.Assert().NotEmpty(session.AccessToken, "Access token should not be empty")
		s.Assert().NotZero(session.ExpiresAt.Unix(), "Expiration should be populated")
	})

	// Test server token issuance and validation through the gateway
	s.Run("ServerTokenGeneration", func() {
		if !s.AreServicesReady() {
			s.T().Skip("Services not ready for server token generation test")
		}

		token := s.AuthenticateAndGetServerToken("bmc-dc-docker-01-e2e-virtualbmc-01-623")
		s.Assert().NotEmpty(token, "Generated token is empty")

		valid := s.GatewayClient.ValidateToken(s.Ctx, token)
		s.Assert().True(valid, "Gateway should accept freshly minted token")
	})

	if s.Config.TestScenarios.Authentication.TestExpiredTokens {
		s.Run("TokenExpiration", func() {
			s.T().Skip("Manager service currently issues fixed-duration tokens; custom TTL not supported")
		})
	}

	if s.Config.TestScenarios.Authentication.TestInvalidTokens {
		s.Run("InvalidTokens", func() {
			s.Assert().False(s.GatewayClient.ValidateToken(s.Ctx, "invalid.jwt.token"), "Random token should be rejected")
			s.Assert().False(s.GatewayClient.ValidateToken(s.Ctx, "not-a-jwt-token"), "Malformed token should be rejected")
		})
	}
}

func (s *AuthTestSuite) TestMultiTenantIsolation() {
	if !s.Config.TestScenarios.MultiTenant.Enabled {
		s.T().Skip("Multi-tenant testing is disabled")
	}

	s.Require().GreaterOrEqual(len(s.Config.TestCustomers), 2, "Need at least 2 test customers")
	s.Require().GreaterOrEqual(len(s.Config.IPMIEndpoints), 2, "Need at least 2 IPMI endpoints")

	customer1 := &s.Config.TestCustomers[0]
	customer2 := &s.Config.TestCustomers[1]
	server1 := s.GetTestServer(server01)
	server2 := s.GetTestServer(server02)

	token1, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer1, server1, customer1.Permissions)
	s.Require().NoError(err, "Token generation for customer 1 failed")

	token2, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer2, server2, customer2.Permissions)
	s.Require().NoError(err, "Token generation for customer 2 failed")

	if s.Config.TestScenarios.MultiTenant.TestIsolation {
		s.Run("Customer1AccessOwnServer", func() {
			status, err := s.GatewayClient.PowerStatus(s.Ctx, token1, server1.ID)
			s.Assert().NoError(err)
			s.Assert().NotEmpty(status)
		})

		s.Run("Customer2AccessOwnServer", func() {
			status, err := s.GatewayClient.PowerStatus(s.Ctx, token2, server2.ID)
			s.Assert().NoError(err)
			s.Assert().NotEmpty(status)
		})
	}

	if s.Config.TestScenarios.MultiTenant.TestUnauthorizedAccess {
		s.Run("Customer1CannotAccessServer2", func() {
			_, err := s.GatewayClient.PowerStatus(s.Ctx, token1, server2.ID)
			s.Assert().Error(err)
		})

		s.Run("Customer2CannotAccessServer1", func() {
			_, err := s.GatewayClient.PowerStatus(s.Ctx, token2, server1.ID)
			s.Assert().Error(err)
		})
	}
}

func (s *AuthTestSuite) TestPermissionEnforcement() {
	if !s.Config.TestScenarios.Authentication.Enabled {
		s.T().Skip("Authentication testing is disabled")
	}

	server := s.GetTestServer(server01)
	s.Require().NotNil(server)

	var limitedCustomer *framework.TestCustomer
	for i, customer := range s.Config.TestCustomers {
		if !framework.StringInSlice("admin", customer.Permissions) && len(customer.Permissions) < 3 {
			limitedCustomer = &s.Config.TestCustomers[i]
			break
		}
	}

	if limitedCustomer != nil {
		s.Run("LimitedPermissions", func() {
			token, err := s.ManagerClient.GenerateServerToken(s.Ctx, limitedCustomer, server, limitedCustomer.Permissions)
			s.Require().NoError(err)

			if framework.StringInSlice("status", limitedCustomer.Permissions) {
				_, err := s.GatewayClient.PowerStatus(s.Ctx, token, server.ID)
				s.Assert().NoError(err)
			}

			if !framework.StringInSlice("power", limitedCustomer.Permissions) {
				_, err := s.GatewayClient.PowerOperation(s.Ctx, token, server.ID, "off")
				s.Assert().Error(err)
			}
		})
	}

	var adminCustomer *framework.TestCustomer
	for i, customer := range s.Config.TestCustomers {
		if framework.StringInSlice("admin", customer.Permissions) {
			adminCustomer = &s.Config.TestCustomers[i]
			break
		}
	}

	if adminCustomer != nil {
		s.Run("FullPermissions", func() {
			token, err := s.ManagerClient.GenerateServerToken(s.Ctx, adminCustomer, server, adminCustomer.Permissions)
			s.Require().NoError(err)

			_, err = s.GatewayClient.PowerStatus(s.Ctx, token, server.ID)
			s.Assert().NoError(err)

			_, err = s.GatewayClient.PowerOperation(s.Ctx, token, server.ID, "status")
			s.Assert().NoError(err)
		})
	}
}

func (s *AuthTestSuite) TestTokenRefresh() {
	if !s.Config.TestScenarios.Authentication.Enabled {
		s.T().Skip("Authentication testing is disabled")
	}

	customer := &s.Config.TestCustomers[0]
	server := s.GetTestServer(server01)

	token1, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer, server, customer.Permissions)
	s.Require().NoError(err)

	token2, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer, server, customer.Permissions)
	s.Require().NoError(err)

	s.Assert().NotEqual(token1, token2, "Refreshed token should differ from original")
	s.Assert().True(s.GatewayClient.ValidateToken(s.Ctx, token1))
	s.Assert().True(s.GatewayClient.ValidateToken(s.Ctx, token2))
}

func (s *AuthTestSuite) TestAuthenticationLoadTest() {
	if !s.Config.TestScenarios.Authentication.Enabled {
		s.T().Skip("Authentication testing is disabled")
	}

	concurrency := 5
	totalAuths := concurrency * 3
	var successCount int32
	var wg sync.WaitGroup

	customer := s.Config.TestCustomers[0]
	ctx := s.Ctx

	for i := 0; i < totalAuths; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := s.ManagerClient.Authenticate(ctx, customer.Email, customer.Password)
			if err == nil && session.AccessToken != "" {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.T().Log("Load test timed out before all authentications completed")
	}

	s.Assert().GreaterOrEqual(float64(successCount)/float64(totalAuths), 0.95, "Authentication success rate should be at least 95%%")
}

func (s *AuthTestSuite) TestCrossCustomerTokenValidation() {
	if !s.Config.TestScenarios.MultiTenant.Enabled || len(s.Config.TestCustomers) < 2 {
		s.T().Skip("Multi-tenant testing is disabled or insufficient customers")
	}

	customer1 := &s.Config.TestCustomers[0]
	customer2 := &s.Config.TestCustomers[1]
	server := s.GetTestServer(server01)

	token1, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer1, server, customer1.Permissions)
	s.Require().NoError(err)

	_, err = s.ManagerClient.Authenticate(s.Ctx, customer2.Email, customer2.Password)
	s.Require().NoError(err)

	valid := s.GatewayClient.ValidateToken(s.Ctx, token1)
	s.Assert().True(valid, "Tokens should remain valid regardless of latest authentication context")

	_, err = s.GatewayClient.PowerStatus(s.Ctx, token1, server.ID)
	s.Assert().NoError(err, "Token permissions should govern access, not current login context")
}

// TestAuthTestSuite runs the auth test suite
func TestAuthTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
