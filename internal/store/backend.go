package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pheelee/gslb/internal/types"
)

// CreateBackend creates a new backend record.
func (s *backendStore) CreateBackend(ctx context.Context, backend *types.Backend) error {
	if backend.ID == "" {
		backend.ID = uuid.New().String()
	}

	query := `
		INSERT INTO backends (id, config_id, ip, port, weight, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	createdAt := time.Now()
	_, err := s.db.ExecContext(ctx, query,
		backend.ID, backend.ConfigID, backend.IP, backend.Port, backend.Weight, backend.Enabled, createdAt,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("insert backend: %w", err)
	}
	return nil
}

// GetBackend retrieves a backend by ID.
func (s *backendStore) GetBackend(ctx context.Context, id string) (*types.Backend, error) {
	query := `
		SELECT id, config_id, ip, port, weight, enabled
		FROM backends
		WHERE id = ?
	`
	backend := &types.Backend{}
	var port sql.NullInt64
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&backend.ID, &backend.ConfigID, &backend.IP, &port, &backend.Weight, &backend.Enabled,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query backend: %w", err)
	}
	if port.Valid {
		p := int(port.Int64)
		backend.Port = &p
	}
	return backend, nil
}

// UpdateBackend updates an existing backend record.
func (s *backendStore) UpdateBackend(ctx context.Context, backend *types.Backend) error {
	query := `
		UPDATE backends
		SET config_id = ?, ip = ?, port = ?, weight = ?, enabled = ?
		WHERE id = ?
	`
	result, err := s.db.ExecContext(ctx, query,
		backend.ConfigID, backend.IP, backend.Port, backend.Weight, backend.Enabled, backend.ID,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("update backend: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteBackend deletes a backend by ID.
func (s *backendStore) DeleteBackend(ctx context.Context, id string) error {
	query := `DELETE FROM backends WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete backend: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBackends returns all backends for a given config ID.
func (s *backendStore) ListBackends(ctx context.Context, configID string) ([]types.Backend, error) {
	query := `
		SELECT id, config_id, ip, port, weight, enabled
		FROM backends
		WHERE config_id = ?
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, configID)
	if err != nil {
		return nil, fmt.Errorf("query backends: %w", err)
	}
	defer rows.Close()

	var backends []types.Backend
	for rows.Next() {
		var backend types.Backend
		var port sql.NullInt64
		if err := rows.Scan(
			&backend.ID, &backend.ConfigID, &backend.IP, &port, &backend.Weight, &backend.Enabled,
		); err != nil {
			return nil, fmt.Errorf("scan backend: %w", err)
		}
		if port.Valid {
			p := int(port.Int64)
			backend.Port = &p
		}
		backends = append(backends, backend)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return backends, nil
}
