package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// CreateConfigRequest represents a request to create a config.
type CreateConfigRequest struct {
	Name     string         `json:"name" binding:"required"`
	DNSName  string         `json:"dns_name" binding:"required"`
	DNSTTL   int            `json:"dns_ttl" binding:"min=1"`
	LBMethod types.LBMethod `json:"lb_method" binding:"required,oneof=round_robin weighted"`
	// Roles lists the role names that may access this config (OIDC mode only).
	// Empty means all authenticated users may access it.
	Roles []string `json:"roles"`
}

// UpdateConfigRequest represents a request to update a config.
type UpdateConfigRequest struct {
	Name     string         `json:"name" binding:"required"`
	DNSName  string         `json:"dns_name" binding:"required"`
	DNSTTL   int            `json:"dns_ttl" binding:"min=1"`
	LBMethod types.LBMethod `json:"lb_method" binding:"required,oneof=round_robin weighted"`
}

// CreateConfig handles POST /api/v1/configs.
func (h *Handlers) CreateConfig(c *gin.Context) {
	var req CreateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	cfg := &types.Config{
		Name:     req.Name,
		DNSName:  req.DNSName,
		DNSTTL:   req.DNSTTL,
		LBMethod: req.LBMethod,
	}

	if err := h.store.CreateConfig(c.Request.Context(), cfg); err != nil {
		h.handleStoreError(c, err)
		return
	}

	// Assign roles when OIDC is enabled
	if h.oidcService != nil {
		rolesToAssign := req.Roles
		if len(rolesToAssign) == 0 {
			// Default: inherit the requesting user's roles
			if userRoles, ok := c.Get("userRoles"); ok {
				rolesToAssign, _ = userRoles.([]string)
			}
		}
		for _, roleName := range rolesToAssign {
			role, err := h.store.GetRoleByName(c.Request.Context(), roleName)
			if err != nil {
				continue
			}
			if err := h.store.AssignConfigRole(c.Request.Context(), cfg.ID, role.ID); err != nil {
				slog.Warn("failed to assign role to config", "role", roleName, "config_id", cfg.ID, "error", err)
			}
		}
	}

	changes := []ChangeEntry{
		{Field: "name", Old: "", New: cfg.Name},
		{Field: "dns_name", Old: "", New: cfg.DNSName},
		{Field: "dns_ttl", Old: "", New: strconv.Itoa(cfg.DNSTTL)},
		{Field: "lb_method", Old: "", New: string(cfg.LBMethod)},
	}
	h.logAudit(c.Request.Context(), "config:create", "config", cfg.ID, cfg.Name, cfg.ID, buildDetails(changes), c)
	sendSuccess(c, cfg)
}

// GetConfig handles GET /api/v1/configs/:id.
func (h *Handlers) GetConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return
	}

	cfg, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, cfg)
}

// UpdateConfig handles PUT /api/v1/configs/:id.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	// Fetch existing config to compute diff for audit log.
	existing, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	cfg := &types.Config{
		ID:       id,
		Name:     req.Name,
		DNSName:  req.DNSName,
		DNSTTL:   req.DNSTTL,
		LBMethod: req.LBMethod,
	}

	if err := h.store.UpdateConfig(c.Request.Context(), cfg); err != nil {
		h.handleStoreError(c, err)
		return
	}

	var changes []ChangeEntry
	if existing.Name != cfg.Name {
		changes = append(changes, ChangeEntry{Field: "name", Old: existing.Name, New: cfg.Name})
	}
	if existing.DNSName != cfg.DNSName {
		changes = append(changes, ChangeEntry{Field: "dns_name", Old: existing.DNSName, New: cfg.DNSName})
	}
	if existing.DNSTTL != cfg.DNSTTL {
		changes = append(changes, ChangeEntry{Field: "dns_ttl", Old: strconv.Itoa(existing.DNSTTL), New: strconv.Itoa(cfg.DNSTTL)})
	}
	if existing.LBMethod != cfg.LBMethod {
		changes = append(changes, ChangeEntry{Field: "lb_method", Old: string(existing.LBMethod), New: string(cfg.LBMethod)})
	}
	h.logAudit(c.Request.Context(), "config:update", "config", cfg.ID, cfg.Name, cfg.ID, buildDetails(changes), c)
	sendSuccess(c, cfg)
}

// DeleteConfig handles DELETE /api/v1/configs/:id.
func (h *Handlers) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return
	}

	// Get config name before deleting for audit log
	cfg, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if err := h.store.DeleteConfig(c.Request.Context(), id); err != nil {
		h.handleStoreError(c, err)
		return
	}

	changes := []ChangeEntry{
		{Field: "name", Old: cfg.Name, New: ""},
		{Field: "dns_name", Old: cfg.DNSName, New: ""},
		{Field: "dns_ttl", Old: strconv.Itoa(cfg.DNSTTL), New: ""},
		{Field: "lb_method", Old: string(cfg.LBMethod), New: ""},
	}
	h.logAudit(c.Request.Context(), "config:delete", "config", id, cfg.Name, id, buildDetails(changes), c)
	c.Status(http.StatusNoContent)
}

// ListConfigs handles GET /api/v1/configs.
// When OIDC is enabled: admins get all configs; other roles get only the configs
// they have access to via config_roles. When OIDC is disabled, all configs are returned.
func (h *Handlers) ListConfigs(c *gin.Context) {
	ctx := c.Request.Context()

	if h.oidcService != nil {
		userRolesVal, _ := c.Get("userRoles")
		userRoles, _ := userRolesVal.([]string)
		for _, r := range userRoles {
			if r == "admin" {
				// Admins see everything — fall through to full list below
				goto fullList
			}
		}
		// Non-admin: filter to configs accessible via their roles
		userIDVal, _ := c.Get("userID")
		userID, _ := userIDVal.(string)
		if userID != "" {
			accessibleIDs, err := h.store.ListConfigsForUser(ctx, userID)
			if err != nil {
				sendInternalError(c, "failed to list configs")
				return
			}
			if len(accessibleIDs) == 0 {
				sendSuccess(c, []types.Config{})
				return
			}
			idSet := make(map[string]bool, len(accessibleIDs))
			for _, id := range accessibleIDs {
				idSet[id] = true
			}
			all, err := h.store.ListConfigs(ctx)
			if err != nil {
				sendInternalError(c, "failed to list configs")
				return
			}
			var filtered []types.Config
			for _, cfg := range all {
				if idSet[cfg.ID] {
					filtered = append(filtered, cfg)
				}
			}
			if filtered == nil {
				filtered = []types.Config{}
			}
			sendSuccess(c, filtered)
			return
		}
	}

fullList:
	configs, err := h.store.ListConfigs(ctx)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}
	if configs == nil {
		configs = []types.Config{}
	}
	sendSuccess(c, configs)
}

// getConfigOr404 retrieves a config and sends 404 if not found.
// Returns the config and true if found, nil and false otherwise.
func (h *Handlers) getConfigOr404(c *gin.Context) (*types.Config, bool) {
	id := c.Param("id")
	if id == "" {
		sendValidationError(c, "config id is required")
		return nil, false
	}

	cfg, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			sendNotFound(c, "Config")
		} else {
			h.handleStoreError(c, err)
		}
		return nil, false
	}

	return cfg, true
}
