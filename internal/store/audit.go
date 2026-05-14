package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pheelee/gslb/internal/types"
)

// auditLogStore implements AuditLogStore interface.
type auditLogStore struct {
	db *sql.DB
}

// CreateAuditLog creates a new audit log entry.
func (s *auditLogStore) CreateAuditLog(ctx context.Context, log *types.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	log.CreatedAt = time.Now()

	query := `
		INSERT INTO audit_logs (id, action, entity_type, entity_id, entity_name, config_id, details, user_id, ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		log.ID, log.Action, log.EntityType, log.EntityID, log.EntityName, log.ConfigID, log.Details, log.UserID, log.IPAddress, log.UserAgent, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ListAuditLogs returns audit logs with optional filtering and pagination.
func (s *auditLogStore) ListAuditLogs(ctx context.Context, filter AuditLogFilter, limit int, offset int) ([]types.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var conditions []string
	args := make([]interface{}, 0, 6)

	if filter.UserID != "" {
		conditions = append(conditions, "COALESCE(user_id,'') = ?")
		args = append(args, filter.UserID)
	}
	if filter.EntityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, filter.EntityType)
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.From != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.To)
	}

	query := `SELECT id, action, entity_type, entity_id, entity_name, COALESCE(config_id, ''), details, COALESCE(user_id, ''), ip_address, user_agent, created_at FROM audit_logs`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []types.AuditLog
	for rows.Next() {
		var log types.AuditLog
		if err := rows.Scan(
			&log.ID, &log.Action, &log.EntityType, &log.EntityID, &log.EntityName,
			&log.ConfigID, &log.Details, &log.UserID, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return logs, nil
}

// GetAuditLog retrieves a single audit log by ID.
func (s *auditLogStore) GetAuditLog(ctx context.Context, id string) (*types.AuditLog, error) {
	query := `
		SELECT id, action, entity_type, entity_id, entity_name, COALESCE(config_id, ''), details, COALESCE(user_id, ''), ip_address, user_agent, created_at
		FROM audit_logs
		WHERE id = ?
	`
	log := &types.AuditLog{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID, &log.Action, &log.EntityType, &log.EntityID, &log.EntityName,
		&log.ConfigID, &log.Details, &log.UserID, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	return log, nil
}

// GetAuditLogsForEntity returns audit logs for a specific entity.
func (s *auditLogStore) GetAuditLogsForEntity(ctx context.Context, entityType string, entityID string, limit int) ([]types.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT id, action, entity_type, entity_id, entity_name, COALESCE(config_id, ''), details, COALESCE(user_id, ''), ip_address, user_agent, created_at
		FROM audit_logs
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, entityType, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit logs for entity: %w", err)
	}
	defer rows.Close()

	var logs []types.AuditLog
	for rows.Next() {
		var log types.AuditLog
		if err := rows.Scan(
			&log.ID, &log.Action, &log.EntityType, &log.EntityID, &log.EntityName,
			&log.ConfigID, &log.Details, &log.UserID, &log.IPAddress, &log.UserAgent, &log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return logs, nil
}

// DeleteOldAuditLogs removes audit logs older than the specified duration.
func (s *auditLogStore) DeleteOldAuditLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM audit_logs WHERE created_at < ?`
	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old audit logs: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return rows, nil
}

// CountAuditLogs returns the number of audit log entries matching the filter.
func (s *auditLogStore) CountAuditLogs(ctx context.Context, filter AuditLogFilter) (int64, error) {
	var conditions []string
	args := make([]interface{}, 0, 5)

	if filter.UserID != "" {
		conditions = append(conditions, "COALESCE(user_id,'') = ?")
		args = append(args, filter.UserID)
	}
	if filter.EntityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, filter.EntityType)
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.From != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.To)
	}

	query := `SELECT COUNT(*) FROM audit_logs`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count audit logs: %w", err)
	}
	return count, nil
}
