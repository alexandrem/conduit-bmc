package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// CommonConfig contains configuration common to all services
type CommonConfig struct {
	// Logging configuration
	Log LogConfig `yaml:"log"`
}

// LogConfig configures logging behavior
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

// DatabaseConfig contains database connection configuration
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn" env:"DATABASE_URL"`
}

// TLSConfig contains TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecretKey string `yaml:"-" env:"JWT_SECRET_KEY"`
}

// LoaderConfig configures how configuration is loaded
type LoaderConfig struct {
	ConfigFile      string
	EnvironmentFile string
	ServiceName     string
}

// ConfigLoader handles loading configuration from multiple sources
type ConfigLoader struct {
	config LoaderConfig
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(cfg LoaderConfig) *ConfigLoader {
	return &ConfigLoader{config: cfg}
}

// Load loads configuration into the provided struct
func (l *ConfigLoader) Load(target interface{}) error {
	// 1. Set defaults from struct tags
	if err := l.setDefaults(target); err != nil {
		return fmt.Errorf("failed to set defaults: %w", err)
	}

	// 2. Load from config file if it exists
	if l.config.ConfigFile != "" {
		if err := l.loadFromYAML(target, l.config.ConfigFile); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// 3. Load from environment file if it exists
	if l.config.EnvironmentFile != "" {
		if err := l.loadEnvironmentFile(l.config.EnvironmentFile); err != nil {
			return fmt.Errorf("failed to load environment file: %w", err)
		}
	}

	// 4. Override with environment variables
	if err := l.loadFromEnv(target); err != nil {
		return fmt.Errorf("failed to load from environment: %w", err)
	}

	return nil
}

// setDefaults sets default values from struct tags
func (l *ConfigLoader) setDefaults(target interface{}) error {
	return l.setDefaultsRecursive(reflect.ValueOf(target))
}

func (l *ConfigLoader) setDefaultsRecursive(v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct || (field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct) {
			if err := l.setDefaultsRecursive(field); err != nil {
				return err
			}
			continue
		}

		// Set defaults from tag
		defaultValue := fieldType.Tag.Get("default")
		if defaultValue != "" {
			if err := l.setFieldValue(field, defaultValue); err != nil {
				return fmt.Errorf("failed to set default for field %s: %w", fieldType.Name, err)
			}
		}
	}

	return nil
}

// loadFromYAML loads configuration from a YAML file
func (l *ConfigLoader) loadFromYAML(target interface{}, filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil // Config file is optional
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	return nil
}

// loadEnvironmentFile loads environment variables from a file
func (l *ConfigLoader) loadEnvironmentFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil // Environment file is optional
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read environment file %s: %w", filename, err)
	}

	lines := strings.Split(string(data), "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid line %d in environment file %s: %s", lineNum+1, filename, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Only set from environment file if not already set in actual environment
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (l *ConfigLoader) loadFromEnv(target interface{}) error {
	return l.loadFromEnvRecursive(reflect.ValueOf(target), "")
}

func (l *ConfigLoader) loadFromEnvRecursive(v reflect.Value, prefix string) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct || (field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct) {
			nestedPrefix := prefix
			if prefix != "" {
				nestedPrefix += "_"
			}
			nestedPrefix += strings.ToUpper(fieldType.Name)
			if err := l.loadFromEnvRecursive(field, nestedPrefix); err != nil {
				return err
			}
			continue
		}

		// Get environment variable name from tag or generate from field name
		envName := fieldType.Tag.Get("env")
		if envName == "" {
			envName = prefix
			if prefix != "" {
				envName += "_"
			}
			envName += strings.ToUpper(fieldType.Name)
		}

		// Check for service-specific override
		if l.config.ServiceName != "" {
			serviceSpecificName := strings.ToUpper(l.config.ServiceName) + "_" + envName
			if value, exists := os.LookupEnv(serviceSpecificName); exists {
				if err := l.setFieldValue(field, value); err != nil {
					return fmt.Errorf("failed to set field %s from env %s: %w", fieldType.Name, serviceSpecificName, err)
				}
				continue
			}
		}

		// Check for regular environment variable
		if value, exists := os.LookupEnv(envName); exists {
			if err := l.setFieldValue(field, value); err != nil {
				return fmt.Errorf("failed to set field %s from env %s: %w", fieldType.Name, envName, err)
			}
		}
	}

	return nil
}

// setFieldValue sets a field value from a string
func (l *ConfigLoader) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			field.SetBool(true)
		case "false", "0", "no", "off":
			field.SetBool(false)
		default:
			return fmt.Errorf("invalid boolean value: %s", value)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration value: %s", value)
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := parseIntValue(value)
			if err != nil {
				return fmt.Errorf("invalid integer value: %s", value)
			}
			field.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := parseUintValue(value)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %s", value)
		}
		field.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := parseFloatValue(value, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid float value: %s", value)
		}
		field.SetFloat(floatVal)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}

	return nil
}

// FindConfigFile searches for a configuration file in standard locations
func FindConfigFile(serviceName string) string {
	configName := serviceName + ".yaml"

	// Search order:
	// 1. Current directory
	// 2. ./config/
	// 3. ./configs/
	// 4. /etc/{serviceName}/
	// 5. $HOME/.{serviceName}/
	searchPaths := []string{
		configName,
		filepath.Join("config", configName),
		filepath.Join("configs", configName),
		filepath.Join("/etc", serviceName, configName),
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths, filepath.Join(homeDir, "."+serviceName, configName))
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// FindEnvironmentFile searches for an environment file
func FindEnvironmentFile(serviceName string) string {
	envName := serviceName + ".env"

	searchPaths := []string{
		".env",
		envName,
		filepath.Join("config", ".env"),
		filepath.Join("config", envName),
		filepath.Join("configs", ".env"),
		filepath.Join("configs", envName),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
