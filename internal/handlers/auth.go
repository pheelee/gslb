package handlers

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/pheelee/gslb/internal/auth"
	"github.com/pheelee/gslb/internal/types"
)

// sameSiteFromString maps a config string to the http.SameSite constant.
func sameSiteFromString(s string) http.SameSite {
	switch s {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// Login initiates OIDC authentication by generating state + nonce + PKCE verifier,
// storing them in short-lived HttpOnly cookies, and redirecting to the OIDC provider.
func (h *Handlers) Login(c *gin.Context) {
	state, err := auth.GenerateStateNonce()
	if err != nil {
		sendInternalError(c, "failed to generate state")
		return
	}
	nonce, err := auth.GenerateStateNonce()
	if err != nil {
		sendInternalError(c, "failed to generate nonce")
		return
	}
	pkceVerifier, err := auth.GeneratePKCEVerifier()
	if err != nil {
		sendInternalError(c, "failed to generate PKCE verifier")
		return
	}

	jwtCfg := h.oidcService.JWTConfig()
	secure := jwtCfg.CookieSecure
	sameSite := sameSiteFromString(jwtCfg.CookieSameSite)

	c.SetSameSite(sameSite)
	c.SetCookie("oidc_state", state, 600, "/", "", secure, true)
	c.SetCookie("oidc_nonce", nonce, 600, "/", "", secure, true)
	c.SetCookie("oidc_pkce", pkceVerifier, 600, "/", "", secure, true)

	pkceChallenge := auth.PKCEChallenge(pkceVerifier)
	c.Redirect(http.StatusFound, h.oidcService.GetAuthURL(state, nonce, pkceChallenge))
}

// Callback handles the OIDC provider redirect, verifies state and nonce,
// exchanges the code with PKCE verifier, syncs the user, and sets the session cookie.
func (h *Handlers) Callback(c *gin.Context) {
	stateCookie, err := c.Cookie("oidc_state")
	if err != nil || stateCookie != c.Query("state") {
		sendError(c, http.StatusBadRequest, "invalid state parameter")
		return
	}

	nonce, err := c.Cookie("oidc_nonce")
	if err != nil {
		sendError(c, http.StatusBadRequest, "missing nonce cookie")
		return
	}

	pkceVerifier, err := c.Cookie("oidc_pkce")
	if err != nil {
		sendError(c, http.StatusBadRequest, "missing PKCE verifier cookie")
		return
	}

	code := c.Query("code")
	if code == "" {
		sendError(c, http.StatusBadRequest, "authorization code not provided")
		return
	}

	token, err := h.oidcService.Exchange(c.Request.Context(), code, pkceVerifier)
	if err != nil {
		sendError(c, http.StatusUnauthorized, "failed to exchange token")
		return
	}

	idToken, err := h.oidcService.VerifyIDToken(c.Request.Context(), token, nonce)
	if err != nil {
		sendError(c, http.StatusUnauthorized, "failed to verify ID token")
		return
	}

	oidcUser, oidcRoles, err := h.oidcService.ExtractUserInfo(idToken)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "failed to extract user info")
		return
	}

	user, err := h.oidcService.SyncUser(c.Request.Context(), oidcUser, oidcRoles)
	if err != nil {
		sendError(c, http.StatusForbidden, err.Error())
		return
	}

	sessionToken, err := h.oidcService.CreateSession(c.Request.Context(), user)
	if err != nil {
		sendInternalError(c, "failed to create session")
		return
	}

	jwtCfg := h.oidcService.JWTConfig()
	maxAge := int(jwtCfg.Expiry.Seconds())
	if maxAge == 0 {
		maxAge = 86400
	}
	sameSite := sameSiteFromString(jwtCfg.CookieSameSite)

	c.SetSameSite(sameSite)
	c.SetCookie(jwtCfg.CookieName, sessionToken, maxAge, "/", "", jwtCfg.CookieSecure, jwtCfg.CookieHTTPOnly)

	// Clear OIDC flow cookies
	c.SetCookie("oidc_state", "", -1, "/", "", jwtCfg.CookieSecure, true)
	c.SetCookie("oidc_nonce", "", -1, "/", "", jwtCfg.CookieSecure, true)
	c.SetCookie("oidc_pkce", "", -1, "/", "", jwtCfg.CookieSecure, true)

	h.logAudit(c.Request.Context(), "auth:login", "user", user.ID, user.Email, "", "", c)
	c.Redirect(http.StatusFound, "/")
}

// Logout revokes the server-side session, clears the cookie, and returns the OIDC
// provider's end-session URL if available.
func (h *Handlers) Logout(c *gin.Context) {
	// Revoke the session server-side
	if session, ok := auth.SessionFromContext(c); ok && session.JTI != "" {
		_ = h.store.RevokeSession(c.Request.Context(), session.JTI)
	}

	jwtCfg := h.oidcService.JWTConfig()
	c.SetCookie(jwtCfg.CookieName, "", -1, "/", "", jwtCfg.CookieSecure, jwtCfg.CookieHTTPOnly)

	endSessionURL := h.oidcService.EndSessionURL()
	if endSessionURL != "" {
		u, err := url.Parse(endSessionURL)
		if err == nil {
			q := u.Query()
			q.Set("post_logout_redirect_uri", c.Request.Header.Get("Origin"))
			u.RawQuery = q.Encode()
			c.JSON(http.StatusOK, Response{Data: map[string]string{
				"logout_url": u.String(),
			}})
			return
		}
	}

	c.JSON(http.StatusOK, Response{Data: "logged out successfully"})
}

// GetCurrentUser returns the authenticated user's full profile including roles.
func (h *Handlers) GetCurrentUser(c *gin.Context) {
	session, exists := c.Get("session")
	if !exists {
		sendError(c, http.StatusUnauthorized, "not authenticated")
		return
	}
	s := session.(*types.Session)
	user, err := h.store.GetUserByID(c.Request.Context(), s.UserID)
	if err != nil {
		sendInternalError(c, "failed to get user")
		return
	}
	sendSuccess(c, user)
}

// ListRoles returns all available roles.
func (h *Handlers) ListRoles(c *gin.Context) {
	roles, err := h.store.ListRoles(c.Request.Context())
	if err != nil {
		sendInternalError(c, "failed to list roles")
		return
	}
	sendSuccess(c, roles)
}

// ListUsers returns all users (admin only).
func (h *Handlers) ListUsers(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		sendInternalError(c, "failed to list users")
		return
	}
	sendSuccess(c, users)
}

// AssignUserRole assigns a role to a user (admin only).
func (h *Handlers) AssignUserRole(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		RoleID string `json:"role_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		sendValidationError(c, err.Error())
		return
	}

	session, _ := c.Get("session")
	assignedBy := session.(*types.Session).UserID

	if err := h.store.AssignRole(c.Request.Context(), userID, req.RoleID, assignedBy); err != nil {
		h.handleStoreError(c, err)
		return
	}

	h.logAudit(c.Request.Context(), "user:role:assign", "user", userID, "", "", req.RoleID, c)
	c.JSON(http.StatusOK, Response{Data: "role assigned"})
}

// RemoveUserRole removes a role from a user (admin only).
func (h *Handlers) RemoveUserRole(c *gin.Context) {
	userID := c.Param("id")
	roleID := c.Param("role_id")

	if err := h.store.RemoveRole(c.Request.Context(), userID, roleID); err != nil {
		h.handleStoreError(c, err)
		return
	}

	h.logAudit(c.Request.Context(), "user:role:remove", "user", userID, "", "", roleID, c)
	c.JSON(http.StatusOK, Response{Data: "role removed"})
}
