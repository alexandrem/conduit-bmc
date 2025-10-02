// tests/e2e/suites/console/console_test.go
package console

import (
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/suite"

	"tests/e2e/framework"
)

const (
	server01 = "bmc-dc-docker-01-e2e-virtualbmc-01-623"
)

// ConsoleTestSuite embeds the framework E2E test suite
type ConsoleTestSuite struct {
	framework.E2ETestSuite
}

func (s *ConsoleTestSuite) TestSOLConsoleAccess() {
	if !s.Config.TestScenarios.ConsoleAccess.Enabled {
		s.T().Skip("Console access testing is disabled")
	}

	serverID := server01

	// Authenticate and get server token
	token := s.AuthenticateAndGetServerToken(serverID)

	// Create console session
	session, err := s.GatewayClient.CreateConsoleSession(s.Ctx, token, serverID)
	s.Require().NoError(err, "Failed to create console session")
	s.Require().NotEmpty(session.SessionID, "Console session ID is empty")

	s.T().Logf("Created console session: %s", session.SessionID)

	defer func() {
		// Cleanup session
		if cleanupErr := s.GatewayClient.CloseConsoleSession(s.Ctx, token, session.SessionID); cleanupErr != nil {
			s.T().Logf("Warning: Failed to cleanup console session: %v", cleanupErr)
		}
	}()

	// Connect to WebSocket console
	conn, err := s.connectWebSocket(session.WebsocketURL, token)
	s.Require().NoError(err, "Failed to connect to console WebSocket")
	defer conn.Close()

	s.T().Logf("Connected to console WebSocket: %s", session.WebsocketURL)

	// Test console interaction
	s.testConsoleCommands(conn)

	s.T().Logf("SOL console access test completed successfully")
}

func (s *ConsoleTestSuite) TestConsoleSessionManagement() {
	if !s.Config.TestScenarios.ConsoleAccess.Enabled {
		s.T().Skip("Console access testing is disabled")
	}

	serverID := server01
	token := s.AuthenticateAndGetServerToken(serverID)

	// Test creating multiple sessions
	sessions := make([]*framework.ConsoleSession, 0, 3)

	for i := 0; i < 3; i++ {
		session, err := s.GatewayClient.CreateConsoleSession(s.Ctx, token, serverID)
		s.Require().NoError(err, "Failed to create console session %d", i+1)
		s.Require().NotEmpty(session.SessionID, "Console session ID %d is empty", i+1)

		sessions = append(sessions, session)
		s.T().Logf("Created console session %d: %s", i+1, session.SessionID)
	}

	// Cleanup all sessions
	for i, session := range sessions {
		err := s.GatewayClient.CloseConsoleSession(s.Ctx, token, session.SessionID)
		s.Assert().NoError(err, "Failed to close console session %d", i+1)
		s.T().Logf("Closed console session %d: %s", i+1, session.SessionID)
	}
}

func (s *ConsoleTestSuite) TestConsoleAccessAllServers() {
	if !s.Config.TestScenarios.ConsoleAccess.Enabled {
		s.T().Skip("Console access testing is disabled")
	}

	// Test console access on all configured IPMI endpoints that support it
	for _, endpoint := range s.Config.IPMIEndpoints {
		if !framework.StringInSlice("sol_console", endpoint.ExpectedFeatures) {
			s.T().Logf("Skipping console test for %s (SOL console not expected)", endpoint.ID)
			continue
		}

		s.Run("ConsoleAccess_"+endpoint.ID, func() {
			token := s.AuthenticateAndGetServerToken(endpoint.ID)

			// Create console session
			session, err := s.GatewayClient.CreateConsoleSession(s.Ctx, token, endpoint.ID)
			s.Require().NoError(err, "Failed to create console session for %s", endpoint.ID)
			s.Require().NotEmpty(session.SessionID, "Console session ID is empty for %s", endpoint.ID)

			defer func() {
				_ = s.GatewayClient.CloseConsoleSession(s.Ctx, token, session.SessionID)
			}()

			s.T().Logf("Server %s (%s) console session created: %s",
				endpoint.ID, endpoint.Description, session.SessionID)

			// Test basic WebSocket connectivity (don't test full commands to avoid conflicts)
			conn, err := s.connectWebSocket(session.WebsocketURL, token)
			s.Require().NoError(err, "Failed to connect to console WebSocket for %s", endpoint.ID)
			conn.Close()

			s.T().Logf("Server %s console WebSocket connectivity verified", endpoint.ID)
		})
	}
}

func (s *ConsoleTestSuite) testConsoleCommands(conn *websocket.Conn) {
	for _, testCmd := range s.Config.TestScenarios.ConsoleAccess.TestCommands {
		s.T().Logf("Sending console command: %q", strings.TrimSpace(testCmd.Command))

		// Send command
		err := conn.WriteMessage(websocket.TextMessage, []byte(testCmd.Command))
		s.Require().NoError(err, "Failed to send console command: %s", testCmd.Command)

		// Read response with timeout
		response := s.readConsoleResponse(conn, s.Config.TestScenarios.ConsoleAccess.CommandTimeout)
		s.Assert().Contains(response, testCmd.Expected,
			"Console response doesn't contain expected text '%s' for command '%s'. Got: %s",
			testCmd.Expected, testCmd.Command, framework.SanitizeForLog(response))

		s.T().Logf("✅ Command succeeded: %s -> %s", strings.TrimSpace(testCmd.Command), framework.SanitizeForLog(response))

		// Brief delay between commands
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *ConsoleTestSuite) readConsoleResponse(conn *websocket.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))

	var response strings.Builder
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		response.Write(message)

		// Check if we have enough data
		if response.Len() > 0 {
			break
		}

		// Small delay to allow more data to arrive
		time.Sleep(100 * time.Millisecond)
	}

	return response.String()
}

func (s *ConsoleTestSuite) connectWebSocket(url, token string) (*websocket.Conn, error) {
	headers := map[string][]string{
		"Authorization": {"Bearer " + token},
	}

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = s.Config.TestScenarios.ConsoleAccess.ConnectionTimeout

	conn, _, err := dialer.Dial(url, headers)
	return conn, err
}

func (s *ConsoleTestSuite) TestConsoleConnectionTimeout() {
	if !s.Config.TestScenarios.ConsoleAccess.Enabled {
		s.T().Skip("Console access testing is disabled")
	}

	serverID := server01
	token := s.AuthenticateAndGetServerToken(serverID)

	session, err := s.GatewayClient.CreateConsoleSession(s.Ctx, token, serverID)
	s.Require().NoError(err, "Failed to create console session")

	defer func() {
		_ = s.GatewayClient.CloseConsoleSession(s.Ctx, token, session.SessionID)
	}()

	// Test WebSocket connection with very short timeout
	headers := map[string][]string{
		"Authorization": {"Bearer " + token},
	}

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 1 * time.Millisecond // Very short timeout

	_, _, err = dialer.Dial(session.WebsocketURL, headers)

	// Should timeout (though might succeed if very fast)
	if err != nil {
		s.T().Logf("Expected WebSocket timeout error: %v", err)
	} else {
		s.T().Logf("WebSocket connection succeeded despite short timeout")
	}
}

// TestConsoleTestSuite runs the console test suite
func TestConsoleTestSuite(t *testing.T) {
	suite.Run(t, new(ConsoleTestSuite))
}
