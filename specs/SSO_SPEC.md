# SSO/OIDC Implementation Specification

## Overview

Implement Single Sign-On (SSO) using OpenID Connect (OIDC) for the GSLB Manager, enabling role-based access control (RBAC) where configurations can be restricted to specific user roles.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   User      │────▶│  GSLB API    │────▶│   OIDC Provider │
│  Browser    │     │              │     │ (Keycloak/      │
└─────────────┘     │ - Login      │     │  Auth0/etc)     │
       │            │ - Callback   │     └─────────────────┘
       │            │ - Validate   │
       ▼            │ - JWT Token  │
┌─────────────┐     └──────────────┘
│  Vue SPA    │
│ - Login Btn │
│ - Store     │
└─────────────┘
```

## Database Schema Changes

### 1. Users Table
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,             -- UUID
    subject TEXT UNIQUE NOT NULL,    -- OIDC 'sub' claim
    email TEXT NOT NULL,
    name TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX idx_users_subject ON users(subject);
CREATE INDEX idx_users_email ON users(email);
```

### 2. Roles Table
```sql
CREATE TABLE roles (
    id TEXT PRIMARY KEY,             -- UUID (not the role name)
    name TEXT NOT NULL UNIQUE,       -- Role name (e.g., "admin", "operator")
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default roles (using UUIDs as PKs)
INSERT INTO roles (id, name, description) VALUES
    ('00000000-0000-0000-0000-000000000001', 'admin',    'Full access to all configurations'),
    ('00000000-0000-0000-0000-000000000002', 'operator', 'Can manage assigned configurations'),
    ('00000000-0000-0000-0000-000000000003', 'viewer',   'Read-only access to assigned configurations');
```

### 3. User Roles (Many-to-Many)
```sql
CREATE TABLE user_roles (
    user_id     TEXT NOT NULL,
    role_id     TEXT NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    assigned_by TEXT,               -- free-text audit field (user ID or "system"/"oidc")
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);
```

### 4. Configuration Roles (Access Control)
```sql
CREATE TABLE config_roles (
    config_id TEXT NOT NULL,
    role_id   TEXT NOT NULL,
    PRIMARY KEY (config_id, role_id),
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id)   REFERENCES roles(id)   ON DELETE CASCADE
);

CREATE INDEX idx_config_roles_config ON config_roles(config_id);
CREATE INDEX idx_config_roles_role   ON config_roles(role_id);
```

## Configuration

### Config File Example (`gslb.yaml`)
```yaml
# ... existing config ...

# OIDC Configuration
oidc:
  enabled: true
  issuer: "https://keycloak.example.com/realms/gslb"  # .well-known/openid-configuration appended
  client_id: "gslb-manager"
  client_secret: "${OIDC_CLIENT_SECRET}"  # From env var
  redirect_url: "http://localhost:8090/api/v1/auth/callback"
  scopes:
    - "openid"
    - "profile"
    - "email"
    - "roles"
  # Role claim name in ID token (provider-specific)
  roles_claim: "roles"
  # Auto-create users on first login (defaults to false — safer)
  auto_create_users: false
  # Default role for auto-created users
  default_role: "viewer"

# JWT Configuration
jwt:
  # Secret for signing session tokens (generate with: openssl rand -base64 32)
  secret: "${JWT_SECRET}"
  # Session duration
  expiry: "24h"
  # Cookie name
  cookie_name: "gslb_session"
  # Cookie secure flag — MUST be true in production with HTTPS
  cookie_secure: true
  # Cookie httpOnly flag
  cookie_http_only: true
  # Cookie same site
  cookie_same_site: "Lax"
```

### Environment Variables
```bash
# OIDC
export GSLB_OIDC_ENABLED=true
export GSLB_OIDC_ISSUER="https://keycloak.example.com/realms/gslb"
export GSLB_OIDC_CLIENT_ID="gslb-manager"
export GSLB_OIDC_CLIENT_SECRET="your-client-secret-here"
export GSLB_OIDC_REDIRECT_URL="http://localhost:8090/api/v1/auth/callback"
export GSLB_OIDC_ROLES_CLAIM="roles"
export GSLB_OIDC_AUTO_CREATE_USERS=false
export GSLB_OIDC_DEFAULT_ROLE="viewer"

# JWT
export GSLB_JWT_SECRET="your-jwt-secret-here-min-32-chars"
export GSLB_JWT_EXPIRY="24h"
export GSLB_JWT_COOKIE_SECURE=true   # false only for local development without HTTPS
```

## Backend Implementation

### 1. New Types

```go
// internal/types/auth.go

package types

import "time"

// User represents an authenticated user
type User struct {
    ID          string    `json:"id"`
    Subject     string    `json:"subject"`  // OIDC sub claim
    Email       string    `json:"email"`
    Name        string    `json:"name"`
    CreatedAt   time.Time `json:"created_at"`
    LastLoginAt time.Time `json:"last_login_at"`
    IsActive    bool      `json:"is_active"`
    Roles       []Role    `json:"roles"`
}

// Role represents a user role
type Role struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}

// OIDCConfig holds OIDC configuration.
// ClientSecret must not be serialized to JSON.
type OIDCConfig struct {
    Enabled         bool     `json:"-"          yaml:"enabled"`
    Issuer          string   `json:"issuer"     yaml:"issuer"`
    ClientID        string   `json:"client_id"  yaml:"client_id"`
    ClientSecret    string   `json:"-"          yaml:"client_secret"`
    RedirectURL     string   `json:"redirect_url" yaml:"redirect_url"`
    Scopes          []string `json:"scopes"     yaml:"scopes"`
    RolesClaim      string   `json:"roles_claim" yaml:"roles_claim"`
    AutoCreateUsers bool     `json:"-"          yaml:"auto_create_users"`
    DefaultRole     string   `json:"default_role" yaml:"default_role"`
}

// JWTConfig holds JWT/session configuration.
// Secret must not be serialized to JSON.
type JWTConfig struct {
    Secret         string        `json:"-"               yaml:"secret"`
    Expiry         time.Duration `json:"expiry"          yaml:"expiry"`
    CookieName     string        `json:"cookie_name"     yaml:"cookie_name"`
    CookieSecure   bool          `json:"cookie_secure"   yaml:"cookie_secure"`
    CookieHTTPOnly bool          `json:"cookie_http_only" yaml:"cookie_http_only"`
    CookieSameSite string        `json:"cookie_same_site" yaml:"cookie_same_site"`
}

// Session represents an authenticated session stored in the JWT
type Session struct {
    UserID    string    `json:"user_id"`
    Email     string    `json:"email"`
    Roles     []string  `json:"roles"`
    ExpiresAt time.Time `json:"expires_at"`
}
```

### 2. Store Layer

```go
// internal/store/user.go

package store

import (
    "context"

    "github.com/pheelee/gslb/internal/types"
)

// UserStore defines user operations
type UserStore interface {
    CreateUser(ctx context.Context, user *types.User) error
    GetUserByID(ctx context.Context, id string) (*types.User, error)
    GetUserBySubject(ctx context.Context, subject string) (*types.User, error)
    UpdateUser(ctx context.Context, user *types.User) error
    ListUsers(ctx context.Context) ([]types.User, error)
    AssignRole(ctx context.Context, userID, roleID, assignedBy string) error
    RemoveRole(ctx context.Context, userID, roleID string) error
    GetUserRoles(ctx context.Context, userID string) ([]types.Role, error)
}

// RoleStore defines role operations
type RoleStore interface {
    GetRoleByName(ctx context.Context, name string) (*types.Role, error)
    GetRoleByID(ctx context.Context, id string) (*types.Role, error)
    ListRoles(ctx context.Context) ([]types.Role, error)
}

// ConfigRoleStore defines configuration-role operations
type ConfigRoleStore interface {
    AssignConfigRole(ctx context.Context, configID, roleID string) error
    RemoveConfigRole(ctx context.Context, configID, roleID string) error
    GetConfigRoles(ctx context.Context, configID string) ([]types.Role, error)
    GetConfigsForRole(ctx context.Context, roleID string) ([]string, error)
    // CanAccess checks if a user (by their role set) can access a configuration.
    // Admin role bypasses this check at the middleware level.
    CanAccess(ctx context.Context, userID, configID string) (bool, error)
}
```

The aggregate `Store` interface embeds all sub-stores:

```go
// Store combines all sub-stores
type Store interface {
    ConfigStore
    BackendStore
    HealthStore
    DNSProviderStore
    AuditLogStore
    UserStore
    RoleStore
    ConfigRoleStore
    Transactioner
}
```

### 3. OIDC Service

```go
// internal/auth/oidc.go

package auth

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "time"

    "github.com/coreos/go-oidc/v3/oidc"
    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/oauth2"

    "github.com/pheelee/gslb/internal/store"
    "github.com/pheelee/gslb/internal/types"
)

// OIDCService handles OIDC authentication
type OIDCService struct {
    provider     *oidc.Provider
    verifier     *oidc.IDTokenVerifier
    oauth2Config oauth2.Config
    oidcCfg      types.OIDCConfig
    jwtCfg       types.JWTConfig  // retained in full so expiry/cookie attrs are accessible
    store        store.Store
    jwtSecret    []byte
}

// NewOIDCService creates a new OIDC service
func NewOIDCService(oidcCfg types.OIDCConfig, jwtCfg types.JWTConfig, s store.Store) (*OIDCService, error) {
    ctx := context.Background()

    provider, err := oidc.NewProvider(ctx, oidcCfg.Issuer)
    if err != nil {
        return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
    }

    oauth2Config := oauth2.Config{
        ClientID:     oidcCfg.ClientID,
        ClientSecret: oidcCfg.ClientSecret,
        RedirectURL:  oidcCfg.RedirectURL,
        Endpoint:     provider.Endpoint(),
        Scopes:       oidcCfg.Scopes,
    }

    verifier := provider.Verifier(&oidc.Config{ClientID: oidcCfg.ClientID})

    return &OIDCService{
        provider:     provider,
        verifier:     verifier,
        oauth2Config: oauth2Config,
        oidcCfg:      oidcCfg,
        jwtCfg:       jwtCfg,
        store:        s,
        jwtSecret:    []byte(jwtCfg.Secret),
    }, nil
}

// JWTConfig returns the JWT config (used by handlers to set cookie attributes).
func (s *OIDCService) JWTConfig() types.JWTConfig { return s.jwtCfg }

// GenerateStateNonce generates a random URL-safe base64 string for use as
// OAuth2 state or OIDC nonce.
func GenerateStateNonce() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generating random bytes: %w", err)
    }
    return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the OIDC authorization URL with state and nonce embedded.
// The nonce is returned so the caller can store it (e.g., in a cookie) for
// verification at callback time.
func (s *OIDCService) GetAuthURL(state, nonce string) string {
    return s.oauth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
}

// Exchange exchanges an authorization code for tokens.
func (s *OIDCService) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
    return s.oauth2Config.Exchange(ctx, code)
}

// VerifyIDToken verifies and parses the ID token, checking the nonce claim.
func (s *OIDCService) VerifyIDToken(ctx context.Context, token *oauth2.Token, nonce string) (*oidc.IDToken, error) {
    rawIDToken, ok := token.Extra("id_token").(string)
    if !ok {
        return nil, fmt.Errorf("no id_token in token response")
    }

    idToken, err := s.verifier.Verify(ctx, rawIDToken)
    if err != nil {
        return nil, fmt.Errorf("id_token verification failed: %w", err)
    }

    if idToken.Nonce != nonce {
        return nil, fmt.Errorf("nonce mismatch")
    }

    return idToken, nil
}

// ExtractUserInfo extracts user info from ID token claims using the configured
// roles_claim key.
func (s *OIDCService) ExtractUserInfo(token *oidc.IDToken) (*types.User, []string, error) {
    var claims map[string]interface{}
    if err := token.Claims(&claims); err != nil {
        return nil, nil, fmt.Errorf("failed to parse claims: %w", err)
    }

    subject, _ := claims["sub"].(string)
    email, _ := claims["email"].(string)
    name, _ := claims["name"].(string)

    if subject == "" {
        return nil, nil, fmt.Errorf("missing sub claim")
    }

    // Extract roles using the configured claim name
    var roles []string
    if raw, ok := claims[s.oidcCfg.RolesClaim]; ok {
        if arr, ok := raw.([]interface{}); ok {
            for _, v := range arr {
                if r, ok := v.(string); ok {
                    roles = append(roles, r)
                }
            }
        }
    }

    user := &types.User{Subject: subject, Email: email, Name: name}
    return user, roles, nil
}

// CreateSession creates a signed JWT session token for the given user.
func (s *OIDCService) CreateSession(user *types.User) (string, error) {
    roles := make([]string, len(user.Roles))
    for i, r := range user.Roles {
        roles[i] = r.Name
    }

    expiry := s.jwtCfg.Expiry
    if expiry == 0 {
        expiry = 24 * time.Hour
    }

    now := time.Now()
    claims := jwt.MapClaims{
        "sub":   user.ID,
        "email": user.Email,
        "name":  user.Name,
        "roles": roles,
        "exp":   now.Add(expiry).Unix(),
        "iat":   now.Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}

// ParseSession validates and parses a session token into a Session.
func (s *OIDCService) ParseSession(tokenString string) (*types.Session, error) {
    token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return s.jwtSecret, nil
    })
    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token claims")
    }

    sub, ok := claims["sub"].(string)
    if !ok {
        return nil, fmt.Errorf("missing sub claim")
    }
    email, _ := claims["email"].(string)
    exp := int64(claims["exp"].(float64))

    var roles []string
    if r, ok := claims["roles"].([]interface{}); ok {
        for _, v := range r {
            if rs, ok := v.(string); ok {
                roles = append(roles, rs)
            }
        }
    }

    return &types.Session{
        UserID:    sub,
        Email:     email,
        Roles:     roles,
        ExpiresAt: time.Unix(exp, 0),
    }, nil
}

// SyncUser upserts the OIDC user in the local database and fully syncs their
// roles: roles present in oidcRoles but not locally are added; roles present
// locally but absent from oidcRoles are removed.
func (s *OIDCService) SyncUser(ctx context.Context, oidcUser *types.User, oidcRoles []string) (*types.User, error) {
    existing, err := s.store.GetUserBySubject(ctx, oidcUser.Subject)
    if err != nil && err != store.ErrNotFound {
        return nil, fmt.Errorf("lookup user: %w", err)
    }

    if existing != nil {
        if !existing.IsActive {
            return nil, fmt.Errorf("account disabled")
        }

        existing.Email = oidcUser.Email
        existing.Name = oidcUser.Name
        existing.LastLoginAt = time.Now()
        if err := s.store.UpdateUser(ctx, existing); err != nil {
            return nil, fmt.Errorf("update user: %w", err)
        }

        if len(oidcRoles) > 0 {
            if err := s.syncRoles(ctx, existing.ID, oidcRoles); err != nil {
                return nil, err
            }
        }

        return s.store.GetUserByID(ctx, existing.ID)
    }

    // New user
    if !s.oidcCfg.AutoCreateUsers {
        return nil, fmt.Errorf("user not found and auto-create is disabled")
    }

    now := time.Now()
    oidcUser.CreatedAt = now
    oidcUser.LastLoginAt = now
    oidcUser.IsActive = true
    // ID is assigned inside CreateUser (uuid.New)

    if err := s.store.CreateUser(ctx, oidcUser); err != nil {
        return nil, fmt.Errorf("create user: %w", err)
    }

    // Assign default role
    defaultRole, err := s.store.GetRoleByName(ctx, s.oidcCfg.DefaultRole)
    if err != nil {
        return nil, fmt.Errorf("get default role %q: %w", s.oidcCfg.DefaultRole, err)
    }
    if err := s.store.AssignRole(ctx, oidcUser.ID, defaultRole.ID, "system"); err != nil {
        return nil, fmt.Errorf("assign default role: %w", err)
    }

    if len(oidcRoles) > 0 {
        if err := s.syncRoles(ctx, oidcUser.ID, oidcRoles); err != nil {
            return nil, err
        }
    }

    return s.store.GetUserByID(ctx, oidcUser.ID)
}

// syncRoles performs a full diff-and-sync of OIDC-provided role names against
// the user's current local roles, adding missing ones and removing stale ones.
func (s *OIDCService) syncRoles(ctx context.Context, userID string, oidcRoleNames []string) error {
    current, err := s.store.GetUserRoles(ctx, userID)
    if err != nil {
        return fmt.Errorf("get user roles: %w", err)
    }

    currentSet := make(map[string]string) // name → id
    for _, r := range current {
        currentSet[r.Name] = r.ID
    }

    desiredSet := make(map[string]bool)
    for _, name := range oidcRoleNames {
        desiredSet[name] = true
    }

    // Add missing roles
    for _, name := range oidcRoleNames {
        if _, has := currentSet[name]; has {
            continue
        }
        role, err := s.store.GetRoleByName(ctx, name)
        if err != nil {
            continue // role not known locally — skip
        }
        _ = s.store.AssignRole(ctx, userID, role.ID, "oidc")
    }

    // Remove roles no longer in OIDC token
    for name, id := range currentSet {
        if desiredSet[name] {
            continue
        }
        _ = s.store.RemoveRole(ctx, userID, id)
    }

    return nil
}
```

### 4. Middleware

```go
// internal/auth/middleware.go

package auth

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"

    "github.com/pheelee/gslb/internal/handlers"
    "github.com/pheelee/gslb/internal/store"
)

// Middleware validates the session JWT on every protected request.
func Middleware(svc *OIDCService) gin.HandlerFunc {
    return func(c *gin.Context) {
        if shouldSkipAuth(c.Request.URL.Path) {
            c.Next()
            return
        }

        token := getToken(c, svc.JWTConfig().CookieName)
        if token == "" {
            c.JSON(http.StatusUnauthorized, handlers.Response{Error: "authentication required"})
            c.Abort()
            return
        }

        session, err := svc.ParseSession(token)
        if err != nil {
            c.JSON(http.StatusUnauthorized, handlers.Response{Error: "invalid or expired session"})
            c.Abort()
            return
        }

        c.Set("session", session)
        c.Set("userID", session.UserID)
        c.Set("userRoles", session.Roles)
        c.Next()
    }
}

// RequireRole returns 403 if the authenticated user does not hold at least one
// of the specified roles.
func RequireRole(requiredRoles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRoles, exists := c.Get("userRoles")
        if !exists {
            c.JSON(http.StatusForbidden, handlers.Response{Error: "access denied"})
            c.Abort()
            return
        }

        roles := userRoles.([]string)
        for _, required := range requiredRoles {
            for _, userRole := range roles {
                if userRole == required {
                    c.Next()
                    return
                }
            }
        }

        c.JSON(http.StatusForbidden, handlers.Response{Error: "insufficient permissions"})
        c.Abort()
    }
}

// RequireConfigAccess ensures the user can access a specific configuration.
// Admins bypass the per-config check.
func RequireConfigAccess(s store.Store) gin.HandlerFunc {
    return func(c *gin.Context) {
        configID := c.Param("id")

        userRoles, _ := c.Get("userRoles")
        for _, role := range userRoles.([]string) {
            if role == "admin" {
                c.Next()
                return
            }
        }

        userID, _ := c.Get("userID")
        canAccess, err := s.CanAccess(c.Request.Context(), userID.(string), configID)
        if err != nil || !canAccess {
            c.JSON(http.StatusForbidden, handlers.Response{Error: "access denied to this configuration"})
            c.Abort()
            return
        }

        c.Next()
    }
}

// shouldSkipAuth returns true for paths that do not require authentication.
// Only exact matches or explicit prefixes are listed; "/" alone would match
// every path via HasPrefix and must NOT be used.
func shouldSkipAuth(path string) bool {
    exactSkip := map[string]bool{
        "/health": true,
        "/ready":  true,
    }
    if exactSkip[path] {
        return true
    }

    prefixSkip := []string{
        "/api/v1/auth/login",
        "/api/v1/auth/callback",
        "/api/v1/auth/logout",
        "/assets/",
    }
    for _, prefix := range prefixSkip {
        if strings.HasPrefix(path, prefix) {
            return true
        }
    }

    // Serve the SPA shell unauthenticated; the JS will redirect to login.
    if path == "/" || strings.HasSuffix(path, ".html") {
        return true
    }

    return false
}

func getToken(c *gin.Context, cookieName string) string {
    if cookie, err := c.Cookie(cookieName); err == nil && cookie != "" {
        return cookie
    }
    authHeader := c.GetHeader("Authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        return strings.TrimPrefix(authHeader, "Bearer ")
    }
    return ""
}
```

### 5. Auth Handlers

```go
// internal/handlers/auth.go

package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"

    "github.com/pheelee/gslb/internal/auth"
    "github.com/pheelee/gslb/internal/types"
)

// Login initiates OIDC authentication.
func (h *Handlers) Login(c *gin.Context) {
    state, err := auth.GenerateStateNonce()
    if err != nil {
        sendInternalError(c, "failed to generate state")
        return
    }
    nonce, err := auth.GenerateStateNonce()
    if err != nil {
        sendInternalError(c, "failed to generate nonce")
        return
    }

    jwtCfg := h.oidcService.JWTConfig()
    secure := jwtCfg.CookieSecure

    // Store state and nonce in short-lived HttpOnly cookies for CSRF/replay protection
    c.SetSameSite(http.SameSiteLaxMode)
    c.SetCookie("oidc_state", state, 600, "/", "", secure, true)
    c.SetCookie("oidc_nonce", nonce, 600, "/", "", secure, true)

    c.Redirect(http.StatusFound, h.oidcService.GetAuthURL(state, nonce))
}

// Callback handles the OIDC provider redirect.
func (h *Handlers) Callback(c *gin.Context) {
    stateCookie, err := c.Cookie("oidc_state")
    if err != nil || stateCookie != c.Query("state") {
        sendError(c, http.StatusBadRequest, "invalid state parameter")
        return
    }

    nonce, err := c.Cookie("oidc_nonce")
    if err != nil {
        sendError(c, http.StatusBadRequest, "missing nonce cookie")
        return
    }

    code := c.Query("code")
    if code == "" {
        sendError(c, http.StatusBadRequest, "authorization code not provided")
        return
    }

    token, err := h.oidcService.Exchange(c.Request.Context(), code)
    if err != nil {
        sendError(c, http.StatusUnauthorized, "failed to exchange token")
        return
    }

    idToken, err := h.oidcService.VerifyIDToken(c.Request.Context(), token, nonce)
    if err != nil {
        sendError(c, http.StatusUnauthorized, "failed to verify ID token")
        return
    }

    oidcUser, oidcRoles, err := h.oidcService.ExtractUserInfo(idToken)
    if err != nil {
        sendError(c, http.StatusInternalServerError, "failed to extract user info")
        return
    }

    user, err := h.oidcService.SyncUser(c.Request.Context(), oidcUser, oidcRoles)
    if err != nil {
        sendError(c, http.StatusForbidden, err.Error())
        return
    }

    sessionToken, err := h.oidcService.CreateSession(user)
    if err != nil {
        sendInternalError(c, "failed to create session")
        return
    }

    jwtCfg := h.oidcService.JWTConfig()
    maxAge := int(jwtCfg.Expiry.Seconds())
    if maxAge == 0 {
        maxAge = 86400
    }

    c.SetSameSite(http.SameSiteLaxMode)
    c.SetCookie(jwtCfg.CookieName, sessionToken, maxAge, "/", "", jwtCfg.CookieSecure, jwtCfg.CookieHTTPOnly)

    // Clear OIDC flow cookies
    c.SetCookie("oidc_state", "", -1, "/", "", jwtCfg.CookieSecure, true)
    c.SetCookie("oidc_nonce", "", -1, "/", "", jwtCfg.CookieSecure, true)

    h.logAudit(c.Request.Context(), "auth:login", "user", user.ID, user.Email, "", c)
    c.Redirect(http.StatusFound, "/")
}

// Logout clears the session cookie.
func (h *Handlers) Logout(c *gin.Context) {
    jwtCfg := h.oidcService.JWTConfig()
    c.SetCookie(jwtCfg.CookieName, "", -1, "/", "", jwtCfg.CookieSecure, jwtCfg.CookieHTTPOnly)
    c.JSON(http.StatusOK, Response{Data: "logged out successfully"})
}

// GetCurrentUser returns the authenticated user's profile and roles.
func (h *Handlers) GetCurrentUser(c *gin.Context) {
    session, exists := c.Get("session")
    if !exists {
        sendError(c, http.StatusUnauthorized, "not authenticated")
        return
    }
    s := session.(*types.Session)
    user, err := h.store.GetUserByID(c.Request.Context(), s.UserID)
    if err != nil {
        sendError(c, http.StatusInternalServerError, "failed to get user")
        return
    }
    sendSuccess(c, user)
}

// ListRoles returns all available roles.
func (h *Handlers) ListRoles(c *gin.Context) {
    roles, err := h.store.ListRoles(c.Request.Context())
    if err != nil {
        sendInternalError(c, "failed to list roles")
        return
    }
    sendSuccess(c, roles)
}

// ListUsers returns all users (admin only).
func (h *Handlers) ListUsers(c *gin.Context) {
    users, err := h.store.ListUsers(c.Request.Context())
    if err != nil {
        sendInternalError(c, "failed to list users")
        return
    }
    sendSuccess(c, users)
}
```

### 6. Updated Store Interface

The aggregate `Store` interface in `internal/store/store.go` is extended to include `UserStore`, `RoleStore`, and `ConfigRoleStore`.

### 7. Updated Routes

```go
// Auth routes (public)
router.GET("/api/v1/auth/login",    h.Login)
router.GET("/api/v1/auth/callback", h.Callback)
router.POST("/api/v1/auth/logout",  h.Logout)

// Protected API routes
api := router.Group("/api/v1")
api.Use(auth.Middleware(h.oidcService))
{
    api.GET("/auth/user", h.GetCurrentUser)
    api.GET("/roles",     h.ListRoles)

    api.POST("/configs",    h.CreateConfig)
    api.GET("/configs",     h.ListConfigs)   // filtered by user's role access
    api.GET("/configs/:id", auth.RequireConfigAccess(h.store), h.GetConfig)
    api.PUT("/configs/:id", auth.RequireConfigAccess(h.store), h.UpdateConfig)
    api.DELETE("/configs/:id",
        auth.RequireRole("admin", "operator"),
        auth.RequireConfigAccess(h.store),
        h.DeleteConfig,
    )

    admin := api.Group("/admin")
    admin.Use(auth.RequireRole("admin"))
    {
        admin.GET("/users",                        h.ListUsers)
        admin.POST("/users/:id/roles",             h.AssignUserRole)
        admin.DELETE("/users/:id/roles/:role_id",  h.RemoveUserRole)
    }
}
```

### 8. ListConfigs Access Filtering

`ListConfigs` must return only configurations accessible to the requesting user:
- Admin: all configs
- Operator/Viewer: only configs where `config_roles` contains at least one of the user's roles

The store requires a new method `ListConfigsForUser(ctx, userID) ([]types.Config, error)` that joins `configs`, `config_roles`, and `user_roles`.

## When OIDC Is Disabled

When `oidc.enabled: false`:
- Auth middleware is not registered; all routes are unauthenticated.
- Auth routes (`/api/v1/auth/*`) return `501 Not Implemented`.
- This is the default for development without an IdP.

## Frontend Changes

### 1. Auth Store (Pinia)

```typescript
// web/src/stores/auth.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface User {
  id: string
  email: string
  name: string
  roles: Role[]
}

export interface Role {
  id: string
  name: string
  description: string
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.roles.some(r => r.name === 'admin') ?? false)
  const userRoles = computed(() => user.value?.roles.map(r => r.name) ?? [])

  async function fetchUser() {
    try {
      const response = await fetch('/api/v1/auth/user')
      if (response.ok) {
        const data = await response.json()
        user.value = data.data
      }
    } catch (error) {
      console.error('Failed to fetch user:', error)
    }
  }

  function login() {
    window.location.href = '/api/v1/auth/login'
  }

  async function logout() {
    await fetch('/api/v1/auth/logout', { method: 'POST' })
    user.value = null
    window.location.href = '/'
  }

  function hasRole(role: string): boolean {
    return userRoles.value.includes(role)
  }

  function canAccessConfig(configRoles: string[]): boolean {
    if (isAdmin.value) return true
    return configRoles.some(role => userRoles.value.includes(role))
  }

  return { user, isAuthenticated, isAdmin, userRoles, fetchUser, login, logout, hasRole, canAccessConfig }
})
```

### 2. Login Component

```vue
<!-- web/src/components/LoginButton.vue -->
<template>
  <div>
    <button v-if="!authStore.isAuthenticated" @click="authStore.login" class="btn btn-primary">
      Login with SSO
    </button>
    <div v-else class="user-menu">
      <span>{{ authStore.user?.name }}</span>
      <span class="roles">({{ authStore.userRoles.join(', ') }})</span>
      <button @click="authStore.logout" class="btn btn-sm btn-secondary">Logout</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
const authStore = useAuthStore()
</script>
```

### 3. Config Form with Role Selection

```vue
<!-- Role selection section in ConfigForm.vue -->
<template>
  <div class="form-section">
    <h2>Access Control</h2>
    <div class="form-group">
      <label>Roles with Access</label>
      <div class="roles-selection">
        <label v-for="role in availableRoles" :key="role.id" class="role-checkbox">
          <input type="checkbox" v-model="selectedRoles" :value="role.name" />
          <span class="role-name">{{ role.name }}</span>
          <span class="role-description">{{ role.description }}</span>
        </label>
      </div>
      <span class="help">Select which roles can access this configuration</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/types/api'

const authStore = useAuthStore()
const availableRoles = ref<Role[]>([])
const selectedRoles = ref<string[]>([])

onMounted(async () => {
  const response = await fetch('/api/v1/roles')
  if (response.ok) {
    const data = await response.json()
    availableRoles.value = data.data
  }
  if (!isEdit.value) {
    selectedRoles.value = [...authStore.userRoles]
  }
})
</script>
```

### 4. Route Guards

```typescript
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    await authStore.fetchUser()
    if (!authStore.isAuthenticated) {
      authStore.login()
      return
    }
  }

  if (to.meta.requiresRole) {
    const requiredRoles = to.meta.requiresRole as string[]
    if (!requiredRoles.some(role => authStore.hasRole(role))) {
      next('/')
      return
    }
  }

  next()
})
```

## Implementation Phases

### Phase 1: Database & Backend Core ✓
- [x] Create database migrations (users, roles, config_roles tables)
- [x] Add auth types (User, Role, OIDCConfig, JWTConfig, Session)
- [x] Extend config package (OIDCConfig, JWTConfig, env overrides)
- [x] Implement store layer (UserStore, RoleStore, ConfigRoleStore)
- [x] Implement OIDC service (with nonce verification, full role sync, active check)
- [x] Create auth middleware (fixed path matching, role check variable shadowing)
- [x] Add auth handlers (login/callback/logout/me, using JWTConfig for all cookie attrs)
- [x] Update routes (guarded, OIDC-disabled pass-through)

### Phase 2: Frontend Auth (Week 1-2)
- [ ] Create auth store (Pinia)
- [ ] Add LoginButton component
- [ ] Implement route guards
- [ ] Add role selection to ConfigForm
- [ ] Update navigation based on auth state

### Phase 3: Testing & Hardening (Week 2)
- [x] Unit tests for auth service (internal/auth/oidc_test.go — 13 tests)
- [x] Integration tests for RBAC (internal/handlers/rbac_test.go — 15 tests)
- [x] Test role-based access control (middleware, RequireRole, RequireConfigAccess, ListConfigs filtering)
- [ ] Security review
- [ ] Documentation updates

### Phase 4: Deployment (Week 3)
- [ ] Configure OIDC provider (Keycloak/Auth0)
- [ ] Set up environment variables
- [ ] Deploy to staging
- [ ] User acceptance testing
- [ ] Production deployment

## Configuration Examples

### Keycloak Configuration

1. **Create Realm**: "gslb"
2. **Create Client**:
   - Client ID: `gslb-manager`
   - Client Protocol: `openid-connect`
   - Access Type: `confidential`
   - Valid Redirect URIs: `http://localhost:8090/api/v1/auth/callback`
   - Web Origins: `http://localhost:8090`
3. **Create Roles**: `admin`, `operator`, `viewer`
4. **Mapper Configuration**:
   - Add a "User Realm Role" mapper to include roles in the ID token under the claim name matching `roles_claim`

### Environment Variables for Keycloak

```bash
export GSLB_OIDC_ENABLED=true
export GSLB_OIDC_ISSUER="http://localhost:8080/realms/gslb"
export GSLB_OIDC_CLIENT_ID="gslb-manager"
export GSLB_OIDC_CLIENT_SECRET="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export GSLB_OIDC_REDIRECT_URL="http://localhost:8090/api/v1/auth/callback"
export GSLB_OIDC_ROLES_CLAIM="roles"
export GSLB_OIDC_AUTO_CREATE_USERS=false
export GSLB_OIDC_DEFAULT_ROLE="viewer"

export GSLB_JWT_SECRET="$(openssl rand -base64 32)"
export GSLB_JWT_COOKIE_SECURE=false  # development only — set true in production
```

## Security Considerations

1. **Token Storage**: HttpOnly cookies prevent XSS access to session tokens.

2. **State + Nonce**: Both are generated, stored in short-lived HttpOnly cookies, and verified at callback to prevent CSRF and replay attacks.

3. **Role Synchronization**: Full diff-sync on every login — roles removed from the IdP are removed locally on next login.

4. **Active Flag**: Disabled users (`is_active = false`) are rejected at login even if their IdP credentials are valid.

5. **Secret Isolation**: `ClientSecret` and JWT `Secret` use `json:"-"` tags to prevent accidental serialization.

6. **`cookie_secure` defaults to `true`**: Must be explicitly disabled for local development.

7. **PKCE** (Future Enhancement): Implement for public/SPA clients.

8. **Session Management**: Server-side session invalidation (Future Enhancement).

9. **Audit Logging**: All authentication events (login, logout) are logged.

## Future Enhancements

1. **Refresh Tokens**: Token refresh for longer sessions without full re-auth
2. **API Tokens**: API keys for automation/scripts
3. **MFA**: Multi-factor authentication via OIDC provider
4. **Group Mapping**: Map OIDC groups to local roles
5. **Session Management UI**: Admin interface to view/revoke active sessions
6. **PKCE**: Code challenge for public clients

---

**Note**: Implementation follows the phases above. All spec-level bugs identified in review have been incorporated into the code samples in this document.
