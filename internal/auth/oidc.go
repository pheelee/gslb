// Package auth provides OIDC authentication and JWT session management.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// OIDCService handles OIDC authentication and JWT session management.
type OIDCService struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config oauth2.Config
	oidcCfg      types.OIDCConfig
	jwtCfg       types.JWTConfig
	store        store.Store
	jwtSecret    []byte
}

// NewOIDCService creates a new OIDCService by discovering the provider's OIDC metadata.
func NewOIDCService(oidcCfg types.OIDCConfig, jwtCfg types.JWTConfig, s store.Store) (*OIDCService, error) {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, oidcCfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}

	oauth2Cfg := oauth2.Config{
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
		oauth2Config: oauth2Cfg,
		oidcCfg:      oidcCfg,
		jwtCfg:       jwtCfg,
		store:        s,
		jwtSecret:    []byte(jwtCfg.Secret),
	}, nil
}

// JWTConfig returns the JWT configuration (used by handlers for cookie attributes).
func (s *OIDCService) JWTConfig() types.JWTConfig { return s.jwtCfg }

// EndSessionURL returns the OIDC provider's end-session URL, if available.
// Returns an empty string if the provider does not advertise one or is not configured.
func (s *OIDCService) EndSessionURL() string {
	if s.provider == nil {
		return ""
	}
	var claims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := s.provider.Claims(&claims); err != nil {
		return ""
	}
	return claims.EndSessionEndpoint
}

// GenerateStateNonce generates a cryptographically random URL-safe base64 string
// suitable for use as an OAuth2 state or OIDC nonce parameter.
func GenerateStateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GeneratePKCEVerifier generates a cryptographically random PKCE code verifier (43-128 chars).
func GeneratePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCEChallenge computes the S256 challenge for a given verifier.
func PKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GetAuthURL returns the OIDC provider authorization URL with state, nonce, and PKCE challenge.
// The caller must store the verifier (e.g., in an HttpOnly cookie) for the callback.
func (s *OIDCService) GetAuthURL(state, nonce, pkceChallenge string) string {
	return s.oauth2Config.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange exchanges an authorization code for an OAuth2 token set,
// providing the PKCE verifier to prove possession.
func (s *OIDCService) Exchange(ctx context.Context, code, pkceVerifier string) (*oauth2.Token, error) {
	return s.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", pkceVerifier),
	)
}

// VerifyIDToken verifies the ID token signature, expiry, audience, and nonce.
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

// ExtractUserInfo extracts user info from the ID token claims.
// Roles are read from the claim named by OIDCConfig.RolesClaim.
func (s *OIDCService) ExtractUserInfo(token *oidc.IDToken) (*types.User, []string, error) {
	var claims map[string]interface{}
	if err := token.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("parse claims: %w", err)
	}

	subject, _ := claims["sub"].(string)
	if subject == "" {
		return nil, nil, fmt.Errorf("missing sub claim")
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)

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

	return &types.User{Subject: subject, Email: email, Name: name}, roles, nil
}

// CreateSession issues a signed JWT session token for the given user.
// A server-side session record is created for revocation support.
// Expiry is taken from JWTConfig (falls back to 24h if zero).
func (s *OIDCService) CreateSession(ctx context.Context, user *types.User) (string, error) {
	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = r.Name
	}

	expiry := s.jwtCfg.Expiry
	if expiry == 0 {
		expiry = 24 * time.Hour
	}

	jti := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(expiry)

	claims := jwt.MapClaims{
		"jti":   jti,
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"roles": roles,
		"iat":   now.Unix(),
		"exp":   expiresAt.Unix(),
	}

	// Persist session record for server-side revocation
	record := &types.SessionRecord{
		JTI:       jti,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	if err := s.store.CreateSessionRecord(ctx, record); err != nil {
		return "", fmt.Errorf("persisting session record: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ParseSession validates a session JWT and returns the embedded Session.
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
	if !ok || sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}
	jti, _ := claims["jti"].(string)
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
		JTI:       jti,
		UserID:    sub,
		Email:     email,
		Roles:     roles,
		ExpiresAt: time.Unix(exp, 0),
	}, nil
}

// ValidateSession checks that a parsed session has not been revoked server-side.
func (s *OIDCService) ValidateSession(ctx context.Context, session *types.Session) error {
	if session.JTI == "" {
		return nil // legacy tokens without JTI are allowed (migration path)
	}
	record, err := s.store.GetSessionRecord(ctx, session.JTI)
	if err != nil {
		if err == store.ErrNotFound {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("checking session: %w", err)
	}
	if record.RevokedAt != nil {
		return fmt.Errorf("session revoked")
	}
	return nil
}

// SyncUser upserts the OIDC user and performs a full diff-sync of their roles:
// roles present in oidcRoles are added; roles absent from oidcRoles are removed.
// Returns an error if the user is disabled.
func (s *OIDCService) SyncUser(ctx context.Context, oidcUser *types.User, oidcRoles []string) (*types.User, error) {
	existing, err := s.store.GetUserBySubject(ctx, oidcUser.Subject)
	if err != nil && err != store.ErrNotFound {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if existing != nil {
		if !existing.IsActive {
			return nil, fmt.Errorf("authentication failed")
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

	// New user — always create on first login.
	now := time.Now()
	oidcUser.CreatedAt = now
	oidcUser.LastLoginAt = now
	oidcUser.IsActive = true
	// ID is assigned inside CreateUser

	if err := s.store.CreateUser(ctx, oidcUser); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

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

// syncRoles performs a full diff between the OIDC-provided role names and the
// user's current local roles. Missing roles are added; stale roles are removed.
// Roles not known locally are silently skipped.
func (s *OIDCService) syncRoles(ctx context.Context, userID string, oidcRoleNames []string) error {
	current, err := s.store.GetUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user roles: %w", err)
	}

	// Build lookup maps
	currentByName := make(map[string]string, len(current)) // name → id
	for _, r := range current {
		currentByName[r.Name] = r.ID
	}
	desiredNames := make(map[string]bool, len(oidcRoleNames))
	for _, n := range oidcRoleNames {
		desiredNames[n] = true
	}

	// Add missing roles
	for _, name := range oidcRoleNames {
		if _, has := currentByName[name]; has {
			continue
		}
		role, err := s.store.GetRoleByName(ctx, name)
		if err != nil {
			continue // unknown role locally — skip
		}
		_ = s.store.AssignRole(ctx, userID, role.ID, "oidc")
	}

	// Remove stale roles
	for name, id := range currentByName {
		if desiredNames[name] {
			continue
		}
		_ = s.store.RemoveRole(ctx, userID, id)
	}

	return nil
}
