package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvVarAuditLogRetentionDays(t *testing.T) {
	os.Setenv("GSLB_AUDIT_LOG_RETENTION_DAYS", "90")
	defer os.Unsetenv("GSLB_AUDIT_LOG_RETENTION_DAYS")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 90, cfg.AuditLogRetentionDays)
}

func TestEnvVarAuditLogRetentionDays_Invalid(t *testing.T) {
	os.Setenv("GSLB_AUDIT_LOG_RETENTION_DAYS", "not-a-number")
	defer os.Unsetenv("GSLB_AUDIT_LOG_RETENTION_DAYS")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 30, cfg.AuditLogRetentionDays) // default unchanged
}

func TestEnvVarOIDCSettings(t *testing.T) {
	os.Setenv("GSLB_OIDC_ISSUER", "https://auth.example.com")
	os.Setenv("GSLB_OIDC_CLIENT_ID", "my-client-id")
	defer func() {
		os.Unsetenv("GSLB_OIDC_ISSUER")
		os.Unsetenv("GSLB_OIDC_CLIENT_ID")
	}()

	cfg, err := Load("")
	require.NoError(t, err)
	// OIDC is disabled by default, so just check the fields are set
	assert.Equal(t, "https://auth.example.com", cfg.OIDC.Issuer)
	assert.Equal(t, "my-client-id", cfg.OIDC.ClientID)
}

func TestEnvVarOIDCDisabled(t *testing.T) {
	os.Setenv("GSLB_OIDC_ENABLED", "false")
	defer os.Unsetenv("GSLB_OIDC_ENABLED")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.False(t, cfg.OIDC.Enabled)
}

func TestEnvVarJWTSecret(t *testing.T) {
	os.Setenv("GSLB_JWT_SECRET", "supersecretkey")
	defer os.Unsetenv("GSLB_JWT_SECRET")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "supersecretkey", cfg.JWT.Secret)
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
	}
	assert.NoError(t, Validate(cfg))
}

func TestValidate_MissingListenAddr(t *testing.T) {
	cfg := &Config{
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
	}
	err := Validate(cfg)
	assert.Error(t, err)
}

func TestDefaultConfig_AuditLogRetentionDays(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 30, cfg.AuditLogRetentionDays)
}

func TestEnvVarDatabasePath(t *testing.T) {
	os.Setenv("GSLB_DATABASE_PATH", "/tmp/test.db")
	defer os.Unsetenv("GSLB_DATABASE_PATH")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/test.db", cfg.DatabasePath)
}

func TestEnvVarListenAddr(t *testing.T) {
	os.Setenv("GSLB_LISTEN_ADDR", ":9999")
	defer os.Unsetenv("GSLB_LISTEN_ADDR")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, ":9999", cfg.ListenAddr)
}

func TestEnvVarHealthCheckWorkers(t *testing.T) {
	os.Setenv("GSLB_HEALTH_CHECK_WORKERS", "10")
	defer os.Unsetenv("GSLB_HEALTH_CHECK_WORKERS")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.HealthCheckWorkers)
}

func TestEnvVarHealthCheckWorkers_Invalid(t *testing.T) {
	os.Setenv("GSLB_HEALTH_CHECK_WORKERS", "not-a-number")
	defer os.Unsetenv("GSLB_HEALTH_CHECK_WORKERS")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.HealthCheckWorkers) // default unchanged
}

func TestEnvVarReconcileInterval(t *testing.T) {
	os.Setenv("GSLB_RECONCILE_INTERVAL", "2m")
	defer os.Unsetenv("GSLB_RECONCILE_INTERVAL")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 2*time.Minute, cfg.ReconcileInterval)
}

func TestEnvVarEncryptionKey(t *testing.T) {
	os.Setenv("GSLB_ENCRYPTION_KEY", "my-encryption-key")
	defer os.Unsetenv("GSLB_ENCRYPTION_KEY")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "my-encryption-key", cfg.EncryptionKey)
}

func TestEnvVarOIDCFullSettings(t *testing.T) {
	os.Setenv("GSLB_OIDC_CLIENT_SECRET", "my-secret")
	os.Setenv("GSLB_OIDC_REDIRECT_URL", "https://app.example.com/callback")
	os.Setenv("GSLB_OIDC_ROLES_CLAIM", "groups")
	os.Setenv("GSLB_OIDC_DEFAULT_ROLE", "viewer")
	defer func() {
		os.Unsetenv("GSLB_OIDC_CLIENT_SECRET")
		os.Unsetenv("GSLB_OIDC_REDIRECT_URL")
		os.Unsetenv("GSLB_OIDC_ROLES_CLAIM")
		os.Unsetenv("GSLB_OIDC_DEFAULT_ROLE")
	}()

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "my-secret", cfg.OIDC.ClientSecret)
	assert.Equal(t, "https://app.example.com/callback", cfg.OIDC.RedirectURL)
	assert.Equal(t, "groups", cfg.OIDC.RolesClaim)
	assert.Equal(t, "viewer", cfg.OIDC.DefaultRole)
}

func TestEnvVarJWTFullSettings(t *testing.T) {
	os.Setenv("GSLB_JWT_EXPIRY", "2h")
	os.Setenv("GSLB_JWT_COOKIE_SECURE", "true")
	os.Setenv("GSLB_JWT_COOKIE_NAME", "my_session")
	defer func() {
		os.Unsetenv("GSLB_JWT_EXPIRY")
		os.Unsetenv("GSLB_JWT_COOKIE_SECURE")
		os.Unsetenv("GSLB_JWT_COOKIE_NAME")
	}()

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 2*time.Hour, cfg.JWT.Expiry)
	assert.True(t, cfg.JWT.CookieSecure)
	assert.Equal(t, "my_session", cfg.JWT.CookieName)
}

func TestValidate_ZeroWorkers(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 0,
		ReconcileInterval:  30 * time.Second,
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health_check_workers")
}

func TestValidate_ZeroReconcileInterval(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  0,
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reconcile_interval")
}

func TestValidate_OIDCEnabled_ShortJWTSecret(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC:               OIDCConfig{Enabled: true},
		JWT:                JWTConfig{Secret: "short"},
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jwt.secret")
}

func TestValidate_OIDCEnabled_MissingIssuer(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC:               OIDCConfig{Enabled: true},
		JWT:                JWTConfig{Secret: "this-secret-is-long-enough-32ch!"},
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.issuer")
}

func TestValidate_OIDCEnabled_MissingClientID(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC:               OIDCConfig{Enabled: true, Issuer: "https://auth.example.com"},
		JWT:                JWTConfig{Secret: "this-secret-is-long-enough-32ch!"},
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.client_id")
}

func TestValidate_OIDCEnabled_MissingClientSecret(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC:               OIDCConfig{Enabled: true, Issuer: "https://auth.example.com", ClientID: "id"},
		JWT:                JWTConfig{Secret: "this-secret-is-long-enough-32ch!"},
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.client_secret")
}

func TestValidate_OIDCEnabled_MissingRedirectURL(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC:               OIDCConfig{Enabled: true, Issuer: "https://auth.example.com", ClientID: "id", ClientSecret: "sec"},
		JWT:                JWTConfig{Secret: "this-secret-is-long-enough-32ch!"},
	}
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.redirect_url")
}

func TestValidate_OIDCEnabled_AllValid(t *testing.T) {
	cfg := &Config{
		ListenAddr:         ":8080",
		HealthCheckWorkers: 5,
		ReconcileInterval:  30 * time.Second,
		OIDC: OIDCConfig{
			Enabled:      true,
			Issuer:       "https://auth.example.com",
			ClientID:     "id",
			ClientSecret: "sec",
			RedirectURL:  "https://app.example.com/callback",
		},
		JWT: JWTConfig{Secret: "this-secret-is-long-enough-32ch!"},
	}
	err := Validate(cfg)
	assert.NoError(t, err)
}
