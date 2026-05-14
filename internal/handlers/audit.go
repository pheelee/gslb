package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// ListAuditLogs handles GET /api/v1/audit-logs.
//
// Supports optional query parameters for filtering:
//   - user_id       — exact match on the authenticated user ID
//   - entity_type   — e.g. "config", "backend", "health_check", "dns_provider"
//   - action        — e.g. "config:create", "backend:update"
//   - from          — RFC3339 lower bound on created_at (inclusive)
//   - to            — RFC3339 upper bound on created_at (inclusive)
//   - limit         — max results (default 100, max 1000)
//   - offset        — pagination offset (default 0)
func (h *Handlers) ListAuditLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	var filter store.AuditLogFilter
	if v := c.Query("user_id"); v != "" {
		filter.UserID = v
	}
	if v := c.Query("entity_type"); v != "" {
		filter.EntityType = v
	}
	if v := c.Query("action"); v != "" {
		filter.Action = v
	}
	if v := c.Query("from"); v != "" {
		if t, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
			filter.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
			filter.To = &t
		}
	}

	logs, err := h.store.ListAuditLogs(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if logs == nil {
		logs = []types.AuditLog{}
	}

	sendSuccess(c, logs)
}

// GetAuditLog handles GET /api/v1/audit-logs/:id.
func (h *Handlers) GetAuditLog(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "audit log id is required")
		return
	}

	log, err := h.store.GetAuditLog(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, log)
}

// GetAuditLogsForEntity handles GET /api/v1/audit-logs/entity/:type/:id.
func (h *Handlers) GetAuditLogsForEntity(c *gin.Context) {
	entityType := c.Param("type")
	entityID := c.Param("id")

	if entityType == "" || entityID == "" {
		sendValidationError(c, "entity type and id are required")
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logs, err := h.store.GetAuditLogsForEntity(c.Request.Context(), entityType, entityID, limit)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if logs == nil {
		logs = []types.AuditLog{}
	}

	sendSuccess(c, logs)
}

// GetAuditLogCount handles GET /api/v1/audit-logs/count.
// Accepts the same filter query params as ListAuditLogs to return a filtered count.
func (h *Handlers) GetAuditLogCount(c *gin.Context) {
	var filter store.AuditLogFilter
	if v := c.Query("user_id"); v != "" {
		filter.UserID = v
	}
	if v := c.Query("entity_type"); v != "" {
		filter.EntityType = v
	}
	if v := c.Query("action"); v != "" {
		filter.Action = v
	}
	if v := c.Query("from"); v != "" {
		if t, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
			filter.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
			filter.To = &t
		}
	}

	count, err := h.store.CountAuditLogs(c.Request.Context(), filter)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, gin.H{"count": count})
}
