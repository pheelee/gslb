package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/auth"
	"github.com/pheelee/gslb/web"
)

// RateLimiter implements simple per-IP rate limiting with periodic cleanup.
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter and starts a background cleanup goroutine.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var validRequests []time.Time
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= rl.limit {
		rl.requests[ip] = validRequests
		return false
	}

	rl.requests[ip] = append(validRequests, now)
	return true
}

// cleanup periodically removes stale entries to prevent unbounded memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.window)
		for ip, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit middleware implements rate limiting
func RateLimit(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.Allow(ip) {
			c.JSON(429, Response{Error: "Rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// MaxBodySize middleware limits the request body size.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// RegisterRoutes registers all API routes on the given router.
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	globalLimiter := NewRateLimiter(100, time.Minute)
	authLimiter := NewRateLimiter(10, time.Minute)

	router.Use(SecurityHeaders())
	router.Use(CORS())
	router.Use(RequestLogger())
	router.Use(RateLimit(globalLimiter))
	router.Use(MaxBodySize(1 << 20)) // 1 MB
	router.Use(gin.Recovery())

	// Health check endpoints (always public)
	router.GET("/health", h.HealthHandler)
	router.GET("/ready", h.ReadyHandler)

	if h.oidcService != nil {
		h.registerAuthRoutes(router, authLimiter)
	} else {
		// OIDC disabled — return 501 for auth endpoints
		router.GET("/api/v1/auth/login", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, Response{Error: "SSO not enabled"})
		})
		router.GET("/api/v1/auth/callback", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, Response{Error: "SSO not enabled"})
		})
		router.POST("/api/v1/auth/logout", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, Response{Error: "SSO not enabled"})
		})
	}

	// API v1 routes (protected when OIDC is enabled)
	api := router.Group("/api/v1")
	if h.oidcService != nil {
		api.Use(auth.Middleware(h.oidcService))
	}
	{
		// Auth info (only meaningful when OIDC enabled)
		if h.oidcService != nil {
			api.GET("/auth/user", h.GetCurrentUser)
			api.GET("/roles", h.ListRoles)
		}

		// Status
		api.GET("/status", h.GetSystemStatus)

		// Configs
		api.POST("/configs", h.CreateConfig)
		api.GET("/configs", h.ListConfigs)

		if h.oidcService != nil {
			// With OIDC: per-config access control enforced
			api.GET("/configs/:id", auth.RequireConfigAccess(h.store), h.GetConfig)
			api.PUT("/configs/:id", auth.RequireConfigAccess(h.store), h.UpdateConfig)
			api.DELETE("/configs/:id", auth.RequireRole("admin", "operator"), auth.RequireConfigAccess(h.store), h.DeleteConfig)
			api.GET("/configs/:id/status", auth.RequireConfigAccess(h.store), h.GetConfigStatus)
			api.GET("/configs/:id/roles", auth.RequireConfigAccess(h.store), h.GetConfigRoles)
			api.PUT("/configs/:id/roles", auth.RequireRole("admin", "operator"), auth.RequireConfigAccess(h.store), h.UpdateConfigRoles)

			// Config Backends
			api.POST("/configs/:id/backends", auth.RequireConfigAccess(h.store), h.AddBackend)
			api.GET("/configs/:id/backends", auth.RequireConfigAccess(h.store), h.ListBackends)

			// Config Health Checks
			api.POST("/configs/:id/health-check", auth.RequireConfigAccess(h.store), h.ConfigureHealthCheck)
			api.GET("/configs/:id/health-check", auth.RequireConfigAccess(h.store), h.GetHealthCheck)
			api.DELETE("/configs/:id/health-check", auth.RequireRole("admin", "operator"), auth.RequireConfigAccess(h.store), h.DeleteHealthCheck)

			// Config DNS Providers
			api.POST("/configs/:id/dns-provider", auth.RequireConfigAccess(h.store), h.ConfigureDNSProvider)
			api.GET("/configs/:id/dns-provider", auth.RequireConfigAccess(h.store), h.GetDNSProvider)
			api.DELETE("/configs/:id/dns-provider", auth.RequireRole("admin", "operator"), auth.RequireConfigAccess(h.store), h.DeleteDNSProvider)
		} else {
			// Without OIDC: no access control
			api.GET("/configs/:id", h.GetConfig)
			api.PUT("/configs/:id", h.UpdateConfig)
			api.DELETE("/configs/:id", h.DeleteConfig)
			api.GET("/configs/:id/status", h.GetConfigStatus)

			// Config Backends
			api.POST("/configs/:id/backends", h.AddBackend)
			api.GET("/configs/:id/backends", h.ListBackends)

			// Config Health Checks
			api.POST("/configs/:id/health-check", h.ConfigureHealthCheck)
			api.GET("/configs/:id/health-check", h.GetHealthCheck)
			api.DELETE("/configs/:id/health-check", h.DeleteHealthCheck)

			// Config DNS Providers
			api.POST("/configs/:id/dns-provider", h.ConfigureDNSProvider)
			api.GET("/configs/:id/dns-provider", h.GetDNSProvider)
			api.DELETE("/configs/:id/dns-provider", h.DeleteDNSProvider)
		}

		// Backends by ID — with per-config access control when OIDC is enabled
		if h.oidcService != nil {
			api.GET("/backends/:id", auth.RequireBackendConfigAccess(h.store), h.GetBackend)
			api.PUT("/backends/:id", auth.RequireBackendConfigAccess(h.store), h.UpdateBackend)
			api.DELETE("/backends/:id", auth.RequireRole("admin", "operator"), auth.RequireBackendConfigAccess(h.store), h.DeleteBackend)
			api.GET("/backends/:id/health", auth.RequireBackendConfigAccess(h.store), h.GetBackendHealth)
			api.GET("/backends/:id/history", auth.RequireBackendConfigAccess(h.store), h.GetBackendHistory)
		} else {
			api.GET("/backends/:id", h.GetBackend)
			api.PUT("/backends/:id", h.UpdateBackend)
			api.DELETE("/backends/:id", h.DeleteBackend)
			api.GET("/backends/:id/health", h.GetBackendHealth)
			api.GET("/backends/:id/history", h.GetBackendHistory)
		}

		// Audit Logs (admin-only when OIDC is enabled)
		if h.oidcService != nil {
			auditGroup := api.Group("/audit-logs")
			auditGroup.Use(auth.RequireRole("admin"))
			{
				auditGroup.GET("", h.ListAuditLogs)
				auditGroup.GET("/count", h.GetAuditLogCount)
				auditGroup.GET("/:id", h.GetAuditLog)
				auditGroup.GET("/entity/:type/:id", h.GetAuditLogsForEntity)
			}
		} else {
			api.GET("/audit-logs", h.ListAuditLogs)
			api.GET("/audit-logs/count", h.GetAuditLogCount)
			api.GET("/audit-logs/:id", h.GetAuditLog)
			api.GET("/audit-logs/entity/:type/:id", h.GetAuditLogsForEntity)
		}

		// Admin routes (OIDC only)
		if h.oidcService != nil {
			admin := api.Group("/admin")
			admin.Use(auth.RequireRole("admin"))
			{
				admin.GET("/users", h.ListUsers)
				admin.POST("/users/:id/roles", h.AssignUserRole)
				admin.DELETE("/users/:id/roles/:role_id", h.RemoveUserRole)
			}
		}
	}

	// Serve embedded frontend (SPA)
	router.Use(web.Handler())
}

// registerAuthRoutes registers the public OIDC auth endpoints with a stricter rate limiter.
func (h *Handlers) registerAuthRoutes(router *gin.Engine, authLimiter *RateLimiter) {
	authGroup := router.Group("/api/v1/auth")
	authGroup.Use(RateLimit(authLimiter))
	{
		authGroup.GET("/login", h.Login)
		authGroup.GET("/callback", h.Callback)
		authGroup.POST("/logout", h.Logout)
	}
}

// NewRouter creates and configures a new gin.Engine with all routes registered.
func NewRouter(h *Handlers) *gin.Engine {
	router := gin.New()
	h.RegisterRoutes(router)
	return router
}
