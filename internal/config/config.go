// Package config provides application configuration management.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents application configuration.
type Config struct {
	DatabasePath            string        `json:"database_path"              yaml:"database_path"`
	ListenAddr              string        `json:"listen_addr"                yaml:"listen_addr"`
	HealthCheckWorkers      int           `json:"health_check_workers"       yaml:"health_check_workers"`
	ReconcileInterval       time.Duration `json:"reconcile_interval"         yaml:"reconcile_interval"`
	AuditLogRetentionDays   int           `json:"audit_log_retention_days"   yaml:"audit_log_retention_days"`
	EncryptionKey           string        `json:"-"                          yaml:"encryption_key"`
	OIDC                    OIDCConfig    `json:"oidc"                       yaml:"oidc"`
	JWT                     JWTConfig     `json:"jwt"                        yaml:"jwt"`
}

// OIDCConfig holds OIDC provider settings.
type OIDCConfig struct {
	Enabled         bool     `json:"-"            yaml:"enabled"`
	Issuer          string   `json:"issuer"       yaml:"issuer"`
	ClientID        string   `json:"client_id"    yaml:"client_id"`
	ClientSecret    string   `json:"-"            yaml:"client_secret"`
	RedirectURL     string   `json:"redirect_url" yaml:"redirect_url"`
	Scopes          []string `json:"scopes"       yaml:"scopes"`
	RolesClaim      string   `json:"roles_claim"  yaml:"roles_claim"`
	DefaultRole     string   `json:"default_role" yaml:"default_role"`
}

// JWTConfig holds JWT/session cookie settings.
type JWTConfig struct {
	Secret         string        `json:"-"                yaml:"secret"`
	Expiry         time.Duration `json:"expiry"           yaml:"expiry"`
	CookieName     string        `json:"cookie_name"      yaml:"cookie_name"`
	CookieSecure   bool          `json:"cookie_secure"    yaml:"cookie_secure"`
	CookieHTTPOnly bool          `json:"cookie_http_only" yaml:"cookie_http_only"`
	CookieSameSite string        `json:"cookie_same_site" yaml:"cookie_same_site"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		DatabasePath:          "gslb.db",
		ListenAddr:            ":8090",
		HealthCheckWorkers:    10,
		ReconcileInterval:     30 * time.Second,
		AuditLogRetentionDays: 30,
		OIDC: OIDCConfig{
			Scopes:      []string{"openid", "profile", "email"},
			RolesClaim:  "roles",
			DefaultRole: "viewer",
		},
		JWT: JWTConfig{
			Expiry:         24 * time.Hour,
			CookieName:     "gslb_session",
			CookieSecure:   false, // set to true when serving behind HTTPS reverse proxy
			CookieHTTPOnly: true,
			CookieSameSite: "Lax",
		},
	}
}

// Load loads configuration from file and environment variables.
// Supports both YAML and JSON formats based on file extension.
// Environment variables override file values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from file if it exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Use defaults if file doesn't exist
			} else {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		} else {
			// Determine format from extension
			ext := strings.ToLower(path)
			if strings.HasSuffix(ext, ".yaml") || strings.HasSuffix(ext, ".yml") {
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("parsing YAML config: %w", err)
				}
			} else if strings.HasSuffix(ext, ".json") {
				// yaml.Unmarshal handles JSON as well since JSON is valid YAML
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("parsing JSON config: %w", err)
				}
			} else {
				// Try YAML by default
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("parsing config file: %w", err)
				}
			}
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Validate configuration
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GSLB_DATABASE_PATH"); v != "" {
		cfg.DatabasePath = v
	}
	if v := os.Getenv("GSLB_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("GSLB_HEALTH_CHECK_WORKERS"); v != "" {
		if workers, err := parseInt(v); err == nil && workers > 0 {
			cfg.HealthCheckWorkers = workers
		}
	}
	if v := os.Getenv("GSLB_RECONCILE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.ReconcileInterval = d
		}
	}
	if v := os.Getenv("GSLB_AUDIT_LOG_RETENTION_DAYS"); v != "" {
		if days, err := parseInt(v); err == nil && days > 0 {
			cfg.AuditLogRetentionDays = days
		}
	}

	if v := os.Getenv("GSLB_ENCRYPTION_KEY"); v != "" {
		cfg.EncryptionKey = v
	}

	// OIDC overrides
	if v := os.Getenv("GSLB_OIDC_ENABLED"); v != "" {
		cfg.OIDC.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("GSLB_OIDC_ISSUER"); v != "" {
		cfg.OIDC.Issuer = v
	}
	if v := os.Getenv("GSLB_OIDC_CLIENT_ID"); v != "" {
		cfg.OIDC.ClientID = v
	}
	if v := os.Getenv("GSLB_OIDC_CLIENT_SECRET"); v != "" {
		cfg.OIDC.ClientSecret = v
	}
	if v := os.Getenv("GSLB_OIDC_REDIRECT_URL"); v != "" {
		cfg.OIDC.RedirectURL = v
	}
	if v := os.Getenv("GSLB_OIDC_ROLES_CLAIM"); v != "" {
		cfg.OIDC.RolesClaim = v
	}
	if v := os.Getenv("GSLB_OIDC_DEFAULT_ROLE"); v != "" {
		cfg.OIDC.DefaultRole = v
	}

	// JWT overrides
	if v := os.Getenv("GSLB_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("GSLB_JWT_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.JWT.Expiry = d
		}
	}
	if v := os.Getenv("GSLB_JWT_COOKIE_SECURE"); v != "" {
		cfg.JWT.CookieSecure = v == "true" || v == "1"
	}
	if v := os.Getenv("GSLB_JWT_COOKIE_NAME"); v != "" {
		cfg.JWT.CookieName = v
	}
}

// parseInt parses a string to int.
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// Validate validates the configuration.
func Validate(cfg *Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("listen_addr is required")
	}
	// Basic validation: listen addr should contain a colon or be a valid host:port
	if !strings.Contains(cfg.ListenAddr, ":") {
		// Could be just a port, which is valid
		if cfg.ListenAddr != "" {
			// Allow numeric ports like ":8080" or "8080"
		}
	}
	if cfg.HealthCheckWorkers <= 0 {
		return fmt.Errorf("health_check_workers must be greater than 0")
	}
	if cfg.ReconcileInterval <= 0 {
		return fmt.Errorf("reconcile_interval must be greater than 0")
	}

	// Validate OIDC-related config when enabled
	if cfg.OIDC.Enabled {
		if len(cfg.JWT.Secret) < 32 {
			return fmt.Errorf("jwt.secret must be at least 32 bytes when OIDC is enabled")
		}
		if cfg.OIDC.Issuer == "" {
			return fmt.Errorf("oidc.issuer is required when OIDC is enabled")
		}
		if cfg.OIDC.ClientID == "" {
			return fmt.Errorf("oidc.client_id is required when OIDC is enabled")
		}
		if cfg.OIDC.ClientSecret == "" {
			return fmt.Errorf("oidc.client_secret is required when OIDC is enabled")
		}
		if cfg.OIDC.RedirectURL == "" {
			return fmt.Errorf("oidc.redirect_url is required when OIDC is enabled")
		}
	}

	return nil
}
