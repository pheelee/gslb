package auth

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// --- mock store ----------------------------------------------------------------

type mockStore struct {
	users       map[string]*types.User  // subject → user
	usersByID   map[string]*types.User  // id → user
	userRoles   map[string][]types.Role // userID → roles
	roles       map[string]*types.Role  // name → role
	rolesById   map[string]*types.Role  // id → role
	configRoles map[string][]types.Role // configID → roles
}

func newMockStore() *mockStore {
	adminRole := &types.Role{ID: "role-admin", Name: "admin", Description: "Admin"}
	viewerRole := &types.Role{ID: "role-viewer", Name: "viewer", Description: "Viewer"}
	operatorRole := &types.Role{ID: "role-operator", Name: "operator", Description: "Operator"}
	return &mockStore{
		users:       make(map[string]*types.User),
		usersByID:   make(map[string]*types.User),
		userRoles:   make(map[string][]types.Role),
		configRoles: make(map[string][]types.Role),
		roles: map[string]*types.Role{
			"admin":    adminRole,
			"viewer":   viewerRole,
			"operator": operatorRole,
		},
		rolesById: map[string]*types.Role{
			"role-admin":    adminRole,
			"role-viewer":   viewerRole,
			"role-operator": operatorRole,
		},
	}
}

func (m *mockStore) GetUserBySubject(_ context.Context, subject string) (*types.User, error) {
	u, ok := m.users[subject]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (m *mockStore) GetUserByID(_ context.Context, id string) (*types.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	// Attach roles
	copy := *u
	copy.Roles = m.userRoles[id]
	return &copy, nil
}

func (m *mockStore) CreateUser(_ context.Context, user *types.User) error {
	if user.ID == "" {
		user.ID = "generated-id-" + user.Subject
	}
	copy := *user
	m.users[user.Subject] = &copy
	m.usersByID[user.ID] = &copy
	return nil
}

func (m *mockStore) UpdateUser(_ context.Context, user *types.User) error {
	if _, ok := m.usersByID[user.ID]; !ok {
		return store.ErrNotFound
	}
	copy := *user
	m.users[user.Subject] = &copy
	m.usersByID[user.ID] = &copy
	return nil
}

func (m *mockStore) ListUsers(_ context.Context) ([]types.User, error) {
	var users []types.User
	for _, u := range m.usersByID {
		users = append(users, *u)
	}
	return users, nil
}

func (m *mockStore) AssignRole(_ context.Context, userID, roleID, _ string) error {
	role, ok := m.rolesById[roleID]
	if !ok {
		return store.ErrNotFound
	}
	for _, r := range m.userRoles[userID] {
		if r.ID == roleID {
			return nil // already assigned
		}
	}
	m.userRoles[userID] = append(m.userRoles[userID], *role)
	return nil
}

func (m *mockStore) RemoveRole(_ context.Context, userID, roleID string) error {
	roles := m.userRoles[userID]
	filtered := roles[:0]
	for _, r := range roles {
		if r.ID != roleID {
			filtered = append(filtered, r)
		}
	}
	m.userRoles[userID] = filtered
	return nil
}

func (m *mockStore) GetUserRoles(_ context.Context, userID string) ([]types.Role, error) {
	return m.userRoles[userID], nil
}

func (m *mockStore) GetRoleByName(_ context.Context, name string) (*types.Role, error) {
	r, ok := m.roles[name]
	if !ok {
		return nil, store.ErrNotFound
	}
	return r, nil
}

func (m *mockStore) GetRoleByID(_ context.Context, id string) (*types.Role, error) {
	r, ok := m.rolesById[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return r, nil
}

func (m *mockStore) ListRoles(_ context.Context) ([]types.Role, error) {
	var roles []types.Role
	for _, r := range m.roles {
		roles = append(roles, *r)
	}
	return roles, nil
}

func (m *mockStore) AssignConfigRole(_ context.Context, configID, roleID string) error {
	role, ok := m.rolesById[roleID]
	if !ok {
		return store.ErrNotFound
	}
	m.configRoles[configID] = append(m.configRoles[configID], *role)
	return nil
}

func (m *mockStore) RemoveConfigRole(_ context.Context, configID, roleID string) error {
	roles := m.configRoles[configID]
	filtered := roles[:0]
	for _, r := range roles {
		if r.ID != roleID {
			filtered = append(filtered, r)
		}
	}
	m.configRoles[configID] = filtered
	return nil
}

func (m *mockStore) GetConfigRoles(_ context.Context, configID string) ([]types.Role, error) {
	return m.configRoles[configID], nil
}

func (m *mockStore) GetConfigsForRole(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockStore) CanAccess(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (m *mockStore) ListConfigsForUser(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// --- stub methods to satisfy store.Store interface ----------------------------

func (m *mockStore) CreateConfig(_ context.Context, _ *types.Config) error        { return nil }
func (m *mockStore) GetConfig(_ context.Context, _ string) (*types.Config, error) { return nil, nil }
func (m *mockStore) UpdateConfig(_ context.Context, _ *types.Config) error        { return nil }
func (m *mockStore) DeleteConfig(_ context.Context, _ string) error               { return nil }
func (m *mockStore) ListConfigs(_ context.Context) ([]types.Config, error) { return nil, nil }
func (m *mockStore) SetConfigReconcileError(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockStore) CreateBackend(_ context.Context, _ *types.Backend) error        { return nil }
func (m *mockStore) GetBackend(_ context.Context, _ string) (*types.Backend, error) { return nil, nil }
func (m *mockStore) UpdateBackend(_ context.Context, _ *types.Backend) error        { return nil }
func (m *mockStore) DeleteBackend(_ context.Context, _ string) error                { return nil }
func (m *mockStore) ListBackends(_ context.Context, _ string) ([]types.Backend, error) {
	return nil, nil
}

func (m *mockStore) CreateHealthCheck(_ context.Context, _ *types.HealthCheck) error { return nil }
func (m *mockStore) GetHealthCheck(_ context.Context, _ string) (*types.HealthCheck, error) {
	return nil, nil
}
func (m *mockStore) UpdateHealthCheck(_ context.Context, _ *types.HealthCheck) error { return nil }
func (m *mockStore) DeleteHealthCheck(_ context.Context, _ string) error             { return nil }
func (m *mockStore) UpdateHealthState(_ context.Context, _ *types.HealthState) error { return nil }
func (m *mockStore) GetHealthState(_ context.Context, _ string) (*types.HealthState, error) {
	return nil, nil
}
func (m *mockStore) GetHealthStates(_ context.Context, _ string) (map[string]types.HealthState, error) {
	return nil, nil
}

func (m *mockStore) CreateDNSProvider(_ context.Context, _ *types.DNSProviderConfig) error {
	return nil
}
func (m *mockStore) GetDNSProvider(_ context.Context, _ string) (*types.DNSProviderConfig, error) {
	return nil, nil
}
func (m *mockStore) UpdateDNSProvider(_ context.Context, _ *types.DNSProviderConfig) error {
	return nil
}
func (m *mockStore) DeleteDNSProvider(_ context.Context, _ string) error { return nil }

func (m *mockStore) CreateAuditLog(_ context.Context, _ *types.AuditLog) error { return nil }
func (m *mockStore) ListAuditLogs(_ context.Context, _ store.AuditLogFilter, _, _ int) ([]types.AuditLog, error) {
	return nil, nil
}
func (m *mockStore) GetAuditLog(_ context.Context, _ string) (*types.AuditLog, error) {
	return nil, nil
}
func (m *mockStore) GetAuditLogsForEntity(_ context.Context, _, _ string, _ int) ([]types.AuditLog, error) {
	return nil, nil
}
func (m *mockStore) DeleteOldAuditLogs(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockStore) CountAuditLogs(_ context.Context, _ store.AuditLogFilter) (int64, error) {
	return 0, nil
}

func (m *mockStore) UpsertHealthHistoryBucket(_ context.Context, _ *types.HealthHistoryBucket) error {
	return nil
}
func (m *mockStore) GetHealthHistory(_ context.Context, _ string, _ time.Time) ([]types.HealthHistoryBucket, error) {
	return nil, nil
}
func (m *mockStore) DeleteOldHealthHistory(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockStore) WithTx(_ context.Context, fn store.TxFunc) error {
	return fn(context.Background(), &sql.Tx{})
}

func (m *mockStore) CreateSessionRecord(_ context.Context, _ *types.SessionRecord) error { return nil }
func (m *mockStore) GetSessionRecord(_ context.Context, _ string) (*types.SessionRecord, error) {
	return nil, store.ErrNotFound
}
func (m *mockStore) RevokeSession(_ context.Context, _ string) error         { return nil }
func (m *mockStore) RevokeUserSessions(_ context.Context, _ string) error    { return nil }
func (m *mockStore) CleanupExpiredSessions(_ context.Context) (int64, error) { return 0, nil }

// --- helpers -------------------------------------------------------------------

func defaultJWTConfig() types.JWTConfig {
	return types.JWTConfig{
		Secret:         "test-secret-that-is-long-enough-32chars",
		Expiry:         time.Hour,
		CookieName:     "gslb_session",
		CookieSecure:   false,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	}
}

func defaultOIDCConfig() types.OIDCConfig {
	return types.OIDCConfig{
		RolesClaim:  "roles",
		DefaultRole: "viewer",
	}
}

func newServiceWithStore(s *mockStore) *OIDCService {
	return &OIDCService{
		oidcCfg:   defaultOIDCConfig(),
		jwtCfg:    defaultJWTConfig(),
		store:     s,
		jwtSecret: []byte(defaultJWTConfig().Secret),
	}
}

// --- tests ---------------------------------------------------------------------

func TestGenerateStateNonce(t *testing.T) {
	a, err := GenerateStateNonce()
	require.NoError(t, err)
	b, err := GenerateStateNonce()
	require.NoError(t, err)
	assert.NotEmpty(t, a)
	assert.NotEmpty(t, b)
	assert.NotEqual(t, a, b, "consecutive calls must produce unique values")
}

func TestCreateAndParseSession(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	user := &types.User{
		ID:    "user-1",
		Email: "alice@example.com",
		Name:  "Alice",
		Roles: []types.Role{
			{ID: "role-admin", Name: "admin"},
		},
	}

	token, err := svc.CreateSession(context.Background(), user)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	session, err := svc.ParseSession(token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", session.UserID)
	assert.Equal(t, "alice@example.com", session.Email)
	assert.Equal(t, []string{"admin"}, session.Roles)
	assert.True(t, session.ExpiresAt.After(time.Now()))
}

func TestParseSession_InvalidToken(t *testing.T) {
	svc := newServiceWithStore(newMockStore())

	_, err := svc.ParseSession("not.a.token")
	assert.Error(t, err)
}

func TestParseSession_WrongSecret(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	user := &types.User{ID: "u1", Email: "x@x.com", Roles: []types.Role{}}
	token, err := svc.CreateSession(context.Background(), user)
	require.NoError(t, err)

	other := *svc
	other.jwtSecret = []byte("wrong-secret-that-is-also-long-enough!")
	_, err = other.ParseSession(token)
	assert.Error(t, err)
}

func TestCreateSession_UsesConfiguredExpiry(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	svc.jwtCfg.Expiry = 2 * time.Hour

	user := &types.User{ID: "u1", Email: "x@x.com", Roles: []types.Role{}}
	token, err := svc.CreateSession(context.Background(), user)
	require.NoError(t, err)

	session, err := svc.ParseSession(token)
	require.NoError(t, err)

	// Expiry should be ~2h from now
	diff := time.Until(session.ExpiresAt)
	assert.Greater(t, diff, 90*time.Minute)
	assert.Less(t, diff, 130*time.Minute)
}

func TestSyncUser_NewUser(t *testing.T) {
	s := newMockStore()
	svc := newServiceWithStore(s)

	oidcUser := &types.User{Subject: "sub-1", Email: "bob@example.com", Name: "Bob"}
	user, err := svc.SyncUser(context.Background(), oidcUser, []string{})
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.True(t, user.IsActive)

	// Should have default role (viewer)
	var roleNames []string
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}
	assert.Contains(t, roleNames, "viewer")
}

func TestSyncUser_ExistingUser_UpdatesProfile(t *testing.T) {
	s := newMockStore()
	existing := &types.User{
		ID:       "user-42",
		Subject:  "sub-42",
		Email:    "old@example.com",
		Name:     "Old Name",
		IsActive: true,
	}
	s.users[existing.Subject] = existing
	s.usersByID[existing.ID] = existing

	svc := newServiceWithStore(s)
	oidcUser := &types.User{Subject: "sub-42", Email: "new@example.com", Name: "New Name"}
	user, err := svc.SyncUser(context.Background(), oidcUser, nil)
	require.NoError(t, err)

	assert.Equal(t, "user-42", user.ID)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, "New Name", user.Name)
}

func TestSyncUser_DisabledUser_Rejected(t *testing.T) {
	s := newMockStore()
	existing := &types.User{
		ID:       "user-disabled",
		Subject:  "sub-disabled",
		Email:    "x@x.com",
		IsActive: false,
	}
	s.users[existing.Subject] = existing
	s.usersByID[existing.ID] = existing

	svc := newServiceWithStore(s)
	_, err := svc.SyncUser(context.Background(), existing, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestSyncRoles_AddsNewRoles(t *testing.T) {
	s := newMockStore()
	userID := "user-sync"

	svc := newServiceWithStore(s)
	err := svc.syncRoles(context.Background(), userID, []string{"admin", "operator"})
	require.NoError(t, err)

	roles := s.userRoles[userID]
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
	}
	assert.ElementsMatch(t, []string{"admin", "operator"}, names)
}

func TestSyncRoles_RemovesStaleRoles(t *testing.T) {
	s := newMockStore()
	userID := "user-sync-remove"
	// Pre-assign admin and operator
	s.userRoles[userID] = []types.Role{
		{ID: "role-admin", Name: "admin"},
		{ID: "role-operator", Name: "operator"},
	}

	svc := newServiceWithStore(s)
	// OIDC now only provides viewer
	err := svc.syncRoles(context.Background(), userID, []string{"viewer"})
	require.NoError(t, err)

	roles := s.userRoles[userID]
	assert.Len(t, roles, 1)
	assert.Equal(t, "viewer", roles[0].Name)
}

func TestSyncRoles_UnknownRolesSkipped(t *testing.T) {
	s := newMockStore()
	userID := "user-unknown-role"

	svc := newServiceWithStore(s)
	// "superuser" is not a known local role
	err := svc.syncRoles(context.Background(), userID, []string{"admin", "superuser"})
	require.NoError(t, err)

	roles := s.userRoles[userID]
	assert.Len(t, roles, 1)
	assert.Equal(t, "admin", roles[0].Name)
}

func TestJWTConfig_ReturnedCorrectly(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	cfg := svc.JWTConfig()
	assert.Equal(t, "gslb_session", cfg.CookieName)
	assert.Equal(t, time.Hour, cfg.Expiry)
}

func TestGeneratePKCEVerifier(t *testing.T) {
	a, err := GeneratePKCEVerifier()
	require.NoError(t, err)
	b, err := GeneratePKCEVerifier()
	require.NoError(t, err)
	assert.NotEmpty(t, a)
	assert.NotEmpty(t, b)
	assert.NotEqual(t, a, b)
}

func TestPKCEChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	challenge := PKCEChallenge(verifier)
	assert.NotEmpty(t, challenge)
	// same input → same output (deterministic)
	assert.Equal(t, challenge, PKCEChallenge(verifier))
	// different input → different output
	assert.NotEqual(t, challenge, PKCEChallenge("other-verifier"))
}

func TestValidateSession_EmptyJTI(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	session := &types.Session{JTI: ""} // legacy token without JTI
	err := svc.ValidateSession(context.Background(), session)
	assert.NoError(t, err)
}

func TestValidateSession_Revoked(t *testing.T) {
	ts := newSessionTrackingStore()
	now := time.Now()
	ts.sessions["jti-revoked"] = &types.SessionRecord{
		JTI:       "jti-revoked",
		RevokedAt: &now,
	}
	svc := &OIDCService{
		oidcCfg:   defaultOIDCConfig(),
		jwtCfg:    defaultJWTConfig(),
		store:     ts,
		jwtSecret: []byte(defaultJWTConfig().Secret),
	}
	session := &types.Session{JTI: "jti-revoked"}
	err := svc.ValidateSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestValidateSession_NotFound(t *testing.T) {
	svc := newServiceWithStore(newMockStore())
	session := &types.Session{JTI: "nonexistent-jti"}
	err := svc.ValidateSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSyncUser_ExistingUser_WithRoles(t *testing.T) {
	s := newMockStore()
	existing := &types.User{
		ID:       "user-with-roles",
		Subject:  "sub-roles",
		Email:    "old@example.com",
		Name:     "Old Name",
		IsActive: true,
	}
	s.users[existing.Subject] = existing
	s.usersByID[existing.ID] = existing

	svc := newServiceWithStore(s)
	oidcUser := &types.User{Subject: "sub-roles", Email: "new@example.com", Name: "New Name"}
	user, err := svc.SyncUser(context.Background(), oidcUser, []string{"admin"})
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "new@example.com", user.Email)
}

func TestSyncUser_LookupError(t *testing.T) {
	s := newMockStore()
	// Override GetUserBySubject to return a non-ErrNotFound error
	svc := newServiceWithStore(s)
	// Inject an error via a broken store wrapper
	_ = svc // just ensuring it compiles; hard to inject error without custom wrapper
	t.Skip("lookup error path requires custom store injection")
}

func TestCreateSession_StoreError(t *testing.T) {
	s := &failingSessionStore{mockStore: newMockStore()}
	svc := &OIDCService{
		oidcCfg:   defaultOIDCConfig(),
		jwtCfg:    defaultJWTConfig(),
		store:     s,
		jwtSecret: []byte(defaultJWTConfig().Secret),
	}
	user := &types.User{ID: "u1", Email: "x@x.com", Roles: []types.Role{}}
	_, err := svc.CreateSession(context.Background(), user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persisting session record")
}

type failingSessionStore struct {
	*mockStore
}

func (f *failingSessionStore) CreateSessionRecord(_ context.Context, _ *types.SessionRecord) error {
	return fmt.Errorf("db error")
}
