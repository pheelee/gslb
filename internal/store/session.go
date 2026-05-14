package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pheelee/gslb/internal/types"
)

// sessionStore implements SessionStore interface.
type sessionStore struct {
	db *sql.DB
}

// CreateSessionRecord inserts a new session record.
func (s *sessionStore) CreateSessionRecord(ctx context.Context, sess *types.SessionRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (jti, user_id, created_at, expires_at)
		 VALUES (?, ?, ?, ?)`,
		sess.JTI, sess.UserID, sess.CreatedAt, sess.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetSessionRecord retrieves a session by JTI.
func (s *sessionStore) GetSessionRecord(ctx context.Context, jti string) (*types.SessionRecord, error) {
	sess := &types.SessionRecord{}
	err := s.db.QueryRowContext(ctx,
		`SELECT jti, user_id, created_at, expires_at, revoked_at
		 FROM sessions WHERE jti = ?`, jti,
	).Scan(&sess.JTI, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &sess.RevokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query session: %w", err)
	}
	return sess, nil
}

// RevokeSession marks a session as revoked.
func (s *sessionStore) RevokeSession(ctx context.Context, jti string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE jti = ? AND revoked_at IS NULL`,
		time.Now(), jti,
	)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeUserSessions revokes all active sessions for a user.
func (s *sessionStore) RevokeUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		time.Now(), userID,
	)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

// CleanupExpiredSessions deletes sessions that have expired.
func (s *sessionStore) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup sessions: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return rows, nil
}
