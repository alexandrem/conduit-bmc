package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"manager/pkg/models"
)

type JWTManager struct {
	secretKey            string
	serverContextService *ServerContextService
}

func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey:            secretKey,
		serverContextService: NewServerContextService(secretKey),
	}
}

func (j *JWTManager) GenerateToken(customer *models.Customer) (string, error) {
	if j.secretKey == "" {
		return "", fmt.Errorf("JWT secret key is empty")
	}

	claims := &models.AuthClaims{
		CustomerID: customer.ID,
		Email:      customer.Email,
		UUID:       uuid.New(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"customer_id": claims.CustomerID,
		"email":       claims.Email,
		"jti":         claims.UUID.String(),
		"exp":         time.Now().UTC().Add(24 * time.Hour).Unix(),
		"iat":         time.Now().UTC().Unix(),
	})

	return token.SignedString([]byte(j.secretKey))
}

// GenerateServerToken generates a JWT token with encrypted server context
func (j *JWTManager) GenerateServerToken(customer *models.Customer, server *models.Server, permissions []string) (string, error) {
	if j.secretKey == "" {
		return "", fmt.Errorf("JWT secret key is empty")
	}

	// Create server context
	serverContext := j.serverContextService.CreateServerContext(server, permissions)

	// Encrypt server context
	encryptedContext, err := j.serverContextService.EncryptServerContext(serverContext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt server context: %w", err)
	}

	// Create JWT claims with encrypted server context
	claims := &EncryptedJWT{
		CustomerID:    customer.ID,
		Email:         customer.Email,
		JTI:           uuid.New().String(),
		IssuedAt:      time.Now().UTC().Unix(),
		ExpiresAt:     time.Now().UTC().Add(1 * time.Hour).Unix(), // Match server context expiration
		ServerContext: encryptedContext,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"customer_id":    claims.CustomerID,
		"email":          claims.Email,
		"jti":            claims.JTI,
		"iat":            claims.IssuedAt,
		"exp":            claims.ExpiresAt,
		"server_context": claims.ServerContext,
	})

	return token.SignedString([]byte(j.secretKey))
}

func (j *JWTManager) ValidateToken(tokenString string) (*models.AuthClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	customerID, ok := claims["customer_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid customer_id claim")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid email claim")
	}

	jtiStr, ok := claims["jti"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid jti claim")
	}

	jti, err := uuid.Parse(jtiStr)
	if err != nil {
		return nil, fmt.Errorf("invalid jti format")
	}

	return &models.AuthClaims{
		CustomerID: customerID,
		Email:      email,
		UUID:       jti,
	}, nil
}

// ValidateServerToken validates a server token and returns both auth claims and server context
func (j *JWTManager) ValidateServerToken(tokenString string) (*models.AuthClaims, *ServerContext, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, nil, err
	}

	if !token.Valid {
		return nil, nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("invalid token claims")
	}

	// Extract standard claims
	customerID, ok := claims["customer_id"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid customer_id claim")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid email claim")
	}

	jtiStr, ok := claims["jti"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid jti claim")
	}

	jti, err := uuid.Parse(jtiStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid jti format")
	}

	authClaims := &models.AuthClaims{
		CustomerID: customerID,
		Email:      email,
		UUID:       jti,
	}

	// Extract and decrypt server context if present
	var serverContext *ServerContext
	if encryptedContextStr, ok := claims["server_context"].(string); ok && encryptedContextStr != "" {
		serverContext, err = j.serverContextService.DecryptServerContext(encryptedContextStr)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decrypt server context: %w", err)
		}

		// TEMPORARY: Customer ID validation disabled for new server-customer mapping architecture
		// TODO: Replace with proper server-customer mapping validation using ServerCustomerMapping table
		// TODO: Implement: Query database to verify customer has access to the server in server context
		// TODO: Servers now belong to "system" and customer access is managed via separate mapping table
		//
		// if serverContext.CustomerID != customerID {
		//     return nil, nil, fmt.Errorf("server context customer ID mismatch")
		// }
	}

	return authClaims, serverContext, nil
}

// GetServerContextService returns the server context service for direct access
func (j *JWTManager) GetServerContextService() *ServerContextService {
	return j.serverContextService
}
