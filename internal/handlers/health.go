package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// ConfigureHealthCheckRequest represents a request to configure health check.
type ConfigureHealthCheckRequest struct {
	Type               string `json:"type" binding:"required,oneof=icmp tcp"`
	IntervalSeconds    int    `json:"interval_seconds" binding:"min=1,max=300"`
	TimeoutSeconds     int    `json:"timeout_seconds" binding:"min=1,max=60"`
	ThresholdHealthy   int    `json:"threshold_healthy" binding:"min=1,max=10"`
	ThresholdUnhealthy int    `json:"threshold_unhealthy" binding:"min=1,max=10"`
}

// ConfigureHealthCheck handles POST /api/v1/configs/:id/health-check.
func (h *Handlers) ConfigureHealthCheck(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		sendValidationError(c, "config id is required")
		return
	}

	// Verify config exists
	_, err := h.store.GetConfig(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	var req ConfigureHealthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	hc := &types.HealthCheck{
		ConfigID:           configID,
		Type:               req.Type,
		IntervalSeconds:    req.IntervalSeconds,
		TimeoutSeconds:     req.TimeoutSeconds,
		ThresholdHealthy:   req.ThresholdHealthy,
		ThresholdUnhealthy: req.ThresholdUnhealthy,
	}

	// Get existing health check to determine create vs update and compute audit diff.
	existingHC, getErr := h.store.GetHealthCheck(c.Request.Context(), configID)
	isUpdate := getErr == nil

	if isUpdate {
		hc.ID = existingHC.ID
		if updateErr := h.store.UpdateHealthCheck(c.Request.Context(), hc); updateErr != nil {
			h.handleStoreError(c, updateErr)
			return
		}
	} else {
		if createErr := h.store.CreateHealthCheck(c.Request.Context(), hc); createErr != nil {
			h.handleStoreError(c, createErr)
			return
		}
	}

	var changes []ChangeEntry
	if isUpdate {
		if existingHC.Type != hc.Type {
			changes = append(changes, ChangeEntry{Field: "type", Old: existingHC.Type, New: hc.Type})
		}
		if existingHC.IntervalSeconds != hc.IntervalSeconds {
			changes = append(changes, ChangeEntry{Field: "interval_seconds", Old: strconv.Itoa(existingHC.IntervalSeconds), New: strconv.Itoa(hc.IntervalSeconds)})
		}
		if existingHC.TimeoutSeconds != hc.TimeoutSeconds {
			changes = append(changes, ChangeEntry{Field: "timeout_seconds", Old: strconv.Itoa(existingHC.TimeoutSeconds), New: strconv.Itoa(hc.TimeoutSeconds)})
		}
		if existingHC.ThresholdHealthy != hc.ThresholdHealthy {
			changes = append(changes, ChangeEntry{Field: "threshold_healthy", Old: strconv.Itoa(existingHC.ThresholdHealthy), New: strconv.Itoa(hc.ThresholdHealthy)})
		}
		if existingHC.ThresholdUnhealthy != hc.ThresholdUnhealthy {
			changes = append(changes, ChangeEntry{Field: "threshold_unhealthy", Old: strconv.Itoa(existingHC.ThresholdUnhealthy), New: strconv.Itoa(hc.ThresholdUnhealthy)})
		}
		h.logAudit(c.Request.Context(), "health_check:update", "health_check", hc.ID, hc.Type, configID, buildDetails(changes), c)
	} else {
		changes = []ChangeEntry{
			{Field: "type", Old: "", New: hc.Type},
			{Field: "interval_seconds", Old: "", New: strconv.Itoa(hc.IntervalSeconds)},
			{Field: "timeout_seconds", Old: "", New: strconv.Itoa(hc.TimeoutSeconds)},
			{Field: "threshold_healthy", Old: "", New: strconv.Itoa(hc.ThresholdHealthy)},
			{Field: "threshold_unhealthy", Old: "", New: strconv.Itoa(hc.ThresholdUnhealthy)},
		}
		h.logAudit(c.Request.Context(), "health_check:create", "health_check", hc.ID, hc.Type, configID, buildDetails(changes), c)
	}

	sendSuccess(c, hc)
}

// GetHealthCheck handles GET /api/v1/configs/:id/health-check.
func (h *Handlers) GetHealthCheck(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		sendValidationError(c, "config id is required")
		return
	}

	// Verify config exists
	_, err := h.store.GetConfig(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	hc, err := h.store.GetHealthCheck(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, hc)
}

// GetBackendHealth handles GET /api/v1/backends/:id/health.
func (h *Handlers) GetBackendHealth(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return
	}

	// Verify backend exists
	_, err := h.store.GetBackend(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	state, err := h.store.GetHealthState(c.Request.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			// Return unknown status if no health state exists
			sendSuccess(c, &types.HealthState{
				BackendID: id,
				Status:    types.StatusUnknown,
			})
			return
		}
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, state)
}

// DeleteHealthCheck handles DELETE /api/v1/configs/:id/health-check.
func (h *Handlers) DeleteHealthCheck(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		sendValidationError(c, "config id is required")
		return
	}

	// Verify config exists
	_, err := h.store.GetConfig(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Fetch health check before deleting for audit log.
	hc, err := h.store.GetHealthCheck(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if err := h.store.DeleteHealthCheck(c.Request.Context(), configID); err != nil {
		h.handleStoreError(c, err)
		return
	}

	changes := []ChangeEntry{
		{Field: "type", Old: hc.Type, New: ""},
		{Field: "interval_seconds", Old: strconv.Itoa(hc.IntervalSeconds), New: ""},
		{Field: "timeout_seconds", Old: strconv.Itoa(hc.TimeoutSeconds), New: ""},
		{Field: "threshold_healthy", Old: strconv.Itoa(hc.ThresholdHealthy), New: ""},
		{Field: "threshold_unhealthy", Old: strconv.Itoa(hc.ThresholdUnhealthy), New: ""},
	}
	h.logAudit(c.Request.Context(), "health_check:delete", "health_check", hc.ID, hc.Type, configID, buildDetails(changes), c)

	c.Status(http.StatusNoContent)
}
