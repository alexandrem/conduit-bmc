package synthetic

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

// HTTPRedfishServer is a simple HTTP-based synthetic Redfish server for testing
type HTTPRedfishServer struct {
	server     *http.Server
	powerState string
	mu         sync.RWMutex
	Endpoint   string
	Username   string
	Password   string
}

// NewHTTPRedfishServer creates a new HTTP-based synthetic Redfish server
func NewHTTPRedfishServer() (*HTTPRedfishServer, error) {
	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := &HTTPRedfishServer{
		powerState: "On",
		Username:   "admin",
		Password:   "admin123",
		Endpoint:   fmt.Sprintf("http://localhost:%d", port),
	}

	// Create router
	r := mux.NewRouter()

	// Add basic auth middleware
	r.Use(server.basicAuthMiddleware)

	// Service root
	r.HandleFunc("/redfish/v1/", server.serviceRoot).Methods("GET")
	r.HandleFunc("/redfish/v1", server.serviceRoot).Methods("GET")

	// Systems
	r.HandleFunc("/redfish/v1/Systems", server.systemsCollection).Methods("GET")
	r.HandleFunc("/redfish/v1/Systems/", server.systemsCollection).Methods("GET")
	r.HandleFunc("/redfish/v1/Systems/1", server.computerSystem).Methods("GET")
	r.HandleFunc("/redfish/v1/Systems/1/Actions/ComputerSystem.Reset", server.systemReset).Methods("POST")

	// Create HTTP server
	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	return server, nil
}

// Start starts the synthetic Redfish server
func (s *HTTPRedfishServer) Start() error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Redfish server error: %v\n", err)
		}
	}()
	return nil
}

// Stop stops the synthetic Redfish server
func (s *HTTPRedfishServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// SetPowerState sets the power state for testing
func (s *HTTPRedfishServer) SetPowerState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.powerState = state
}

// GetPowerState gets the current power state
func (s *HTTPRedfishServer) GetPowerState() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.powerState
}

// IsAccessible checks if the server is responding
func (s *HTTPRedfishServer) IsAccessible() bool {
	resp, err := http.Get(s.Endpoint + "/redfish/v1/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}

// basicAuthMiddleware provides basic authentication
func (s *HTTPRedfishServer) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != s.Username || password != s.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Redfish"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// serviceRoot handles the service root endpoint
func (s *HTTPRedfishServer) serviceRoot(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"Id":             "RootService",
		"Name":           "Root Service",
		"RedfishVersion": "1.15.1",
		"UUID":           "12345678-1234-1234-1234-123456789012",
		"Systems":        map[string]string{"@odata.id": "/redfish/v1/Systems"},
		"Chassis":        map[string]string{"@odata.id": "/redfish/v1/Chassis"},
		"Managers":       map[string]string{"@odata.id": "/redfish/v1/Managers"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// systemsCollection handles the systems collection endpoint
func (s *HTTPRedfishServer) systemsCollection(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"Name": "Computer System Collection",
		"Members": []map[string]string{
			{"@odata.id": "/redfish/v1/Systems/1"},
		},
		"Members@odata.count": 1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// computerSystem handles the computer system endpoint
func (s *HTTPRedfishServer) computerSystem(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	powerState := s.powerState
	s.mu.RUnlock()

	response := map[string]interface{}{
		"Id":           "1",
		"Name":         "Test Server",
		"Manufacturer": "Synthetic Corp",
		"Model":        "TestServer 3000",
		"SerialNumber": "TEST123456",
		"PowerState":   powerState,
		"Actions": map[string]interface{}{
			"#ComputerSystem.Reset": map[string]interface{}{
				"target":                            "/redfish/v1/Systems/1/Actions/ComputerSystem.Reset",
				"ResetType@Redfish.AllowableValues": []string{"On", "ForceOff", "PowerCycle", "ForceRestart"},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// systemReset handles power operations
func (s *HTTPRedfishServer) systemReset(w http.ResponseWriter, r *http.Request) {
	var resetReq struct {
		ResetType string `json:"ResetType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&resetReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	switch resetReq.ResetType {
	case "On":
		s.powerState = "On"
	case "ForceOff":
		s.powerState = "Off"
	case "PowerCycle":
		s.powerState = "On" // Simulate power cycle ending in On state
	case "ForceRestart":
		s.powerState = "On" // Simulate restart ending in On state
	default:
		s.mu.Unlock()
		http.Error(w, "Invalid ResetType", http.StatusBadRequest)
		return
	}
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}