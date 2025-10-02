package vnc

import (
	"testing"
)

func TestDetectTransportType(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     EndpointType
	}{
		// WebSocket endpoints
		{
			name:     "WebSocket with ws scheme",
			endpoint: "ws://localhost:8080/kvm/0",
			want:     TypeWebSocket,
		},
		{
			name:     "WebSocket with wss scheme",
			endpoint: "wss://bmc.example.com/kvm/0",
			want:     TypeWebSocket,
		},
		{
			name:     "OpenBMC WebSocket endpoint",
			endpoint: "wss://192.168.1.100/kvm/0",
			want:     TypeWebSocket,
		},
		{
			name:     "Redfish GraphicalConsole WebSocket",
			endpoint: "wss://bmc.example.com/redfish/v1/Systems/1/GraphicalConsole",
			want:     TypeWebSocket,
		},

		// Native TCP endpoints
		{
			name:     "Native VNC with vnc scheme",
			endpoint: "vnc://localhost:5900",
			want:     TypeNative,
		},
		{
			name:     "Host and port without scheme",
			endpoint: "192.168.1.100:5900",
			want:     TypeNative,
		},
		{
			name:     "Hostname and port without scheme",
			endpoint: "bmc-server:5900",
			want:     TypeNative,
		},
		{
			name:     "IP address without port",
			endpoint: "192.168.1.100",
			want:     TypeNative,
		},
		{
			name:     "Hostname without port",
			endpoint: "bmc-server",
			want:     TypeNative,
		},

		// Unknown/invalid endpoints
		{
			name:     "HTTP scheme (unknown)",
			endpoint: "http://localhost:8080",
			want:     TypeUnknown,
		},
		{
			name:     "HTTPS scheme (unknown)",
			endpoint: "https://bmc.example.com",
			want:     TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTransportType(tt.endpoint)
			if got != tt.want {
				t.Errorf("detectTransportType(%q) = %v, want %v", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		// Valid native VNC endpoints
		{
			name:     "Host and port",
			endpoint: "192.168.1.100:5900",
			wantHost: "192.168.1.100",
			wantPort: 5900,
			wantErr:  false,
		},
		{
			name:     "VNC scheme with port",
			endpoint: "vnc://bmc-server:5901",
			wantHost: "bmc-server",
			wantPort: 5901,
			wantErr:  false,
		},
		{
			name:     "VNC scheme without port (defaults to 5900)",
			endpoint: "vnc://192.168.1.100",
			wantHost: "192.168.1.100",
			wantPort: 5900,
			wantErr:  false,
		},
		{
			name:     "Hostname only (defaults to 5900)",
			endpoint: "bmc-server",
			wantHost: "bmc-server",
			wantPort: 5900,
			wantErr:  false,
		},
		{
			name:     "IP address only (defaults to 5900)",
			endpoint: "10.0.0.50",
			wantHost: "10.0.0.50",
			wantPort: 5900,
			wantErr:  false,
		},

		// Invalid endpoints
		{
			name:     "WebSocket URL (should error)",
			endpoint: "ws://localhost:8080/kvm",
			wantErr:  true,
		},
		{
			name:     "WebSocket secure URL (should error)",
			endpoint: "wss://bmc.example.com/kvm/0",
			wantErr:  true,
		},
		{
			name:     "Invalid port",
			endpoint: "bmc-server:invalid",
			wantErr:  true,
		},
		{
			name:     "Invalid URL format",
			endpoint: "http://[::1:5900",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPort, err := parseEndpoint(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEndpoint(%q) error = %v, wantErr %v", tt.endpoint, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotHost != tt.wantHost {
					t.Errorf("parseEndpoint(%q) host = %v, want %v", tt.endpoint, gotHost, tt.wantHost)
				}
				if gotPort != tt.wantPort {
					t.Errorf("parseEndpoint(%q) port = %v, want %v", tt.endpoint, gotPort, tt.wantPort)
				}
			}
		})
	}
}

func TestEndpointTypeString(t *testing.T) {
	tests := []struct {
		name string
		t    EndpointType
		want string
	}{
		{"TypeNative", TypeNative, "native"},
		{"TypeWebSocket", TypeWebSocket, "websocket"},
		{"TypeUnknown", TypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.t.String()
			if got != tt.want {
				t.Errorf("EndpointType(%d).String() = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestParseEndpointType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  EndpointType
	}{
		{"native", "native", TypeNative},
		{"websocket", "websocket", TypeWebSocket},
		{"unknown string", "foobar", TypeUnknown},
		{"empty string", "", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEndpointType(tt.input)
			if got != tt.want {
				t.Errorf("ParseEndpointType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewTransport(t *testing.T) {
	tests := []struct {
		name     string
		config   *Endpoint
		wantErr  bool
		wantType string // Type name for verification
	}{
		{
			name: "WebSocket endpoint creates WebSocketTransport",
			config: &Endpoint{
				Endpoint: "wss://bmc.example.com/kvm/0",
				Username: "admin",
				Password: "password",
			},
			wantErr:  false,
			wantType: "*vnc.WebSocketTransport",
		},
		{
			name: "Native VNC endpoint creates NativeTransport",
			config: &Endpoint{
				Endpoint: "192.168.1.100:5900",
			},
			wantErr:  false,
			wantType: "*vnc.NativeTransport",
		},
		{
			name: "VNC scheme creates NativeTransport",
			config: &Endpoint{
				Endpoint: "vnc://localhost:5900",
			},
			wantErr:  false,
			wantType: "*vnc.NativeTransport",
		},
		{
			name:    "Nil config returns error",
			config:  nil,
			wantErr: true,
		},
		{
			name: "Empty endpoint returns error",
			config: &Endpoint{
				Endpoint: "",
			},
			wantErr: true,
		},
		{
			name: "Unknown scheme returns error",
			config: &Endpoint{
				Endpoint: "http://bmc.example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTransport(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != nil {
				gotType := getTypeName(got)
				if gotType != tt.wantType {
					t.Errorf("NewTransport() type = %v, want %v", gotType, tt.wantType)
				}
			}
		})
	}
}

// Helper to get type name for testing
func getTypeName(v interface{}) string {
	switch v.(type) {
	case *NativeTransport:
		return "*vnc.NativeTransport"
	case *WebSocketTransport:
		return "*vnc.WebSocketTransport"
	default:
		return "unknown"
	}
}
