package gateway

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"gateway/internal/session"
)

// Session Helper - Web Session Management for Console Sessions
//
// This file contains helper functions for creating and managing web sessions
// that map session cookies to JWT tokens for console authentication.
//
// CURRENT USAGE (Phase 1):
// - CreateWebSessionForConsole: Called by session interceptor after CreateSOLSession/CreateVNCSession
// - GetWebSessionStore: Used by power operation handlers to validate session cookies
//
// FUTURE USAGE (Phase 2):
// - Token renewal will update WebSession.CustomerJWT when tokens are refreshed
// - Background worker will call UpdateSessionToken() to keep sessions alive

// CreateWebSessionForConsole creates a web session for a console session
// This is called by the session interceptor after CreateSOLSession/CreateVNCSession
// to establish cookie-based authentication for the web console
func (h *RegionalGatewayHandler) CreateWebSessionForConsole(
	solOrVNCSessionID string,
	customerJWT string,
	serverID string,
) (*session.WebSession, error) {
	// Extract JWT claims
	claims, err := session.ExtractJWTClaims(customerJWT)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JWT claims: %w", err)
	}

	// Generate secure session ID for cookie
	webSessionID, err := session.GenerateSecureSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Calculate token expiration and renewal times
	tokenExpiresAt := time.Unix(claims.ExpiresAt, 0)
	tokenRenewalAt := session.CalculateRenewalTime(tokenExpiresAt)

	// Create web session
	now := time.Now()
	webSession := &session.WebSession{
		ID:             webSessionID,
		SOLSessionID:   solOrVNCSessionID, // Will be set for SOL sessions
		VNCSessionID:   solOrVNCSessionID, // Will be set for VNC sessions (same ID)
		CustomerJWT:    customerJWT,
		CreatedAt:      now,
		LastActivityAt: now,
		ExpiresAt:      now.Add(session.DefaultSessionDuration), // 24 hours
		TokenExpiresAt: tokenExpiresAt,
		TokenRenewalAt: tokenRenewalAt,
		CustomerID:     claims.CustomerID,
		ServerID:       serverID,
	}

	// Store web session
	if err := h.webSessionStore.Create(webSession); err != nil {
		return nil, fmt.Errorf("failed to create web session: %w", err)
	}

	log.Info().
		Str("web_session_id", webSessionID).
		Str("console_session_id", solOrVNCSessionID).
		Str("customer_id", claims.CustomerID).
		Str("server_id", serverID).
		Time("token_expires_at", tokenExpiresAt).
		Time("token_renewal_at", tokenRenewalAt).
		Msg("Created web session for console")

	return webSession, nil
}

// GetWebSessionStore returns the web session store for middleware use
func (h *RegionalGatewayHandler) GetWebSessionStore() session.Store {
	return h.webSessionStore
}
