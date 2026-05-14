package store

import (
	"context"
	"fmt"
	"time"

	"github.com/pheelee/gslb/internal/types"
)

// UpsertHealthHistoryBucket inserts or updates a 5-minute probe bucket.
// On conflict (same backend_id + bucket_start) it accumulates counts.
func (s *healthHistoryStore) UpsertHealthHistoryBucket(ctx context.Context, bucket *types.HealthHistoryBucket) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO health_history (backend_id, bucket_start, success_count, failure_count, selected)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(backend_id, bucket_start) DO UPDATE SET
			success_count = success_count + excluded.success_count,
			failure_count = failure_count + excluded.failure_count,
			selected      = MAX(selected, excluded.selected)
	`, bucket.BackendID, bucket.BucketStart, bucket.SuccessCount, bucket.FailureCount, bucket.Selected)
	if err != nil {
		return fmt.Errorf("upsert health history bucket: %w", err)
	}
	return nil
}

// GetHealthHistory returns all buckets for a backend since the given time, ordered by bucket_start ASC.
func (s *healthHistoryStore) GetHealthHistory(ctx context.Context, backendID string, since time.Time) ([]types.HealthHistoryBucket, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, backend_id, bucket_start, success_count, failure_count, selected, created_at
		FROM health_history
		WHERE backend_id = ? AND bucket_start >= ?
		ORDER BY bucket_start ASC
	`, backendID, since)
	if err != nil {
		return nil, fmt.Errorf("get health history: %w", err)
	}
	defer rows.Close()

	var buckets []types.HealthHistoryBucket
	for rows.Next() {
		var b types.HealthHistoryBucket
		if err := rows.Scan(
			&b.ID, &b.BackendID, &b.BucketStart,
			&b.SuccessCount, &b.FailureCount, &b.Selected,
			&b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan health history row: %w", err)
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// DeleteOldHealthHistory removes buckets older than the given duration.
func (s *healthHistoryStore) DeleteOldHealthHistory(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, `DELETE FROM health_history WHERE bucket_start < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old health history: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
