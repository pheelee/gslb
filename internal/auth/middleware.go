package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// Response mirrors handlers.Response to avoid an import cycle.
type authResponse struct {
	Error string `json:"error,omitempty"`
}

// Middleware validates the session JWT on every request that requires auth.
func Middleware(svc *OIDCService) gin.HandlerFunc {
	cookieName := svc.JWTConfig().CookieName
	return func(c *gin.Context) {
		if shouldSkipAuth(c.Request.URL.Path) {
			c.Next()
			return
		}

		token := getToken(c, cookieName)
		if token == "" {
			c.JSON(http.StatusUnauthorized, authResponse{Error: "authentication required"})
			c.Abort()
			return
		}

		session, err := svc.ParseSession(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, authResponse{Error: "invalid or expired session"})
			c.Abort()
			return
		}

		// Check server-side revocation
		if err := svc.ValidateSession(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusUnauthorized, authResponse{Error: "session revoked"})
			c.Abort()
			return
		}

		c.Set("session", session)
		c.Set("userID", session.UserID)
		c.Set("userRoles", session.Roles)
		c.Next()
	}
}

// RequireRole returns 403 if the authenticated user does not hold at least one
// of the specified roles.
func RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRolesVal, exists := c.Get("userRoles")
		if !exists {
			c.JSON(http.StatusForbidden, authResponse{Error: "access denied"})
			c.Abort()
			return
		}

		userRoles, _ := userRolesVal.([]string)
		for _, required := range requiredRoles {
			for _, userRole := range userRoles {
				if userRole == required {
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusForbidden, authResponse{Error: "insufficient permissions"})
		c.Abort()
	}
}

// RequireConfigAccess ensures the user can access a specific configuration.
// Admin role bypasses the per-config check.
func RequireConfigAccess(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRolesVal, _ := c.Get("userRoles")
		userRoles, _ := userRolesVal.([]string)
		for _, role := range userRoles {
			if role == "admin" {
				c.Next()
				return
			}
		}

		userIDVal, _ := c.Get("userID")
		userID, _ := userIDVal.(string)
		configID := c.Param("id")

		canAccess, err := s.CanAccess(c.Request.Context(), userID, configID)
		if err != nil || !canAccess {
			c.JSON(http.StatusForbidden, authResponse{Error: "access denied to this configuration"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireBackendConfigAccess looks up a backend by its :id param, resolves its
// parent config_id, and checks that the authenticated user has access to that config.
// Admin role bypasses the check.
func RequireBackendConfigAccess(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRolesVal, _ := c.Get("userRoles")
		userRoles, _ := userRolesVal.([]string)
		for _, role := range userRoles {
			if role == "admin" {
				c.Next()
				return
			}
		}

		backendID := c.Param("id")
		backend, err := s.GetBackend(c.Request.Context(), backendID)
		if err != nil || backend == nil {
			c.JSON(http.StatusNotFound, authResponse{Error: "backend not found"})
			c.Abort()
			return
		}

		userIDVal, _ := c.Get("userID")
		userID, _ := userIDVal.(string)

		canAccess, err := s.CanAccess(c.Request.Context(), userID, backend.ConfigID)
		if err != nil || !canAccess {
			c.JSON(http.StatusForbidden, authResponse{Error: "access denied to this configuration"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SessionFromContext extracts the Session from a gin context (set by Middleware).
func SessionFromContext(c *gin.Context) (*types.Session, bool) {
	v, ok := c.Get("session")
	if !ok {
		return nil, false
	}
	s, ok := v.(*types.Session)
	return s, ok
}

// shouldSkipAuth returns true for paths that do not require a session token.
// Note: "/" alone must NOT be used as a prefix — it would match every path.
func shouldSkipAuth(path string) bool {
	switch path {
	case "/health", "/ready", "/":
		return true
	}

	skipPrefixes := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/callback",
		"/api/v1/auth/logout",
		"/assets/",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Serve HTML pages unauthenticated; the SPA redirects to login.
	if strings.HasSuffix(path, ".html") {
		return true
	}

	return false
}

func getToken(c *gin.Context, cookieName string) string {
	if cookie, err := c.Cookie(cookieName); err == nil && cookie != "" {
		return cookie
	}
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}
