package types

// BMCControlEndpoint represents BMC control API configuration.
// This type is used across manager, CLI, and local-agent packages.
type BMCControlEndpoint struct {
	Endpoint     string     `json:"endpoint"`
	Type         BMCType    `json:"type"`
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	TLS          *TLSConfig `json:"tls"`
	Capabilities []string   `json:"capabilities"`
}

// SOLEndpoint represents Serial-over-LAN configuration.
type SOLEndpoint struct {
	Type     SOLType    `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *SOLConfig `json:"config"`
}

// VNCEndpoint represents VNC/KVM access configuration.
type VNCEndpoint struct {
	Type     VNCType    `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *VNCConfig `json:"config"`
	TLS      *TLSConfig `json:"tls"` // Optional TLS configuration for VeNCrypt/RFB-over-TLS
}

// TLSConfig holds TLS-specific configuration for BMC connections.
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACert             string `json:"ca_cert"`
}

// SOLConfig holds SOL-specific configuration.
type SOLConfig struct {
	BaudRate       int    `json:"baud_rate"`
	FlowControl    string `json:"flow_control"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// VNCConfig holds VNC-specific configuration.
type VNCConfig struct {
	Protocol string `json:"protocol"`
	Path     string `json:"path"`
	Display  int    `json:"display"`
	ReadOnly bool   `json:"read_only"`
}
