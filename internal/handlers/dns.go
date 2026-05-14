package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/types"
)

// ConfigureDNSProviderRequest represents a request to configure DNS provider.
type ConfigureDNSProviderRequest struct {
	ProviderType string `json:"provider_type" binding:"required,oneof=cloudflare mock"`
	ConfigJSON   string `json:"config_json" binding:"required"`
}

// ConfigureDNSProvider handles POST /api/v1/configs/:id/dns-provider.
func (h *Handlers) ConfigureDNSProvider(c *gin.Context) {
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

	var req ConfigureDNSProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	provider := &types.DNSProviderConfig{
		ConfigID:     configID,
		ProviderType: req.ProviderType,
		ConfigJSON:   req.ConfigJSON,
	}

	// Get existing provider to determine create vs update and compute audit diff.
	existingProvider, getErr := h.store.GetDNSProvider(c.Request.Context(), configID)
	isUpdate := getErr == nil

	if isUpdate {
		provider.ID = existingProvider.ID
		if updateErr := h.store.UpdateDNSProvider(c.Request.Context(), provider); updateErr != nil {
			h.handleStoreError(c, updateErr)
			return
		}
	} else {
		if createErr := h.store.CreateDNSProvider(c.Request.Context(), provider); createErr != nil {
			h.handleStoreError(c, createErr)
			return
		}
	}

	var changes []ChangeEntry
	if isUpdate {
		if existingProvider.ProviderType != provider.ProviderType {
			changes = append(changes, ChangeEntry{Field: "provider_type", Old: existingProvider.ProviderType, New: provider.ProviderType})
		}
		// Credentials are never logged in plaintext; just indicate they changed.
		if existingProvider.ConfigJSON != provider.ConfigJSON {
			changes = append(changes, ChangeEntry{Field: "credentials", Old: "[redacted]", New: "[updated]"})
		}
		h.logAudit(c.Request.Context(), "dns_provider:update", "dns_provider", provider.ID, provider.ProviderType, configID, buildDetails(changes), c)
	} else {
		changes = []ChangeEntry{
			{Field: "provider_type", Old: "", New: provider.ProviderType},
			{Field: "credentials", Old: "", New: "[configured]"},
		}
		h.logAudit(c.Request.Context(), "dns_provider:create", "dns_provider", provider.ID, provider.ProviderType, configID, buildDetails(changes), c)
	}

	sendSuccess(c, provider)
}

// GetDNSProvider handles GET /api/v1/configs/:id/dns-provider.
func (h *Handlers) GetDNSProvider(c *gin.Context) {
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

	provider, err := h.store.GetDNSProvider(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	sendSuccess(c, provider)
}

// DeleteDNSProvider handles DELETE /api/v1/configs/:id/dns-provider.
func (h *Handlers) DeleteDNSProvider(c *gin.Context) {
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

	// Fetch provider before deleting for audit log.
	provider, err := h.store.GetDNSProvider(c.Request.Context(), configID)
	if err != nil {
		h.handleStoreError(c, err)
		return
	}

	if err := h.store.DeleteDNSProvider(c.Request.Context(), configID); err != nil {
		h.handleStoreError(c, err)
		return
	}

	changes := []ChangeEntry{
		{Field: "provider_type", Old: provider.ProviderType, New: ""},
		{Field: "credentials", Old: "[redacted]", New: ""},
	}
	h.logAudit(c.Request.Context(), "dns_provider:delete", "dns_provider", provider.ID, provider.ProviderType, configID, buildDetails(changes), c)

	c.Status(http.StatusNoContent)
}
