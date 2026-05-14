package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestAuditLog(t *testing.T, h *Handlers, action, entityType, entityID, configID, userID string) *types.AuditLog {
	t.Helper()
	log := &types.AuditLog{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: entityID + "-name",
		ConfigID:   configID,
		UserID:     userID,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test",
		Details:    `{}`,
	}
	require.NoError(t, h.store.CreateAuditLog(context.Background(), log))
	return log
}

func TestListAuditLogs_Empty(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, logs, 0)
}

func TestListAuditLogs_Basic(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "user-1")
	createTestAuditLog(t, h, "backend:create", "backend", "b1", "c1", "user-2")
	createTestAuditLog(t, h, "config:update", "config", "c1", "c1", "user-1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, logs, 3)
}

func TestListAuditLogs_FilterByEntityType(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")
	createTestAuditLog(t, h, "backend:create", "backend", "b1", "c1", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs?entity_type=config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs := resp.Data.([]interface{})
	assert.Len(t, logs, 1)
}

func TestListAuditLogs_FilterByAction(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")
	createTestAuditLog(t, h, "config:delete", "config", "c2", "c2", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs?action=config:create", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs := resp.Data.([]interface{})
	assert.Len(t, logs, 1)
}

func TestListAuditLogs_FilterByUserID(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "alice")
	createTestAuditLog(t, h, "config:create", "config", "c2", "c2", "bob")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs?user_id=alice", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs := resp.Data.([]interface{})
	assert.Len(t, logs, 1)
}

func TestListAuditLogs_FilterByTimeRange(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")

	from := time.Now().Add(-time.Minute).Format(time.RFC3339)
	to := time.Now().Add(time.Minute).Format(time.RFC3339)

	url := fmt.Sprintf("/api/v1/audit-logs?from=%s&to=%s", from, to)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", url, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs := resp.Data.([]interface{})
	assert.Len(t, logs, 1)
}

func TestListAuditLogs_Pagination(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	for i := 0; i < 5; i++ {
		createTestAuditLog(t, h, "config:create", "config", fmt.Sprintf("c%d", i), fmt.Sprintf("c%d", i), "u1")
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs?limit=2&offset=0", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	page1 := resp.Data.([]interface{})
	assert.Len(t, page1, 2)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/audit-logs?limit=2&offset=2", nil)
	router.ServeHTTP(w2, req2)

	var resp2 Response
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	page2 := resp2.Data.([]interface{})
	assert.Len(t, page2, 2)
}

func TestGetAuditLog_Found(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	log := createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/"+log.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, log.ID, result["id"])
	assert.Equal(t, "config:create", result["action"])
}

func TestGetAuditLog_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/nonexistent-id", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAuditLogsForEntity(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "cfg-xyz", "cfg-xyz", "u1")
	createTestAuditLog(t, h, "config:update", "config", "cfg-xyz", "cfg-xyz", "u1")
	createTestAuditLog(t, h, "backend:create", "backend", "be-1", "cfg-xyz", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/entity/config/cfg-xyz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	logs := resp.Data.([]interface{})
	assert.Len(t, logs, 2)
}

func TestGetAuditLogCount_NoFilter(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")
	createTestAuditLog(t, h, "config:create", "config", "c2", "c2", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/count", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(2), data["count"])
}

func TestGetAuditLogCount_WithFilter(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	createTestAuditLog(t, h, "config:create", "config", "c1", "c1", "u1")
	createTestAuditLog(t, h, "backend:create", "backend", "b1", "c1", "u1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/audit-logs/count?entity_type=config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["count"])
}
