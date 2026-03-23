package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromPath(t *testing.T) {
	t.Parallel()

	t.Run("valid config file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `
arc-mainnet:
  url: "https://api.taal.com/arc"
  api_key: "mainnet-key"
  timeout: "30s"
arc-testnet:
  url: "https://arc-test.taal.com/arc"
  api_key: "testnet-key"
  timeout: "30s"
polling:
  interval: "5s"
  max_retries: 10
  backoff_factor: 1.5
targets:
  default: "MINED"
  wait_for_mining: true
`
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		require.NoError(t, err)

		cfg, err := LoadFromPath(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, "https://api.taal.com/arc", cfg.ARCMainnet.URL)
		assert.Equal(t, "mainnet-key", cfg.ARCMainnet.APIKey)
		assert.Equal(t, "30s", cfg.ARCMainnet.Timeout)

		assert.Equal(t, "https://arc-test.taal.com/arc", cfg.ARCTestnet.URL)
		assert.Equal(t, "testnet-key", cfg.ARCTestnet.APIKey)

		assert.Equal(t, "5s", cfg.Polling.Interval)
		assert.Equal(t, 10, cfg.Polling.MaxRetries)
		assert.InEpsilon(t, 1.5, cfg.Polling.BackoffFactor, 1e-9)

		assert.Equal(t, "MINED", cfg.Targets.Default)
		assert.True(t, cfg.Targets.WaitForMining)
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromPath("/nonexistent/path/config.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("invalid YAML", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		invalidYAML := `
arc-mainnet:
  url: [invalid yaml
  this is broken
`
		err := os.WriteFile(configPath, []byte(invalidYAML), 0o600)
		require.NoError(t, err)

		_, err = LoadFromPath(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	t.Run("empty config file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		err := os.WriteFile(configPath, []byte(""), 0o600)
		require.NoError(t, err)

		cfg, err := LoadFromPath(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// All fields should be zero values
		assert.Empty(t, cfg.ARCMainnet.URL)
		assert.Empty(t, cfg.ARCTestnet.URL)
	})

	t.Run("partial config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		partialConfig := `
arc-mainnet:
  url: "https://api.taal.com/arc"
`
		err := os.WriteFile(configPath, []byte(partialConfig), 0o600)
		require.NoError(t, err)

		cfg, err := LoadFromPath(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, "https://api.taal.com/arc", cfg.ARCMainnet.URL)
		assert.Empty(t, cfg.ARCMainnet.APIKey) // Not set
		assert.Empty(t, cfg.ARCTestnet.URL)    // Not set
	})

	t.Run("extra fields ignored", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configWithExtra := `
arc-mainnet:
  url: "https://api.taal.com/arc"
  extra_field: "should be ignored"
unknown_section:
  foo: "bar"
`
		err := os.WriteFile(configPath, []byte(configWithExtra), 0o600)
		require.NoError(t, err)

		cfg, err := LoadFromPath(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "https://api.taal.com/arc", cfg.ARCMainnet.URL)
	})
}

func TestGetARCConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		ARCMainnet: ARCConfig{
			URL:     "https://mainnet.example.com",
			APIKey:  "mainnet-key",
			Timeout: "30s",
		},
		ARCTestnet: ARCConfig{
			URL:     "https://testnet.example.com",
			APIKey:  "testnet-key",
			Timeout: "60s",
		},
	}

	t.Run("returns mainnet config when testnet is false", func(t *testing.T) {
		t.Parallel()
		result := cfg.GetARCConfig(false)
		assert.Equal(t, "https://mainnet.example.com", result.URL)
		assert.Equal(t, "mainnet-key", result.APIKey)
		assert.Equal(t, "30s", result.Timeout)
	})

	t.Run("returns testnet config when testnet is true", func(t *testing.T) {
		t.Parallel()
		result := cfg.GetARCConfig(true)
		assert.Equal(t, "https://testnet.example.com", result.URL)
		assert.Equal(t, "testnet-key", result.APIKey)
		assert.Equal(t, "60s", result.Timeout)
	})

	t.Run("returns empty config when not set", func(t *testing.T) {
		t.Parallel()
		emptyConfig := &Config{}
		result := emptyConfig.GetARCConfig(false)
		assert.Empty(t, result.URL)
		assert.Empty(t, result.APIKey)
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid mainnet config", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCMainnet: ARCConfig{URL: "https://api.taal.com/arc"},
		}
		err := cfg.Validate(false)
		require.NoError(t, err)
	})

	t.Run("valid testnet config", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCTestnet: ARCConfig{URL: "https://arc-test.taal.com/arc"},
		}
		err := cfg.Validate(true)
		require.NoError(t, err)
	})

	t.Run("missing mainnet URL", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCMainnet: ARCConfig{APIKey: "some-key"}, // URL missing
		}
		err := cfg.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ARC URL is required for mainnet")
	})

	t.Run("missing testnet URL", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCTestnet: ARCConfig{APIKey: "some-key"}, // URL missing
		}
		err := cfg.Validate(true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ARC URL is required for testnet")
	})

	t.Run("empty URL string", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCMainnet: ARCConfig{URL: ""},
		}
		err := cfg.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ARC URL is required")
	})

	t.Run("mainnet URL set but validating testnet", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCMainnet: ARCConfig{URL: "https://mainnet.example.com"},
			// ARCTestnet URL not set
		}
		err := cfg.Validate(true) // Validating for testnet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "testnet")
	})

	t.Run("both networks configured", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			ARCMainnet: ARCConfig{URL: "https://mainnet.example.com"},
			ARCTestnet: ARCConfig{URL: "https://testnet.example.com"},
		}

		err := cfg.Validate(false)
		require.NoError(t, err)

		err = cfg.Validate(true)
		require.NoError(t, err)
	})
}

func TestARCConfigStruct(t *testing.T) {
	t.Parallel()

	t.Run("yaml tags work correctly", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		// Test that YAML tags map correctly
		configContent := `
arc-mainnet:
  url: "test-url"
  api_key: "test-key"
  timeout: "45s"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		require.NoError(t, err)

		cfg, err := LoadFromPath(configPath)
		require.NoError(t, err)

		assert.Equal(t, "test-url", cfg.ARCMainnet.URL)
		assert.Equal(t, "test-key", cfg.ARCMainnet.APIKey)
		assert.Equal(t, "45s", cfg.ARCMainnet.Timeout)
	})
}

func TestPollingConfigStruct(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
polling:
  interval: "10s"
  max_retries: 5
  backoff_factor: 2.0
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	cfg, err := LoadFromPath(configPath)
	require.NoError(t, err)

	assert.Equal(t, "10s", cfg.Polling.Interval)
	assert.Equal(t, 5, cfg.Polling.MaxRetries)
	assert.InEpsilon(t, 2.0, cfg.Polling.BackoffFactor, 1e-9)
}

func TestTargetsConfigStruct(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
targets:
  default: "SEEN_ON_NETWORK"
  wait_for_mining: false
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	cfg, err := LoadFromPath(configPath)
	require.NoError(t, err)

	assert.Equal(t, "SEEN_ON_NETWORK", cfg.Targets.Default)
	assert.False(t, cfg.Targets.WaitForMining)
}

// Test Load() function which uses default paths
// This test modifies the working directory, so it's not parallelized
func TestLoad(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	t.Run("loads from current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := `
arc-mainnet:
  url: "https://current-dir.example.com"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		require.NoError(t, err)

		// Change to temp directory
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "https://current-dir.example.com", cfg.ARCMainnet.URL)
	})

	t.Run("returns error when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Change to empty temp directory (no config.yaml)
		err := os.Chdir(tmpDir)
		require.NoError(t, err)

		_, err = Load()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})
}

// Test LoadFromPath with empty path (should use default behavior)
func TestLoadFromPathEmptyPath(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
arc-mainnet:
  url: "https://empty-path-test.example.com"
`
	err = os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Change to temp directory
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Empty path should trigger default behavior
	cfg, err := LoadFromPath("")
	require.NoError(t, err)
	assert.Equal(t, "https://empty-path-test.example.com", cfg.ARCMainnet.URL)
}
