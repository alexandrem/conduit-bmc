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

// RedfishTransport implements Transport using Redfish WebSocket
type RedfishTransport struct {
	mu         sync.RWMutex
	httpClient *http.Client
	wsConn     *websocket.Conn
	status     TransportStatus
	stopCh     chan struct{}
	readCh     chan []byte
	writeCh    chan []byte
}

// NewRedfishTransport creates a new Redfish transport
func NewRedfishTransport() *RedfishTransport {
	return &RedfishTransport{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		status:  TransportStatus{Connected: false, Protocol: "redfish", Message: "disconnected"},
		stopCh:  make(chan struct{}),
		readCh:  make(chan []byte, 1024),
		writeCh: make(chan []byte, 1024),
	}
}

// Connect establishes Redfish WebSocket connection for serial console
func (t *RedfishTransport) Connect(ctx context.Context, endpoint, username, password string, config *Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status.Connected {
		return nil
	}

	// Find system ID
	systemID, err := t.findSystemID(ctx, endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to find system ID: %w", err)
	}

	// Get the serial console WebSocket URI
	wsURI, err := t.getSerialConsoleURI(ctx, endpoint, username, password, systemID)
	if err != nil {
		return fmt.Errorf("failed to get serial console URI: %w", err)
	}

	// Establish WebSocket connection
	if err := t.connectWebSocket(ctx, wsURI, username, password); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	// Start WebSocket data handling
	go t.handleWebSocketData(ctx)

	t.status = TransportStatus{Connected: true, Protocol: "redfish", Message: "WebSocket connected"}
	return nil
}

// Read reads console output from the Redfish transport
func (t *RedfishTransport) Read(ctx context.Context) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.status.Connected {
		return nil, fmt.Errorf("transport not connected")
	}

	select {
	case data := <-t.readCh:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.stopCh:
		return nil, fmt.Errorf("transport stopped")
	}
}

// Write sends console input to the Redfish transport
func (t *RedfishTransport) Write(ctx context.Context, data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.status.Connected || t.wsConn == nil {
		return fmt.Errorf("transport not connected")
	}

	select {
	case t.writeCh <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.stopCh:
		return fmt.Errorf("transport stopped")
	}
}

// Close terminates the Redfish transport
func (t *RedfishTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.status.Connected {
		return nil
	}

	close(t.stopCh)

	if t.wsConn != nil {
		t.wsConn.Close()
		t.wsConn = nil
	}

	t.status = TransportStatus{Connected: false, Protocol: "redfish", Message: "disconnected"}
	return nil
}

// Status returns the current transport status
func (t *RedfishTransport) Status() TransportStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// SupportsSOL checks if the BMC supports Redfish Serial Console functionality
func (t *RedfishTransport) SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error) {
	baseURL := normalizeRedfishEndpoint(endpoint)

	// Check if the Redfish service root is accessible
	serviceRootURL := fmt.Sprintf("%s/redfish/v1/", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", serviceRootURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(username, password)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to access Redfish service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Redfish service not available: %d", resp.StatusCode)
	}

	// Check for Systems collection
	systemsURL := fmt.Sprintf("%s/redfish/v1/Systems", baseURL)
	req, err = http.NewRequestWithContext(ctx, "GET", systemsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create systems request: %w", err)
	}

	req.SetBasicAuth(username, password)

	resp, err = t.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to access Systems collection: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// findSystemID discovers the first available system ID
func (t *RedfishTransport) findSystemID(ctx context.Context, endpoint, username, password string) (string, error) {
	// Normalize endpoint - it may already have http:// or https:// prefix
	baseURL := normalizeRedfishEndpoint(endpoint)
	systemsURL := fmt.Sprintf("%s/redfish/v1/Systems", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", systemsURL, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(username, password)

	resp, err := t.httpClient.Do(req)
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
func (t *RedfishTransport) getSerialConsoleURI(ctx context.Context, endpoint, username, password, systemID string) (string, error) {
	baseURL := normalizeRedfishEndpoint(endpoint)
	serialConsoleURL := fmt.Sprintf("%s/redfish/v1/Systems/%s/SerialConsole", baseURL, systemID)

	req, err := http.NewRequestWithContext(ctx, "GET", serialConsoleURL, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(username, password)

	resp, err := t.httpClient.Do(req)
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
func (t *RedfishTransport) connectWebSocket(ctx context.Context, wsURI, username, password string) error {
	// Set up WebSocket dialer with authentication
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}

	// Create request headers with basic auth
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+redfishBasicAuth(username, password))

	// Connect to WebSocket
	conn, _, err := dialer.DialContext(ctx, wsURI, headers)
	if err != nil {
		return err
	}

	t.wsConn = conn
	return nil
}

// handleWebSocketData manages bidirectional WebSocket data flow
func (t *RedfishTransport) handleWebSocketData(ctx context.Context) {
	defer func() {
		close(t.readCh)
		close(t.writeCh)
		if t.wsConn != nil {
			t.wsConn.Close()
		}
	}()

	// Start read goroutine
	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			default:
				_, message, err := t.wsConn.ReadMessage()
				if err != nil {
					if !t.isConnectionClosed(err) {
						t.mu.Lock()
						t.status = TransportStatus{Connected: false, Protocol: "redfish", Message: fmt.Sprintf("WebSocket error: %v", err)}
						t.mu.Unlock()
					}
					return
				}

				if len(message) > 0 {
					select {
					case t.readCh <- message:
					case <-t.stopCh:
						return
					default:
						// Buffer full, drop data
					}
				}
			}
		}
	}()

	// Handle write data
	for {
		select {
		case <-t.stopCh:
			return
		case <-ctx.Done():
			return
		case data := <-t.writeCh:
			if err := t.wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
				t.mu.Lock()
				t.status = TransportStatus{Connected: false, Protocol: "redfish", Message: fmt.Sprintf("write error: %v", err)}
				t.mu.Unlock()
				return
			}
		}
	}
}

// isConnectionClosed checks if the error indicates a closed connection
func (t *RedfishTransport) isConnectionClosed(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
}

// redfishBasicAuth creates a basic auth string for Redfish
func redfishBasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// normalizeRedfishEndpoint ensures endpoint has proper URL scheme
func normalizeRedfishEndpoint(endpoint string) string {
	// If already has scheme, return as-is
	if len(endpoint) > 7 && (endpoint[:7] == "http://" || endpoint[:8] == "https://") {
		return endpoint
	}
	// Default to http:// for endpoints without scheme
	return "http://" + endpoint
}
