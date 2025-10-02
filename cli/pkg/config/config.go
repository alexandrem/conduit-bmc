package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Manager ManagerConfig `mapstructure:"manager"`
	Auth    AuthConfig    `mapstructure:"auth"`
	// Legacy gateway config for backward compatibility
	Gateway GatewayConfig `mapstructure:"gateway"`
}

type ManagerConfig struct {
	Endpoint string `mapstructure:"endpoint"`
}

type GatewayConfig struct {
	URL string `mapstructure:"url"`
}

type AuthConfig struct {
	// New delegated token system
	AccessToken    string    `mapstructure:"access_token"`
	RefreshToken   string    `mapstructure:"refresh_token"`
	TokenExpiresAt time.Time `mapstructure:"token_expires_at"`
	Email          string    `mapstructure:"email"`

	// Legacy auth for backward compatibility
	APIKey string `mapstructure:"api_key"`
	Token  string `mapstructure:"token"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config search paths
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.bmc-cli")
	viper.AddConfigPath("/etc/bmc-cli/")

	// Environment variable overrides
	viper.SetEnvPrefix("BMC")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind environment variables for nested config
	// With SetEnvPrefix("BMC"), these become: BMC_MANAGER_ENDPOINT, BMC_AUTH_ACCESS_TOKEN, etc.
	viper.BindEnv("manager.endpoint")
	viper.BindEnv("auth.access_token")
	viper.BindEnv("auth.refresh_token")
	viper.BindEnv("auth.email")
	viper.BindEnv("auth.api_key")
	viper.BindEnv("gateway.url")

	// Set defaults
	viper.SetDefault("manager.endpoint", "http://localhost:8080")
	viper.SetDefault("gateway.url", "http://localhost:8081") // Legacy - Gateway on 8081

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".bmc-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	viper.SetConfigFile(configFile)

	// Update viper with current config values
	viper.Set("manager.endpoint", c.Manager.Endpoint)
	viper.Set("gateway.url", c.Gateway.URL)
	viper.Set("auth.access_token", c.Auth.AccessToken)
	viper.Set("auth.refresh_token", c.Auth.RefreshToken)
	viper.Set("auth.token_expires_at", c.Auth.TokenExpiresAt)
	viper.Set("auth.email", c.Auth.Email)
	viper.Set("auth.api_key", c.Auth.APIKey)
	viper.Set("auth.token", c.Auth.Token)

	return viper.WriteConfig()
}
