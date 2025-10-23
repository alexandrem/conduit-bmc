package gateway

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"

	coreauth "core/auth"
	"gateway/internal/session"
)

// AuthInterceptor extracts JWT tokens from either Authorization header or session cookies
// and adds them to the request context for use by RPC handlers.
//
// FLOW:
// 1. Browser sends request with session cookie (set during CreateSOLSession/CreateVNCSession)
// 2. Interceptor extracts session ID from cookie
// 3. Interceptor looks up web session to get JWT
// 4. Interceptor adds JWT to context with key "token"
// 5. RPC handlers (PowerOn, PowerOff, etc.) extract token from context
//
// This allows web consoles to use the same RPC handlers as the CLI,
// but authenticate via session cookies instead of Authorization headers.

// AuthInterceptor intercepts all RPC requests to extract and validate authentication
type AuthInterceptor struct {
	handler *RegionalGatewayHandler
}

// NewAuthInterceptor creates a new authentication interceptor
func NewAuthInterceptor(handler *RegionalGatewayHandler) *AuthInterceptor {
	return &AuthInterceptor{
		handler: handler,
	}
}

// WrapUnary implements connect.Interceptor for unary RPCs
func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract JWT from either Authorization header or session cookie
		jwt := i.extractJWT(ctx, req)

		// Add JWT to context if found
		if jwt != "" {
			ctx = context.WithValue(ctx, "token", jwt)
		}

		// Call the actual handler with enriched context
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor for client streaming
func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No special handling needed for streaming
}

// WrapStreamingHandler implements connect.Interceptor for server streaming
func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // No special handling needed for streaming
}

// extractJWT extracts JWT from Authorization header or session cookie
func (i *AuthInterceptor) extractJWT(ctx context.Context, req connect.AnyRequest) string {
	// First try Authorization header (for CLI/API calls)
	authHeader := req.Header().Get("Authorization")
	if authHeader != "" {
		jwt, err := coreauth.ExtractJWTFromAuthHeader(authHeader)
		if err == nil && jwt != "" {
			log.Debug().Msg("Extracted JWT from Authorization header")
			return jwt
		}
	}

	// Then try session cookie (for browser calls)
	httpReqPtr, ok := ctx.Value(httpRequestKey{}).(*http.Request)
	if !ok {
		log.Debug().Msg("No HTTP request in context")
		return ""
	}

	// Log all cookies for debugging
	log.Debug().
		Str("url", httpReqPtr.URL.Path).
		Int("cookie_count", len(httpReqPtr.Cookies())).
		Msg("Checking for session cookie")

	for _, c := range httpReqPtr.Cookies() {
		valuePrefix := c.Value
		if len(valuePrefix) > 10 {
			valuePrefix = valuePrefix[:10]
		}
		log.Debug().
			Str("cookie_name", c.Name).
			Str("cookie_value_prefix", valuePrefix).
			Msg("Found cookie")
	}

	// Extract session cookie
	cookie, err := httpReqPtr.Cookie(session.CookieName)
	if err != nil {
		log.Debug().
			Err(err).
			Str("expected_cookie_name", session.CookieName).
			Msg("No session cookie found")
		return ""
	}

	// Look up web session
	sessionStore := i.handler.GetWebSessionStore()
	webSession, err := sessionStore.Get(cookie.Value)
	if err != nil {
		log.Warn().Err(err).Str("cookie_value", cookie.Value).Msg("Failed to get web session")
		return ""
	}

	if webSession == nil {
		log.Warn().Str("cookie_value", cookie.Value).Msg("Web session not found")
		return ""
	}

	log.Debug().
		Str("web_session_id", webSession.ID).
		Str("server_id", webSession.ServerID).
		Msg("Extracted JWT from session cookie")

	return webSession.CustomerJWT
}

// httpRequestKey is a context key for HTTP request
type httpRequestKey struct{}

// WithHTTPRequest adds HTTP request to context
func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, r)
}
