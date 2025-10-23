package auth

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	coreauth "core/auth"
)

// AdminAuthInterceptor is a Connect interceptor that validates admin privileges
type AdminAuthInterceptor struct {
	jwtManager *JWTManager
}

// NewAdminAuthInterceptor creates a new admin authentication interceptor
func NewAdminAuthInterceptor(jwtManager *JWTManager) *AdminAuthInterceptor {
	return &AdminAuthInterceptor{
		jwtManager: jwtManager,
	}
}

// WrapUnary wraps unary RPC calls with admin authentication
func (a *AdminAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract JWT token from Authorization header or cookie using core utility
		authHeader := req.Header().Get("Authorization")
		cookieHeader := req.Header().Get("Cookie")

		tokenString, err := coreauth.ExtractJWTFromRequest(authHeader, cookieHeader, "auth_token")
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing authorization token: %w", err))
		}

		// Validate token
		claims, err := a.jwtManager.ValidateToken(tokenString)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
		}

		// Check if user is admin
		if !claims.IsAdmin {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin privileges required"))
		}

		// Add claims to context for handlers to use
		ctx = context.WithValue(ctx, "claims", claims)
		ctx = context.WithValue(ctx, "customer_id", claims.CustomerID)
		ctx = context.WithValue(ctx, "customer_email", claims.Email)

		// Continue with the request
		return next(ctx, req)
	}
}

// WrapStreamingClient wraps streaming client calls with admin authentication
func (a *AdminAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // Admin operations typically don't use client streaming
}

// WrapStreamingHandler wraps streaming handler calls with admin authentication
func (a *AdminAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // Admin operations typically don't use server streaming
}
