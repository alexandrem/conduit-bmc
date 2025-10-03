package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"core/identity"
	"core/types"
	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"
	"local-agent/internal/discovery"
	solservice "local-agent/internal/sol"
	"local-agent/pkg/bmc"
	"local-agent/pkg/config"
)

func init() {
	// Configure zerolog for human-friendly console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// LocalAgent represents a Local Agent that runs in each datacenter
type LocalAgent struct {
	config           *config.Config
	discoveryService *discovery.Service
	gatewayClient    gatewayv1connect.GatewayServiceClient
	httpClient       *http.Client
	bmcClient        *bmc.Client

	// Services
	solService *solservice.Service
	httpServer *http.Server

	// Current state
	discoveredServers map[string]*discovery.Server
	registered        bool
}

func NewLocalAgent(cfg *config.Config, discoveryService *discovery.Service, bmcClient *bmc.Client) *LocalAgent {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	gatewayClient := gatewayv1connect.NewGatewayServiceClient(
		httpClient,
		cfg.Agent.GatewayEndpoint,
	)

	// Initialize SOL service
	solService := solservice.NewService()

	agent := &LocalAgent{
		config:            cfg,
		discoveryService:  discoveryService,
		gatewayClient:     gatewayClient,
		httpClient:        httpClient,
		bmcClient:         bmcClient,
		solService:        solService,
		discoveredServers: make(map[string]*discovery.Server),
	}

	// Setup HTTP/Connect server
	agent.setupServer(cfg.Agent.HTTPPort)

	return agent
}

// validateDependencies checks that required system dependencies are available
func (a *LocalAgent) validateDependencies() error {
	// Check if IPMI discovery is enabled or any static IPMI servers exist
	ipmiEnabled := a.config.Agent.BMCDiscovery.Enabled && a.config.Agent.BMCDiscovery.EnableIPMIDetection

	// Also check static hosts
	hasStaticIPMI := false
	for _, host := range a.config.Static.Hosts {
		if host.ControlEndpoint != nil && host.ControlEndpoint.InferType() == types.BMCTypeIPMI {
			hasStaticIPMI = true
			break
		}
		if host.SOLEndpoint != nil && host.SOLEndpoint.InferType() == types.SOLTypeIPMI {
			hasStaticIPMI = true
			break
		}
	}

	// If IPMI support is enabled (via discovery or static config), validate ipmiconsole is available
	if ipmiEnabled || hasStaticIPMI {
		// Check common paths first
		ipmiconsole := "/usr/sbin/ipmiconsole"
		if _, err := os.Stat(ipmiconsole); os.IsNotExist(err) {
			// Try to find in PATH
			if _, err := exec.LookPath("ipmiconsole"); err != nil {
				reason := ""
				if ipmiEnabled && hasStaticIPMI {
					reason = "IPMI discovery is enabled and static IPMI servers are configured"
				} else if ipmiEnabled {
					reason = "IPMI discovery is enabled"
				} else {
					reason = "static IPMI servers are configured"
				}

				return fmt.Errorf(
					"ipmiconsole binary not found but %s. "+
						"Please install the freeipmi package for your system",
					reason,
				)
			}
		}
		log.Info().
			Bool("ipmi_discovery", ipmiEnabled).
			Bool("static_ipmi", hasStaticIPMI).
			Msg("ipmiconsole found - IPMI SOL console support enabled")
	} else {
		log.Debug().Msg("IPMI support not enabled - ipmiconsole not required")
	}

	return nil
}

// Start begins the agent's operation
func (a *LocalAgent) Start(ctx context.Context) error {
	log.Info().
		Str("agent_id", a.config.Agent.ID).
		Int("port", a.config.Agent.HTTPPort).
		Msg("Starting Local Agent")

	// Validate required dependencies before starting
	if err := a.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	// Start HTTP server in goroutine
	go func() {
		log.Info().
			Int("port", a.config.Agent.HTTPPort).
			Msg("Starting HTTP server for agent endpoints")
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// Initial registration with exponential backoff
	a.retryRegistration(ctx)

	// Start periodic discovery and heartbeat
	ticker := time.NewTicker(a.config.Agent.BMCDiscovery.ScanInterval)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Retry ticker for failed registrations (starts disabled)
	retryTicker := time.NewTicker(5 * time.Second)
	retryTicker.Stop()
	defer retryTicker.Stop()

	log.Info().
		Str("agent_id", a.config.Agent.ID).
		Msg("Agent started successfully, entering main loop")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Agent stopping due to context cancellation")
			return ctx.Err()

		case <-ticker.C:
			if err := a.discoverAndRegister(ctx); err != nil {
				log.Warn().Err(err).Msg("Discovery/registration failed")
				a.registered = false
				// Enable fast retry
				retryTicker.Reset(5 * time.Second)
			}

		case <-retryTicker.C:
			// Fast retry for failed registrations
			if !a.registered {
				if err := a.discoverAndRegister(ctx); err != nil {
					log.Warn().Err(err).Msg("Retry registration failed")
				} else {
					// Success! Stop fast retry
					retryTicker.Stop()
				}
			}

		case <-heartbeatTicker.C:
			if err := a.sendHeartbeat(ctx); err != nil {
				log.Warn().Err(err).Msg("Heartbeat failed")
				a.registered = false
				// Enable fast retry
				retryTicker.Reset(5 * time.Second)
			}
		}
	}
}

// Stop gracefully shuts down the agent
func (a *LocalAgent) Stop(ctx context.Context) error {
	log.Info().Str("agent_id", a.config.Agent.ID).Msg("Stopping Local Agent")

	// VNC streaming is now handled via gRPC streaming (no separate service)

	// Stop SOL service
	if a.solService != nil {
		if err := a.solService.Stop(); err != nil {
			log.Error().Err(err).Msg("Error stopping SOL service")
		}
	}

	// Stop HTTP server
	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error stopping HTTP server")
			return err
		}
	}

	return nil
}

// retryRegistration attempts initial registration with exponential backoff.
//
// This handles the common case where the agent starts before the gateway/manager
// are ready. The retry strategy:
//   - Attempt 1: Immediate
//   - Attempt 2: Wait 1s
//   - Attempt 3: Wait 2s
//   - Attempt 4: Wait 4s
//   - Attempt 5: Wait 8s
//
// After 5 attempts (~15s total), the agent enters normal operation mode where
// it will retry every 5 seconds until successful, then switch to periodic
// discovery based on the configured scan interval.
func (a *LocalAgent) retryRegistration(ctx context.Context) {
	maxRetries := 5
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := a.discoverAndRegister(ctx); err != nil {
			delay := time.Duration(1<<uint(attempt)) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}

			log.Printf("Registration attempt %d/%d failed: %v. Retrying in %v",
				attempt+1, maxRetries, err, delay)

			select {
			case <-ctx.Done():
				log.Printf("Context cancelled during registration retry")
				return
			case <-time.After(delay):
				continue
			}
		} else {
			log.Printf("Successfully registered on attempt %d/%d", attempt+1, maxRetries)
			return
		}
	}

	log.Printf("Failed to register after %d attempts. Will retry periodically in main loop", maxRetries)
}

// discoverAndRegister discovers BMCs and registers with Regional Gateway
func (a *LocalAgent) discoverAndRegister(ctx context.Context) error {
	// Always perform discovery
	servers, err := a.discoveryService.DiscoverServers(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	log.Info().
		Int("server_count", len(servers)).
		Str("datacenter_id", a.config.Agent.DatacenterID).
		Msg("Discovered servers")

	// Update internal state
	// Index servers by both their config ID and BMC endpoint to handle manager's ID format
	a.discoveredServers = make(map[string]*discovery.Server)
	for _, server := range servers {
		// Index by original server ID
		a.discoveredServers[server.ID] = server

		// Also index by BMC endpoint for manager-generated IDs
		if server.ControlEndpoint != nil {
			// Use shared logic to generate manager-compatible server ID
			managerID := identity.GenerateServerIDFromBMCEndpoint(
				a.config.Agent.DatacenterID,
				server.ControlEndpoint.Endpoint)
			a.discoveredServers[managerID] = server
			log.Debug().
				Str("config_id", server.ID).
				Str("manager_id", managerID).
				Msg("Indexed server for manager compatibility")
		}
	}

	// Always register to keep server information up-to-date
	// This ensures database has latest endpoint information (SOL/VNC)
	logMsg := "Re-registering with gateway to update server information"
	if !a.registered {
		logMsg = "Agent not registered, attempting initial registration with gateway"
	}
	log.Info().Msg(logMsg)

	if err := a.registerWithGateway(ctx, servers); err != nil {
		return fmt.Errorf("gateway registration failed: %w", err)
	}
	a.registered = true
	log.Debug().Msg("Successfully registered/updated with gateway")

	return nil
}

// registerWithGateway registers this agent and its discovered servers with the Regional Gateway
func (a *LocalAgent) registerWithGateway(ctx context.Context, servers []*discovery.Server) error {
	// Convert servers to BMC endpoint registrations
	var bmcEndpoints []*gatewayv1.BMCEndpointRegistration
	for _, server := range servers {
		bmcEndpoint := &gatewayv1.BMCEndpointRegistration{
			ServerId: server.ID,
			Features: server.Features,
			Status:   server.Status,
			Metadata: server.Metadata,
		}

		// Convert control endpoint
		if server.ControlEndpoint != nil {
			var bmcType gatewayv1.BMCType
			switch server.ControlEndpoint.Type {
			case "ipmi":
				bmcType = gatewayv1.BMCType_BMC_IPMI
			case "redfish":
				bmcType = gatewayv1.BMCType_BMC_REDFISH
			default:
				bmcType = gatewayv1.BMCType_BMC_UNSPECIFIED
			}

			bmcEndpoint.ControlEndpoint = &gatewayv1.BMCControlEndpoint{
				Endpoint:     server.ControlEndpoint.Endpoint,
				Type:         bmcType,
				Username:     server.ControlEndpoint.Username,
				Password:     server.ControlEndpoint.Password,
				Capabilities: server.ControlEndpoint.Capabilities,
				Tls: &gatewayv1.TLSConfig{
					Enabled:            true,
					InsecureSkipVerify: true, // Default for dev
				},
			}
		}

		// Convert SOL endpoint
		if server.SOLEndpoint != nil {
			var solType gatewayv1.SOLType
			switch server.SOLEndpoint.Type {
			case types.SOLTypeIPMI:
				solType = gatewayv1.SOLType_SOL_IPMI
			case types.SOLTypeRedfishSerial:
				solType = gatewayv1.SOLType_SOL_REDFISH_SERIAL
			default:
				solType = gatewayv1.SOLType_SOL_UNSPECIFIED
			}

			bmcEndpoint.SolEndpoint = &gatewayv1.SOLEndpoint{
				Type:     solType,
				Endpoint: server.SOLEndpoint.Endpoint,
				Username: server.SOLEndpoint.Username,
				Password: server.SOLEndpoint.Password,
				Config: &gatewayv1.SOLConfig{
					BaudRate:       115200,
					TimeoutSeconds: 300,
				},
			}
		}

		// Convert VNC endpoint
		if server.VNCEndpoint != nil {
			var vncType gatewayv1.VNCType
			switch server.VNCEndpoint.Type {
			case types.VNCTypeNative:
				vncType = gatewayv1.VNCType_VNC_NATIVE
			case types.VNCTypeWebSocket:
				vncType = gatewayv1.VNCType_VNC_WEBSOCKET
			default:
				vncType = gatewayv1.VNCType_VNC_UNSPECIFIED
			}

			bmcEndpoint.VncEndpoint = &gatewayv1.VNCEndpoint{
				Type:     vncType,
				Endpoint: server.VNCEndpoint.Endpoint,
				Username: server.VNCEndpoint.Username,
				Password: server.VNCEndpoint.Password,
			}
		}

		bmcEndpoints = append(bmcEndpoints, bmcEndpoint)
	}

	// Create registration request
	req := connect.NewRequest(&gatewayv1.RegisterAgentRequest{
		AgentId:      a.config.Agent.ID,
		DatacenterId: a.config.Agent.DatacenterID,
		Endpoint:     a.config.Agent.Endpoint,
		BmcEndpoints: bmcEndpoints,
	})

	// Call Regional Gateway
	resp, err := a.gatewayClient.RegisterAgent(ctx, req)
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("registration rejected: %s", resp.Msg.Message)
	}

	log.Info().
		Str("message", resp.Msg.Message).
		Msg("Successfully registered with Regional Gateway")
	return nil
}

// sendHeartbeat sends a heartbeat to the Regional Gateway
func (a *LocalAgent) sendHeartbeat(ctx context.Context) error {
	if !a.registered {
		log.Debug().Msg("Not registered, skipping heartbeat")
		return nil
	}

	// Convert current servers to BMC endpoint registrations (same as registration)
	var bmcEndpoints []*gatewayv1.BMCEndpointRegistration
	for _, server := range a.discoveredServers {
		bmcEndpoint := &gatewayv1.BMCEndpointRegistration{
			ServerId: server.ID,
			Features: server.Features,
			Status:   server.Status,
			Metadata: server.Metadata,
		}

		// Convert control endpoint
		if server.ControlEndpoint != nil {
			var bmcType gatewayv1.BMCType
			switch server.ControlEndpoint.Type {
			case "ipmi":
				bmcType = gatewayv1.BMCType_BMC_IPMI
			case "redfish":
				bmcType = gatewayv1.BMCType_BMC_REDFISH
			default:
				bmcType = gatewayv1.BMCType_BMC_UNSPECIFIED
			}

			bmcEndpoint.ControlEndpoint = &gatewayv1.BMCControlEndpoint{
				Endpoint:     server.ControlEndpoint.Endpoint,
				Type:         bmcType,
				Username:     server.ControlEndpoint.Username,
				Password:     server.ControlEndpoint.Password,
				Capabilities: server.ControlEndpoint.Capabilities,
			}
		}

		// Convert SOL endpoint
		if server.SOLEndpoint != nil {
			var solType gatewayv1.SOLType
			switch server.SOLEndpoint.Type {
			case types.SOLTypeIPMI:
				solType = gatewayv1.SOLType_SOL_IPMI
			case types.SOLTypeRedfishSerial:
				solType = gatewayv1.SOLType_SOL_REDFISH_SERIAL
			default:
				solType = gatewayv1.SOLType_SOL_UNSPECIFIED
			}

			bmcEndpoint.SolEndpoint = &gatewayv1.SOLEndpoint{
				Type:     solType,
				Endpoint: server.SOLEndpoint.Endpoint,
				Username: server.SOLEndpoint.Username,
				Password: server.SOLEndpoint.Password,
			}
		}

		// Convert VNC endpoint
		if server.VNCEndpoint != nil {
			var vncType gatewayv1.VNCType
			switch server.VNCEndpoint.Type {
			case types.VNCTypeNative:
				vncType = gatewayv1.VNCType_VNC_NATIVE
			case types.VNCTypeWebSocket:
				vncType = gatewayv1.VNCType_VNC_WEBSOCKET
			default:
				vncType = gatewayv1.VNCType_VNC_UNSPECIFIED
			}

			bmcEndpoint.VncEndpoint = &gatewayv1.VNCEndpoint{
				Type:     vncType,
				Endpoint: server.VNCEndpoint.Endpoint,
				Username: server.VNCEndpoint.Username,
				Password: server.VNCEndpoint.Password,
			}
		}

		bmcEndpoints = append(bmcEndpoints, bmcEndpoint)
	}

	// Create heartbeat request
	req := connect.NewRequest(&gatewayv1.AgentHeartbeatRequest{
		AgentId:      a.config.Agent.ID,
		BmcEndpoints: bmcEndpoints,
	})

	// Send heartbeat
	resp, err := a.gatewayClient.AgentHeartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}

	if !resp.Msg.Success {
		log.Warn().Msg("Heartbeat not acknowledged")
		return fmt.Errorf("heartbeat rejected")
	}

	log.Debug().
		Int32("next_interval_seconds", resp.Msg.HeartbeatIntervalSeconds).
		Msg("Heartbeat sent successfully")
	return nil
}

// GetServerCount returns the number of discovered servers
func (a *LocalAgent) GetServerCount() int {
	return len(a.discoveredServers)
}

// IsRegistered returns true if the agent is registered with the Regional Gateway
func (a *LocalAgent) IsRegistered() bool {
	return a.registered
}

// setupServer configures the HTTP server with both REST and Connect RPC endpoints
func (a *LocalAgent) setupServer(port int) {
	router := mux.NewRouter()

	// Register Connect RPC service handler for streaming
	path, handler := gatewayv1connect.NewGatewayServiceHandler(a)
	router.PathPrefix(path).Handler(handler)

	// Setup legacy HTTP routes
	a.setupHTTPRoutes(router)

	// Enable HTTP/2 support for Connect RPC streaming
	a.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: h2c.NewHandler(router, &http2.Server{}),
	}
}

// setupHTTPRoutes configures HTTP endpoints for the agent
func (a *LocalAgent) setupHTTPRoutes(router *mux.Router) {
	// Health check endpoint
	router.HandleFunc("/health", a.handleHealth).Methods("GET")

	// VNC streaming now handled via gRPC StreamVNCData

	// SOL WebSocket endpoint - this is where the gateway will connect
	router.HandleFunc("/sol/{sessionId}/{bmcHost}", a.handleSOLWebSocket).Methods("GET")

	// Agent status endpoint
	router.HandleFunc("/status", a.handleStatus).Methods("GET")

	// VNC sessions now handled via gRPC streaming (no HTTP endpoint needed)

	// Active SOL sessions endpoint
	router.HandleFunc("/sol/sessions", a.handleSOLSessions).Methods("GET")
}

// handleHealth responds to health check requests
func (a *LocalAgent) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":     "healthy",
		"agent_id":   a.config.Agent.ID,
		"registered": a.registered,
		"servers":    len(a.discoveredServers),
	}
	json.NewEncoder(w).Encode(response)
}

// handleStatus provides detailed agent status including full discovery information
func (a *LocalAgent) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Build list of discovered servers with full details
	servers := make([]map[string]interface{}, 0, len(a.discoveredServers))
	serverIDs := make(map[string]bool) // Track unique servers (avoid duplicates from dual indexing)

	for _, server := range a.discoveredServers {
		// Skip if we've already added this server object
		if serverIDs[server.ID] {
			continue
		}
		serverIDs[server.ID] = true

		serverInfo := map[string]interface{}{
			"id":          server.ID,
			"customer_id": server.CustomerID,
			"features":    server.Features,
			"status":      server.Status,
			"metadata":    server.Metadata,
		}

		// Add control endpoint info
		if server.ControlEndpoint != nil {
			serverInfo["control_endpoint"] = map[string]interface{}{
				"endpoint":     server.ControlEndpoint.Endpoint,
				"type":         server.ControlEndpoint.Type,
				"username":     server.ControlEndpoint.Username,
				"capabilities": server.ControlEndpoint.Capabilities,
			}
		}

		// Add SOL endpoint info
		if server.SOLEndpoint != nil {
			serverInfo["sol_endpoint"] = map[string]interface{}{
				"endpoint": server.SOLEndpoint.Endpoint,
				"type":     server.SOLEndpoint.Type,
				"username": server.SOLEndpoint.Username,
			}
		}

		// Add VNC endpoint info
		if server.VNCEndpoint != nil {
			serverInfo["vnc_endpoint"] = map[string]interface{}{
				"endpoint": server.VNCEndpoint.Endpoint,
				"type":     server.VNCEndpoint.Type,
				"username": server.VNCEndpoint.Username,
			}
		}

		// Add all index keys for this server (shows dual indexing)
		indexKeys := []string{}
		for key, val := range a.discoveredServers {
			if val == server {
				indexKeys = append(indexKeys, key)
			}
		}
		serverInfo["index_keys"] = indexKeys

		servers = append(servers, serverInfo)
	}

	response := map[string]interface{}{
		"agent": map[string]interface{}{
			"id":            a.config.Agent.ID,
			"datacenter_id": a.config.Agent.DatacenterID,
			"endpoint":      a.config.Agent.Endpoint,
			"http_port":     a.config.Agent.HTTPPort,
			"registered":    a.registered,
		},
		"discovery": map[string]interface{}{
			"server_count": len(serverIDs),           // Unique server count
			"index_count":  len(a.discoveredServers), // Total index entries (includes dual indexing)
			"servers":      servers,
		},
		"sessions": map[string]interface{}{
			"sol_count": len(a.solService.GetActiveSessions()),
			// VNC sessions now handled via gRPC streaming
		},
	}

	// Pretty print with indentation
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(response)
}

// VNC sessions are now handled via gRPC streaming - no HTTP handler needed

// handleSOLSessions lists active SOL sessions
func (a *LocalAgent) handleSOLSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sessions := a.solService.GetActiveSessions()

	result := make([]map[string]interface{}, 0, len(sessions))
	for sessionID, session := range sessions {
		result = append(result, map[string]interface{}{
			"session_id": sessionID,
			"server_id":  session.ServerID,
			"active":     session.Active,
			"start_time": session.StartTime,
		})
	}

	json.NewEncoder(w).Encode(result)
}

// VNC WebSocket handler removed - now using gRPC StreamVNCData for native TCP streaming

// handleSOLWebSocket handles incoming SOL WebSocket connections from the gateway
func (a *LocalAgent) handleSOLWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]
	serverID := vars["bmcHost"] // Actually server ID now

	if sessionID == "" || serverID == "" {
		http.Error(w, "Missing sessionId or serverID", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Msg("Handling SOL WebSocket connection")

	// Look up server in discovered servers
	server, exists := a.discoveredServers[serverID]
	if !exists {
		http.Error(w, fmt.Sprintf("Server %s not found", serverID), http.StatusNotFound)
		return
	}

	// Check if server has SOL endpoint
	if server.SOLEndpoint == nil {
		http.Error(w, fmt.Sprintf("SOL not available for server %s", serverID), http.StatusNotFound)
		return
	}

	log.Debug().
		Str("server_id", serverID).
		Str("endpoint", server.SOLEndpoint.Endpoint).
		Str("type", server.SOLEndpoint.Type.String()).
		Msg("Found SOL endpoint")

	// Delegate to SOL service with SOL endpoint information
	if err := a.solService.HandleConnectionForServer(w, r, sessionID, server); err != nil {
		log.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("server_id", serverID).
			Msg("SOL connection failed")
		http.Error(w, fmt.Sprintf("SOL connection failed: %v", err), http.StatusInternalServerError)
		return
	}
}
