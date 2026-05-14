package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/types"
)

// SystemStatus represents the overall system status.
type SystemStatus struct {
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	Timestamp   time.Time `json:"timestamp"`
	HealthStats struct {
		Running     bool `json:"running"`
		WorkerCount int  `json:"worker_count"`
	} `json:"health_engine"`
}

// ConfigStatus represents the status of a specific config.
type ConfigStatus struct {
	ConfigID       string                       `json:"config_id"`
	Name           string                       `json:"name"`
	BackendCount   int                          `json:"backend_count"`
	HealthyCount   int                          `json:"healthy_count"`
	UnhealthyCount int                          `json:"unhealthy_count"`
	UnknownCount   int                          `json:"unknown_count"`
	HealthStates   map[string]types.HealthState `json:"health_states"`
}

// GetSystemStatus handles GET /api/v1/status.
func (h *Handlers) GetSystemStatus(c *gin.Context) {
	stats := h.health.Stats()

	status := SystemStatus{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	status.HealthStats.Running = stats.Running
	status.HealthStats.WorkerCount = stats.WorkerCount

	sendSuccess(c, status)
}

// GetConfigStatus handles GET /api/v1/configs/:id/status.
func (h *Handlers) GetConfigStatus(c *gin.Context) {
	configID := c.Param("id")
	if configID == "" {
		sendValidationError(c, "config id is required")
		return
	}

	// Get config details
	cfg, err := h.store.GetConfig(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Get all backends for this config
	backends, err := h.store.ListBackends(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Get health states
	healthStates, err := h.store.GetHealthStates(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Count statuses
	var healthyCount, unhealthyCount, unknownCount int
	for _, backend := range backends {
		state, exists := healthStates[backend.ID]
		if !exists {
			unknownCount++
			continue
		}
		switch state.Status {
		case types.StatusHealthy:
			healthyCount++
		case types.StatusUnhealthy:
			unhealthyCount++
		default:
			unknownCount++
		}
	}

	status := ConfigStatus{
		ConfigID:       cfg.ID,
		Name:           cfg.Name,
		BackendCount:   len(backends),
		HealthyCount:   healthyCount,
		UnhealthyCount: unhealthyCount,
		UnknownCount:   unknownCount,
		HealthStates:   healthStates,
	}

	sendSuccess(c, status)
}

// HealthHandler provides a simple health check endpoint.
// This is useful for load balancers and monitoring.
func (h *Handlers) HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// ReadyHandler provides a readiness check endpoint.
func (h *Handlers) ReadyHandler(c *gin.Context) {
	// Check if health engine is running
	if !h.health.IsRunning() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "health engine not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
