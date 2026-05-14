package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/auth"
	"github.com/pheelee/gslb/internal/db"
	"github.com/pheelee/gslb/internal/health"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var adminTestDBCounter uint64

// setupTestDBNamed creates a uniquely-named in-memory SQLite database with
// shared-cache so multiple pool connections see the same schema (required for
// nested queries that join the roles/user_roles tables).
func setupTestDBNamed(t *testing.T) *sql.DB {
	t.Helper()
	id := atomic.AddUint64(&adminTestDBCounter, 1)
	dsn := fmt.Sprintf("file:admintest%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", id)
	conn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(conn))
	t.Cleanup(func() { conn.Close() })
	return conn
}

// setupOIDCHandlersSingle uses a shared-cache in-memory DB so nested queries work.
func setupOIDCHandlersSingle(t *testing.T) (*Handlers, *gin.Engine) {
	t.Helper()
	dbConn := setupTestDBNamed(t)
	s := store.New(dbConn, nil)
	eng := health.NewEngine(2)
	eng.Start()
	t.Cleanup(func() { eng.Stop() })
	base := New(s, eng)

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

// ─── ListUsers ─────────────────────────────────────────────────────────────────

func TestListUsers_AdminOnly(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	// Create some users
	for _, name := range []string{"alice", "bob"} {
		u := &types.User{Subject: name, Email: name + "@test.com", Name: name, IsActive: true}
		require.NoError(t, h.store.CreateUser(t.Context(), u))
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/users", nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-user", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	users, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(users), 2)
}

func TestListUsers_ViewerForbidden(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/users", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-user", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ─── AssignUserRole ────────────────────────────────────────────────────────────

func TestAssignUserRole(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	user := &types.User{Subject: "sub-assign", Email: "assign@test.com", Name: "Assign", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))

	role, err := h.store.GetRoleByName(t.Context(), "operator")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{"role_id": role.ID})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/users/"+user.ID+"/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-assign", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify role was assigned
	roles, err := h.store.GetUserRoles(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "operator", roles[0].Name)
}

func TestAssignUserRole_MissingRoleID(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	body := `{}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/users/some-user/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-bad", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── RemoveUserRole ────────────────────────────────────────────────────────────

func TestRemoveUserRole(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	user := &types.User{Subject: "sub-remove", Email: "remove@test.com", Name: "Remove", IsActive: true}
	require.NoError(t, h.store.CreateUser(t.Context(), user))

	role, err := h.store.GetRoleByName(t.Context(), "viewer")
	require.NoError(t, err)

	require.NoError(t, h.store.AssignRole(t.Context(), user.ID, role.ID, "test"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/admin/users/"+user.ID+"/roles/"+role.ID, nil)
	withCookie(req, "gslb_session", sessionToken(t, "admin-remove", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	roles, err := h.store.GetUserRoles(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, roles)
}

// ─── GetCurrentUser ────────────────────────────────────────────────────────────

func TestGetCurrentUser(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	// The user "me-user" must exist in the DB for GetUserByID to work
	user := &types.User{
		ID:       "me-user",
		Subject:  "me-user",
		Email:    "me-user@test.com",
		Name:     "me-user",
		IsActive: true,
	}
	require.NoError(t, h.store.CreateUser(t.Context(), user))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/user", nil)
	withCookie(req, "gslb_session", sessionToken(t, "me-user", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, "me-user@test.com", result["email"])
}

// ─── ListRoles ─────────────────────────────────────────────────────────────────

func TestListRoles(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/roles", nil)
	withCookie(req, "gslb_session", sessionToken(t, "viewer-roles", []string{"viewer"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	roles, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(roles), 3) // admin, operator, viewer
}

// ─── UpdateConfigRoles ─────────────────────────────────────────────────────────

func TestUpdateConfigRoles(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "roles-update", DNSName: "ru.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	// Assign viewer role initially
	viewerRole, _ := h.store.GetRoleByName(t.Context(), "viewer")
	require.NoError(t, h.store.AssignConfigRole(t.Context(), cfg.ID, viewerRole.ID))

	// Replace with operator role
	body, _ := json.Marshal(map[string][]string{"roles": {"operator"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID+"/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-roles", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify only operator role is now assigned
	roles, err := h.store.GetConfigRoles(t.Context(), cfg.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "operator", roles[0].Name)
}

func TestUpdateConfigRoles_UnknownRolesSkipped(t *testing.T) {
	h, router := setupOIDCHandlersSingle(t)

	cfg := &types.Config{Name: "skip-roles", DNSName: "sr.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, h.store.CreateConfig(t.Context(), cfg))

	body, _ := json.Marshal(map[string][]string{"roles": {"nonexistent-role"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID+"/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-skip", []string{"admin"}))
	router.ServeHTTP(w, req)

	// Unknown roles are skipped, no error
	assert.Equal(t, http.StatusOK, w.Code)

	roles, err := h.store.GetConfigRoles(t.Context(), cfg.ID)
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestUpdateConfigRoles_ConfigNotFound(t *testing.T) {
	_, router := setupOIDCHandlersSingle(t)

	body, _ := json.Marshal(map[string][]string{"roles": {"viewer"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/nonexistent/roles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	withCookie(req, "gslb_session", sessionToken(t, "admin-404", []string{"admin"}))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── DNS Reconcile Error surface ───────────────────────────────────────────────

func TestGetConfig_HasReconcileError(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "err-config",
		DNSName:  "err.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(context.Background(), cfg))
	require.NoError(t, h.store.SetConfigReconcileError(context.Background(), cfg.ID, "TTL must be between 60 and 86400"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, "TTL must be between 60 and 86400", result["last_reconcile_error"])
}

func TestGetConfig_ReconcileErrorClearedOnSuccess(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "clear-err-config",
		DNSName:  "clear.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	require.NoError(t, h.store.CreateConfig(context.Background(), cfg))
	require.NoError(t, h.store.SetConfigReconcileError(context.Background(), cfg.ID, "some error"))
	require.NoError(t, h.store.SetConfigReconcileError(context.Background(), cfg.ID, ""))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	// Either absent or empty string
	errVal, _ := result["last_reconcile_error"].(string)
	assert.Empty(t, errVal)
}
