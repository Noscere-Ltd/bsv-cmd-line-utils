// Package config provides shared configuration management for BSV CLI tools.
//
// This package handles loading and parsing of config.yaml files used by
// broadcast, txstatus, and other ARC-based CLI tools.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ARCConfig holds the configuration for an ARC endpoint (mainnet or testnet).
type ARCConfig struct {
	URL     string `yaml:"url"`     // ARC endpoint URL (e.g., "https://api.taal.com")
	APIKey  string `yaml:"api_key"` // API key for authentication
	Timeout string `yaml:"timeout"` // HTTP timeout duration (e.g., "30s")
}

// PollingConfig defines parameters for transaction status polling when monitoring is enabled.
type PollingConfig struct {
	Interval      string  `yaml:"interval"`       // Time between status checks (e.g., "3s")
	MaxRetries    int     `yaml:"max_retries"`    // Maximum number of retry attempts
	BackoffFactor float64 `yaml:"backoff_factor"` // Multiplier for exponential backoff
}

// TargetsConfig specifies target states for transaction monitoring.
type TargetsConfig struct {
	Default       string `yaml:"default"`         // Default target status to wait for
	WaitForMining bool   `yaml:"wait_for_mining"` // Whether to wait for MINED status
}

// Config is the root configuration structure loaded from config.yaml.
type Config struct {
	ARCMainnet ARCConfig     `yaml:"arc-mainnet"` // Mainnet ARC configuration
	ARCTestnet ARCConfig     `yaml:"arc-testnet"` // Testnet ARC configuration
	Polling    PollingConfig `yaml:"polling"`     // Polling parameters for monitoring
	Targets    TargetsConfig `yaml:"targets"`     // Target status configuration
}

// Load reads and parses a config.yaml file.
// It first checks the executable directory, then falls back to the current working directory.
// Returns the parsed config or an error if the config file cannot be found or parsed.
func Load() (*Config, error) {
	return LoadFromPath("")
}

// LoadFromPath reads and parses a config.yaml file from the specified path.
// If path is empty, it searches the executable directory then the current working directory.
// Returns the parsed config or an error if the config file cannot be found or parsed.
func LoadFromPath(path string) (*Config, error) {
	var configPath string

	if path != "" {
		configPath = path
	} else {
		// Get the executable directory
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
		exeDir := filepath.Dir(exePath)

		// Try config.yaml in the executable directory first
		configPath = filepath.Join(exeDir, "config.yaml")

		// If not found, try the current working directory
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = "config.yaml"
		}
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // G304: config file path is user-controlled by design
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// GetARCConfig returns the appropriate ARC configuration based on the testnet flag.
func (c *Config) GetARCConfig(testnet bool) ARCConfig {
	if testnet {
		return c.ARCTestnet
	}
	return c.ARCMainnet
}

// Validate checks that required configuration fields are present.
// Returns an error if required fields are missing.
func (c *Config) Validate(testnet bool) error {
	arcConfig := c.GetARCConfig(testnet)
	if arcConfig.URL == "" {
		network := "mainnet"
		if testnet {
			network = "testnet"
		}
		return fmt.Errorf("ARC URL is required for %s in config.yaml", network)
	}
	return nil
}
