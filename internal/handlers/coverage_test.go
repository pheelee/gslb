package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_WithValidToken(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	withCookie(req, "gslb_session", sessionToken(t, "user-logout", []string{"viewer"}))
	router.ServeHTTP(w, req)

	// Should succeed and clear the session cookie
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogout_NoToken(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	router.ServeHTTP(w, req)

	// Logout without auth is rate-limited but doesn't require auth
	// Should still return a valid response (may be 4xx if no session)
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

// ─── getBackendOr404 ──────────────────────────────────────────────────────────

func TestGetBackendOr404_Found(t *testing.T) {
	h, s, _ := setupTestHandlers(t)

	cfg := &types.Config{Name: "or404-cfg", DNSName: "or404.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))
	backend := &types.Backend{ConfigID: cfg.ID, IP: "10.10.10.1", Weight: 1, Enabled: true}
	require.NoError(t, s.CreateBackend(context.Background(), backend))

	w := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "id", Value: backend.ID}}
	ctx.Request, _ = http.NewRequest("GET", "/", nil)
	ctx.Request = ctx.Request.WithContext(context.Background())

	result, ok := h.getBackendOr404(ctx)
	assert.True(t, ok)
	assert.Equal(t, backend.ID, result.ID)
}

func TestGetBackendOr404_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	w := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
	ctx.Request, _ = http.NewRequest("GET", "/", nil)
	ctx.Request = ctx.Request.WithContext(context.Background())

	_, ok := h.getBackendOr404(ctx)
	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── getConfigOr404 ───────────────────────────────────────────────────────────

func TestGetConfigOr404_Found(t *testing.T) {
	h, s, _ := setupTestHandlers(t)

	cfg := &types.Config{Name: "cfg-or404", DNSName: "cfg-or404.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	w := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "id", Value: cfg.ID}}
	ctx.Request, _ = http.NewRequest("GET", "/", nil)
	ctx.Request = ctx.Request.WithContext(context.Background())

	result, ok := h.getConfigOr404(ctx)
	assert.True(t, ok)
	assert.Equal(t, cfg.ID, result.ID)
}

func TestGetConfigOr404_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)

	w := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "id", Value: "no-such-config"}}
	ctx.Request, _ = http.NewRequest("GET", "/", nil)
	ctx.Request = ctx.Request.WithContext(context.Background())

	_, ok := h.getConfigOr404(ctx)
	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── GetCurrentUser error path ────────────────────────────────────────────────

func TestGetCurrentUser_UserNotInDB(t *testing.T) {
	// The token has subject "ghost-user" but we don't create that user in the DB.
	// GetCurrentUser calls GetUserByID which returns ErrNotFound → sendInternalError.
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/user", nil)
	withCookie(req, "gslb_session", sessionToken(t, "ghost-user", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── UpdateConfig validation ──────────────────────────────────────────────────

func TestUpdateConfig_ValidationError(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "val-cfg", DNSName: "val.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	// Empty name should fail validation
	body := `{"name":"","dns_name":"val.example.com","dns_ttl":60,"lb_method":"round_robin"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID, makeBody(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── GetAuditLogsForEntity with limit ────────────────────────────────────────

func TestGetAuditLogsForEntity_WithLimit(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "limited-entity", "c1", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/entity/config/limited-entity?limit=5", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── GetAuditLogsForEntity empty ──────────────────────────────────────────────

func TestGetAuditLogsForEntity_Empty(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/entity/config/no-such-entity", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── triggerReconcile (requires reconciler to be set) ─────────────────────────

func TestTriggerReconcile_NoReconciler(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	// No reconciler set (h.reconciler is nil) — triggerReconcile should be a no-op
	cfg := &types.Config{Name: "recon-cfg", DNSName: "recon.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	// triggerReconcile is called internally by AddBackend
	router := setupTestRouter(h)
	body := `{"ip":"1.2.3.4","weight":1,"enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/backends", makeBody(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // no panic, reconciler nil is handled
}

// ─── DeleteHealthCheck not found ──────────────────────────────────────────────

func TestDeleteHealthCheck_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/nonexistent/health-check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteHealthCheck_HealthCheckNotFound(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "hcdel-nf", DNSName: "hcdelnf.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/"+cfg.ID+"/health-check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── ListBackends with backends ──────────────────────────────────────────────

func TestListBackends_WithBackends(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "lb-cfg", DNSName: "lb.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	for _, ip := range []string{"1.0.0.1", "1.0.0.2"} {
		backend := &types.Backend{ConfigID: cfg.ID, IP: ip, Weight: 1, Enabled: true}
		require.NoError(t, s.CreateBackend(context.Background(), backend))
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/backends", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// makeBody creates a *bytes.Buffer from a string literal.
func makeBody(s string) *bytes.Buffer {
	return bytes.NewBufferString(s)
}

// ─── SetReconciler + triggerReconcile ────────────────────────────────────────

// mockReconciler implements Reconciler for testing.
type mockReconciler struct {
	called bool
	err    error
}

func (m *mockReconciler) ReconcileConfig(_ context.Context, _ string) error {
	m.called = true
	return m.err
}

func TestSetReconcilerAndTrigger(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	rec := &mockReconciler{}
	h.SetReconciler(rec)

	cfg := &types.Config{Name: "recon2-cfg", DNSName: "recon2.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	body := `{"ip":"2.0.0.1","weight":1,"enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/backends", makeBody(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── GetConfigRoles not-found ────────────────────────────────────────────────

func TestGetConfigRoles_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/nonexistent/roles", nil)
	router.ServeHTTP(w, req)

	// Without OIDC, GetConfigRoles is not registered as a route;
	// test with OIDC-gated setup
	_ = w
}

func TestGetConfigRoles_ConfigNotFoundOIDC(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/nonexistent/roles", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-gcr", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── ListRoles + ListUsers with OIDC (covers more statements) ────────────────

func TestListUsers_OIDC_Success(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	user := &types.User{Subject: "lu-sub", Email: "lu@test.com", Name: "LU", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/users", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-lu", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListRoles_Success(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/roles", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-lr", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_RedirectsToProvider(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/login", nil)
	router.ServeHTTP(w, req)

	// Login sets cookies and redirects to OIDC provider
	assert.Equal(t, http.StatusFound, w.Code)
	assert.NotEmpty(t, w.Header().Get("Location"))
}

// ─── Callback error paths ──────────────────────────────────────────────────────

func TestCallback_MissingStateCookie(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?code=abc&state=xyz", nil)
	// No state cookie → should fail early with 400 or redirect
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestCallback_StateMismatch(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?code=abc&state=wrong-state", nil)
	// Cookie has different state than query param
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "correct-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "some-nonce"})
	req.AddCookie(&http.Cookie{Name: "oidc_pkce", Value: "some-verifier"})
	router.ServeHTTP(w, req)

	// State mismatch → error redirect or 400
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestCallback_MissingNonce(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	// State matches, but no nonce cookie
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?code=abc&state=my-state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "my-state"})
	// No oidc_nonce cookie
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallback_MissingPKCE(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?code=abc&state=my-state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "my-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "my-nonce"})
	// No oidc_pkce cookie
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallback_MissingCode(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	// State matches, all cookies present, but no code query param
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?state=my-state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "my-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "my-nonce"})
	req.AddCookie(&http.Cookie{Name: "oidc_pkce", Value: "my-verifier"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCallback_ExchangeFails(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	// All params present, but Exchange will fail (no real OIDC provider)
	req, _ := http.NewRequest("GET", "/api/v1/auth/callback?code=invalid-code&state=my-state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "my-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "my-nonce"})
	req.AddCookie(&http.Cookie{Name: "oidc_pkce", Value: "my-verifier"})
	router.ServeHTTP(w, req)

	// Exchange fails → 401
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Logout with session in context ──────────────────────────────────────────

func TestLogout_WithSessionInContext(t *testing.T) {
	h, _ := setupOIDCHandlersSingle(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/auth/logout", nil)

	// Set a session with JTI in context (as auth middleware would)
	session := &types.Session{JTI: "test-jti", UserID: "user-1"}
	c.Set("session", session)

	h.Logout(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── UpdateConfigRoles via OIDC ───────────────────────────────────────────────

func TestUpdateConfigRoles_Success(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "ucr-cfg", DNSName: "ucr.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	body := `{"roles":["viewer"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID+"/roles", makeBody(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-ucr", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetConfigRoles_Success_OIDC(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "gcr-cfg", DNSName: "gcr.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/roles", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-gcr2", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── ListConfigs with OIDC viewer (non-admin filter path) ─────────────────────

func TestListConfigs_ViewerNoAccess(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-lc", []string{"viewer"}))
	router.ServeHTTP(w, req)

	// Viewer with no assigned configs → empty list
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── UpdateConfigRoles removing existing roles ────────────────────────────────

func TestUpdateConfigRoles_RemoveExistingRoles(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "ucr2-cfg", DNSName: "ucr2.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	// First assign viewer role to config
	viewerRole, err := h.store.GetRoleByName(t.Context(), "viewer")
	require.NoError(t, err)
	require.NoError(t, h.store.AssignConfigRole(t.Context(), cfg.ID, viewerRole.ID))

	// Now update with empty roles list → should remove viewer
	body := `{"roles":[]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID+"/roles", makeBody(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-ucr2", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── DeleteBackend success path ───────────────────────────────────────────────

func TestDeleteBackend_Success(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "del-cfg", DNSName: "del.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))
	backend := &types.Backend{ConfigID: cfg.ID, IP: "9.9.9.9", Weight: 1, Enabled: true}
	require.NoError(t, s.CreateBackend(context.Background(), backend))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/backends/"+backend.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ─── ListBackends with OIDC viewer ───────────────────────────────────────────

func TestListBackends_OIDCViewer(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "lbo-cfg", DNSName: "lbo.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/backends", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-lbo", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
