package types

import "time"

// DiscoveryMethod represents how a BMC was discovered
type DiscoveryMethod string

const (
	DiscoveryMethodUnspecified     DiscoveryMethod = "unspecified"
	DiscoveryMethodStaticConfig    DiscoveryMethod = "static_config"    // From agent config file
	DiscoveryMethodNetworkScan     DiscoveryMethod = "network_scan"     // Auto-discovered via network scan
	DiscoveryMethodAPIRegistration DiscoveryMethod = "api_registration" // Registered via API
	DiscoveryMethodManual          DiscoveryMethod = "manual"           // Manually added by admin
)

// String returns the string representation of DiscoveryMethod
func (d DiscoveryMethod) String() string {
	return string(d)
}

// DiscoveryMetadata contains detailed information about how a BMC was discovered and configured
type DiscoveryMetadata struct {
	// Discovery information
	DiscoveryMethod DiscoveryMethod `json:"discovery_method"`
	DiscoveredAt    time.Time       `json:"discovered_at"`
	DiscoverySource string          `json:"discovery_source"` // Agent ID that discovered this BMC
	ConfigSource    string          `json:"config_source"`    // Config file path or API endpoint

	// Vendor information
	Vendor *VendorInfo `json:"vendor,omitempty"`

	// Protocol configuration
	Protocol *ProtocolConfig `json:"protocol,omitempty"`

	// Endpoint details
	Endpoints *EndpointDetails `json:"endpoints,omitempty"`

	// Security configuration
	Security *SecurityConfig `json:"security,omitempty"`

	// Network information
	Network *NetworkInfo `json:"network,omitempty"`

	// Capability discovery
	Capabilities *CapabilityInfo `json:"capabilities,omitempty"`

	// Additional metadata
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}

// VendorInfo contains BMC vendor/hardware information
type VendorInfo struct {
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	BMCVersion      string `json:"bmc_version"`
}

// ProtocolConfig contains protocol-specific configuration
type ProtocolConfig struct {
	PrimaryProtocol  string `json:"primary_protocol"`  // "ipmi" or "redfish"
	PrimaryVersion   string `json:"primary_version"`   // "2.0", "1.6.0"
	FallbackProtocol string `json:"fallback_protocol"` // "ipmi" or empty
	FallbackReason   string `json:"fallback_reason"`   // Why fallback is needed
	ConsoleType      string `json:"console_type"`      // "ipmi", "redfish_serial"
	ConsolePath      string `json:"console_path"`      // Redfish path to SerialConsole
	VNCTransport     string `json:"vnc_transport"`     // "native", "websocket"
}

// EndpointDetails contains endpoint configuration information
type EndpointDetails struct {
	ControlEndpoint string `json:"control_endpoint"`
	ControlScheme   string `json:"control_scheme"` // "https", "http", "ipmi"
	ControlPort     int32  `json:"control_port"`
	ConsoleEndpoint string `json:"console_endpoint"`
	VNCEndpoint     string `json:"vnc_endpoint"`
	VNCDisplay      int32  `json:"vnc_display"` // VNC display number
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	TLSEnabled        bool   `json:"tls_enabled"`
	TLSVerify         bool   `json:"tls_verify"`
	AuthMethod        string `json:"auth_method"`   // "basic", "session", "digest"
	VNCAuthType       string `json:"vnc_auth_type"` // "password", "vencrypt", "none"
	VNCPasswordLength int32  `json:"vnc_password_length"`
	IPMICipherSuite   string `json:"ipmi_cipher_suite"`
}

// NetworkInfo contains network-related information
type NetworkInfo struct {
	IPAddress      string `json:"ip_address"`
	MACAddress     string `json:"mac_address"`
	NetworkSegment string `json:"network_segment"`
	VLANId         string `json:"vlan_id"`
	Reachable      bool   `json:"reachable"`
	LatencyMs      int32  `json:"latency_ms"` // Ping latency from agent
}

// CapabilityInfo contains discovered capabilities
type CapabilityInfo struct {
	SupportedFeatures   []string `json:"supported_features"`
	UnsupportedFeatures []string `json:"unsupported_features"`
	DiscoveryErrors     []string `json:"discovery_errors"`
	DiscoveryWarnings   []string `json:"discovery_warnings"`
}
