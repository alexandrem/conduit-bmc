package config

import (
	"fmt"
	"strings"
	"time"

	"core/config"
	"github.com/rs/zerolog"
)

// Config contains all configuration for the gateway service
type Config struct {
	// Logging configuration
	Log LogConfig `yaml:"log"`

	// Service-specific configuration
	Gateway GatewayConfig `yaml:"gateway"`

	// Authentication configuration (minimal - gateway validates tokens, doesn't create them)
	Auth AuthConfig `yaml:"auth"`

	// TLS configuration
	TLS config.TLSConfig `yaml:"tls"`
}

// LogConfig contains gateway-specific logging configuration
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

// AuthConfig contains gateway-specific authentication configuration
type AuthConfig struct {
	JWTSecretKey string `yaml:"-" env:"JWT_SECRET_KEY"`
}

// GatewayConfig contains gateway-specific configuration
type GatewayConfig struct {
	// Server configuration
	Host string `yaml:"host" env:"GATEWAY_HOST" default:"0.0.0.0"`
	Port int    `yaml:"port" env:"GATEWAY_PORT" default:"8081"`

	// External service configuration
	ManagerEndpoint string `yaml:"manager_endpoint" env:"BMC_MANAGER_ENDPOINT" default:"http://localhost:8080"`

	// Region and datacenter configuration
	Region      string   `yaml:"region" default:"default"`
	Datacenters []string `yaml:"datacenters"` // TODO: Not currently used in code

	// Proxy configuration (TODO: Not currently used in code)
	Proxy ProxyConfig `yaml:"proxy"`

	// WebSocket configuration (TODO: Not currently used in code)
	WebSocket WebSocketConfig `yaml:"websocket"`

	// Session management (TODO: Not currently used in code)
	SessionManagement SessionManagementConfig `yaml:"session_management"`

	// Web UI configuration (TODO: Not currently used in code)
	WebUI WebUIConfig `yaml:"webui"`

	// Agent connection management (TODO: Not currently used in code)
	AgentConnections AgentConnectionConfig `yaml:"agent_connections"`

	// Rate limiting (only .Enabled is currently used)
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ProxyConfig configures proxy behavior
// TODO: Not currently used in code - reserved for future implementation
type ProxyConfig struct {
	ReadTimeout       time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout      time.Duration `yaml:"write_timeout" default:"30s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" default:"300s"`
	MaxHeaderSize     int           `yaml:"max_header_size" default:"8192"`
	BufferSize        int           `yaml:"buffer_size" default:"32768"`
	EnableCompression bool          `yaml:"enable_compression" default:"true"`

	// BMC-specific proxy settings
	BMCTimeout     time.Duration `yaml:"bmc_timeout" default:"60s"`
	IPMITimeout    time.Duration `yaml:"ipmi_timeout" default:"30s"`
	RedfishTimeout time.Duration `yaml:"redfish_timeout" default:"45s"`
	MaxRetries     int           `yaml:"max_retries" default:"3"`
	RetryBackoff   time.Duration `yaml:"retry_backoff" default:"1s"`
}

// WebSocketConfig configures WebSocket behavior
// TODO: Not currently used in code - reserved for future implementation
type WebSocketConfig struct {
	ReadBufferSize     int           `yaml:"read_buffer_size" default:"4096"`
	WriteBufferSize    int           `yaml:"write_buffer_size" default:"4096"`
	PingInterval       time.Duration `yaml:"ping_interval" default:"30s"`
	PongTimeout        time.Duration `yaml:"pong_timeout" default:"60s"`
	MessageSizeLimit   int64         `yaml:"message_size_limit" default:"32768"`
	CompressionEnabled bool          `yaml:"compression_enabled" default:"true"`

	// VNC-specific settings
	VNCFrameRate   int `yaml:"vnc_frame_rate" default:"15"`
	VNCQuality     int `yaml:"vnc_quality" default:"6"`
	VNCCompression int `yaml:"vnc_compression" default:"2"`
}

// SessionManagementConfig configures session management
// TODO: Not currently used in code - reserved for future implementation
type SessionManagementConfig struct {
	ProxySessionTTL    time.Duration `yaml:"proxy_session_ttl" default:"1h"`
	VNCSessionTTL      time.Duration `yaml:"vnc_session_ttl" default:"4h"`
	ConsoleSessionTTL  time.Duration `yaml:"console_session_ttl" default:"2h"`
	CleanupInterval    time.Duration `yaml:"cleanup_interval" default:"5m"`
	SessionTokenLength int           `yaml:"session_token_length" default:"32"`

	// Session storage configuration
	UseInMemoryStore bool `yaml:"use_in_memory_store" default:"true"`
}

// WebUIConfig configures the web user interface
// TODO: Not currently used in code - reserved for future implementation
type WebUIConfig struct {
	Enabled      bool   `yaml:"enabled" default:"true"`
	StaticPath   string `yaml:"static_path" default:"/static"`
	TemplatePath string `yaml:"template_path" default:"/templates"`
	Title        string `yaml:"title" default:"BMC Management Console"`

	// VNC viewer configuration
	VNCViewerURL    string `yaml:"vnc_viewer_url" default:"/vnc"`
	NoVNCPath       string `yaml:"novnc_path" default:"/novnc"`
	VNCAutoConnect  bool   `yaml:"vnc_auto_connect" default:"true"`
	VNCShowPassword bool   `yaml:"vnc_show_password" default:"false"`

	// Console viewer configuration
	ConsoleViewerURL  string `yaml:"console_viewer_url" default:"/console"`
	ConsoleScrollback int    `yaml:"console_scrollback" default:"1000"`
}

// AgentConnectionConfig configures agent connection management
// TODO: Not currently used in code - reserved for future implementation
type AgentConnectionConfig struct {
	MaxConnections      int           `yaml:"max_connections" default:"100"`
	ConnectionTimeout   time.Duration `yaml:"connection_timeout" default:"30s"`
	HeartbeatInterval   time.Duration `yaml:"heartbeat_interval" default:"30s"`
	HeartbeatTimeout    time.Duration `yaml:"heartbeat_timeout" default:"90s"`
	ReconnectBackoff    time.Duration `yaml:"reconnect_backoff" default:"5s"`
	MaxReconnectBackoff time.Duration `yaml:"max_reconnect_backoff" default:"300s"`
}

// RateLimitConfig configures rate limiting
// Note: Currently only .Enabled is used in code
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled" default:"true"`
	RequestsPerMinute int  `yaml:"requests_per_minute" default:"1000"` // TODO: Not currently used
	BurstSize         int  `yaml:"burst_size" default:"100"`           // TODO: Not currently used

	// Per-endpoint rate limits (TODO: Not currently used)
	ProxyRequestsPerMinute   int `yaml:"proxy_requests_per_minute" default:"100"`
	VNCRequestsPerMinute     int `yaml:"vnc_requests_per_minute" default:"10"`
	ConsoleRequestsPerMinute int `yaml:"console_requests_per_minute" default:"20"`
}

// Load loads the gateway configuration from multiple sources
func Load(configFile, envFile string) (*Config, error) {
	cfg := &Config{}

	loader := config.NewConfigLoader(config.LoaderConfig{
		ConfigFile:      configFile,
		EnvironmentFile: envFile,
		ServiceName:     "gateway",
	})

	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load gateway configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("gateway configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate required configuration values (these now have defaults)
	if c.Gateway.ManagerEndpoint == "" {
		return fmt.Errorf("gateway manager endpoint is required")
	}

	// Validate port ranges
	if c.Gateway.Port < 1 || c.Gateway.Port > 65535 {
		return fmt.Errorf("gateway port must be between 1 and 65535")
	}

	// Validate proxy configuration
	if c.Gateway.Proxy.ReadTimeout <= 0 {
		return fmt.Errorf("proxy read timeout must be positive")
	}

	if c.Gateway.Proxy.WriteTimeout <= 0 {
		return fmt.Errorf("proxy write timeout must be positive")
	}

	if c.Gateway.Proxy.BufferSize <= 0 {
		return fmt.Errorf("proxy buffer size must be positive")
	}

	// Validate WebSocket configuration
	if c.Gateway.WebSocket.ReadBufferSize <= 0 {
		return fmt.Errorf("WebSocket read buffer size must be positive")
	}

	if c.Gateway.WebSocket.WriteBufferSize <= 0 {
		return fmt.Errorf("WebSocket write buffer size must be positive")
	}

	if c.Gateway.WebSocket.VNCFrameRate <= 0 || c.Gateway.WebSocket.VNCFrameRate > 60 {
		return fmt.Errorf("VNC frame rate must be between 1 and 60")
	}

	// Validate session management
	if c.Gateway.SessionManagement.ProxySessionTTL <= 0 {
		return fmt.Errorf("proxy session TTL must be positive")
	}

	if c.Gateway.SessionManagement.VNCSessionTTL <= 0 {
		return fmt.Errorf("VNC session TTL must be positive")
	}

	if c.Gateway.SessionManagement.SessionTokenLength < 16 {
		return fmt.Errorf("session token length must be at least 16")
	}

	// Validate agent connections
	if c.Gateway.AgentConnections.MaxConnections <= 0 {
		return fmt.Errorf("max agent connections must be positive")
	}

	if c.Gateway.AgentConnections.ConnectionTimeout <= 0 {
		return fmt.Errorf("agent connection timeout must be positive")
	}

	// Validate rate limiting
	if c.Gateway.RateLimit.Enabled {
		if c.Gateway.RateLimit.RequestsPerMinute <= 0 {
			return fmt.Errorf("rate limit requests per minute must be positive")
		}
		if c.Gateway.RateLimit.BurstSize <= 0 {
			return fmt.Errorf("rate limit burst size must be positive")
		}
	}

	return nil
}

// GetListenAddress returns the address the gateway should listen on
func (c *Config) GetListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Gateway.Host, c.Gateway.Port)
}
