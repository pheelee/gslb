// Package handlers provides HTTP REST API handlers for GSLB.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/auth"
	"github.com/pheelee/gslb/internal/health"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// ChangeEntry records a single field change for audit log details.
type ChangeEntry struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// buildDetails serialises a slice of ChangeEntry to a JSON string for audit_logs.details.
func buildDetails(changes []ChangeEntry) string {
	if len(changes) == 0 {
		return "{}"
	}
	b, err := json.Marshal(map[string][]ChangeEntry{"changes": changes})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// formatPort converts a nullable port to a string for audit details.
func formatPort(port *int) string {
	if port == nil {
		return ""
	}
	return strconv.Itoa(*port)
}

// formatBool converts a bool to a string for audit details.
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Response is the standard API response format.
type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// Reconciler is the interface for triggering on-demand DNS reconciliation.
type Reconciler interface {
	ReconcileConfig(ctx context.Context, configID string) error
}

// Handlers holds all HTTP handlers.
type Handlers struct {
	store       store.Store
	health      *health.Engine
	oidcService *auth.OIDCService // nil when OIDC is disabled
	reconciler  Reconciler        // nil when no reconciler is configured
}

// New creates a new Handlers instance without OIDC (unauthenticated mode).
func New(store store.Store, health *health.Engine) *Handlers {
	return &Handlers{store: store, health: health}
}

// NewWithOIDC creates a new Handlers instance with OIDC authentication enabled.
func NewWithOIDC(store store.Store, health *health.Engine, oidcService *auth.OIDCService) *Handlers {
	return &Handlers{store: store, health: health, oidcService: oidcService}
}

// SetReconciler attaches a DNS reconciler so backend mutations trigger immediate updates.
func (h *Handlers) SetReconciler(r Reconciler) {
	h.reconciler = r
}

// triggerReconcile fires an asynchronous DNS reconciliation for the given config.
func (h *Handlers) triggerReconcile(configID string) {
	if h.reconciler == nil {
		return
	}
	go func() {
		if err := h.reconciler.ReconcileConfig(context.Background(), configID); err != nil {
			slog.Warn("on-demand reconcile failed", "config_id", configID, "error", err)
		}
	}()
}

// sendSuccess sends a successful response with data.
func sendSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Data: data})
}

// sendError sends an error response with the given status code.
func sendError(c *gin.Context, status int, message string) {
	c.JSON(status, Response{Error: message})
}

// sendValidationError sends a 400 Bad Request error.
func sendValidationError(c *gin.Context, message string) {
	sendError(c, http.StatusBadRequest, message)
}

// sendNotFound sends a 404 Not Found error.
func sendNotFound(c *gin.Context, resource string) {
	sendError(c, http.StatusNotFound, resource+" not found")
}

// sendInternalError sends a 500 Internal Server Error.
func sendInternalError(c *gin.Context, message string) {
	sendError(c, http.StatusInternalServerError, message)
}

// handleStoreError converts store errors to HTTP responses.
func (h *Handlers) handleStoreError(c *gin.Context, err error) {
	switch {
	case err == store.ErrNotFound:
		sendNotFound(c, "Resource")
	case err == store.ErrAlreadyExists:
		sendError(c, http.StatusConflict, "Resource already exists")
	case err == store.ErrForeignKeyViolation:
		sendValidationError(c, "Foreign key violation")
	default:
		slog.Error("store error", "error", err)
		sendInternalError(c, "internal error")
	}
}

// CORS middleware enables Cross-Origin Resource Sharing only for development.
// When backend serves frontend (production), no CORS is needed (same-origin).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		host := c.Request.Host

		// Allow same-origin requests (no origin header) and specific dev origins
		isSameOrigin := origin == "" || origin == "http://"+host || origin == "https://"+host
		isDevOrigin := origin == "http://localhost:5173" || origin == "http://localhost:3000"

		if isSameOrigin || isDevOrigin {
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeaders middleware adds security headers to all responses.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// XSS Protection
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS — instructs browsers to always use HTTPS (effective when behind TLS-terminating proxy)
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

		// Content Security Policy
		c.Writer.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self';")

		c.Next()
	}
}

// RequestLogger middleware logs HTTP requests.
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return "\"" + param.TimeStamp.Format("2006-01-02 15:04:05") + "\" " +
			"\"" + param.Method + " " + param.Path + "\" " +
			"\"status: " + strconv.Itoa(param.StatusCode) + "\" " +
			"\"latency: " + param.Latency.String() + "\" " +
			"\"client: " + param.ClientIP + "\"\n"
	})
}

// logAudit creates an audit log entry for user actions.
func (h *Handlers) logAudit(ctx context.Context, action string, entityType string, entityID string, entityName string, configID string, details string, c *gin.Context) {
	logEntry := &types.AuditLog{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: entityName,
		ConfigID:   configID,
		Details:    details,
	}

	if c != nil {
		logEntry.IPAddress = c.ClientIP()
		logEntry.UserAgent = c.Request.UserAgent()
		// Record authenticated user identity when available
		if session, ok := auth.SessionFromContext(c); ok {
			logEntry.UserID = session.UserID
		}
	}

	slog.Info("audit", "action", action, "entity_type", entityType, "entity_id", entityID, "user_id", logEntry.UserID)

	if err := h.store.CreateAuditLog(ctx, logEntry); err != nil {
		slog.Error("failed to create audit log", "error", err)
	}
}
