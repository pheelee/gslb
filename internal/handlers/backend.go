package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// CreateBackendRequest represents a request to create a backend.
type CreateBackendRequest struct {
	IP      string `json:"ip" binding:"required,ip"`
	Port    *int   `json:"port,omitempty"`
	Weight  int    `json:"weight" binding:"min=0,max=100"`
	Enabled bool   `json:"enabled"`
}

// UpdateBackendRequest represents a request to update a backend.
type UpdateBackendRequest struct {
	ConfigID string `json:"config_id,omitempty"`
	IP       string `json:"ip,omitempty"`
	Port     *int   `json:"port,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// AddBackend handles POST /api/v1/configs/:id/backends.
func (h *Handlers) AddBackend(c *gin.Context) {
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

	var req CreateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	backend := &types.Backend{
		ConfigID: configID,
		IP:       req.IP,
		Port:     req.Port,
		Weight:   req.Weight,
		Enabled:  req.Enabled,
	}

	if err := h.store.CreateBackend(c.Request.Context(), backend); err != nil {
		h.handleStoreError(c, err)
		return
	}

	changes := []ChangeEntry{
		{Field: "ip", Old: "", New: backend.IP},
		{Field: "port", Old: "", New: formatPort(backend.Port)},
		{Field: "weight", Old: "", New: strconv.Itoa(backend.Weight)},
		{Field: "enabled", Old: "", New: formatBool(backend.Enabled)},
	}
	h.logAudit(c.Request.Context(), "backend:create", "backend", backend.ID, backend.IP, configID, buildDetails(changes), c)
	h.triggerReconcile(configID)
	sendSuccess(c, backend)
}

// GetBackend handles GET /api/v1/backends/:id.
func (h *Handlers) GetBackend(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return
	}

	backend, err := h.store.GetBackend(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, backend)
}

// UpdateBackend handles PUT /api/v1/backends/:id.
func (h *Handlers) UpdateBackend(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return
	}

	var req UpdateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	// Get existing backend
	existing, err := h.store.GetBackend(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Snapshot old values before mutation for audit diff.
	oldIP := existing.IP
	oldPort := existing.Port
	oldWeight := existing.Weight
	oldEnabled := existing.Enabled

	// Apply updates only for fields that were provided
	if req.ConfigID != "" {
		existing.ConfigID = req.ConfigID
	}
	if req.IP != "" {
		existing.IP = req.IP
	}
	if req.Port != nil {
		existing.Port = req.Port
	}
	if req.Weight != 0 {
		existing.Weight = req.Weight
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.store.UpdateBackend(c.Request.Context(), existing); err != nil {
		h.handleStoreError(c, err)
		return
	}

	var changes []ChangeEntry
	if oldIP != existing.IP {
		changes = append(changes, ChangeEntry{Field: "ip", Old: oldIP, New: existing.IP})
	}
	if formatPort(oldPort) != formatPort(existing.Port) {
		changes = append(changes, ChangeEntry{Field: "port", Old: formatPort(oldPort), New: formatPort(existing.Port)})
	}
	if oldWeight != existing.Weight {
		changes = append(changes, ChangeEntry{Field: "weight", Old: strconv.Itoa(oldWeight), New: strconv.Itoa(existing.Weight)})
	}
	if oldEnabled != existing.Enabled {
		changes = append(changes, ChangeEntry{Field: "enabled", Old: formatBool(oldEnabled), New: formatBool(existing.Enabled)})
	}
	h.logAudit(c.Request.Context(), "backend:update", "backend", existing.ID, existing.IP, existing.ConfigID, buildDetails(changes), c)
	h.triggerReconcile(existing.ConfigID)
	sendSuccess(c, existing)
}

// DeleteBackend handles DELETE /api/v1/backends/:id.
func (h *Handlers) DeleteBackend(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return
	}

	// Get backend info before deleting for audit log
	backend, err := h.store.GetBackend(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if err := h.store.DeleteBackend(c.Request.Context(), id); err != nil {
		h.handleStoreError(c, err)
		return
	}

	changes := []ChangeEntry{
		{Field: "ip", Old: backend.IP, New: ""},
		{Field: "port", Old: formatPort(backend.Port), New: ""},
		{Field: "weight", Old: strconv.Itoa(backend.Weight), New: ""},
		{Field: "enabled", Old: formatBool(backend.Enabled), New: ""},
	}
	h.logAudit(c.Request.Context(), "backend:delete", "backend", id, backend.IP, backend.ConfigID, buildDetails(changes), c)
	h.triggerReconcile(backend.ConfigID)
	c.Status(http.StatusNoContent)
}

// ListBackends handles GET /api/v1/configs/:id/backends.
func (h *Handlers) ListBackends(c *gin.Context) {
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

	backends, err := h.store.ListBackends(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Return empty array instead of nil
	if backends == nil {
		backends = []types.Backend{}
	}

	sendSuccess(c, backends)
}

// getBackendOr404 retrieves a backend and sends 404 if not found.
// Returns the backend and true if found, nil and false otherwise.
func (h *Handlers) getBackendOr404(c *gin.Context) (*types.Backend, bool) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "backend id is required")
		return nil, false
	}

	backend, err := h.store.GetBackend(c.Request.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			sendNotFound(c, "Backend")
		} else {
			h.handleStoreError(c, err)
		}
		return nil, false
	}

	return backend, true
}
