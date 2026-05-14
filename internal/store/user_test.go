package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/db"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDBCounter uint64

// newTestDBSingle creates a unique named in-memory database with shared-cache mode.
// This allows multiple connections to see the same schema without connection-pool deadlocks.
func newTestDBSingle(t *testing.T) *sql.DB {
	t.Helper()
	id := atomic.AddUint64(&testDBCounter, 1)
	dsn := fmt.Sprintf("file:testdb%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", id)
	conn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	err = db.Migrate(conn)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

// ─── User CRUD ─────────────────────────────────────────────────────────────────

func TestCreateUser(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{
		Subject:  "sub-1",
		Email:    "alice@example.com",
		Name:     "Alice",
		IsActive: true,
	}

	err := s.CreateUser(ctx, user)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.False(t, user.CreatedAt.IsZero())
}

func TestGetUserByID(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-2", Email: "bob@example.com", Name: "Bob", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	got, err := s.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "bob@example.com", got.Email)
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	_, err := s.GetUserByID(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetUserBySubject(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-3", Email: "carol@example.com", Name: "Carol", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	got, err := s.GetUserBySubject(ctx, "sub-3")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestGetUserBySubject_NotFound(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	_, err := s.GetUserBySubject(context.Background(), "unknown-subject")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateUser(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-4", Email: "dave@example.com", Name: "Dave", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	user.Email = "dave-updated@example.com"
	user.Name = "Dave Updated"
	user.LastLoginAt = time.Now()

	err := s.UpdateUser(ctx, user)
	require.NoError(t, err)

	got, err := s.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "dave-updated@example.com", got.Email)
	assert.Equal(t, "Dave Updated", got.Name)
}

func TestUpdateUser_NotFound(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	err := s.UpdateUser(context.Background(), &types.User{ID: "ghost", Email: "x@x.com"})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListUsers(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	for _, name := range []string{"u1", "u2", "u3"} {
		u := &types.User{Subject: name, Email: name + "@example.com", Name: name, IsActive: true}
		require.NoError(t, s.CreateUser(ctx, u))
	}

	users, err := s.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 3)
}

// ─── Role operations ────────────────────────────────────────────────────────────

func TestGetRoleByName(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	// Roles are seeded by migration
	role, err := s.GetRoleByName(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", role.Name)
}

func TestGetRoleByName_NotFound(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	_, err := s.GetRoleByName(context.Background(), "nonexistent-role")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetRoleByID(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	role, err := s.GetRoleByName(ctx, "viewer")
	require.NoError(t, err)

	got, err := s.GetRoleByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "viewer", got.Name)
}

func TestGetRoleByID_NotFound(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	_, err := s.GetRoleByID(context.Background(), "nonexistent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListRoles(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	roles, err := s.ListRoles(context.Background())
	require.NoError(t, err)
	// Seeded roles: admin, operator, viewer
	assert.GreaterOrEqual(t, len(roles), 3)
}

// ─── User-Role assignments ──────────────────────────────────────────────────────

func TestAssignAndRemoveRole(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-r1", Email: "r1@example.com", Name: "R1", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	role, err := s.GetRoleByName(ctx, "operator")
	require.NoError(t, err)

	// Assign
	err = s.AssignRole(ctx, user.ID, role.ID, "test")
	require.NoError(t, err)

	roles, err := s.GetUserRoles(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "operator", roles[0].Name)

	// Re-assign idempotent (INSERT OR IGNORE)
	err = s.AssignRole(ctx, user.ID, role.ID, "test")
	require.NoError(t, err)

	roles, err = s.GetUserRoles(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)

	// Remove
	err = s.RemoveRole(ctx, user.ID, role.ID)
	require.NoError(t, err)

	roles, err = s.GetUserRoles(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, roles)
}

// ─── Config-Role access control ─────────────────────────────────────────────────

func TestConfigRoleAccess(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	cfg := &types.Config{Name: "access-cfg", DNSName: "access.example.com", DNSTTL: 300, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(ctx, cfg))

	user := &types.User{Subject: "sub-a1", Email: "a1@example.com", Name: "A1", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	viewerRole, err := s.GetRoleByName(ctx, "viewer")
	require.NoError(t, err)

	// No access yet
	ok, err := s.CanAccess(ctx, user.ID, cfg.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Assign role to user and config
	require.NoError(t, s.AssignRole(ctx, user.ID, viewerRole.ID, "test"))
	require.NoError(t, s.AssignConfigRole(ctx, cfg.ID, viewerRole.ID))

	ok, err = s.CanAccess(ctx, user.ID, cfg.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	// GetConfigRoles
	roles, err := s.GetConfigRoles(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "viewer", roles[0].Name)

	// GetConfigsForRole
	ids, err := s.GetConfigsForRole(ctx, viewerRole.ID)
	require.NoError(t, err)
	assert.Contains(t, ids, cfg.ID)

	// ListConfigsForUser
	userCfgs, err := s.ListConfigsForUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Contains(t, userCfgs, cfg.ID)

	// RemoveConfigRole
	require.NoError(t, s.RemoveConfigRole(ctx, cfg.ID, viewerRole.ID))
	ok, err = s.CanAccess(ctx, user.ID, cfg.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAssignConfigRole_Idempotent(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	cfg := &types.Config{Name: "idem-cfg", DNSName: "idem.example.com", DNSTTL: 300, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(ctx, cfg))

	viewerRole, err := s.GetRoleByName(ctx, "viewer")
	require.NoError(t, err)

	require.NoError(t, s.AssignConfigRole(ctx, cfg.ID, viewerRole.ID))
	require.NoError(t, s.AssignConfigRole(ctx, cfg.ID, viewerRole.ID)) // second call must not error

	roles, err := s.GetConfigRoles(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
}

// ─── Sessions ──────────────────────────────────────────────────────────────────

func TestSessionCRUD(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-s1", Email: "s1@example.com", Name: "S1", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	sess := &types.SessionRecord{
		JTI:       "jti-test-1",
		UserID:    user.ID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	t.Run("CreateSession", func(t *testing.T) {
		err := s.CreateSessionRecord(ctx, sess)
		require.NoError(t, err)
	})

	t.Run("GetSession", func(t *testing.T) {
		got, err := s.GetSessionRecord(ctx, sess.JTI)
		require.NoError(t, err)
		assert.Equal(t, sess.JTI, got.JTI)
		assert.Equal(t, user.ID, got.UserID)
		assert.Nil(t, got.RevokedAt)
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		_, err := s.GetSessionRecord(ctx, "nonexistent-jti")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("RevokeSession", func(t *testing.T) {
		err := s.RevokeSession(ctx, sess.JTI)
		require.NoError(t, err)

		got, err := s.GetSessionRecord(ctx, sess.JTI)
		require.NoError(t, err)
		assert.NotNil(t, got.RevokedAt)
	})

	t.Run("RevokeSession_AlreadyRevoked", func(t *testing.T) {
		err := s.RevokeSession(ctx, sess.JTI) // already revoked
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestRevokeUserSessions(t *testing.T) {
	s := New(newTestDBSingle(t), nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-s2", Email: "s2@example.com", Name: "S2", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create two sessions
	for _, jti := range []string{"jti-a", "jti-b"} {
		sess := &types.SessionRecord{
			JTI:       jti,
			UserID:    user.ID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, s.CreateSessionRecord(ctx, sess))
	}

	err := s.RevokeUserSessions(ctx, user.ID)
	require.NoError(t, err)

	for _, jti := range []string{"jti-a", "jti-b"} {
		got, err := s.GetSessionRecord(ctx, jti)
		require.NoError(t, err)
		assert.NotNil(t, got.RevokedAt)
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	db := newTestDBSingle(t)
	s := New(db, nil)
	ctx := context.Background()

	user := &types.User{Subject: "sub-s3", Email: "s3@example.com", Name: "S3", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create an already-expired session
	expired := &types.SessionRecord{
		JTI:       "jti-expired",
		UserID:    user.ID,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, s.CreateSessionRecord(ctx, expired))

	// Create an active session
	active := &types.SessionRecord{
		JTI:       "jti-active",
		UserID:    user.ID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, s.CreateSessionRecord(ctx, active))

	deleted, err := s.CleanupExpiredSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Active session still exists
	_, err = s.GetSessionRecord(ctx, "jti-active")
	require.NoError(t, err)

	// Expired session is gone
	_, err = s.GetSessionRecord(ctx, "jti-expired")
	assert.ErrorIs(t, err, ErrNotFound)
}
