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
		w.Write([]byte(`{"@odata.id": "/redfish/v1/SessionService/Sessions/1", "Id": "1"}`))
	}))
	defer server.Close()

	client := NewClient()
	token, sessionURI, err := client.SessionManager.CreateSession(context.Background(), server.URL, "user", "pass")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", token)
	}
	if sessionURI == "" {
		t.Error("Expected non-empty sessionURI")
	}
}

func TestDiscoverSerialConsole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redfish/v1/SessionService/Sessions":
			if r.Method == "POST" {
				w.Header().Set("X-Auth-Token", "test-token")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"@odata.id": "/redfish/v1/SessionService/Sessions/1", "Id": "1"}`))
			} else if r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
			}
		case "/redfish/v1/SessionService/Sessions/1":
			w.WriteHeader(http.StatusNoContent)
		case "/redfish/v1/Managers":
			w.Write([]byte(`{"Members": [{"@odata.id": "/redfish/v1/Managers/1"}]}`))
		case "/redfish/v1/Managers/1":
			w.Write([]byte(`{"Id": "1", "Manufacturer": "Generic", "SerialConsole": {"ServiceEnabled": true, "MaxConcurrentSessions": 1}}`))
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
