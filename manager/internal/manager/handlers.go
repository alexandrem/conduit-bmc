package manager

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"manager/pkg/auth"
	"manager/pkg/database"
	"manager/pkg/models"
)

type Handler struct {
	db         *database.DB
	jwtManager *auth.JWTManager
}

func NewHandler(db *database.DB, jwtManager *auth.JWTManager) *Handler {
	return &Handler{
		db:         db,
		jwtManager: jwtManager,
	}
}

// Middleware for authentication
func (h *Handler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := h.jwtManager.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Store claims in request context
		r = r.WithContext(r.Context())
		r.Header.Set("X-Customer-ID", claims.CustomerID)
		r.Header.Set("X-Customer-Email", claims.Email)

		next.ServeHTTP(w, r)
	}
}

// Alternative API Key authentication
func (h *Handler) APIKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key required", http.StatusUnauthorized)
			return
		}

		customer, err := h.db.GetCustomerByAPIKey(apiKey)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		r.Header.Set("X-Customer-ID", customer.ID)
		r.Header.Set("X-Customer-Email", customer.Email)
		next.ServeHTTP(w, r)
	}
}

// Health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// GetServer gets server information.
func (h *Handler) GetServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverId"]
	customerID := r.Header.Get("X-Customer-ID")

	server, err := h.db.GetServerByID(serverID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Server not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Check if customer owns the server
	if server.CustomerID != customerID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	serverInfo := models.ServerInfo{
		ID:              server.ID,
		ControlEndpoint: server.ControlEndpoint,
		SOLEndpoint:     server.SOLEndpoint,
		VNCEndpoint:     server.VNCEndpoint,
		Features:        server.Features,
		Status:          server.Status,
		DatacenterID:    server.DatacenterID,
		Metadata:        server.Metadata,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverInfo)
}

// ListServers lists all servers for the authenticated customer.
func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	customerID := r.Header.Get("X-Customer-ID")
	log.Debug().Str("customer_id", customerID).Msg("ListServers called")

	servers, err := h.db.GetServersByCustomer(customerID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var serverInfos []models.ServerInfo
	for _, server := range servers {
		serverInfos = append(serverInfos, models.ServerInfo{
			ID:              server.ID,
			ControlEndpoint: server.ControlEndpoint,
			SOLEndpoint:     server.SOLEndpoint,
			VNCEndpoint:     server.VNCEndpoint,
			Features:        server.Features,
			Status:          server.Status,
			DatacenterID:    server.DatacenterID,
			Metadata:        server.Metadata,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverInfos)
}

// Create a proxy session for BMC access
func (h *Handler) CreateProxySession(w http.ResponseWriter, r *http.Request) {
	customerID := r.Header.Get("X-Customer-ID")

	var req models.CreateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate server ownership
	server, err := h.db.GetServerByID(req.ServerID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Server not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if server.CustomerID != customerID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Find agent for the server's datacenter
	agent, err := h.db.GetAgentByDatacenter(server.DatacenterID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "No agent available for datacenter", http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Create proxy session
	sessionID := uuid.New().String()
	session := &models.ProxySession{
		ID:         sessionID,
		CustomerID: customerID,
		ServerID:   req.ServerID,
		AgentID:    agent.ID,
		Status:     "active",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(4 * time.Hour), // 4 hour session
	}

	if err := h.db.CreateProxySession(session); err != nil {
		http.Error(w, "Failed to create proxy session", http.StatusInternalServerError)
		return
	}

	response := models.CreateProxyResponse{
		SessionID: sessionID,
		Endpoint:  fmt.Sprintf("wss://gateway.example.com/proxy/%s", sessionID),
		ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Get proxy session information
func (h *Handler) GetProxySession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]
	customerID := r.Header.Get("X-Customer-ID")

	session, err := h.db.GetProxySession(sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Check if customer owns the session
	if session.CustomerID != customerID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		h.db.UpdateProxySessionStatus(sessionID, "expired")
		http.Error(w, "Session expired", http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// Close a proxy session
func (h *Handler) CloseProxySession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]
	customerID := r.Header.Get("X-Customer-ID")

	session, err := h.db.GetProxySession(sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Check if customer owns the session
	if session.CustomerID != customerID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	if err := h.db.UpdateProxySessionStatus(sessionID, "closed"); err != nil {
		http.Error(w, "Failed to close session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
