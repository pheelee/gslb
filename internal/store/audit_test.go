package store

import (
	"context"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAuditLog(action, entityType, entityID, configID string) *types.AuditLog {
	return &types.AuditLog{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: entityID + "-name",
		ConfigID:   configID,
		Details:    `{"key":"value"}`,
		UserID:     "user-1",
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
	}
}

func TestCreateAuditLog(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log := makeAuditLog("config:create", "config", "cfg-1", "cfg-1")
	err := s.CreateAuditLog(ctx, log)
	require.NoError(t, err)
	assert.NotEmpty(t, log.ID)
	assert.False(t, log.CreatedAt.IsZero())
}

func TestCreateAuditLog_GeneratesIDIfEmpty(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log := makeAuditLog("config:create", "config", "cfg-2", "cfg-2")
	log.ID = "" // ensure empty
	err := s.CreateAuditLog(ctx, log)
	require.NoError(t, err)
	assert.NotEmpty(t, log.ID)
}

func TestGetAuditLog(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log := makeAuditLog("backend:update", "backend", "be-1", "cfg-1")
	require.NoError(t, s.CreateAuditLog(ctx, log))

	got, err := s.GetAuditLog(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.ID, got.ID)
	assert.Equal(t, "backend:update", got.Action)
	assert.Equal(t, "backend", got.EntityType)
	assert.Equal(t, "be-1", got.EntityID)
	assert.Equal(t, "cfg-1", got.ConfigID)
	assert.Equal(t, "user-1", got.UserID)
}

func TestGetAuditLog_NotFound(t *testing.T) {
	s := New(newTestDB(t), nil)
	_, err := s.GetAuditLog(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListAuditLogs_NoFilter(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "cfg", "cfg")))
	}

	logs, err := s.ListAuditLogs(ctx, AuditLogFilter{}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 5)
}

func TestListAuditLogs_FilterByUserID(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log1 := makeAuditLog("config:create", "config", "c1", "c1")
	log1.UserID = "alice"
	require.NoError(t, s.CreateAuditLog(ctx, log1))

	log2 := makeAuditLog("config:create", "config", "c2", "c2")
	log2.UserID = "bob"
	require.NoError(t, s.CreateAuditLog(ctx, log2))

	logs, err := s.ListAuditLogs(ctx, AuditLogFilter{UserID: "alice"}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "alice", logs[0].UserID)
}

func TestListAuditLogs_FilterByEntityType(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c1", "c1")))
	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("backend:create", "backend", "b1", "c1")))

	logs, err := s.ListAuditLogs(ctx, AuditLogFilter{EntityType: "backend"}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "backend", logs[0].EntityType)
}

func TestListAuditLogs_FilterByAction(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c1", "c1")))
	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:delete", "config", "c2", "c2")))
	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c3", "c3")))

	logs, err := s.ListAuditLogs(ctx, AuditLogFilter{Action: "config:create"}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestListAuditLogs_FilterByTimeRange(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	// Create log and record the time before and after
	before := time.Now().Add(-time.Second)
	require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c1", "c1")))
	after := time.Now().Add(time.Second)

	// Filter within range — should find it
	logs, err := s.ListAuditLogs(ctx, AuditLogFilter{From: &before, To: &after}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)

	// Filter with past range — should not find it
	pastStart := time.Now().Add(-10 * time.Minute)
	pastEnd := time.Now().Add(-5 * time.Minute)
	logs, err = s.ListAuditLogs(ctx, AuditLogFilter{From: &pastStart, To: &pastEnd}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 0)
}

func TestListAuditLogs_Pagination(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c", "c")))
	}

	page1, err := s.ListAuditLogs(ctx, AuditLogFilter{}, 3, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 3)

	page2, err := s.ListAuditLogs(ctx, AuditLogFilter{}, 3, 3)
	require.NoError(t, err)
	assert.Len(t, page2, 3)

	// IDs should be different across pages
	assert.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestListAuditLogs_LimitClamped(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()
	// Limit <= 0 defaults to 100; limit > 1000 clamps to 1000.
	// Just ensure no error; with 0 rows result is nil or empty.
	_, err := s.ListAuditLogs(ctx, AuditLogFilter{}, 0, 0)
	require.NoError(t, err)
}

func TestGetAuditLogsForEntity(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log1 := makeAuditLog("config:create", "config", "cfg-abc", "cfg-abc")
	log2 := makeAuditLog("config:update", "config", "cfg-abc", "cfg-abc")
	log3 := makeAuditLog("backend:create", "backend", "be-xyz", "cfg-abc")

	require.NoError(t, s.CreateAuditLog(ctx, log1))
	require.NoError(t, s.CreateAuditLog(ctx, log2))
	require.NoError(t, s.CreateAuditLog(ctx, log3))

	logs, err := s.GetAuditLogsForEntity(ctx, "config", "cfg-abc", 50)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
	for _, l := range logs {
		assert.Equal(t, "config", l.EntityType)
		assert.Equal(t, "cfg-abc", l.EntityID)
	}
}

func TestGetAuditLogsForEntity_DefaultLimit(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log := makeAuditLog("config:create", "config", "e1", "e1")
	require.NoError(t, s.CreateAuditLog(ctx, log))

	// limit <= 0 defaults to 50
	logs, err := s.GetAuditLogsForEntity(ctx, "config", "e1", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestCountAuditLogs_NoFilter(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	for i := 0; i < 7; i++ {
		require.NoError(t, s.CreateAuditLog(ctx, makeAuditLog("config:create", "config", "c", "c")))
	}

	count, err := s.CountAuditLogs(ctx, AuditLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(7), count)
}

func TestCountAuditLogs_WithFilter(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	log1 := makeAuditLog("config:create", "config", "c1", "c1")
	log1.UserID = "carol"
	require.NoError(t, s.CreateAuditLog(ctx, log1))

	log2 := makeAuditLog("backend:delete", "backend", "b1", "c1")
	log2.UserID = "dave"
	require.NoError(t, s.CreateAuditLog(ctx, log2))

	count, err := s.CountAuditLogs(ctx, AuditLogFilter{UserID: "carol"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = s.CountAuditLogs(ctx, AuditLogFilter{EntityType: "backend"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestDeleteOldAuditLogs(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	// Create a log normally (recent)
	recentLog := makeAuditLog("config:create", "config", "c-recent", "c")
	require.NoError(t, s.CreateAuditLog(ctx, recentLog))

	// Insert an old log directly with an old timestamp
	db := newTestDB(t)
	s2 := New(db, nil)
	oldLog := makeAuditLog("config:delete", "config", "c-old", "c")
	oldLog.ID = "old-log-id"
	require.NoError(t, s2.CreateAuditLog(ctx, oldLog))

	// Manually backdate the old log
	_, err := db.ExecContext(ctx, `UPDATE audit_logs SET created_at = ? WHERE id = ?`,
		time.Now().Add(-48*time.Hour), oldLog.ID)
	require.NoError(t, err)

	// Create a recent log in the same store
	recentLog2 := makeAuditLog("config:create", "config", "c-recent2", "c")
	require.NoError(t, s2.CreateAuditLog(ctx, recentLog2))

	// Delete logs older than 24 hours
	deleted, err := s2.DeleteOldAuditLogs(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Recent log should still exist
	remaining, err := s2.ListAuditLogs(ctx, AuditLogFilter{}, 100, 0)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "c-recent2", remaining[0].EntityID)
}

func TestSetConfigReconcileError(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	require.NoError(t, s.CreateConfig(ctx, cfg))

	t.Run("SetError", func(t *testing.T) {
		err := s.SetConfigReconcileError(ctx, cfg.ID, "some DNS error")
		require.NoError(t, err)

		got, err := s.GetConfig(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, "some DNS error", got.LastReconcileError)
	})

	t.Run("ClearError", func(t *testing.T) {
		err := s.SetConfigReconcileError(ctx, cfg.ID, "")
		require.NoError(t, err)

		got, err := s.GetConfig(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Empty(t, got.LastReconcileError)
	})
}
