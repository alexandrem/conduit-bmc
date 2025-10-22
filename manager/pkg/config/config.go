package config

import (
	"fmt"
	"strings"
	"time"

	"core/config"

	"github.com/rs/zerolog"
)

// Config contains all configuration for the manager service
type Config struct {
	// Logging configuration
	Log LogConfig `yaml:"log"`

	// Service-specific configuration
	Manager ManagerConfig `yaml:"manager"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Authentication configuration
	Auth AuthConfig `yaml:"auth"`

	// TLS configuration
	TLS config.TLSConfig `yaml:"tls"`
}

// LogConfig contains manager-specific logging configuration
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

// DatabaseConfig contains manager-specific database configuration
type DatabaseConfig struct {
	Driver          string        `yaml:"driver" default:"sqlite3"`
	DSN             string        `yaml:"dsn" env:"DATABASE_URL" default:"file:./manager.db"`
	MaxOpenConns    int           `yaml:"max_open_conns" default:"25"`    // TODO: Not currently used in code
	MaxIdleConns    int           `yaml:"max_idle_conns" default:"10"`    // TODO: Not currently used in code
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" default:"5m"` // TODO: Not currently used in code
}

// AuthConfig contains manager-specific authentication configuration
type AuthConfig struct {
	JWTSecretKey    string        `yaml:"-" env:"JWT_SECRET_KEY"`
	TokenTTL        time.Duration `yaml:"token_ttl" default:"24h"`          // TODO: Not currently used in code
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl" default:"168h"` // TODO: Not currently used in code
}

// ManagerConfig contains manager-specific configuration
type ManagerConfig struct {
	// Server configuration
	Host string `yaml:"host" env:"MANAGER_HOST" default:"0.0.0.0"`
	Port int    `yaml:"port" env:"MANAGER_PORT" default:"8080"`

	// External service endpoints
	GatewayDiscovery GatewayDiscoveryConfig `yaml:"gateway_discovery"`

	// Server management
	ServerManagement ServerManagementConfig `yaml:"server_management"`

	// Customer management
	CustomerManagement CustomerManagementConfig `yaml:"customer_management"`

	// Rate limiting
	RateLimit RateLimitConfig `yaml:"rate_limit"`

	// Session management
	SessionManagement SessionManagementConfig `yaml:"session_management"`
}

// GatewayDiscoveryConfig configures how the manager discovers gateways
// TODO: Not currently used in code - reserved for future implementation
type GatewayDiscoveryConfig struct {
	Enabled         bool          `yaml:"enabled" default:"true"`
	UpdateInterval  time.Duration `yaml:"update_interval" default:"30s"`
	HealthCheckPath string        `yaml:"health_check_path" default:"/health"`
	Timeout         time.Duration `yaml:"timeout" default:"5s"`
}

// ServerManagementConfig configures server management behavior
// TODO: Not currently used in code - reserved for future implementation
type ServerManagementConfig struct {
	AutoRegistration       bool          `yaml:"auto_registration" default:"true"`
	HeartbeatInterval      time.Duration `yaml:"heartbeat_interval" default:"60s"`
	HeartbeatTimeout       time.Duration `yaml:"heartbeat_timeout" default:"300s"`
	MaxServersPerCustomer  int           `yaml:"max_servers_per_customer" default:"100"`
	EnableServerValidation bool          `yaml:"enable_server_validation" default:"true"`
}

// CustomerManagementConfig configures customer management behavior
// TODO: Not currently used in code - reserved for future implementation
type CustomerManagementConfig struct {
	AllowSelfRegistration     bool          `yaml:"allow_self_registration" default:"false"`
	EmailVerificationRequired bool          `yaml:"email_verification_required" default:"true"`
	PasswordMinLength         int           `yaml:"password_min_length" default:"8"`
	APIKeyLength              int           `yaml:"api_key_length" default:"32"`
	MaxAPIKeysPerCustomer     int           `yaml:"max_api_keys_per_customer" default:"5"`
	SessionInactivityTimeout  time.Duration `yaml:"session_inactivity_timeout" default:"30m"`
}

// RateLimitConfig configures rate limiting
// Note: Currently only .Enabled is used in code
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled" default:"true"`
	RequestsPerMinute int  `yaml:"requests_per_minute" default:"100"` // TODO: Not currently used
	BurstSize         int  `yaml:"burst_size" default:"20"`           // TODO: Not currently used

	// Per-endpoint rate limits (TODO: Not currently used)
	AuthRequestsPerMinute    int `yaml:"auth_requests_per_minute" default:"10"`
	PowerRequestsPerMinute   int `yaml:"power_requests_per_minute" default:"30"`
	ConsoleRequestsPerMinute int `yaml:"console_requests_per_minute" default:"5"`
}

// SessionManagementConfig configures session management
// TODO: Not currently used in code - reserved for future implementation
type SessionManagementConfig struct {
	ProxySessionTTL       time.Duration `yaml:"proxy_session_ttl" default:"1h"`
	VNCSessionTTL         time.Duration `yaml:"vnc_session_ttl" default:"4h"`
	ConsoleSessionTTL     time.Duration `yaml:"console_session_ttl" default:"2h"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval" default:"5m"`
	MaxConcurrentSessions int           `yaml:"max_concurrent_sessions" default:"10"`
}

// Load loads the manager configuration from multiple sources
func Load(configFile, envFile string) (*Config, error) {
	cfg := &Config{}

	loader := config.NewConfigLoader(config.LoaderConfig{
		ConfigFile:      configFile,
		EnvironmentFile: envFile,
		ServiceName:     "manager",
	})

	if err := loader.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load manager configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("manager configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate required environment variables for security-sensitive settings
	if c.Auth.JWTSecretKey == "" {
		return fmt.Errorf("JWT_SECRET_KEY environment variable is required")
	}

	if len(c.Auth.JWTSecretKey) < 32 {
		return fmt.Errorf("JWT_SECRET_KEY must be at least 32 characters long")
	}

	// Validate database configuration
	if c.Database.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Validate port ranges
	if c.Manager.Port < 1 || c.Manager.Port > 65535 {
		return fmt.Errorf("manager port must be between 1 and 65535")
	}

	// Validate rate limiting
	if c.Manager.RateLimit.Enabled {
		if c.Manager.RateLimit.RequestsPerMinute <= 0 {
			return fmt.Errorf("rate limit requests per minute must be positive")
		}
		if c.Manager.RateLimit.BurstSize <= 0 {
			return fmt.Errorf("rate limit burst size must be positive")
		}
	}

	// Validate customer management
	if c.Manager.CustomerManagement.PasswordMinLength < 6 {
		return fmt.Errorf("password minimum length must be at least 6")
	}

	if c.Manager.CustomerManagement.APIKeyLength < 16 {
		return fmt.Errorf("API key length must be at least 16")
	}

	// Validate session timeouts
	if c.Manager.SessionManagement.ProxySessionTTL <= 0 {
		return fmt.Errorf("proxy session TTL must be positive")
	}

	if c.Manager.SessionManagement.VNCSessionTTL <= 0 {
		return fmt.Errorf("VNC session TTL must be positive")
	}

	if c.Manager.SessionManagement.ConsoleSessionTTL <= 0 {
		return fmt.Errorf("console session TTL must be positive")
	}

	return nil
}

// GetListenAddress returns the address the manager should listen on
func (c *Config) GetListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Manager.Host, c.Manager.Port)
}
