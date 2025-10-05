package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	baseconf "core/config"
	"core/streaming"
	"gateway/gen/gateway/v1/gatewayv1connect"
	"gateway/internal/gateway"
	"gateway/internal/session"
	gatewaystreaming "gateway/internal/streaming"
	"gateway/internal/webui"
	"gateway/pkg/config"
	"manager/pkg/auth"
)

func init() {
	// Configure zerolog for human-friendly console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.Kitchen})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// Load configuration
	configFile := baseconf.FindConfigFile("gateway")
	envFile := baseconf.FindEnvironmentFile("gateway")

	cfg, err := config.Load(configFile, envFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure logging based on config
	cfg.Log.ConfigureZerolog()

	log.Info().Msg("Starting BMC Gateway Service")
	log.Info().Str("config_file", configFile).Msg("Configuration loaded")
	log.Info().Str("env_file", envFile).Msg("Environment loaded")
	log.Info().
		Str("log_level", cfg.Log.Level).
		Bool("debug", cfg.Log.Debug).
		Msg("Log level configured")

	// Initialize JWT manager for token validation
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecretKey)

	// Initialize Gateway handler
	gatewayHandler := gateway.NewGatewayHandler(cfg.Gateway.ManagerEndpoint, jwtManager, "gateway-01", cfg.Gateway.Region, cfg.GetListenAddress())

	// Start periodic gateway registration with manager
	ctx := context.Background()
	gatewayHandler.StartPeriodicRegistration(ctx)

	// Create interceptors for authentication, token validation, and session management
	// Order matters: auth extracts JWT → token validation validates it → session sets cookies
	authInterceptor := gateway.NewAuthInterceptor(gatewayHandler)
	sessionInterceptor := gateway.NewSessionCookieInterceptor(gatewayHandler)
	interceptors := connect.WithInterceptors(
		authInterceptor, // 1. Extract JWT from header or session cookie
		gatewayHandler.TokenValidationInterceptor(), // 2. Validate the JWT token
		sessionInterceptor,                          // 3. Set session cookies for CreateSOLSession/CreateVNCSession
	)

	// Create the Connect service handler
	path, handler := gatewayv1connect.NewGatewayServiceHandler(
		gatewayHandler,
		interceptors,
	)

	log.Info().Msg("Gateway starting with shared webui templates")

	corsHandler := setupRouter(path, cfg.Gateway.Region, cfg.Gateway.ManagerEndpoint, handler, gatewayHandler)

	// Create server with HTTP/2 support
	server := &http.Server{
		Addr:    cfg.GetListenAddress(),
		Handler: h2c.NewHandler(corsHandler, &http2.Server{}),
	}

	log.Info().
		Str("address", cfg.GetListenAddress()).
		Str("gateway_id", "gateway-01").
		Str("region", cfg.Gateway.Region).
		Str("manager_endpoint", cfg.Gateway.ManagerEndpoint).
		Str("rpc_path", path).
		Bool("rate_limiting", cfg.Gateway.RateLimit.Enabled).
		Msg("Starting gateway server")
	log.Info().Msgf("Health check: http://%s/health", cfg.GetListenAddress())
	log.Info().Msgf("Gateway status: http://%s/status", cfg.GetListenAddress())

	if err := server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

func setupRouter(path, region, managerEndpoint string, handler http.Handler, gatewayHandler *gateway.RegionalGatewayHandler) http.Handler {
	// Create a new Gorilla Mux router
	r := mux.NewRouter()

	// Wrap Connect handler to inject HTTP request and response writer into context
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := gateway.WithHTTPResponseWriter(req.Context(), w)
		ctx = gateway.WithHTTPRequest(ctx, req)
		handler.ServeHTTP(w, req.WithContext(ctx))
	})

	// Register the provided path with the wrapped handler
	r.PathPrefix(path).Handler(wrappedHandler)

	// Add health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "gateway", "region": "` + region + `"}`))
	}).Methods("GET")

	// Add status endpoint (gateway-specific status)
	r.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Gather gateway status information
		agentRegistry := gatewayHandler.GetAgentRegistry()
		agents := agentRegistry.List()

		// Build agent status list
		agentStatuses := make([]map[string]interface{}, 0, len(agents))
		for _, agent := range agents {
			agentStatuses = append(agentStatuses, map[string]interface{}{
				"id":         agent.ID,
				"datacenter": agent.DatacenterID,
				"endpoint":   agent.Endpoint,
				"last_seen":  agent.LastSeen,
				"status":     agent.Status,
			})
		}

		// Get active session counts from handler
		sessionCount := gatewayHandler.GetConsoleSessionCount()

		status := map[string]interface{}{
			"service":                 "gateway",
			"region":                  region,
			"manager_endpoint":        managerEndpoint,
			"agents":                  agentStatuses,
			"agent_count":             len(agents),
			"active_console_sessions": sessionCount,
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(status); err != nil {
			log.Error().Err(err).Msg("Failed to encode status response")
		}
	}).Methods("GET")

	// Add metrics endpoint for monitoring
	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Gateway Metrics\n# TODO: Implement Prometheus metrics\n"))
	}).Methods("GET")

	// Create WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	// VNC HTML viewer handler (serves noVNC interface)
	r.HandleFunc("/vnc/{sessionId}", func(w http.ResponseWriter, r *http.Request) {
		vncViewerHandler(w, r, gatewayHandler)
	}).Methods("GET")

	// VNC WebSocket handler (for data streaming)
	r.HandleFunc("/vnc/{sessionId}/ws", func(w http.ResponseWriter, r *http.Request) {
		vncWebSocketHandler(w, r, gatewayHandler, &upgrader)
	}).Methods("GET")

	// Console HTML viewer handler (serves console interface)
	r.HandleFunc("/console/{sessionId}", func(w http.ResponseWriter, r *http.Request) {
		consoleViewerHandler(w, r, gatewayHandler)
	}).Methods("GET")

	// Console WebSocket handler (for terminal data streaming)
	r.HandleFunc("/console/{sessionId}/ws", func(w http.ResponseWriter, r *http.Request) {
		consoleWebSocketHandler(w, r, gatewayHandler, &upgrader)
	}).Methods("GET")

	// Add CORS middleware for web clients
	corsHandler := addCORS(r)

	return corsHandler
}

// proxyVNCThroughAgent uses buf Connect streaming RPC to proxy VNC data between WebSocket and agent
func proxyVNCThroughAgent(wsConn *websocket.Conn, vncSession *gateway.VNCSession, gatewayHandler *gateway.RegionalGatewayHandler) error {
	log.Info().
		Str("session_id", vncSession.SessionID).
		Str("server_id", vncSession.ServerID).
		Str("agent_id", vncSession.AgentID).
		Msg("Starting buf Connect streaming VNC proxy")

	// Get agent information to create client connection
	agentInfo := gatewayHandler.GetAgentRegistry().Get(vncSession.AgentID)
	if agentInfo == nil {
		return fmt.Errorf("agent not found: %s", vncSession.AgentID)
	}

	// Create Connect client for the agent with HTTP/2 support
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Use plain HTTP connection for h2c (HTTP/2 without TLS)
				return net.Dial(network, addr)
			},
		},
	}
	agentClient := gatewayv1connect.NewGatewayServiceClient(httpClient, agentInfo.Endpoint)

	// Create bidirectional streaming connection to agent
	ctx := context.Background()
	stream := agentClient.StreamVNCData(ctx)

	// Send initial handshake to agent
	helper := streaming.NewHandshakeHelper(&gatewaystreaming.VNCChunkFactory{})
	if err := helper.SendHandshake(stream, vncSession.SessionID, vncSession.ServerID); err != nil {
		return fmt.Errorf("failed to send handshake to agent: %w", err)
	}

	log.Debug().Str("server_id", vncSession.ServerID).Msg("Sent VNC handshake to agent")

	// Use common streaming proxy to handle bidirectional data flow
	logger := log.With().
		Str("session_id", vncSession.SessionID).
		Str("server_id", vncSession.ServerID).
		Str("protocol", "vnc").
		Logger()

	proxy := streaming.NewWebSocketToStreamProxy(
		wsConn,
		vncSession.SessionID,
		vncSession.ServerID,
		logger,
		&gatewaystreaming.VNCChunkFactory{},
	)

	return proxy.ProxyToStream(ctx, stream)
}

// proxySOLThroughAgent establishes a SOL proxy connection through the appropriate agent
func proxySOLThroughAgent(wsConn *websocket.Conn, solSession *gateway.SOLSession, gatewayHandler *gateway.RegionalGatewayHandler) error {
	log.Info().
		Str("session_id", solSession.SessionID).
		Str("server_id", solSession.ServerID).
		Str("agent_id", solSession.AgentID).
		Msg("Starting buf Connect streaming SOL proxy")

	// Get agent information to create client connection
	agentInfo := gatewayHandler.GetAgentRegistry().Get(solSession.AgentID)
	if agentInfo == nil {
		return fmt.Errorf("agent not found: %s", solSession.AgentID)
	}

	// Create Connect client for the agent with HTTP/2 support
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				// Use plain HTTP connection for h2c (HTTP/2 without TLS)
				return net.Dial(network, addr)
			},
		},
	}
	agentClient := gatewayv1connect.NewGatewayServiceClient(httpClient, agentInfo.Endpoint)

	// Create bidirectional streaming connection to agent
	ctx := context.Background()
	stream := agentClient.StreamConsoleData(ctx)

	// Send initial handshake to agent
	helper := streaming.NewHandshakeHelper(&gatewaystreaming.ConsoleChunkFactory{})
	if err := helper.SendHandshake(stream, solSession.SessionID, solSession.ServerID); err != nil {
		return fmt.Errorf("failed to send handshake to agent: %w", err)
	}

	log.Debug().Str("server_id", solSession.ServerID).Msg("Sent SOL handshake to agent")

	// Use common streaming proxy to handle bidirectional data flow
	logger := log.With().
		Str("session_id", solSession.SessionID).
		Str("server_id", solSession.ServerID).
		Str("protocol", "sol").
		Logger()

	proxy := streaming.NewWebSocketToStreamProxy(
		wsConn,
		solSession.SessionID,
		solSession.ServerID,
		logger,
		&gatewaystreaming.ConsoleChunkFactory{},
	)

	return proxy.ProxyToStream(ctx, stream)
}

func vncWebSocketHandler(w http.ResponseWriter, r *http.Request, gatewayHandler *gateway.RegionalGatewayHandler, upgrader *websocket.Upgrader) {
	log.Debug().Str("url_path", r.URL.Path).Msg("VNC WebSocket handler called")

	// Extract session ID from URL parameters
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if sessionID == "" {
		log.Warn().Msg("VNC WebSocket: No session ID provided")
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	log.Debug().Str("session_id", sessionID).Msg("VNC WebSocket: Looking for session")

	// Get VNC session from gateway handler
	vncSession, exists := gatewayHandler.GetVNCSessionByID(sessionID)
	if !exists {
		log.Warn().Str("session_id", sessionID).Msg("VNC WebSocket: Session not found or expired")
		http.Error(w, "VNC session not found or expired", http.StatusNotFound)
		return
	}

	log.Debug().Str("server_id", vncSession.ServerID).Msg("VNC WebSocket: Found session")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("VNC WebSocket: Failed to upgrade to WebSocket")
		return
	}
	defer conn.Close()

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", vncSession.ServerID).
		Msg("VNC WebSocket connection established")

	// Use buf Connect RPC to request agent to start VNC proxy
	err = proxyVNCThroughAgent(conn, vncSession, gatewayHandler)
	if err != nil {
		log.Error().Err(err).Msg("VNC proxy failed")
		return
	}

	log.Info().Str("session_id", sessionID).Msg("VNC WebSocket connection closed")
}

func vncViewerHandler(w http.ResponseWriter, r *http.Request, gatewayHandler *gateway.RegionalGatewayHandler) {
	// Extract session ID from URL parameters
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get VNC session from gateway handler
	vncSession, exists := gatewayHandler.GetVNCSessionByID(sessionID)
	if !exists {
		http.Error(w, "VNC session not found or expired", http.StatusNotFound)
		return
	}

	// Find the associated web session and set the cookie
	// The web session was created when CreateVNCSession was called
	sessionStore := gatewayHandler.GetWebSessionStore()

	// Search for web session with this VNC session ID
	webSession := findWebSessionByVNCSessionID(sessionStore, sessionID)
	if webSession != nil {
		// Set the session cookie for the browser (infer security from request)
		cookie := session.CreateSessionCookieForRequest(webSession.ID, int(session.DefaultSessionDuration.Seconds()), r)
		http.SetCookie(w, cookie)

		log.Debug().
			Str("vnc_session_id", sessionID).
			Str("web_session_id", webSession.ID).
			Bool("secure", cookie.Secure).
			Msg("Set session cookie for VNC viewer")
	} else {
		log.Warn().
			Str("vnc_session_id", sessionID).
			Msg("No web session found for VNC session - cookie not set")
	}

	// Generate WebSocket URL for this session
	protocol := "ws"
	if r.TLS != nil {
		protocol = "wss"
	}
	wsURL := protocol + "://" + r.Host + "/vnc/" + sessionID + "/ws"

	// Prepare data for VNC template
	data := webui.VNCData{
		TemplateData: webui.TemplateData{
			Title:         "VNC Console - " + vncSession.ServerID,
			IconText:      "VNC",
			HeaderTitle:   "VNC Console - " + vncSession.ServerID,
			InitialStatus: "Connecting...",
		},
		SessionID:    sessionID,
		ServerID:     vncSession.ServerID,
		WebSocketURL: wsURL,
	}

	// Render template
	reader, err := webui.RenderVNC(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to render VNC template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Serve HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", vncSession.ServerID).
		Msg("Served VNC viewer")
}

// CORS middleware to handle web clients
func addCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, Connect-Protocol-Version, Connect-Timeout-Ms")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version, Connect-Timeout-Ms")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// consoleViewerHandler serves the console HTML interface
func consoleViewerHandler(w http.ResponseWriter, r *http.Request, gatewayHandler *gateway.RegionalGatewayHandler) {
	// Extract session ID from URL parameters
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get SOL session from gateway handler
	solSession, exists := gatewayHandler.GetSOLSessionByID(sessionID)
	if !exists {
		http.Error(w, "Console session not found or expired", http.StatusNotFound)
		return
	}

	// Find the associated web session and set the cookie
	// The web session was created when CreateSOLSession was called
	sessionStore := gatewayHandler.GetWebSessionStore()

	// Search for web session with this SOL session ID
	webSession := findWebSessionBySOLSessionID(sessionStore, sessionID)
	if webSession != nil {
		// Set the session cookie for the browser (infer security from request)
		cookie := session.CreateSessionCookieForRequest(webSession.ID, int(session.DefaultSessionDuration.Seconds()), r)
		http.SetCookie(w, cookie)

		log.Debug().
			Str("sol_session_id", sessionID).
			Str("web_session_id", webSession.ID).
			Bool("secure", cookie.Secure).
			Msg("Set session cookie for console viewer")
	} else {
		log.Warn().
			Str("sol_session_id", sessionID).
			Msg("No web session found for SOL session - cookie not set")
	}

	// Generate WebSocket URL for this session
	protocol := "ws"
	if r.TLS != nil {
		protocol = "wss"
	}
	wsURL := protocol + "://" + r.Host + "/console/" + sessionID + "/ws"

	// Prepare data for console template
	data := webui.ConsoleData{
		TemplateData: webui.TemplateData{
			Title:         "SOL Console - " + solSession.ServerID,
			IconText:      "SOL",
			HeaderTitle:   "SOL Console - " + solSession.ServerID,
			InitialStatus: "Connecting...",
		},
		SessionID:       sessionID,
		ServerID:        solSession.ServerID,
		GatewayEndpoint: r.Host,
		WebSocketURL:    wsURL,
	}

	// Render console template
	reader, err := webui.RenderConsole(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to render console template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Serve HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", solSession.ServerID).
		Msg("Served console viewer")
}

// consoleWebSocketHandler handles WebSocket connections for console data
func consoleWebSocketHandler(w http.ResponseWriter, r *http.Request, gatewayHandler *gateway.RegionalGatewayHandler, upgrader *websocket.Upgrader) {
	// Extract session ID from URL parameters
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get SOL session from gateway handler
	solSession, exists := gatewayHandler.GetSOLSessionByID(sessionID)
	if !exists {
		http.Error(w, "Console session not found or expired", http.StatusNotFound)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade to WebSocket")
		return
	}
	defer conn.Close()

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", solSession.ServerID).
		Msg("Console WebSocket connection established")

	// Send initial connection message
	welcomeMsg := map[string]interface{}{
		"type": "welcome",
		"data": map[string]string{
			"message":   "Console session connected",
			"sessionId": sessionID,
			"serverId":  solSession.ServerID,
		},
	}

	if err := conn.WriteJSON(welcomeMsg); err != nil {
		log.Error().Err(err).Msg("Failed to write welcome message")
		return
	}

	// Proxy SOL data through the agent
	err = proxySOLThroughAgent(conn, solSession, gatewayHandler)
	if err != nil {
		log.Error().Err(err).Msg("SOL proxy error")
	}

	log.Info().Str("session_id", sessionID).Msg("Console WebSocket connection closed")
}

// findWebSessionBySOLSessionID finds a web session by SOL session ID
func findWebSessionBySOLSessionID(sessionStore session.Store, solSessionID string) *session.WebSession {
	webSession, err := sessionStore.GetBySOLSessionID(solSessionID)
	if err != nil {
		return nil
	}
	return webSession
}

// findWebSessionByVNCSessionID finds a web session by VNC session ID
func findWebSessionByVNCSessionID(sessionStore session.Store, vncSessionID string) *session.WebSession {
	webSession, err := sessionStore.GetByVNCSessionID(vncSessionID)
	if err != nil {
		return nil
	}
	return webSession
}

// getJWTFromRequest extracts JWT token from session cookie or Authorization header
// Priority: 1. Session cookie (for web console), 2. Authorization header (for CLI/API)
func getJWTFromRequest(r *http.Request, gatewayHandler *gateway.RegionalGatewayHandler) (string, error) {
	// Import session package at the top of the file
	// Try session cookie first
	sessionStore := gatewayHandler.GetWebSessionStore()
	sessionID, err := session.GetSessionIDFromCookie(r)
	if err == nil {
		// Session cookie found - retrieve JWT from session
		webSession, err := sessionStore.Get(sessionID)
		if err == nil {
			// Update activity timestamp
			sessionStore.UpdateActivity(sessionID)
			return webSession.CustomerJWT, nil
		}
		// Session expired or invalid - fall through to header auth
	}

	// Fallback to Authorization header (for CLI and direct API access)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("no authentication provided")
	}

	return session.ExtractJWTFromAuthHeader(authHeader)
}

// handleSimpleVNCTest - simple test to verify WebSocket is working before trying full VNC protocol
