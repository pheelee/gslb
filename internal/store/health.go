package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/pheelee/gslb/internal/types"
)

// CreateHealthCheck creates a new health check record.
func (s *healthStore) CreateHealthCheck(ctx context.Context, hc *types.HealthCheck) error {
	if hc.ID == "" {
		hc.ID = uuid.New().String()
	}

	query := `
		INSERT INTO health_checks (id, config_id, type, interval_seconds, timeout_seconds,
			threshold_healthy, threshold_unhealthy)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		hc.ID, hc.ConfigID, hc.Type, hc.IntervalSeconds, hc.TimeoutSeconds,
		hc.ThresholdHealthy, hc.ThresholdUnhealthy,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("insert health check: %w", err)
	}
	return nil
}

// GetHealthCheck retrieves a health check by config ID.
func (s *healthStore) GetHealthCheck(ctx context.Context, configID string) (*types.HealthCheck, error) {
	query := `
		SELECT id, config_id, type, interval_seconds, timeout_seconds,
			threshold_healthy, threshold_unhealthy
		FROM health_checks
		WHERE config_id = ?
	`
	hc := &types.HealthCheck{}
	err := s.db.QueryRowContext(ctx, query, configID).Scan(
		&hc.ID, &hc.ConfigID, &hc.Type, &hc.IntervalSeconds, &hc.TimeoutSeconds,
		&hc.ThresholdHealthy, &hc.ThresholdUnhealthy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query health check: %w", err)
	}
	return hc, nil
}

// UpdateHealthCheck updates an existing health check record.
func (s *healthStore) UpdateHealthCheck(ctx context.Context, hc *types.HealthCheck) error {
	query := `
		UPDATE health_checks
		SET type = ?, interval_seconds = ?, timeout_seconds = ?,
			threshold_healthy = ?, threshold_unhealthy = ?
		WHERE config_id = ?
	`
	result, err := s.db.ExecContext(ctx, query,
		hc.Type, hc.IntervalSeconds, hc.TimeoutSeconds,
		hc.ThresholdHealthy, hc.ThresholdUnhealthy, hc.ConfigID,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("update health check: %w", err)
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

// DeleteHealthCheck deletes a health check by config ID.
func (s *healthStore) DeleteHealthCheck(ctx context.Context, configID string) error {
	query := `DELETE FROM health_checks WHERE config_id = ?`
	result, err := s.db.ExecContext(ctx, query, configID)
	if err != nil {
		return fmt.Errorf("delete health check: %w", err)
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

// UpdateHealthState updates or inserts a health state record.
func (s *healthStore) UpdateHealthState(ctx context.Context, state *types.HealthState) error {
	query := `
		INSERT INTO health_states (backend_id, status, consecutive_successes, consecutive_failures,
			last_check_at, last_healthy_at, last_error)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(backend_id) DO UPDATE SET
			status = excluded.status,
			consecutive_successes = excluded.consecutive_successes,
			consecutive_failures = excluded.consecutive_failures,
			last_check_at = excluded.last_check_at,
			last_healthy_at = excluded.last_healthy_at,
			last_error = excluded.last_error
	`
	_, err := s.db.ExecContext(ctx, query,
		state.BackendID, state.Status, state.ConsecutiveSuccesses, state.ConsecutiveFailures,
		state.LastCheckAt, state.LastHealthyAt, state.LastError,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("%w: %v", ErrForeignKeyViolation, err)
		}
		return fmt.Errorf("update health state: %w", err)
	}
	return nil
}

// GetHealthState retrieves a health state by backend ID.
func (s *healthStore) GetHealthState(ctx context.Context, backendID string) (*types.HealthState, error) {
	query := `
		SELECT backend_id, status, consecutive_successes, consecutive_failures,
			last_check_at, last_healthy_at, last_error
		FROM health_states
		WHERE backend_id = ?
	`
	state := &types.HealthState{}
	var lastCheckAt, lastHealthyAt sql.NullTime
	var lastError sql.NullString
	err := s.db.QueryRowContext(ctx, query, backendID).Scan(
		&state.BackendID, &state.Status, &state.ConsecutiveSuccesses, &state.ConsecutiveFailures,
		&lastCheckAt, &lastHealthyAt, &lastError,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query health state: %w", err)
	}
	if lastCheckAt.Valid {
		state.LastCheckAt = &lastCheckAt.Time
	}
	if lastHealthyAt.Valid {
		state.LastHealthyAt = &lastHealthyAt.Time
	}
	if lastError.Valid {
		state.LastError = lastError.String
	}
	return state, nil
}

// GetHealthStates retrieves all health states for backends in a config.
func (s *healthStore) GetHealthStates(ctx context.Context, configID string) (map[string]types.HealthState, error) {
	query := `
		SELECT hs.backend_id, hs.status, hs.consecutive_successes, hs.consecutive_failures,
			hs.last_check_at, hs.last_healthy_at, hs.last_error
		FROM health_states hs
		JOIN backends b ON b.id = hs.backend_id
		WHERE b.config_id = ?
	`
	rows, err := s.db.QueryContext(ctx, query, configID)
	if err != nil {
		return nil, fmt.Errorf("query health states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]types.HealthState)
	for rows.Next() {
		var state types.HealthState
		var lastCheckAt, lastHealthyAt sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(
			&state.BackendID, &state.Status, &state.ConsecutiveSuccesses, &state.ConsecutiveFailures,
			&lastCheckAt, &lastHealthyAt, &lastError,
		); err != nil {
			return nil, fmt.Errorf("scan health state: %w", err)
		}
		if lastCheckAt.Valid {
			state.LastCheckAt = &lastCheckAt.Time
		}
		if lastHealthyAt.Valid {
			state.LastHealthyAt = &lastHealthyAt.Time
		}
		if lastError.Valid {
			state.LastError = lastError.String
		}
		states[state.BackendID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return states, nil
}
