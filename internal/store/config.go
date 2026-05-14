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

// CreateConfig creates a new config record.
func (s *configStore) CreateConfig(ctx context.Context, cfg *types.Config) error {
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()

	query := `
		INSERT INTO configs (id, name, dns_name, dns_ttl, lb_method, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		cfg.ID, cfg.Name, cfg.DNSName, cfg.DNSTTL, cfg.LBMethod, cfg.CreatedAt, cfg.UpdatedAt,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("insert config: %w", err)
	}
	return nil
}

// GetConfig retrieves a config by ID.
func (s *configStore) GetConfig(ctx context.Context, id string) (*types.Config, error) {
	query := `
		SELECT id, name, dns_name, dns_ttl, lb_method, created_at, updated_at, COALESCE(last_reconcile_error, '')
		FROM configs
		WHERE id = ?
	`
	cfg := &types.Config{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&cfg.ID, &cfg.Name, &cfg.DNSName, &cfg.DNSTTL, &cfg.LBMethod, &cfg.CreatedAt, &cfg.UpdatedAt,
		&cfg.LastReconcileError,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query config: %w", err)
	}
	return cfg, nil
}

// UpdateConfig updates an existing config record.
func (s *configStore) UpdateConfig(ctx context.Context, cfg *types.Config) error {
	cfg.UpdatedAt = time.Now()

	query := `
		UPDATE configs
		SET name = ?, dns_name = ?, dns_ttl = ?, lb_method = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := s.db.ExecContext(ctx, query,
		cfg.Name, cfg.DNSName, cfg.DNSTTL, cfg.LBMethod, cfg.UpdatedAt, cfg.ID,
	)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
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

// DeleteConfig deletes a config by ID.
func (s *configStore) DeleteConfig(ctx context.Context, id string) error {
	query := `DELETE FROM configs WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
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

// ListConfigs returns all configs.
func (s *configStore) ListConfigs(ctx context.Context) ([]types.Config, error) {
	query := `
		SELECT id, name, dns_name, dns_ttl, lb_method, created_at, updated_at, COALESCE(last_reconcile_error, '')
		FROM configs
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query configs: %w", err)
	}
	defer rows.Close()

	var configs []types.Config
	for rows.Next() {
		var cfg types.Config
		if err := rows.Scan(
			&cfg.ID, &cfg.Name, &cfg.DNSName, &cfg.DNSTTL, &cfg.LBMethod, &cfg.CreatedAt, &cfg.UpdatedAt,
			&cfg.LastReconcileError,
		); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return configs, nil
}

// SetConfigReconcileError persists the last reconciler error for a config.
// Passing an empty string clears any existing error.
func (s *configStore) SetConfigReconcileError(ctx context.Context, id string, errMsg string) error {
	var query string
	var arg any
	if errMsg == "" {
		query = `UPDATE configs SET last_reconcile_error = NULL WHERE id = ?`
		arg = id
	} else {
		query = `UPDATE configs SET last_reconcile_error = ? WHERE id = ?`
		_, err := s.db.ExecContext(ctx, query, errMsg, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, query, arg)
	return err
}

// Helper to check for foreign key constraint violations
func isForeignKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "FOREIGN KEY") || contains(err.Error(), "foreign key"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
