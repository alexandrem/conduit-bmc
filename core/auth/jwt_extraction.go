package auth

import (
	"fmt"
	"strings"
)

// ExtractJWTFromAuthHeader extracts the JWT token from an Authorization header.
// Expected format: "Bearer {token}"
// Returns the token string or an error if the format is invalid.
func ExtractJWTFromAuthHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is empty")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}

// ExtractJWTFromCookie extracts a JWT token from a Cookie header string.
// Parses cookies in the format: "name1=value1; name2=value2; ..."
// Returns the token value for the specified cookie name, or empty string if not found.
func ExtractJWTFromCookie(cookieHeader string, cookieName string) string {
	if cookieHeader == "" || cookieName == "" {
		return ""
	}

	// Parse cookies from header (format: "name1=value1; name2=value2")
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if strings.HasPrefix(cookie, cookieName+"=") {
			return strings.TrimPrefix(cookie, cookieName+"=")
		}
	}

	return ""
}

// ExtractJWTFromRequest attempts to extract a JWT token from either an
// Authorization header or a cookie. It tries the Authorization header first,
// then falls back to the cookie if the header is not present or invalid.
//
// Parameters:
//   - authHeader: The value of the Authorization header (may be empty)
//   - cookieHeader: The value of the Cookie header (may be empty)
//   - cookieName: The name of the cookie containing the JWT
//
// Returns the JWT token string or an error if no valid token is found.
func ExtractJWTFromRequest(authHeader, cookieHeader, cookieName string) (string, error) {
	// Try Authorization header first (for CLI/API calls)
	if authHeader != "" {
		token, err := ExtractJWTFromAuthHeader(authHeader)
		if err == nil && token != "" {
			return token, nil
		}
	}

	// Fallback to cookie (for browser calls)
	if cookieHeader != "" && cookieName != "" {
		token := ExtractJWTFromCookie(cookieHeader, cookieName)
		if token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("no authentication token found in header or cookie")
}
