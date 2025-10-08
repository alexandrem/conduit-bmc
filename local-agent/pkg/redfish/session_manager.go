package redfish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// SessionLimitError indicates that the BMC has reached its session limit
type SessionLimitError struct {
	Endpoint string
	Err      error
}

func (e *SessionLimitError) Error() string {
	return fmt.Sprintf("session limit reached for endpoint %s: %v", e.Endpoint, e.Err)
}

// IsSessionLimitError checks if an error is a SessionLimitError
func IsSessionLimitError(err error) bool {
	var e *SessionLimitError
	return errors.As(err, &e)
}

// SessionAuthError indicates an authentication failure during session creation
type SessionAuthError struct {
	Endpoint string
	Err      error
}

func (e *SessionAuthError) Error() string {
	return fmt.Sprintf("session authentication failed for endpoint %s: %v", e.Endpoint, e.Err)
}

// SessionInfo represents an active Redfish session
type SessionInfo struct {
	Token      string
	SessionURI string
	Endpoint   string
}

// SessionManager handles Redfish session lifecycle management
type SessionManager struct {
	httpClient     *http.Client
	activeSessions map[string]*SessionInfo // key: endpoint
	mu             sync.RWMutex
	maxRetries     int
}

// NewSessionManager creates a new SessionManager
func NewSessionManager(httpClient *http.Client) *SessionManager {
	return &SessionManager{
		httpClient:     httpClient,
		activeSessions: make(map[string]*SessionInfo),
		maxRetries:     1, // Retry once after cleanup if session limit reached
	}
}

// CreateSession creates a Redfish session and returns the X-Auth-Token and session URI.
// If the session limit is reached, it will attempt to cleanup old sessions and retry once.
func (sm *SessionManager) CreateSession(ctx context.Context, endpoint, username, password string) (string, string, error) {
	token, sessionURI, err := sm.createSessionInternal(ctx, endpoint, username, password)

	// If session limit error, attempt cleanup and retry once
	if err != nil && sm.isSessionLimitResponse(err) {
		log.Warn().Str("endpoint", endpoint).Msg("Session limit reached, attempting cleanup")

		if cleanupErr := sm.CleanupAllSessions(ctx, endpoint, username, password); cleanupErr != nil {
			log.Warn().Err(cleanupErr).Msg("Failed to cleanup sessions during retry")
			return "", "", &SessionLimitError{Endpoint: endpoint, Err: err}
		}

		// Retry once after cleanup
		token, sessionURI, err = sm.createSessionInternal(ctx, endpoint, username, password)
		if err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	}

	// Track the session
	sm.mu.Lock()
	sm.activeSessions[endpoint] = &SessionInfo{
		Token:      token,
		SessionURI: sessionURI,
		Endpoint:   endpoint,
	}
	sm.mu.Unlock()

	return token, sessionURI, nil
}

// createSessionInternal performs the actual session creation HTTP request
func (sm *SessionManager) createSessionInternal(ctx context.Context, endpoint, username, password string) (string, string, error) {
	sessionURL := BuildSessionsURL(endpoint)

	payload := map[string]string{
		"UserName": username,
		"Password": password,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal session payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sessionURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", "", &SessionAuthError{
			Endpoint: endpoint,
			Err:      fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}
	}

	// Check for session limit or service unavailable
	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("session creation unavailable: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("session creation failed: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	token := resp.Header.Get("X-Auth-Token")
	if token == "" {
		return "", "", fmt.Errorf("no X-Auth-Token in response")
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return token, "", fmt.Errorf("failed to decode session: %w", err)
	}

	return token, session.ODataID, nil
}

// isSessionLimitResponse checks if an error response indicates a session limit
func (sm *SessionManager) isSessionLimitResponse(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "The maximum number of user sessions is reached") ||
		strings.Contains(errStr, "session limit") ||
		strings.Contains(errStr, "StatusServiceUnavailable") ||
		strings.Contains(errStr, "StatusTooManyRequests")
}

// DeleteSession deletes a Redfish session to free up the slot
func (sm *SessionManager) DeleteSession(ctx context.Context, endpoint, sessionURI, token string) error {
	if sessionURI == "" {
		return fmt.Errorf("sessionURI is empty")
	}

	fullURI := BuildRedfishURL(endpoint, sessionURI)
	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURI, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	req.Header.Set("X-Auth-Token", token)

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("session deletion failed: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Remove from tracked sessions
	sm.mu.Lock()
	delete(sm.activeSessions, endpoint)
	sm.mu.Unlock()

	log.Debug().Str("endpoint", endpoint).Msg("Session deleted successfully")
	return nil
}

// CleanupAllSessions cleans up all existing sessions using Basic Auth.
// This is useful when the session limit is reached and we need to free up slots.
func (sm *SessionManager) CleanupAllSessions(ctx context.Context, endpoint, username, password string) error {
	sessions, err := sm.getActiveSessions(ctx, endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to get active sessions: %w", err)
	}

	if len(sessions.Members) == 0 {
		log.Debug().Str("endpoint", endpoint).Msg("No sessions to cleanup")
		return nil
	}

	cleaned := 0
	var lastErr error

	for _, member := range sessions.Members {
		sessionURI := BuildRedfishURL(endpoint, member.ODataID)
		req, err := http.NewRequestWithContext(ctx, "DELETE", sessionURI, nil)
		if err != nil {
			log.Warn().Err(err).Str("session", sessionURI).Msg("Failed to create DELETE request")
			lastErr = err
			continue
		}

		req.Header.Set("Accept", "application/json")
		if username != "" && password != "" {
			req.SetBasicAuth(username, password)
		}

		resp, err := sm.httpClient.Do(req)
		if err != nil {
			log.Warn().Err(err).Str("session", sessionURI).Msg("Failed to delete session")
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
			cleaned++
			log.Debug().Str("session", sessionURI).Msg("Session cleaned up")
		} else {
			log.Warn().Int("status", resp.StatusCode).Str("session", sessionURI).Msg("Failed to delete session")
			lastErr = fmt.Errorf("failed to delete session %s: HTTP %d", sessionURI, resp.StatusCode)
		}
	}

	log.Info().Int("cleaned", cleaned).Int("total", len(sessions.Members)).Str("endpoint", endpoint).Msg("Session cleanup completed")

	if cleaned == 0 && lastErr != nil {
		return fmt.Errorf("no sessions were cleaned up: %w", lastErr)
	}

	return nil
}

// getActiveSessions retrieves all active sessions using Basic Auth
func (sm *SessionManager) getActiveSessions(ctx context.Context, endpoint, username, password string) (*SessionCollection, error) {
	sessionsURL := BuildSessionsURL(endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", sessionsURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get sessions: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var sessions SessionCollection
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	return &sessions, nil
}

// GetActiveSession returns the active session for an endpoint, if any
func (sm *SessionManager) GetActiveSession(endpoint string) (*SessionInfo, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.activeSessions[endpoint]
	return session, exists
}

// ClearTrackedSessions removes all tracked sessions (useful for cleanup)
func (sm *SessionManager) ClearTrackedSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.activeSessions = make(map[string]*SessionInfo)
}
