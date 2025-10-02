package sol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RedfishClient implements Client for Redfish Serial Console
type RedfishClient struct {
	httpClient *http.Client
}

// NewRedfishClient creates a new Redfish SOL client
func NewRedfishClient() *RedfishClient {
	return &RedfishClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateSession creates a new Redfish SOL session
func (c *RedfishClient) CreateSession(ctx context.Context, endpoint, username, password string, config *Config) (Session, error) {
	if config == nil {
		config = DefaultSOLConfig()
	}

	session := &RedfishSession{
		client:     c,
		endpoint:   endpoint,
		username:   username,
		password:   password,
		config:     config,
		status:     SessionStatus{Active: false, Connected: false, Message: "created"},
		stopCh:     make(chan struct{}),
		readBuffer: make(chan []byte, 1024),
	}

	return session, nil
}

// SupportsSOL checks if the BMC supports Redfish Serial Console functionality
func (c *RedfishClient) SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error) {
	// Check if the Redfish service root is accessible
	serviceRootURL := fmt.Sprintf("https://%s/redfish/v1/", endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", serviceRootURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(username, password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to access Redfish service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Redfish service not available: %d", resp.StatusCode)
	}

	// Check for Systems collection
	systemsURL := fmt.Sprintf("https://%s/redfish/v1/Systems", endpoint)
	req, err = http.NewRequestWithContext(ctx, "GET", systemsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create systems request: %w", err)
	}

	req.SetBasicAuth(username, password)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to access Systems collection: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// RedfishSession implements Session for Redfish
type RedfishSession struct {
	mu         sync.RWMutex
	client     *RedfishClient
	endpoint   string
	username   string
	password   string
	config     *Config
	status     SessionStatus
	wsConn     *websocket.Conn
	stopCh     chan struct{}
	readBuffer chan []byte
	closed     bool
}

// Read reads console output from the Redfish SOL session
func (s *RedfishSession) Read(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("session is closed")
	}

	if !s.status.Active {
		if err := s.start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start SOL session: %w", err)
		}
	}

	select {
	case data := <-s.readBuffer:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(s.config.TimeoutSeconds) * time.Second):
		return nil, fmt.Errorf("read timeout")
	}
}

// Write sends console input to the Redfish SOL session
func (s *RedfishSession) Write(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return fmt.Errorf("session is closed")
	}

	if !s.status.Active || s.wsConn == nil {
		return fmt.Errorf("session is not active")
	}

	// Send data through WebSocket connection
	return s.wsConn.WriteMessage(websocket.TextMessage, data)
}

// Close terminates the Redfish SOL session
func (s *RedfishSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.stopCh)

	if s.wsConn != nil {
		s.wsConn.Close()
		s.wsConn = nil
	}

	s.status = SessionStatus{Active: false, Connected: false, Message: "closed"}
	return nil
}

// Status returns the current session status
func (s *RedfishSession) Status() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// start initiates the Redfish SOL session
func (s *RedfishSession) start(ctx context.Context) error {
	if s.status.Active {
		return nil
	}

	// First, find the system ID
	systemID, err := s.findSystemID(ctx)
	if err != nil {
		return fmt.Errorf("failed to find system ID: %w", err)
	}

	// Get the serial console WebSocket URI
	wsURI, err := s.getSerialConsoleURI(ctx, systemID)
	if err != nil {
		return fmt.Errorf("failed to get serial console URI: %w", err)
	}

	// Establish WebSocket connection
	if err := s.connectWebSocket(ctx, wsURI); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	s.status = SessionStatus{Active: true, Connected: true, Message: "Redfish SOL session active"}
	return nil
}

// findSystemID discovers the first available system ID
func (s *RedfishSession) findSystemID(ctx context.Context) (string, error) {
	systemsURL := fmt.Sprintf("https://%s/redfish/v1/Systems", s.endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", systemsURL, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(s.username, s.password)

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get systems: %d", resp.StatusCode)
	}

	var systems struct {
		Members []struct {
			OdataID string `json:"@odata.id"`
		} `json:"Members"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&systems); err != nil {
		return "", err
	}

	if len(systems.Members) == 0 {
		return "", fmt.Errorf("no systems found")
	}

	// Extract system ID from the first member's @odata.id
	// Expected format: "/redfish/v1/Systems/1" or similar
	odataID := systems.Members[0].OdataID
	parts := bytes.Split([]byte(odataID), []byte("/"))
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid system @odata.id format: %s", odataID)
	}

	return string(parts[len(parts)-1]), nil
}

// getSerialConsoleURI gets the WebSocket URI for the serial console
func (s *RedfishSession) getSerialConsoleURI(ctx context.Context, systemID string) (string, error) {
	serialConsoleURL := fmt.Sprintf("https://%s/redfish/v1/Systems/%s/SerialConsole", s.endpoint, systemID)

	req, err := http.NewRequestWithContext(ctx, "GET", serialConsoleURL, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(s.username, s.password)

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get serial console info: %d", resp.StatusCode)
	}

	var console struct {
		ConnectTypesSupported []string `json:"ConnectTypesSupported"`
		ServiceEnabled        bool     `json:"ServiceEnabled"`
		Actions               struct {
			Connect struct {
				Target string `json:"target"`
			} `json:"#SerialConsole.Connect"`
		} `json:"Actions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&console); err != nil {
		return "", err
	}

	if !console.ServiceEnabled {
		return "", fmt.Errorf("serial console service is disabled")
	}

	// Convert HTTPS URL to WSS URL
	connectURL := console.Actions.Connect.Target
	if connectURL == "" {
		return "", fmt.Errorf("no serial console connect target found")
	}

	// Parse and convert to WebSocket URL
	u, err := url.Parse(connectURL)
	if err != nil {
		return "", fmt.Errorf("invalid connect URL: %w", err)
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else if u.Scheme == "http" {
		u.Scheme = "ws"
	}

	return u.String(), nil
}

// connectWebSocket establishes WebSocket connection for console access
func (s *RedfishSession) connectWebSocket(ctx context.Context, wsURI string) error {
	// Set up WebSocket dialer with authentication
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}

	// Create request headers with basic auth
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+basicAuth(s.username, s.password))

	// Connect to WebSocket
	conn, _, err := dialer.DialContext(ctx, wsURI, headers)
	if err != nil {
		return err
	}

	s.wsConn = conn

	// Start reading from WebSocket in a goroutine
	go func() {
		defer func() {
			if s.wsConn != nil {
				s.wsConn.Close()
			}
		}()

		for {
			select {
			case <-s.stopCh:
				return
			default:
				_, message, err := s.wsConn.ReadMessage()
				if err != nil {
					if !s.closed {
						s.status = SessionStatus{Active: false, Connected: false, Message: fmt.Sprintf("WebSocket error: %v", err)}
					}
					return
				}

				if len(message) > 0 {
					select {
					case s.readBuffer <- message:
					case <-s.stopCh:
						return
					default:
						// Buffer full, drop data
					}
				}
			}
		}
	}()

	return nil
}

// basicAuth creates a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
