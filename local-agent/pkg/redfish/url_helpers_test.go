package redfish

import "testing"

func TestBuildRedfishURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		path     string
		expected string
	}{
		{
			name:     "endpoint with trailing slash, path with leading slash",
			endpoint: "https://bmc.example.com/",
			path:     "/redfish/v1/Systems",
			expected: "https://bmc.example.com/redfish/v1/Systems",
		},
		{
			name:     "endpoint without trailing slash, path with leading slash",
			endpoint: "https://bmc.example.com",
			path:     "/redfish/v1/Systems",
			expected: "https://bmc.example.com/redfish/v1/Systems",
		},
		{
			name:     "endpoint with trailing slash, path without leading slash",
			endpoint: "https://bmc.example.com/",
			path:     "redfish/v1/Systems",
			expected: "https://bmc.example.com/redfish/v1/Systems",
		},
		{
			name:     "endpoint without trailing slash, path without leading slash",
			endpoint: "https://bmc.example.com",
			path:     "redfish/v1/Systems",
			expected: "https://bmc.example.com/redfish/v1/Systems",
		},
		{
			name:     "with port number",
			endpoint: "https://192.168.1.100:8443",
			path:     "/redfish/v1/Managers",
			expected: "https://192.168.1.100:8443/redfish/v1/Managers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildRedfishURL(tt.endpoint, tt.path)
			if result != tt.expected {
				t.Errorf("BuildRedfishURL(%q, %q) = %q; want %q", tt.endpoint, tt.path, result, tt.expected)
			}
		})
	}
}

func TestBuildServiceRootURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "basic endpoint",
			endpoint: "https://bmc.example.com",
			expected: "https://bmc.example.com/redfish/v1/",
		},
		{
			name:     "endpoint with trailing slash",
			endpoint: "https://bmc.example.com/",
			expected: "https://bmc.example.com/redfish/v1/",
		},
		{
			name:     "endpoint with port",
			endpoint: "https://192.168.1.100:8443",
			expected: "https://192.168.1.100:8443/redfish/v1/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildServiceRootURL(tt.endpoint)
			if result != tt.expected {
				t.Errorf("BuildServiceRootURL(%q) = %q; want %q", tt.endpoint, result, tt.expected)
			}
		})
	}
}

func TestBuildManagersURL(t *testing.T) {
	endpoint := "https://bmc.example.com"
	expected := "https://bmc.example.com/redfish/v1/Managers"
	result := BuildManagersURL(endpoint)
	if result != expected {
		t.Errorf("BuildManagersURL(%q) = %q; want %q", endpoint, result, expected)
	}
}

func TestBuildSessionsURL(t *testing.T) {
	endpoint := "https://bmc.example.com"
	expected := "https://bmc.example.com/redfish/v1/SessionService/Sessions"
	result := BuildSessionsURL(endpoint)
	if result != expected {
		t.Errorf("BuildSessionsURL(%q) = %q; want %q", endpoint, result, expected)
	}
}

func TestBuildSystemsURL(t *testing.T) {
	endpoint := "https://bmc.example.com"
	expected := "https://bmc.example.com/redfish/v1/Systems"
	result := BuildSystemsURL(endpoint)
	if result != expected {
		t.Errorf("BuildSystemsURL(%q) = %q; want %q", endpoint, result, expected)
	}
}

func TestBuildChassisURL(t *testing.T) {
	endpoint := "https://bmc.example.com"
	expected := "https://bmc.example.com/redfish/v1/Chassis"
	result := BuildChassisURL(endpoint)
	if result != expected {
		t.Errorf("BuildChassisURL(%q) = %q; want %q", endpoint, result, expected)
	}
}
