package redfish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionManagerCreateSession(t *testing.T) {
	sessionCreated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/redfish/v1/SessionService/Sessions" {
			w.Header().Set("X-Auth-Token", "test-token-123")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"@odata.id": "/redfish/v1/SessionService/Sessions/1", "Id": "1"}`))
			sessionCreated = true
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{}
	sm := NewSessionManager(client)

	token, sessionURI, err := sm.CreateSession(context.Background(), server.URL, "admin", "password")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", token)
	}

	if sessionURI != "/redfish/v1/SessionService/Sessions/1" {
		t.Errorf("Expected sessionURI '/redfish/v1/SessionService/Sessions/1', got '%s'", sessionURI)
	}

	if !sessionCreated {
		t.Error("Session was not created")
	}

	// Verify session is tracked
	info, exists := sm.GetActiveSession(server.URL)
	if !exists {
		t.Error("Session should be tracked")
	}
	if info.Token != token {
		t.Errorf("Tracked token mismatch: got %s, want %s", info.Token, token)
	}
}

func TestSessionManagerDeleteSession(t *testing.T) {
	sessionDeleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/redfish/v1/SessionService/Sessions/1" {
			authToken := r.Header.Get("X-Auth-Token")
			if authToken == "test-token" {
				w.WriteHeader(http.StatusNoContent)
				sessionDeleted = true
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{}
	sm := NewSessionManager(client)

	// Manually add a tracked session
	sm.activeSessions[server.URL] = &SessionInfo{
		Token:      "test-token",
		SessionURI: "/redfish/v1/SessionService/Sessions/1",
		Endpoint:   server.URL,
	}

	err := sm.DeleteSession(context.Background(), server.URL, "/redfish/v1/SessionService/Sessions/1", "test-token")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	if !sessionDeleted {
		t.Error("Session was not deleted")
	}

	// Verify session is removed from tracking
	_, exists := sm.GetActiveSession(server.URL)
	if exists {
		t.Error("Session should not be tracked after deletion")
	}
}

func TestSessionManagerCleanupAllSessions(t *testing.T) {
	cleanedSessions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/redfish/v1/SessionService/Sessions":
			// Return list of active sessions
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"Members": [
					{"@odata.id": "/redfish/v1/SessionService/Sessions/1"},
					{"@odata.id": "/redfish/v1/SessionService/Sessions/2"}
				]
			}`))
		case r.Method == "DELETE":
			// Track deletions
			cleanedSessions++
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{}
	sm := NewSessionManager(client)

	err := sm.CleanupAllSessions(context.Background(), server.URL, "admin", "password")
	if err != nil {
		t.Fatalf("CleanupAllSessions failed: %v", err)
	}

	if cleanedSessions != 2 {
		t.Errorf("Expected 2 sessions cleaned, got %d", cleanedSessions)
	}
}

func TestSessionManagerRetryOnLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redfish/v1/SessionService/Sessions":
			if r.Method == "POST" {
				attempts++
				if attempts == 1 {
					// First attempt: return session limit error
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte(`{"error": "The maximum number of user sessions is reached"}`))
				} else {
					// Second attempt after cleanup: succeed
					w.Header().Set("X-Auth-Token", "test-token")
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte(`{"@odata.id": "/redfish/v1/SessionService/Sessions/3", "Id": "3"}`))
				}
			} else if r.Method == "GET" {
				// Return sessions for cleanup
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"Members": [{"@odata.id": "/redfish/v1/SessionService/Sessions/1"}]}`))
			} else if r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	defer server.Close()

	client := &http.Client{}
	sm := NewSessionManager(client)

	token, sessionURI, err := sm.CreateSession(context.Background(), server.URL, "admin", "password")
	if err != nil {
		t.Fatalf("CreateSession with retry failed: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts (1 failure + 1 retry), got %d", attempts)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}

	if sessionURI != "/redfish/v1/SessionService/Sessions/3" {
		t.Errorf("Expected sessionURI '/redfish/v1/SessionService/Sessions/3', got '%s'", sessionURI)
	}
}

func TestIsSessionLimitError(t *testing.T) {
	err := &SessionLimitError{Endpoint: "test", Err: nil}
	if !IsSessionLimitError(err) {
		t.Error("IsSessionLimitError should return true for SessionLimitError")
	}

	otherErr := &SessionAuthError{Endpoint: "test", Err: nil}
	if IsSessionLimitError(otherErr) {
		t.Error("IsSessionLimitError should return false for non-SessionLimitError")
	}
}
