package auth

import "github.com/google/uuid"

// AuthClaims represents the authentication claims in a JWT token
type AuthClaims struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	uuid.UUID  `json:"jti"`
}
