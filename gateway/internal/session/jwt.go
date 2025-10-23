package session

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTClaims represents the JWT token claims we care about
type JWTClaims struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	ExpiresAt  int64  `json:"exp"` // Unix timestamp
	IssuedAt   int64  `json:"iat"` // Unix timestamp
}

// ExtractJWTClaims extracts claims from a JWT token without validating signature
// Note: This is only used for extracting metadata for session management.
// The actual JWT validation happens in the manager service.
func ExtractJWTClaims(token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse claims
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return &claims, nil
}

// CalculateRenewalTime determines when a token should be renewed
// Typically set to 80% of the token's lifetime to provide buffer
func CalculateRenewalTime(expiresAt time.Time) time.Time {
	ttl := time.Until(expiresAt)
	// Renew at 80% of TTL (e.g., if token expires in 60min, renew at 48min)
	renewalBuffer := ttl * 20 / 100
	return expiresAt.Add(-renewalBuffer)
}
