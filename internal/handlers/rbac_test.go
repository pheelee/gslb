package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pheelee/gslb/internal/auth"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionStore is a minimal store implementation for testing that only implements CreateSessionRecord
type mockSessionStore struct{}

func (m *mockSessionStore) CreateSessionRecord(_ context.Context, _ *types.SessionRecord) error {
	return nil
}
func (m *mockSessionStore) GetSessionRecord(_ context.Context, _ string) (*types.SessionRecord, error) {
	return &types.SessionRecord{}, nil
}
func (m *mockSessionStore) RevokeSession(_ context.Context, _ string) error         { return nil }
func (m *mockSessionStore) RevokeUserSessions(_ context.Context, _ string) error    { return nil }
func (m *mockSessionStore) CleanupExpiredSessions(_ context.Context) (int64, error) { return 0, nil }
func (m *mockSessionStore) CreateConfig(_ context.Context, _ *types.Config) error   { return nil }
func (m *mockSessionStore) GetConfig(_ context.Context, _ string) (*types.Config, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateConfig(_ context.Context, _ *types.Config) error   { return nil }
func (m *mockSessionStore) DeleteConfig(_ context.Context, _ string) error          { return nil }
func (m *mockSessionStore) ListConfigs(_ context.Context) ([]types.Config, error) { return nil, nil }
func (m *mockSessionStore) SetConfigReconcileError(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *mockSessionStore) CreateBackend(_ context.Context, _ *types.Backend) error { return nil }
func (m *mockSessionStore) GetBackend(_ context.Context, _ string) (*types.Backend, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateBackend(_ context.Context, _ *types.Backend) error { return nil }
func (m *mockSessionStore) DeleteBackend(_ context.Context, _ string) error         { return nil }
func (m *mockSessionStore) ListBackends(_ context.Context, _ string) ([]types.Backend, error) {
	return nil, nil
}
func (m *mockSessionStore) CreateHealthCheck(_ context.Context, _ *types.HealthCheck) error {
	return nil
}
func (m *mockSessionStore) GetHealthCheck(_ context.Context, _ string) (*types.HealthCheck, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateHealthCheck(_ context.Context, _ *types.HealthCheck) error {
	return nil
}
func (m *mockSessionStore) DeleteHealthCheck(_ context.Context, _ string) error { return nil }
func (m *mockSessionStore) UpdateHealthState(_ context.Context, _ *types.HealthState) error {
	return nil
}
func (m *mockSessionStore) GetHealthState(_ context.Context, _ string) (*types.HealthState, error) {
	return nil, nil
}
func (m *mockSessionStore) GetHealthStates(_ context.Context, _ string) (map[string]types.HealthState, error) {
	return nil, nil
}
func (m *mockSessionStore) CreateDNSProvider(_ context.Context, _ *types.DNSProviderConfig) error {
	return nil
}
func (m *mockSessionStore) GetDNSProvider(_ context.Context, _ string) (*types.DNSProviderConfig, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateDNSProvider(_ context.Context, _ *types.DNSProviderConfig) error {
	return nil
}
func (m *mockSessionStore) DeleteDNSProvider(_ context.Context, _ string) error       { return nil }
func (m *mockSessionStore) CreateAuditLog(_ context.Context, _ *types.AuditLog) error { return nil }
func (m *mockSessionStore) ListAuditLogs(_ context.Context, _ store.AuditLogFilter, _, _ int) ([]types.AuditLog, error) {
	return nil, nil
}
func (m *mockSessionStore) GetAuditLog(_ context.Context, _ string) (*types.AuditLog, error) {
	return nil, nil
}
func (m *mockSessionStore) GetAuditLogsForEntity(_ context.Context, _, _ string, _ int) ([]types.AuditLog, error) {
	return nil, nil
}
func (m *mockSessionStore) DeleteOldAuditLogs(_ context.Context, _ time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockSessionStore) CountAuditLogs(_ context.Context, _ store.AuditLogFilter) (int64, error) {
	return 0, nil
}
func (m *mockSessionStore) CreateUser(_ context.Context, _ *types.User) error { return nil }
func (m *mockSessionStore) GetUserByID(_ context.Context, _ string) (*types.User, error) {
	return nil, nil
}
func (m *mockSessionStore) GetUserBySubject(_ context.Context, _ string) (*types.User, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateUser(_ context.Context, _ *types.User) error  { return nil }
func (m *mockSessionStore) ListUsers(_ context.Context) ([]types.User, error)  { return nil, nil }
func (m *mockSessionStore) AssignRole(_ context.Context, _, _, _ string) error { return nil }
func (m *mockSessionStore) RemoveRole(_ context.Context, _, _ string) error    { return nil }
func (m *mockSessionStore) GetUserRoles(_ context.Context, _ string) ([]types.Role, error) {
	return nil, nil
}
func (m *mockSessionStore) GetRoleByName(_ context.Context, _ string) (*types.Role, error) {
	return nil, nil
}
func (m *mockSessionStore) GetRoleByID(_ context.Context, _ string) (*types.Role, error) {
	return nil, nil
}
func (m *mockSessionStore) ListRoles(_ context.Context) ([]types.Role, error)     { return nil, nil }
func (m *mockSessionStore) AssignConfigRole(_ context.Context, _, _ string) error { return nil }
func (m *mockSessionStore) RemoveConfigRole(_ context.Context, _, _ string) error { return nil }
func (m *mockSessionStore) GetConfigRoles(_ context.Context, _ string) ([]types.Role, error) {
	return nil, nil
}
func (m *mockSessionStore) GetConfigsForRole(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockSessionStore) CanAccess(_ context.Context, _, _ string) (bool, error) { return false, nil }
func (m *mockSessionStore) ListConfigsForUser(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockSessionStore) WithTx(_ context.Context, fn store.TxFunc) error {
	return fn(context.Background(), nil)
}

const testJWTSecret = "test-secret-32-chars-long-enough!!"

func testJWTConfig() types.JWTConfig {
	return types.JWTConfig{
		Secret:         testJWTSecret,
		CookieName:     "gslb_session",
		CookieHTTPOnly: true,
	}
}

// setupOIDCHandlers creates a Handlers+router with a stub OIDCService wired in.
func setupOIDCHandlers(t *testing.T) (*Handlers, *gin.Engine) {
	t.Helper()
	base, s, _ := setupTestHandlers(t)

	svc := auth.OIDCServiceForTest(
		types.OIDCConfig{RolesClaim: "roles"},
		testJWTConfig(),
		s,
	)
	h := NewWithOIDC(s, base.health, &svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router)
	return h, router
}

// sessionToken creates a JWT token for a user with the given roles.
// Uses legacy token format (no JTI) to bypass session validation for tests.
func sessionToken(t *testing.T, userID string, roles []string) string {
	t.Helper()

	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r
	}

	claims := jwt.MapClaims{
		"sub":   userID,
		"email": userID + "@test.com",
		"name":  userID,
		"roles": roleNames,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return tokenString
}

func withCookie(req *http.Request, name, value string) *http.Request {
	req.AddCookie(&http.Cookie{Name: name, Value: value})
	return req
}

// ============================================================

func TestAuthMiddleware_RejectsUnauthenticated(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AcceptsValidToken(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs", nil)
	withCookie(req, "gslb_session", sessionToken(t, "user-1", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RejectsInvalidToken(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireRole_AdminAllowed(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-1", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_ViewerForbidden(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-1", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_BearerToken(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken(t, "admin-2", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateConfig_AnyAuthenticatedUserCanCreate(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	for _, role := range []string{"admin", "operator", "viewer"} {
		t.Run(role, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"name": role + "-cfg", "dns_name": role + ".example.com",
				"dns_ttl": 30, "lb_method": "round_robin",
			})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/configs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			withCookie(req, "gslb_session", sessionToken(t, role+"-user", []string{role}))
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "role %s should be able to create configs", role)
		})
	}
}

func TestRequireConfigAccess_AdminBypassesCheck(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfg := &types.Config{
		Name: "access-test", DNSName: "access.example.com",
		DNSTTL: 30, LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs/"+cfg.ID, nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-3", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireConfigAccess_NoRoleDenied(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfg := &types.Config{
		Name: "private-cfg", DNSName: "private.example.com",
		DNSTTL: 30, LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))
	// No config_roles assigned → viewer cannot access

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs/"+cfg.ID, nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-2", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireConfigAccess_GrantedViaRole(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfg := &types.Config{
		Name: "shared-cfg", DNSName: "shared.example.com",
		DNSTTL: 30, LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	user := &types.User{ID: "viewer-3", Subject: "viewer-3", Email: "v3@test.com", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))
	viewerRole, err := h.store.GetRoleByName(t.Context(), "viewer")
	require.NoError(t, err)
	require.NoError(t, h.store.AssignRole(t.Context(), user.ID, viewerRole.ID, "test"))
	require.NoError(t, h.store.AssignConfigRole(t.Context(), cfg.ID, viewerRole.ID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs/"+cfg.ID, nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-3", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteConfig_ViewerForbiddenByRole(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfg := &types.Config{
		Name: "del-cfg", DNSName: "del.example.com",
		DNSTTL: 30, LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	// Give viewer access to the config, but viewer still can't delete
	user := &types.User{ID: "viewer-4", Subject: "viewer-4", Email: "v4@test.com", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))
	viewerRole, _ := h.store.GetRoleByName(t.Context(), "viewer")
	h.store.AssignRole(t.Context(), user.ID, viewerRole.ID, "test") //nolint:errcheck
	h.store.AssignConfigRole(t.Context(), cfg.ID, viewerRole.ID)    //nolint:errcheck

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/configs/"+cfg.ID, nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-4", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListConfigs_AdminGetsAll(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	for _, name := range []string{"a", "b", "c"} {
		cfg := &types.Config{Name: name, DNSName: name + ".example.com", DNSTTL: 30, LBMethod: types.RoundRobin}
		require.NoError(t, h.store.CreateConfig(t.Context(), cfg))
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-4", []string{"admin"}))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	list, _ := resp.Data.([]interface{})
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestListConfigs_ViewerOnlySeesAssigned(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfgVis := &types.Config{Name: "visible", DNSName: "visible.example.com", DNSTTL: 30, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfgVis))
	cfgHid := &types.Config{Name: "hidden", DNSName: "hidden.example.com", DNSTTL: 30, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfgHid))

	user := &types.User{ID: "viewer-5", Subject: "viewer-5", Email: "v5@test.com", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))
	viewerRole, _ := h.store.GetRoleByName(t.Context(), "viewer")
	h.store.AssignRole(t.Context(), user.ID, viewerRole.ID, "test") //nolint:errcheck
	h.store.AssignConfigRole(t.Context(), cfgVis.ID, viewerRole.ID) //nolint:errcheck

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-5", []string{"viewer"}))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	list, _ := resp.Data.([]interface{})
	assert.Len(t, list, 1)
	assert.Equal(t, "visible", list[0].(map[string]interface{})["name"])
}

func TestPublicPaths_NoAuthRequired(t *testing.T) {
	_, router := setupOIDCHandlers(t)

	for _, path := range []string{"/health", "/ready"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "path %s should be public", path)
	}
}

func TestGetConfigRoles_ReturnsAssignedRoles(t *testing.T) {
	h, router := setupOIDCHandlers(t)

	cfg := &types.Config{Name: "roles-cfg", DNSName: "roles.example.com", DNSTTL: 30, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))
	viewerRole, _ := h.store.GetRoleByName(t.Context(), "viewer")
	h.store.AssignConfigRole(t.Context(), cfg.ID, viewerRole.ID) //nolint:errcheck

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/configs/"+cfg.ID+"/roles", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-5", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	list, _ := resp.Data.([]interface{})
	assert.Len(t, list, 1)
}
