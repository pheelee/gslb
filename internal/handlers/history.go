package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/types"
)

// GetBackendHistory handles GET /api/v1/backends/:id/history?range=24h|7d|30d.
func (h *Handlers) GetBackendHistory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return
	}

	// Verify backend exists.
	if _, err := h.store.GetBackend(c.Request.Context(), id); err != nil {
		h.handleStoreError(c, err)
		return
	}

	rangeParam := c.DefaultQuery("range", "24h")

	now := time.Now()
	since30d := now.Add(-30 * 24 * time.Hour)

	// Fetch the full 30d window once; derive all ranges from it.
	allBuckets, err := h.store.GetHealthHistory(c.Request.Context(), id, since30d)
	if err != nil {
		sendInternalError(c, "failed to get history")
		return
	}

	since24h := now.Add(-24 * time.Hour)
	since7d := now.Add(-7 * 24 * time.Hour)

	var requested, buckets7d, buckets24h []types.HealthHistoryBucket

	var sinceRequested time.Time
	switch rangeParam {
	case "24h":
		sinceRequested = since24h
	case "7d":
		sinceRequested = since7d
	case "30d":
		sinceRequested = since30d
	default: // "3h"
		sinceRequested = now.Add(-3 * time.Hour)
	}

	for _, b := range allBuckets {
		if !b.BucketStart.Before(since24h) {
			buckets24h = append(buckets24h, b)
		}
		if !b.BucketStart.Before(since7d) {
			buckets7d = append(buckets7d, b)
		}
		if !b.BucketStart.Before(sinceRequested) {
			requested = append(requested, b)
		}
	}

	if requested == nil {
		requested = []types.HealthHistoryBucket{}
	}

	sendSuccess(c, types.BackendHistoryResponse{
		Buckets: requested,
		Uptime: map[string]float64{
			"24h": calculateUptime(buckets24h),
			"7d":  calculateUptime(buckets7d),
			"30d": calculateUptime(allBuckets),
		},
	})
}

// calculateUptime returns success / (success + failure) * 100, or 0 if no data.
func calculateUptime(buckets []types.HealthHistoryBucket) float64 {
	var success, total int
	for _, b := range buckets {
		success += b.SuccessCount
		total += b.SuccessCount + b.FailureCount
	}
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}
