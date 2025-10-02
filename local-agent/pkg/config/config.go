package config

import (
	"fmt"
	"net"
	"strings"
	"time"

	"core/config"
	"github.com/rs/zerolog"
)

// Config contains all configuration for the local-agent service
type Config struct {
	// Logging configuration
	Log LogConfig `yaml:"log"`

	// Service-specific configuration
	Agent AgentConfig `yaml:"agent"`

	// TLS configuration
	TLS config.TLSConfig `yaml:"tls"`

	// Legacy static hosts configuration (for backward compatibility)
	Static StaticConfig `yaml:"static"`
}

// LogConfig contains agent-specific logging configuration
type LogConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL" default:"info"`
	Format string `yaml:"format" env:"LOG_FORMAT" default:"json"`
	Debug  bool   `yaml:"debug" env:"DEBUG" default:"false"`
}

// ConfigureZerolog configures zerolog based on the log configuration
func (c *LogConfig) ConfigureZerolog() {
	// Set log level
	level := zerolog.InfoLevel
	if c.Debug {
		level = zerolog.DebugLevel
	} else {
		switch strings.ToLower(c.Level) {
		case "trace":
			level = zerolog.TraceLevel
		case "debug":
			level = zerolog.DebugLevel
		case "info":
			level = zerolog.InfoLevel
		case "warn", "warning":
			level = zerolog.WarnLevel
		case "error":
			level = zerolog.ErrorLevel
		case "fatal":
			level = zerolog.FatalLevel
		case "panic":
			level = zerolog.PanicLevel
		}
	}
	zerolog.SetGlobalLevel(level)
}

// AgentConfig contains agent-specific configuration
type AgentConfig struct {
	// Agent identification
	ID           string `yaml:"id" env:"AGENT_ID" default:"local-agent-01"`
	Name         string `yaml:"name" default:"Local Development Agent"`
	DatacenterID string `yaml:"datacenter_id" env:"AGENT_DATACENTER_ID"`
	Region       string `yaml:"region" default:"default"`

	// Gateway connection
	GatewayEndpoint string `yaml:"gateway_endpoint" env:"AGENT_GATEWAY_ENDPOINT" default:"http://localhost:8081"`

	// Local HTTP server configuration
	HTTPPort int    `yaml:"http_port" default:"8090"`
	Endpoint string `yaml:"endpoint"`

	// BMC discovery and management
	BMCDiscovery  BMCDiscoveryConfig  `yaml:"bmc_discovery"`
	BMCOperations BMCOperationsConfig `yaml:"bmc_operations"` // TODO: Most fields not currently used

	// VNC/KVM configuration (TODO: Not currently used in code)
	VNCConfig VNCConfig `yaml:"vnc"`

	// Serial console configuration (TODO: Not currently used in code)
	SerialConsole SerialConsoleConfig `yaml:"serial_console"`

	// Connection management (TODO: Not currently used in code)
	ConnectionManagement ConnectionManagementConfig `yaml:"connection_management"`

	// Health monitoring (TODO: Not currently used in code)
	HealthMonitoring HealthMonitoringConfig `yaml:"health_monitoring"`

	// Security configuration (only .EncryptionKey is currently used)
	Security SecurityConfig `yaml:"security"`
}

// BMCDiscoveryConfig configures BMC discovery behavior
type BMCDiscoveryConfig struct {
	Enabled       bool          `yaml:"enabled" default:"true"`
	ScanInterval  time.Duration `yaml:"scan_interval" default:"5m"`
	NetworkRanges []string      `yaml:"network_ranges"`
	IPMIPorts     []int         `yaml:"ipmi_ports"`
	RedfishPorts  []int         `yaml:"redfish_ports"`
	ScanTimeout   time.Duration `yaml:"scan_timeout" default:"10s"`
	MaxConcurrent int           `yaml:"max_concurrent" default:"50"`

	// Discovery methods
	EnablePortScan         bool `yaml:"enable_port_scan" default:"true"`
	EnableIPMIDetection    bool `yaml:"enable_ipmi_detection" default:"true"`
	EnableRedfishDetection bool `yaml:"enable_redfish_detection" default:"true"`

	// Credential testing
	DefaultCredentials []CredentialConfig `yaml:"default_credentials"`
}

// CredentialConfig contains BMC credentials for discovery
type CredentialConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// BMCOperationsConfig configures BMC operation behavior
// TODO: Most fields not currently used in code - reserved for future implementation
type BMCOperationsConfig struct {
	// Timeouts
	OperationTimeout      time.Duration `yaml:"operation_timeout" default:"30s"`
	PowerOperationTimeout time.Duration `yaml:"power_operation_timeout" default:"60s"`
	ConsoleTimeout        time.Duration `yaml:"console_timeout" default:"300s"`

	// Retries
	MaxRetries      int           `yaml:"max_retries" default:"3"`
	RetryBackoff    time.Duration `yaml:"retry_backoff" default:"2s"`
	RetryMaxBackoff time.Duration `yaml:"retry_max_backoff" default:"30s"`

	// Concurrency
	MaxConcurrentOperations int `yaml:"max_concurrent_operations" default:"10"`

	// Protocol-specific settings
	IPMIConfig    IPMIConfig    `yaml:"ipmi"`
	RedfishConfig RedfishConfig `yaml:"redfish"`
}

// IPMIConfig configures IPMI operations
// TODO: Not currently used in code - reserved for future implementation
type IPMIConfig struct {
	Interface         string        `yaml:"interface" default:"lanplus"`
	CipherSuite       string        `yaml:"cipher_suite" default:"3"`
	PrivilegeLevel    string        `yaml:"privilege_level" default:"ADMINISTRATOR"`
	AuthType          string        `yaml:"auth_type" default:"PASSWORD"`
	SessionTimeout    time.Duration `yaml:"session_timeout" default:"20s"`
	RetransmitTimeout time.Duration `yaml:"retransmit_timeout" default:"1s"`
	MaxRetransmits    int           `yaml:"max_retransmits" default:"3"`

	// SOL-specific settings
	SOLBaudRate       int    `yaml:"sol_baud_rate" default:"115200"`
	SOLFlowControl    string `yaml:"sol_flow_control" default:"none"`
	SOLAuthentication bool   `yaml:"sol_authentication" default:"true"`
	SOLEncryption     bool   `yaml:"sol_encryption" default:"true"`
}

// RedfishConfig configures Redfish operations
// TODO: Not currently used in code - reserved for future implementation
type RedfishConfig struct {
	HTTPTimeout        time.Duration `yaml:"http_timeout" default:"30s"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify" default:"false"`
	MaxIdleConns       int           `yaml:"max_idle_conns" default:"10"`
	MaxConnsPerHost    int           `yaml:"max_conns_per_host" default:"5"`
	IdleConnTimeout    time.Duration `yaml:"idle_conn_timeout" default:"90s"`

	// Authentication
	AuthMethod     string        `yaml:"auth_method" default:"basic"`
	SessionCookie  bool          `yaml:"session_cookie" default:"true"`
	SessionTimeout time.Duration `yaml:"session_timeout" default:"30m"`
}

// VNCConfig configures VNC/KVM operations
// TODO: Not currently used in code - reserved for future implementation
type VNCConfig struct {
	Enabled        bool          `yaml:"enabled" default:"true"`
	Port           int           `yaml:"port" default:"5900"`
	BindAddress    string        `yaml:"bind_address" default:"127.0.0.1"`
	MaxConnections int           `yaml:"max_connections" default:"5"`
	SessionTimeout time.Duration `yaml:"session_timeout" default:"4h"`

	// VNC server configuration
	FrameRate      int  `yaml:"frame_rate" default:"15"`
	Quality        int  `yaml:"quality" default:"6"`
	Compression    int  `yaml:"compression" default:"2"`
	EnableKeyboard bool `yaml:"enable_keyboard" default:"true"`
	EnableMouse    bool `yaml:"enable_mouse" default:"true"`

	// Security
	EnableAuthentication bool     `yaml:"enable_authentication" default:"true"`
	PasswordLength       int      `yaml:"password_length" default:"8"`
	AllowedOrigins       []string `yaml:"allowed_origins"`
}

// SerialConsoleConfig configures serial console operations
// TODO: Not currently used in code - reserved for future implementation
type SerialConsoleConfig struct {
	Enabled         bool          `yaml:"enabled" default:"true"`
	DefaultBaudRate int           `yaml:"default_baud_rate" default:"115200"`
	BufferSize      int           `yaml:"buffer_size" default:"8192"`
	SessionTimeout  time.Duration `yaml:"session_timeout" default:"2h"`
	MaxSessions     int           `yaml:"max_sessions" default:"10"`

	// Flow control
	SupportedBaudRates []int    `yaml:"supported_baud_rates"`
	FlowControlModes   []string `yaml:"flow_control_modes"`
}

// ConnectionManagementConfig configures connection management
// TODO: Not currently used in code - reserved for future implementation
type ConnectionManagementConfig struct {
	// Gateway connection
	ConnectTimeout       time.Duration `yaml:"connect_timeout" default:"10s"`
	ReconnectInterval    time.Duration `yaml:"reconnect_interval" default:"30s"`
	MaxReconnectInterval time.Duration `yaml:"max_reconnect_interval" default:"300s"`
	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval" default:"30s"`
	HeartbeatTimeout     time.Duration `yaml:"heartbeat_timeout" default:"90s"`

	// Connection pooling
	MaxConnections    int           `yaml:"max_connections" default:"100"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" default:"30s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" default:"300s"`
	KeepAliveInterval time.Duration `yaml:"keep_alive_interval" default:"30s"`

	// Registration
	RegistrationInterval time.Duration `yaml:"registration_interval" default:"60s"`
	RegistrationTimeout  time.Duration `yaml:"registration_timeout" default:"30s"`
}

// HealthMonitoringConfig configures health monitoring
// TODO: Not currently used in code - reserved for future implementation
type HealthMonitoringConfig struct {
	Enabled        bool          `yaml:"enabled" default:"true"`
	CheckInterval  time.Duration `yaml:"check_interval" default:"60s"`
	ReportInterval time.Duration `yaml:"report_interval" default:"300s"`

	// Health checks
	EnableBMCHealthCheck     bool `yaml:"enable_bmc_health_check" default:"true"`
	EnableNetworkHealthCheck bool `yaml:"enable_network_health_check" default:"true"`
	EnableSystemHealthCheck  bool `yaml:"enable_system_health_check" default:"true"`

	// Thresholds
	CPUThreshold    float64 `yaml:"cpu_threshold" default:"80.0"`
	MemoryThreshold float64 `yaml:"memory_threshold" default:"85.0"`
	DiskThreshold   float64 `yaml:"disk_threshold" default:"90.0"`
}

// SecurityConfig configures security settings
// Note: Currently only .EncryptionKey is used in code
type SecurityConfig struct {
	// Encryption
	EncryptionKey         string `yaml:"-" env:"AGENT_ENCRYPTION_KEY"`
	EnableTLSVerification bool   `yaml:"enable_tls_verification" default:"true"` // TODO: Not currently used

	// Access control (TODO: Not currently used)
	AllowedNetworks     []string `yaml:"allowed_networks"`
	DenyPrivateNetworks bool     `yaml:"deny_private_networks" default:"false"`

	// Audit logging (TODO: Not currently used)
	EnableAuditLogging bool   `yaml:"enable_audit_logging" default:"true"`
	AuditLogPath       string `yaml:"audit_log_path" default:"/var/log/bmc-agent/audit.log"`
}

// Legacy configuration types for backward compatibility
type StaticConfig struct {
	Hosts []BMCHost `yaml:"hosts"`
}

type BMCHost struct {
	ID              string              `yaml:"id"`
	CustomerID      string              `yaml:"customer_id"`
	ControlEndpoint *BMCControlEndpoint `yaml:"control_endpoint"`
	SOLEndpoint     *SOLEndpoint        `yaml:"sol_endpoint"`
	VNCEndpoint     *VNCEndpoint        `yaml:"vnc_endpoint"`
	Features        []string            `yaml:"features"`
	Metadata        map[string]string   `yaml:"metadata"`
}

type BMCControlEndpoint struct {
	Endpoint     string     `yaml:"endpoint"`
	Type         string     `yaml:"type"`
	Username     string     `yaml:"username"`
	Password     string     `yaml:"password"`
	TLS          *TLSConfig `yaml:"tls"`
	Capabilities []string   `yaml:"capabilities"`
}

type SOLEndpoint struct {
	Type     string     `yaml:"type"`
	Endpoint string     `yaml:"endpoint"`
	Username string     `yaml:"username"`
	Password string     `yaml:"password"`
	Config   *SOLConfig `yaml:"config"`
}

type VNCEndpoint struct {
	Type     string `yaml:"type"`     // "bmc_native", "novnc_proxy", "external_kvm"
	Endpoint string `yaml:"endpoint"` // Full connection URL (e.g., "ws://novnc:6080/websockify")
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CACert             string `yaml:"ca_cert"`
}

type SOLConfig struct {
	BaudRate       int    `yaml:"baud_rate"`
	FlowControl    string `yaml:"flow_control"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// GetControlEndpoint returns the BMC control endpoint
func (h *BMCHost) GetControlEndpoint() string {
	if h.ControlEndpoint == nil {
		return ""
	}
	return h.ControlEndpoint.Endpoint
}

// GetSOLEndpoint returns the SOL endpoint if available
func (h *BMCHost) GetSOLEndpoint() string {
	if h.SOLEndpoint == nil {
		return ""
	}
	return h.SOLEndpoint.Endpoint
}

// GetVNCEndpoint returns the VNC endpoint if available
func (h *BMCHost) GetVNCEndpoint() string {
	if h.VNCEndpoint == nil {
		return ""
	}
	return h.VNCEndpoint.Endpoint
}

// Load loads the agent configuration from multiple sources
func Load(configFile, envFile string) (*Config, error) {
	cfg := &Config{}

	loader := config.NewConfigLoader(config.LoaderConfig{
		ConfigFile:      configFile,
		EnvironmentFile: envFile,
		ServiceName:     "agent",
	})

	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load agent configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("agent configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate required configuration values (these now have defaults)
	if c.Agent.GatewayEndpoint == "" {
		return fmt.Errorf("agent gateway endpoint is required")
	}

	if c.Agent.DatacenterID == "" {
		return fmt.Errorf("agent datacenter id is required")
	}

	// Validate network ranges for BMC discovery
	for _, network := range c.Agent.BMCDiscovery.NetworkRanges {
		if _, _, err := net.ParseCIDR(network); err != nil {
			return fmt.Errorf("invalid network range %s: %w", network, err)
		}
	}

	// Validate BMC discovery ports
	if len(c.Agent.BMCDiscovery.IPMIPorts) == 0 {
		c.Agent.BMCDiscovery.IPMIPorts = []int{623}
	}

	if len(c.Agent.BMCDiscovery.RedfishPorts) == 0 {
		c.Agent.BMCDiscovery.RedfishPorts = []int{443, 8000, 8443}
	}

	// Validate port ranges
	if c.Agent.VNCConfig.Port < 1 || c.Agent.VNCConfig.Port > 65535 {
		return fmt.Errorf("VNC port must be between 1 and 65535")
	}

	// Validate timeouts
	if c.Agent.BMCOperations.OperationTimeout <= 0 {
		return fmt.Errorf("BMC operation timeout must be positive")
	}

	if c.Agent.ConnectionManagement.ConnectTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}

	// Validate concurrency limits
	if c.Agent.BMCOperations.MaxConcurrentOperations <= 0 {
		return fmt.Errorf("max concurrent operations must be positive")
	}

	if c.Agent.VNCConfig.MaxConnections <= 0 {
		return fmt.Errorf("VNC max connections must be positive")
	}

	// Validate IPMI configuration
	validInterfaces := map[string]bool{
		"lan":     true,
		"lanplus": true,
		"serial":  true,
	}

	if !validInterfaces[c.Agent.BMCOperations.IPMIConfig.Interface] {
		return fmt.Errorf("invalid IPMI interface: %s", c.Agent.BMCOperations.IPMIConfig.Interface)
	}

	// Validate VNC configuration
	if c.Agent.VNCConfig.FrameRate <= 0 || c.Agent.VNCConfig.FrameRate > 60 {
		return fmt.Errorf("VNC frame rate must be between 1 and 60")
	}

	if c.Agent.VNCConfig.Quality < 0 || c.Agent.VNCConfig.Quality > 9 {
		return fmt.Errorf("VNC quality must be between 0 and 9")
	}

	// Validate health monitoring thresholds
	if c.Agent.HealthMonitoring.CPUThreshold < 0 || c.Agent.HealthMonitoring.CPUThreshold > 100 {
		return fmt.Errorf("CPU threshold must be between 0 and 100")
	}

	if c.Agent.HealthMonitoring.MemoryThreshold < 0 || c.Agent.HealthMonitoring.MemoryThreshold > 100 {
		return fmt.Errorf("memory threshold must be between 0 and 100")
	}

	if c.Agent.HealthMonitoring.DiskThreshold < 0 || c.Agent.HealthMonitoring.DiskThreshold > 100 {
		return fmt.Errorf("disk threshold must be between 0 and 100")
	}

	// Set defaults for supported baud rates if not specified
	if len(c.Agent.SerialConsole.SupportedBaudRates) == 0 {
		c.Agent.SerialConsole.SupportedBaudRates = []int{9600, 19200, 38400, 57600, 115200}
	}

	// Set defaults for flow control modes if not specified
	if len(c.Agent.SerialConsole.FlowControlModes) == 0 {
		c.Agent.SerialConsole.FlowControlModes = []string{"none", "hardware", "software"}
	}

	return nil
}

// GetVNCListenAddress returns the address the VNC server should listen on
func (c *Config) GetVNCListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Agent.VNCConfig.BindAddress, c.Agent.VNCConfig.Port)
}
