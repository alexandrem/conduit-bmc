package redfish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/redfish/v1/SessionService/Sessions" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("X-Auth-Token", "test-token")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient()
	token, err := client.CreateSession(context.Background(), server.URL, "user", "pass")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}
}

func TestDiscoverSerialConsole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redfish/v1/SessionService/Sessions":
			w.Header().Set("X-Auth-Token", "test-token")
			w.WriteHeader(http.StatusCreated)
		case "/redfish/v1/Managers":
			w.Write([]byte(`{"Members": [{"@odata.id": "/redfish/v1/Managers/1"}]}`))
		case "/redfish/v1/Managers/1":
			w.Write([]byte(`{"SerialConsole": {"ServiceEnabled": true, "MaxConcurrentSessions": 1}}`))
		default:
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient()
	info, err := client.DiscoverSerialConsole(context.Background(), server.URL, "user", "pass")
	if err != nil {
		t.Fatalf("DiscoverSerialConsole failed: %v", err)
	}
	if !info.Supported || !info.Enabled {
		t.Errorf("Expected supported and enabled, got %+v", info)
	}
}
