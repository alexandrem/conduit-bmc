package sol

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"core/domain"
	solpkg "local-agent/pkg/sol"
)

// Service manages SOL sessions for the local agent
type Service struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	upgrader websocket.Upgrader
}

// Session represents an active SOL session
type Session struct {
	ID        string
	ServerID  string
	Client    solpkg.Client
	Session   solpkg.Session
	WSConn    *websocket.Conn
	Active    bool
	StartTime string
	stopCh    chan struct{}
	mu        sync.RWMutex
}

// NewService creates a new SOL service
func NewService() *Service {
	return &Service{
		sessions: make(map[string]*Session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from any origin in development
			},
		},
	}
}

// HandleConnectionForServer handles a WebSocket connection for SOL access to a specific server
func (s *Service) HandleConnectionForServer(w http.ResponseWriter, r *http.Request, sessionID string, server *domain.Server) error {
	// Check if server has SOL endpoint
	if server.SOLEndpoint == nil {
		return fmt.Errorf("SOL not available for server %s", server.ID)
	}

	log.Info().
		Str("server_id", server.ID).
		Str("endpoint", server.SOLEndpoint.Endpoint).
		Str("type", server.SOLEndpoint.Type.String()).
		Msg("Setting up SOL connection")

	// Create SOL client based on type
	solClient, err := solpkg.NewClient(server.SOLEndpoint.Type)
	if err != nil {
		return fmt.Errorf("failed to create SOL client: %w", err)
	}

	// Create SOL configuration
	config := &solpkg.Config{
		BaudRate:       115200,
		FlowControl:    "none",
		TimeoutSeconds: 300,
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade WebSocket: %w", err)
	}

	// Create SOL session
	session := &Session{
		ID:        sessionID,
		ServerID:  server.ID,
		Client:    solClient,
		WSConn:    conn,
		Active:    false,
		StartTime: fmt.Sprintf("%d" /* time.Now().Unix() */, 0), // Simplified for now
		stopCh:    make(chan struct{}),
	}

	// Store session
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	// Start SOL session in goroutine
	go s.handleSOLSession(session, server, config)

	return nil
}

// handleSOLSession manages the SOL session lifecycle
func (s *Service) handleSOLSession(session *Session, server *domain.Server, config *solpkg.Config) {
	defer func() {
		// Clean up session
		s.mu.Lock()
		delete(s.sessions, session.ID)
		s.mu.Unlock()

		// Close WebSocket
		if session.WSConn != nil {
			session.WSConn.Close()
		}

		// Close SOL session
		if session.Session != nil {
			session.Session.Close()
		}

		log.Info().
			Str("session_id", session.ID).
			Str("server_id", session.ServerID).
			Msg("SOL session cleaned up")
	}()

	ctx := context.Background()

	// Create SOL session
	solSession, err := session.Client.CreateSession(
		ctx,
		server.SOLEndpoint.Endpoint,
		server.SOLEndpoint.Username,
		server.SOLEndpoint.Password,
		config,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("session_id", session.ID).
			Str("server_id", session.ServerID).
			Msg("Failed to create SOL session")
		session.WSConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Failed to create SOL session: %v", err)))
		return
	}

	session.mu.Lock()
	session.Session = solSession
	session.Active = true
	session.mu.Unlock()

	log.Info().
		Str("session_id", session.ID).
		Str("server_id", session.ServerID).
		Msg("SOL session started successfully")

	// Start reading from SOL and forwarding to WebSocket
	go s.forwardSOLToWebSocket(session, ctx)

	// Start reading from WebSocket and forwarding to SOL
	go s.forwardWebSocketToSOL(session, ctx)

	// Wait for session to end
	<-session.stopCh
}

// forwardSOLToWebSocket reads from SOL session and forwards to WebSocket
func (s *Service) forwardSOLToWebSocket(session *Session, ctx context.Context) {
	defer func() {
		close(session.stopCh)
	}()

	for {
		select {
		case <-session.stopCh:
			return
		default:
			// Read from SOL session
			data, err := session.Session.Read(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Error reading from SOL session")
				return
			}

			if len(data) > 0 {
				// Forward to WebSocket
				session.mu.RLock()
				if session.WSConn != nil {
					err = session.WSConn.WriteMessage(websocket.BinaryMessage, data)
					if err != nil {
						log.Error().Err(err).Msg("Error writing to WebSocket")
						session.mu.RUnlock()
						return
					}
				}
				session.mu.RUnlock()
			}
		}
	}
}

// forwardWebSocketToSOL reads from WebSocket and forwards to SOL session
func (s *Service) forwardWebSocketToSOL(session *Session, ctx context.Context) {
	defer func() {
		close(session.stopCh)
	}()

	for {
		select {
		case <-session.stopCh:
			return
		default:
			// Read from WebSocket
			_, data, err := session.WSConn.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("Error reading from WebSocket")
				return
			}

			if len(data) > 0 {
				// Forward to SOL session
				err = session.Session.Write(ctx, data)
				if err != nil {
					log.Error().Err(err).Msg("Error writing to SOL session")
					return
				}
			}
		}
	}
}

// GetActiveSessions returns all active SOL sessions
func (s *Service) GetActiveSessions() map[string]*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*Session)
	for id, session := range s.sessions {
		result[id] = session
	}
	return result
}

// Stop gracefully stops the SOL service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Info().Int("active_sessions", len(s.sessions)).Msg("Stopping SOL service")

	for sessionID, session := range s.sessions {
		log.Debug().Str("session_id", sessionID).Msg("Closing SOL session")
		session.mu.Lock()
		if session.Active {
			close(session.stopCh)
		}
		session.mu.Unlock()
	}

	// Clear sessions map
	s.sessions = make(map[string]*Session)

	return nil
}
