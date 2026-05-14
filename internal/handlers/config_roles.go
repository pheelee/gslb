package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/types"
)

// GetConfigRoles handles GET /api/v1/configs/:id/roles.
// Returns the roles that are permitted to access this configuration.
func (h *Handlers) GetConfigRoles(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return
	}

	if _, err := h.store.GetConfig(c.Request.Context(), id); err != nil {
		h.handleStoreError(c, err)
		return
	}

	roles, err := h.store.GetConfigRoles(c.Request.Context(), id)
	if err != nil {
		sendInternalError(c, "failed to get config roles")
		return
	}

	if roles == nil {
		roles = []types.Role{}
	}
	sendSuccess(c, roles)
}

// UpdateConfigRoles handles PUT /api/v1/configs/:id/roles.
// Replaces the full set of roles for this configuration.
func (h *Handlers) UpdateConfigRoles(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return
	}

	if _, err := h.store.GetConfig(c.Request.Context(), id); err != nil {
		h.handleStoreError(c, err)
		return
	}

	var req struct {
		Roles []string `json:"roles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	// Remove all existing role assignments for this config
	existing, err := h.store.GetConfigRoles(ctx, id)
	if err != nil {
		sendInternalError(c, "failed to get existing config roles")
		return
	}
	for _, r := range existing {
		if err := h.store.RemoveConfigRole(ctx, id, r.ID); err != nil {
			sendInternalError(c, "failed to remove config role")
			return
		}
	}

	// Assign the new set
	for _, roleName := range req.Roles {
		role, err := h.store.GetRoleByName(ctx, roleName)
		if err != nil {
			continue // unknown role — skip
		}
		if err := h.store.AssignConfigRole(ctx, id, role.ID); err != nil {
			sendInternalError(c, "failed to assign config role")
			return
		}
	}

	h.logAudit(ctx, "config:roles:update", "config", id, "", id, "", c)
	c.JSON(http.StatusOK, Response{Data: "roles updated"})
}
