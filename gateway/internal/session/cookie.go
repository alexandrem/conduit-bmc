package session

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	// CookieName is the name of the session cookie
	CookieName = "gateway_session"

	// DefaultSessionDuration is the default session duration (24 hours)
	DefaultSessionDuration = 24 * time.Hour
)

// CreateSessionCookie creates a session cookie with appropriate security settings
// Automatically detects HTTP vs HTTPS based on the request
func CreateSessionCookie(sessionID string, maxAge int) *http.Cookie {
	return createSessionCookieWithSecure(sessionID, maxAge, true)
}

// CreateSessionCookieForRequest creates a session cookie based on the request scheme
// Infers security settings from whether the request was made over HTTPS
func CreateSessionCookieForRequest(sessionID string, maxAge int, r *http.Request) *http.Cookie {
	// Determine if request is over HTTPS
	isHTTPS := r.TLS != nil ||
		r.Header.Get("X-Forwarded-Proto") == "https" ||
		strings.HasPrefix(r.URL.Scheme, "https")

	return createSessionCookieWithSecure(sessionID, maxAge, isHTTPS)
}

// createSessionCookieWithSecure creates a session cookie with specified security level
func createSessionCookieWithSecure(sessionID string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,                 // Always prevent JavaScript access (XSS protection)
		Secure:   secure,               // HTTPS only if secure=true
		SameSite: sameSiteMode(secure), // Strict for HTTPS, Lax for HTTP
	}
}

// sameSiteMode returns appropriate SameSite mode based on security level
func sameSiteMode(secure bool) http.SameSite {
	if !secure {
		return http.SameSiteLaxMode // More permissive for HTTP (local development)
	}
	return http.SameSiteStrictMode // Strict CSRF protection for HTTPS (production)
}

// CreateSessionCookieInsecure creates a session cookie without the Secure flag
// This should ONLY be used for local development over HTTP
func CreateSessionCookieInsecure(sessionID string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,                 // Prevent JavaScript access (XSS protection)
		Secure:   false,                // Allow HTTP for local development
		SameSite: http.SameSiteLaxMode, // Lax instead of Strict for HTTP compatibility
	}
}

// DeleteSessionCookie creates a cookie that deletes the session cookie
func DeleteSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// GetSessionIDFromCookie extracts the session ID from request cookies
func GetSessionIDFromCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", ErrSessionNotFound
		}
		return "", err
	}

	if cookie.Value == "" {
		return "", ErrSessionNotFound
	}

	return cookie.Value, nil
}

// GetSessionIDFromRequest extracts session ID from request cookies or headers
// Supports both cookie-based (browser) and header-based (testing) authentication
func GetSessionIDFromRequest(r *http.Request) (string, error) {
	// Try cookie first (primary method for browsers)
	sessionID, err := GetSessionIDFromCookie(r)
	if err == nil {
		return sessionID, nil
	}

	// Fallback to X-Session-ID header (for testing/debugging)
	headerSessionID := r.Header.Get("X-Session-ID")
	if headerSessionID != "" {
		return headerSessionID, nil
	}

	return "", ErrSessionNotFound
}
