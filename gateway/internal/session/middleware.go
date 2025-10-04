package session

import (
	"context"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Session Middleware - Multi-Phase Implementation
//
// CURRENT USAGE (Phase 1):
// - Session validation is handled directly in power operation handlers via getJWTFromRequest()
// - This approach avoids middleware complexity while establishing the session infrastructure
//
// FUTURE USAGE (Phase 3):
// - RequireSession middleware will protect console viewer endpoints (/console/{id}, /vnc/{id})
// - OptionalSession middleware for mixed authenticated/public endpoints
// - WebSocket handlers will use GetSessionFromContext() for authentication
// - REST handlers will migrate to use GetJWTOrFallback() instead of direct store access
//
// Why the middleware isn't used yet:
// - Phase 1 focuses on establishing session storage and cookie handling
// - Direct session lookup in handlers is simpler for initial implementation
// - Phase 3 will refactor to use middleware for cleaner separation of concerns

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// ContextKey is the context key for the web session
	ContextKey contextKey = "web_session"
	// JWTContextKey is the context key for the JWT token
	JWTContextKey contextKey = "jwt_token"
)

// Middleware creates HTTP middleware that validates session cookies
type Middleware struct {
	store Store
}

// NewMiddleware creates a new session middleware
func NewMiddleware(store Store) *Middleware {
	return &Middleware{
		store: store,
	}
}

// RequireSession middleware validates that a valid session cookie is present
// NOTE: Currently unused - prepared for future use when protecting console viewer endpoints
func (m *Middleware) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from cookie
		sessionID, err := GetSessionIDFromRequest(r)
		if err != nil {
			log.Warn().Err(err).Str("path", r.URL.Path).Msg("No session cookie found")
			http.Error(w, "Unauthorized: Session required", http.StatusUnauthorized)
			return
		}

		// Validate session exists and is not expired
		session, err := m.store.Get(sessionID)
		if err != nil {
			if errors.Is(err, ErrSessionExpired) {
				log.Warn().Str("session_id", sessionID).Msg("Session expired")
				http.SetCookie(w, DeleteSessionCookie()) // Clear expired cookie
				http.Error(w, "Unauthorized: Session expired", http.StatusUnauthorized)
				return
			}

			log.Warn().Err(err).Str("session_id", sessionID).Msg("Invalid session")
			http.Error(w, "Unauthorized: Invalid session", http.StatusUnauthorized)
			return
		}

		// Update last activity
		m.store.UpdateActivity(sessionID)

		// Add session and JWT to context
		ctx := context.WithValue(r.Context(), ContextKey, session)
		ctx = context.WithValue(ctx, JWTContextKey, session.CustomerJWT)

		// Continue with session in context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalSession middleware adds session to context if available, but doesn't require it
// NOTE: Currently unused - prepared for future use with mixed authenticated/public endpoints
func (m *Middleware) OptionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract session ID from cookie
		sessionID, err := GetSessionIDFromRequest(r)
		if err == nil {
			// Validate session
			session, err := m.store.Get(sessionID)
			if err == nil {
				// Update activity
				m.store.UpdateActivity(sessionID)

				// Add to context
				ctx := context.WithValue(r.Context(), ContextKey, session)
				ctx = context.WithValue(ctx, JWTContextKey, session.CustomerJWT)
				r = r.WithContext(ctx)
			}
		}

		// Continue regardless of session presence
		next.ServeHTTP(w, r)
	})
}

// GetSessionFromContext retrieves the web session from the request context
// NOTE: Currently unused - prepared for Phase 3 when WebSocket handlers will use middleware
func GetSessionFromContext(ctx context.Context) (*WebSession, bool) {
	session, ok := ctx.Value(ContextKey).(*WebSession)
	return session, ok
}

// GetJWTFromContext retrieves the JWT token from the request context
// NOTE: Currently unused - prepared for Phase 3 when WebSocket handlers will use middleware
func GetJWTFromContext(ctx context.Context) (string, bool) {
	jwt, ok := ctx.Value(JWTContextKey).(string)
	return jwt, ok
}

// GetJWTOrFallback retrieves JWT from context, or falls back to Authorization header
// NOTE: Currently unused - prepared for Phase 3 when handlers use middleware instead of direct session lookup
func GetJWTOrFallback(r *http.Request) (string, error) {
	// Try context first (from session)
	if jwt, ok := GetJWTFromContext(r.Context()); ok {
		return jwt, nil
	}

	// Fallback to Authorization header (for API calls with direct JWT)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		return ExtractJWTFromAuthHeader(authHeader)
	}

	return "", ErrSessionNotFound
}
