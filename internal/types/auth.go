package types

import "time"

// User represents an authenticated user.
type User struct {
	ID          string    `json:"id"`
	Subject     string    `json:"subject"` // OIDC sub claim
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
	IsActive    bool      `json:"is_active"`
	Roles       []Role    `json:"roles"`
}

// Role represents a user role.
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// OIDCConfig holds OIDC provider configuration.
// ClientSecret is excluded from JSON serialization to prevent accidental leakage.
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

// JWTConfig holds JWT session configuration.
// Secret is excluded from JSON serialization to prevent accidental leakage.
type JWTConfig struct {
	Secret         string        `json:"-"                yaml:"secret"`
	Expiry         time.Duration `json:"expiry"           yaml:"expiry"`
	CookieName     string        `json:"cookie_name"      yaml:"cookie_name"`
	CookieSecure   bool          `json:"cookie_secure"    yaml:"cookie_secure"`
	CookieHTTPOnly bool          `json:"cookie_http_only" yaml:"cookie_http_only"`
	CookieSameSite string        `json:"cookie_same_site" yaml:"cookie_same_site"`
}

// Session represents the claims stored in a session JWT.
type Session struct {
	JTI       string    `json:"jti"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionRecord represents a server-side session record for revocation support.
type SessionRecord struct {
	JTI       string     `json:"jti"`
	UserID    string     `json:"user_id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
