package gateway

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"

	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/internal/session"
)

// Session Cookie Interceptor - Connect RPC Integration
//
// This interceptor automatically sets session cookies when CreateSOLSession or
// CreateVNCSession RPCs are called. It extracts the JWT from the RPC request,
// creates a web session, and sets an HttpOnly cookie in the HTTP response.
//
// FLOW:
// 1. CLI calls CreateSOLSession with JWT in Authorization header
// 2. Gateway creates console session (existing logic)
// 3. Interceptor extracts JWT from RPC request
// 4. Interceptor creates web session (cookie_id → JWT mapping)
// 5. Interceptor sets HttpOnly session cookie in HTTP response
// 6. CLI opens browser with URL - browser automatically sends cookie
//
// This approach is transparent to the existing RPC handlers - they don't need
// to know about web sessions or cookies. The interceptor handles all the
// session management automatically.

// SessionCookieInterceptor intercepts CreateSOLSession and CreateVNCSession responses
// to set session cookies for web console authentication
type SessionCookieInterceptor struct {
	handler *RegionalGatewayHandler
}

// NewSessionCookieInterceptor creates a new session cookie interceptor
func NewSessionCookieInterceptor(handler *RegionalGatewayHandler) *SessionCookieInterceptor {
	return &SessionCookieInterceptor{
		handler: handler,
	}
}

// WrapUnary implements connect.Interceptor for unary RPCs
func (i *SessionCookieInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Call the actual handler
		resp, err := next(ctx, req)
		if err != nil {
			return resp, err
		}

		// Extract HTTP response writer from context (if available)
		// This will be set by the HTTP handler layer
		writer, ok := ctx.Value(httpResponseWriterKey{}).(http.ResponseWriter)
		if !ok {
			// No HTTP writer in context - likely a gRPC call or test
			return resp, nil
		}

		// Check if this is CreateSOLSession or CreateVNCSession
		switch req.Spec().Procedure {
		case "/gateway.v1.GatewayService/CreateSOLSession":
			i.handleSOLSessionResponse(ctx, req, resp, writer)
		case "/gateway.v1.GatewayService/CreateVNCSession":
			i.handleVNCSessionResponse(ctx, req, resp, writer)
		}

		return resp, nil
	}
}

// WrapStreamingClient implements connect.Interceptor for client streaming
func (i *SessionCookieInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No special handling needed for streaming
}

// WrapStreamingHandler implements connect.Interceptor for server streaming
func (i *SessionCookieInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // No special handling needed for streaming
}

// handleSOLSessionResponse creates a web session and sets cookie for SOL console
func (i *SessionCookieInterceptor) handleSOLSessionResponse(
	ctx context.Context,
	req connect.AnyRequest,
	resp connect.AnyResponse,
	w http.ResponseWriter,
) {
	// Extract JWT from request header
	authHeader := req.Header().Get("Authorization")
	jwt, err := session.ExtractJWTFromAuthHeader(authHeader)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract JWT from CreateSOLSession request")
		return
	}

	// Get response message
	solResp, ok := resp.Any().(*gatewayv1.CreateSOLSessionResponse)
	if !ok {
		log.Warn().Msg("Unexpected response type for CreateSOLSession")
		return
	}

	// Extract server ID from request
	solReq, ok := req.Any().(*gatewayv1.CreateSOLSessionRequest)
	if !ok {
		log.Warn().Msg("Unexpected request type for CreateSOLSession")
		return
	}

	// Create web session
	webSession, err := i.handler.CreateWebSessionForConsole(
		solResp.SessionId,
		jwt,
		solReq.ServerId,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create web session for SOL console")
		return
	}

	// Get HTTP request from context to detect scheme
	httpReq, ok := ctx.Value(httpRequestKey{}).(*http.Request)
	var cookie *http.Cookie
	if ok {
		// Use request-aware cookie creation for proper HTTP/HTTPS detection
		cookie = session.CreateSessionCookieForRequest(webSession.ID, int(session.DefaultSessionDuration.Seconds()), httpReq)
	} else {
		// Fallback to default (assumes HTTPS)
		cookie = session.CreateSessionCookie(webSession.ID, int(session.DefaultSessionDuration.Seconds()))
	}

	http.SetCookie(w, cookie)

	log.Debug().
		Str("session_id", solResp.SessionId).
		Str("web_session_id", webSession.ID).
		Bool("cookie_secure", cookie.Secure).
		Int("cookie_samesite", int(cookie.SameSite)).
		Msg("Set session cookie for SOL console")
}

// handleVNCSessionResponse creates a web session and sets cookie for VNC console
func (i *SessionCookieInterceptor) handleVNCSessionResponse(
	ctx context.Context,
	req connect.AnyRequest,
	resp connect.AnyResponse,
	w http.ResponseWriter,
) {
	// Extract JWT from request header
	authHeader := req.Header().Get("Authorization")
	jwt, err := session.ExtractJWTFromAuthHeader(authHeader)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract JWT from CreateVNCSession request")
		return
	}

	// Get response message
	vncResp, ok := resp.Any().(*gatewayv1.CreateVNCSessionResponse)
	if !ok {
		log.Warn().Msg("Unexpected response type for CreateVNCSession")
		return
	}

	// Extract server ID from request
	vncReq, ok := req.Any().(*gatewayv1.CreateVNCSessionRequest)
	if !ok {
		log.Warn().Msg("Unexpected request type for CreateVNCSession")
		return
	}

	// Create web session
	webSession, err := i.handler.CreateWebSessionForConsole(
		vncResp.SessionId,
		jwt,
		vncReq.ServerId,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create web session for VNC console")
		return
	}

	// Get HTTP request from context to detect scheme
	httpReq, ok := ctx.Value(httpRequestKey{}).(*http.Request)
	var cookie *http.Cookie
	if ok {
		// Use request-aware cookie creation for proper HTTP/HTTPS detection
		cookie = session.CreateSessionCookieForRequest(webSession.ID, int(session.DefaultSessionDuration.Seconds()), httpReq)
	} else {
		// Fallback to default (assumes HTTPS)
		cookie = session.CreateSessionCookie(webSession.ID, int(session.DefaultSessionDuration.Seconds()))
	}

	http.SetCookie(w, cookie)

	log.Debug().
		Str("session_id", vncResp.SessionId).
		Str("web_session_id", webSession.ID).
		Bool("cookie_secure", cookie.Secure).
		Int("cookie_samesite", int(cookie.SameSite)).
		Msg("Set session cookie for VNC console")
}

// httpResponseWriterKey is a context key for HTTP response writer
type httpResponseWriterKey struct{}

// WithHTTPResponseWriter adds HTTP response writer to context
func WithHTTPResponseWriter(ctx context.Context, w http.ResponseWriter) context.Context {
	return context.WithValue(ctx, httpResponseWriterKey{}, w)
}
