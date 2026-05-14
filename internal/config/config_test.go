package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "gslb.db", cfg.DatabasePath)
	assert.Equal(t, ":8090", cfg.ListenAddr)
	assert.Equal(t, 10, cfg.HealthCheckWorkers)
	assert.Equal(t, 30*time.Second, cfg.ReconcileInterval)
}

func TestLoadFromYAMLFile(t *testing.T) {
	// Create a temporary YAML config file
	content := `
database_path: "/custom/db/path.db"
listen_addr: ":9090"
health_check_workers: 20
reconcile_interval: 60s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "/custom/db/path.db", cfg.DatabasePath)
	assert.Equal(t, ":9090", cfg.ListenAddr)
	assert.Equal(t, 20, cfg.HealthCheckWorkers)
	assert.Equal(t, 60*time.Second, cfg.ReconcileInterval)
}

func TestLoadFromJSONFile(t *testing.T) {
	// Create a temporary JSON config file
	content := `{
		"database_path": "/json/db/path.db",
		"listen_addr": ":3000",
		"health_check_workers": 5,
		"reconcile_interval": "120s"
	}`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "/json/db/path.db", cfg.DatabasePath)
	assert.Equal(t, ":3000", cfg.ListenAddr)
	assert.Equal(t, 5, cfg.HealthCheckWorkers)
	assert.Equal(t, 120*time.Second, cfg.ReconcileInterval)
}

func TestLoadMissingFile(t *testing.T) {
	// Should use defaults when file doesn't exist
	cfg, err := Load("/nonexistent/path/config.yaml")
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, "gslb.db", cfg.DatabasePath)
	assert.Equal(t, ":8090", cfg.ListenAddr)
	assert.Equal(t, 10, cfg.HealthCheckWorkers)
	assert.Equal(t, 30*time.Second, cfg.ReconcileInterval)
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, "gslb.db", cfg.DatabasePath)
	assert.Equal(t, ":8090", cfg.ListenAddr)
	assert.Equal(t, 10, cfg.HealthCheckWorkers)
	assert.Equal(t, 30*time.Second, cfg.ReconcileInterval)
}

func TestEnvVarOverrides(t *testing.T) {
	// Create a temporary YAML config file
	content := `
database_path: "/file/db/path.db"
listen_addr: ":8888"
health_check_workers: 25
reconcile_interval: 45s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set environment variables
	os.Setenv("GSLB_DATABASE_PATH", "/env/db/path.db")
	os.Setenv("GSLB_LISTEN_ADDR", ":9999")
	os.Setenv("GSLB_HEALTH_CHECK_WORKERS", "30")
	os.Setenv("GSLB_RECONCILE_INTERVAL", "90s")
	defer func() {
		os.Unsetenv("GSLB_DATABASE_PATH")
		os.Unsetenv("GSLB_LISTEN_ADDR")
		os.Unsetenv("GSLB_HEALTH_CHECK_WORKERS")
		os.Unsetenv("GSLB_RECONCILE_INTERVAL")
	}()

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Environment variables should override file values
	assert.Equal(t, "/env/db/path.db", cfg.DatabasePath)
	assert.Equal(t, ":9999", cfg.ListenAddr)
	assert.Equal(t, 30, cfg.HealthCheckWorkers)
	assert.Equal(t, 90*time.Second, cfg.ReconcileInterval)
}

func TestEnvVarPartialOverrides(t *testing.T) {
	// Only set some environment variables
	os.Setenv("GSLB_LISTEN_ADDR", ":7777")
	defer os.Unsetenv("GSLB_LISTEN_ADDR")

	content := `
database_path: "/file/db/path.db"
listen_addr: ":8888"
health_check_workers: 25
reconcile_interval: 45s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Only listen_addr should be overridden
	assert.Equal(t, "/file/db/path.db", cfg.DatabasePath)
	assert.Equal(t, ":7777", cfg.ListenAddr)
	assert.Equal(t, 25, cfg.HealthCheckWorkers)
	assert.Equal(t, 45*time.Second, cfg.ReconcileInterval)
}

func TestValidateListenAddrRequired(t *testing.T) {
	cfg := &Config{
		ListenAddr:         "",
		HealthCheckWorkers: 10,
		ReconcileInterval:  30 * time.Second,
	}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listen_addr")
}

func TestValidateWorkersMustBePositive(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8090",
		HealthCheckWorkers: 0,
		ReconcileInterval:  30 * time.Second,
	}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health_check_workers")
}

func TestValidateIntervalMustBePositive(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8090",
		HealthCheckWorkers: 10,
		ReconcileInterval:  0,
	}

	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reconcile_interval")
}

func TestValidateSuccess(t *testing.T) {
	cfg := &Config{
		DatabasePath:       "test.db",
		ListenAddr:         ":8090",
		HealthCheckWorkers: 10,
		ReconcileInterval:  30 * time.Second,
	}

	err := Validate(cfg)
	assert.NoError(t, err)
}

func TestLoadWithEnvVarInvalidWorkers(t *testing.T) {
	content := `
health_check_workers: 10
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set invalid workers env var
	os.Setenv("GSLB_HEALTH_CHECK_WORKERS", "invalid")
	defer os.Unsetenv("GSLB_HEALTH_CHECK_WORKERS")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Invalid env var should be ignored, keeping file value
	assert.Equal(t, 10, cfg.HealthCheckWorkers)
}

func TestLoadWithEnvVarInvalidInterval(t *testing.T) {
	content := `
reconcile_interval: 30s
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set invalid interval env var
	os.Setenv("GSLB_RECONCILE_INTERVAL", "invalid")
	defer os.Unsetenv("GSLB_RECONCILE_INTERVAL")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Invalid env var should be ignored, keeping file value
	assert.Equal(t, 30*time.Second, cfg.ReconcileInterval)
}

func TestLoadInvalidYAML(t *testing.T) {
	content := `
invalid yaml content
  - this is not valid
    because indentation is wrong
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}

func TestLoadInvalidJSON(t *testing.T) {
	content := `{ "database_path": @invalid }`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}
