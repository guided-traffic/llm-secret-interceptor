// Package config provides configuration management for the LLM Secret Interceptor proxy.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Proxy        ProxyConfig        `yaml:"proxy"`
	TLS          TLSConfig          `yaml:"tls"`
	Storage      StorageConfig      `yaml:"storage"`
	Placeholder  PlaceholderConfig  `yaml:"placeholder"`
	Interceptors InterceptorsConfig `yaml:"interceptors"`
	Logging      LoggingConfig      `yaml:"logging"`
	Metrics      MetricsConfig      `yaml:"metrics"`
}

// ProxyConfig contains proxy server settings
type ProxyConfig struct {
	Listen string `yaml:"listen"`
}

// TLSConfig contains TLS/CA certificate settings
type TLSConfig struct {
	CACert string `yaml:"ca_cert"`
	CAKey  string `yaml:"ca_key"`
}

// StorageConfig contains mapping storage settings
type StorageConfig struct {
	Type  string        `yaml:"type"` // "memory" or "redis"
	Redis RedisConfig   `yaml:"redis"`
	TTL   time.Duration `yaml:"ttl"`
}

// RedisConfig contains Redis connection settings
type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"` //#nosec G117 -- Password field is intentional for Redis auth config
	DB       int    `yaml:"db"`
}

// PlaceholderConfig contains placeholder format settings
type PlaceholderConfig struct {
	Prefix string `yaml:"prefix"`
	Suffix string `yaml:"suffix"`
}

// InterceptorsConfig contains settings for all secret interceptors
type InterceptorsConfig struct {
	Entropy   EntropyConfig   `yaml:"entropy"`
	Bitwarden BitwardenConfig `yaml:"bitwarden"`
}

// EntropyConfig contains entropy-based interceptor settings
type EntropyConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Threshold float64 `yaml:"threshold"`
	MinLength int     `yaml:"min_length"`
	MaxLength int     `yaml:"max_length"`
}

// BitwardenConfig contains Bitwarden interceptor settings
type BitwardenConfig struct {
	Enabled   bool   `yaml:"enabled"`
	ServerURL string `yaml:"server_url"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level string      `yaml:"level"`
	Audit AuditConfig `yaml:"audit"`
}

// AuditConfig contains audit logging settings
type AuditConfig struct {
	Enabled            bool `yaml:"enabled"`
	LogInterceptorName bool `yaml:"log_interceptor_name"`
	LogSecretType      bool `yaml:"log_secret_type"`
}

// MetricsConfig contains Prometheus metrics settings
type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	Port     int    `yaml:"port"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			Listen: ":8080",
		},
		TLS: TLSConfig{
			CACert: "./certs/ca.crt",
			CAKey:  "./certs/ca.key",
		},
		Storage: StorageConfig{
			Type: "memory",
			TTL:  24 * time.Hour,
			Redis: RedisConfig{
				Address: "localhost:6379",
				DB:      0,
			},
		},
		Placeholder: PlaceholderConfig{
			Prefix: "__SECRET_",
			Suffix: "__",
		},
		Interceptors: InterceptorsConfig{
			Entropy: EntropyConfig{
				Enabled:   true,
				Threshold: 4.5,
				MinLength: 8,
				MaxLength: 128,
			},
			Bitwarden: BitwardenConfig{
				Enabled: false,
			},
		},
		Logging: LoggingConfig{
			Level: "info",
			Audit: AuditConfig{
				Enabled:            true,
				LogInterceptorName: true,
				LogSecretType:      true,
			},
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Endpoint: "/metrics",
			Port:     9090,
		},
	}
}

// Load loads the configuration from file or environment
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Check for config file path in environment or use default
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Get base directory (working directory or CONFIG_BASE_DIR if set)
	baseDir := os.Getenv("CONFIG_BASE_DIR")
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Sanitize and validate path to prevent path traversal
	safePath, err := sanitizeConfigPath(configPath, baseDir)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	// Try to load config file
	data, err := os.ReadFile(safePath) //#nosec G304,G703 -- path is validated by sanitizeConfigPath to be within baseDir
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// sanitizeConfigPath validates that the given path is within the allowed base directory.
// It returns the absolute, cleaned path if valid, or an error if path traversal is detected.
func sanitizeConfigPath(path, baseDir string) (string, error) {
	// Clean the base directory to get absolute path
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	// Resolve the config path relative to base directory
	var targetPath string
	if filepath.IsAbs(path) {
		targetPath = filepath.Clean(path)
	} else {
		targetPath = filepath.Clean(filepath.Join(absBase, path))
	}

	// Verify the resolved path is within the base directory
	// Use filepath.Rel to check if target is within base
	relPath, err := filepath.Rel(absBase, targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check if the relative path escapes the base directory
	// A path that starts with ".." would escape the base directory
	if len(relPath) >= 2 && relPath[:2] == ".." {
		return "", fmt.Errorf("path traversal detected: path escapes base directory")
	}

	return targetPath, nil
}
