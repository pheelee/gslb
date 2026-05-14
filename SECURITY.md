# Security Documentation

## Overview

This document outlines the security architecture, implemented protections, remaining risks, and hardening recommendations for the GSLB (Global Server Load Balancer) application.

**Last Updated**: April 2026

---

## Authentication & Authorization

### OIDC / OAuth 2.0 (Authorization Code Flow)

When `oidc.enabled: true`, the application integrates with any OpenID Connect provider (e.g., Keycloak, Entra ID, Okta) using the standard authorization code flow.

| Aspect | Implementation |
|--------|----------------|
| Provider discovery | `/.well-known/openid-configuration` via `go-oidc/v3` |
| CSRF protection | Cryptographically random `state` parameter (32 bytes, `crypto/rand`) |
| Replay protection | Cryptographically random `nonce` embedded in the ID token |
| State/nonce storage | Short-lived HttpOnly cookies (600s TTL), cleared after callback |
| ID token verification | Signature, expiry, audience, and nonce validated by library verifier |
| Token exchange | Server-side only — authorization code never exposed to the browser |

**Location**: `internal/auth/oidc.go`, `internal/handlers/auth.go`

### JWT Session Tokens

After successful OIDC login, the server issues a signed JWT stored in a cookie.

| Attribute | Value |
|-----------|-------|
| Algorithm | HS256 with explicit algorithm verification (prevents `alg: none` / key-confusion attacks) |
| Signing key | Configurable via `GSLB_JWT_SECRET` env or `jwt.secret` in config (`json:"-"` — excluded from serialization) |
| Expiry | Configurable (`jwt.expiry`, default 24h) |
| Cookie flags | `HttpOnly`, `Secure` (configurable; set `true` behind HTTPS reverse proxy), `SameSite=Lax` |
| Claims | `sub` (user ID), `email`, `name`, `roles`, `jti` (JWT ID), `iat`, `exp` |
| Server-side revocation | Every token carries a UUID `jti` stored in the `sessions` table; middleware validates the record exists and `revoked_at` is null on every authenticated request |

**Location**: `internal/auth/oidc.go:CreateSession()`, `internal/auth/oidc.go:ParseSession()`, `internal/auth/oidc.go:ValidateSession()`

### Role-Based Access Control (RBAC)

Three built-in roles with clear permission boundaries:

| Role | Permissions |
|------|------------|
| `admin` | Full access to all configurations, user management, role assignment |
| `operator` | Create, edit, delete configurations they have access to |
| `viewer` | Read-only access to assigned configurations |

Access control is enforced at two levels:
1. **Role-level**: `RequireRole()` middleware gates destructive operations (delete config/backend/health-check/dns-provider → `admin` or `operator`)
2. **Config-level**: `RequireConfigAccess()` middleware checks per-config role assignment via the `config_roles` join table; `admin` bypasses this check

**Location**: `internal/auth/middleware.go`, `internal/handlers/routes.go`

### User Lifecycle

- **Auto-create**: Any authenticated OIDC subject receives a local account on first login with the configured `default_role`
- **Role sync**: Full diff-sync on every login — roles present in the OIDC claim are added, stale roles are removed; unknown roles are silently skipped
- **Account disable**: `is_active = false` blocks login immediately; existing sessions are invalidated via server-side revocation (see Session Revocation below)
- **Audit logging**: Login events are recorded with IP address and user agent

**Location**: `internal/auth/oidc.go:SyncUser()`, `internal/auth/oidc.go:syncRoles()`

### OIDC-Disabled Mode

When `oidc.enabled: false` (the default), the application runs without any authentication. All API routes are unprotected. Auth endpoints return HTTP 501. This mode is suitable only for trusted networks or development.

---

## Implemented Security Measures

### 1. HTTP Security Headers

All responses include defensive headers:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing |
| `X-Frame-Options` | `DENY` | Prevents clickjacking |
| `X-XSS-Protection` | `1; mode=block` | Legacy XSS filter hint |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer leakage |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'` | Mitigates XSS/injection |

**Location**: `internal/handlers/handlers.go:SecurityHeaders()`

### 2. CORS Configuration

- **Same-origin by default**: No CORS headers emitted in production (backend serves frontend)
- **Restricted development origins**: Only `localhost:5173` and `localhost:3000`
- **No wildcard origins**: `*` is never used
- **Credentials**: `Access-Control-Allow-Credentials` is not set — cookies are not sent cross-origin

**Location**: `internal/handlers/handlers.go:CORS()`

### 3. CSRF Protection

- **SameSite=Lax cookies**: Session cookie and OIDC flow cookies use `SameSite=Lax`, preventing cross-site POST requests from attaching the cookie
- **State parameter**: OIDC login flow uses a cryptographic state parameter to prevent login CSRF
- **No CORS credential sharing**: Cross-origin requests cannot attach session cookies

### 4. Rate Limiting

Two independent in-memory rate limiters (sliding window, per source IP):

| Scope | Limit | Routes |
|-------|-------|--------|
| Global | 100 req/min | All routes |
| Auth | 10 req/min | `/api/v1/auth/*` only |

- **Response**: HTTP 429 when exceeded
- **Cleanup**: Background goroutine prunes stale entries every 5 minutes

**Location**: `internal/handlers/routes.go:NewRateLimiter()`, `internal/handlers/routes.go:RateLimit()`

### 5. SQL Injection Prevention

- **Parameterized queries**: All database queries use `?` placeholders — no string concatenation
- **Prepared statements**: SQLite driver uses prepared statements internally
- **ORM-free**: Direct SQL gives full visibility into query construction

**Location**: All files under `internal/store/`

### 6. XSS Protection (Frontend)

- **Vue auto-escaping**: Template interpolations `{{ }}` automatically escape HTML
- **No `v-html` usage**: No dangerous raw HTML rendering anywhere in the codebase
- **CSP enforcement**: Content Security Policy blocks inline scripts and restricts resource origins

### 7. Input Validation

**Backend**:
- Gin binding tags enforce required fields, numeric ranges, and enum values (`oneof=round_robin weighted`)
- DNS name validated by regex
- JSON validation for DNS provider configs
- UUID-based resource IDs (not sequential integers)

**Frontend**:
- Form validation before API submission
- TypeScript type checking
- User-facing validation error messages

### 8. Audit Logging

All mutating actions are persisted to the `audit_logs` table:
- Action type (`config:create`, `config:update`, `config:delete`, `auth:login`, `user:role:assign`, etc.)
- Entity type and ID
- Authenticated user ID (`user_id` column — empty in OIDC-disabled mode)
- IP address and user agent
- Indexed by `(entity_type, entity_id)` and `created_at DESC` for efficient querying

**Location**: `internal/handlers/handlers.go:logAudit()`

### 9. Secret Handling

- OIDC `client_secret` and JWT `secret` use `json:"-"` tags — never serialized in API responses
- Environment variable overrides (`GSLB_OIDC_CLIENT_SECRET`, `GSLB_JWT_SECRET`, `GSLB_ENCRYPTION_KEY`) allow injection from secret managers without touching config files
- Config struct defaults to secure values: `CookieHTTPOnly: true`; `CookieSecure` defaults to `false` for plain-HTTP development and should be set `true` behind an HTTPS reverse proxy

### 10. DNS Credential Encryption

DNS provider credentials (`config_json` in the `dns_providers` table) are encrypted at rest using AES-256-GCM:

| Aspect | Detail |
|--------|--------|
| Algorithm | AES-256-GCM (authenticated encryption) |
| Key source | `GSLB_ENCRYPTION_KEY` env var or `encryption_key` in config (64-character hex = 32 bytes) |
| Key generation | `openssl rand -hex 32` |
| Storage format | `enc:<base64(nonce+ciphertext)>` prefix distinguishes encrypted values from plaintext |
| Migration | Existing plaintext values are returned as-is (transparent read); re-saving encrypts them automatically |
| Nil-safe | When no key is configured, encryption is a no-op (passthrough) — behaviour is unchanged |

**Location**: `internal/crypto/crypto.go`, `internal/store/dns_provider.go`

### 11. Server-Side Session Revocation

Sessions can be explicitly invalidated server-side without waiting for JWT expiry:

| Event | Behaviour |
|-------|-----------|
| User logout | `RevokeSession(jti)` sets `revoked_at` in the `sessions` table immediately |
| Every authenticated request | `ValidateSession()` checks the session record exists and `revoked_at IS NULL` |
| Expired sessions | Background cleanup goroutine (`sessionCleanupLoop`) removes expired records every 15 minutes |
| Legacy tokens (no `jti`) | Validation is skipped to maintain backward compatibility |

**Location**: `internal/auth/oidc.go:ValidateSession()`, `internal/store/session.go`, `cmd/gslb/main.go:sessionCleanupLoop()`

### 12. Structured Logging

All application and request logging uses Go's `log/slog` package:

- Structured key-value fields — no free-form string interpolation that could embed sensitive data
- HTTP access log: method, path, status code (as integer), latency, client IP
- Selectable format: `--log-json` flag for JSON output (log aggregators), default plain text for development
- All sensitive fields (`password`, `secret`, `token`, `config_json`) are excluded from log output

**Location**: `cmd/gslb/main.go`, `internal/handlers/handlers.go:RequestLogger()`

### 13. Database Schema Security

- Foreign keys with `ON DELETE CASCADE` prevent orphaned records
- `UNIQUE` constraints on `dns_name`, `subject`, role `name`
- Composite primary keys on join tables (`user_roles`, `config_roles`) prevent duplicate assignments
- `INSERT OR IGNORE` for idempotent role seeding

---

## Risk Assessment

| Threat | Risk Level | Status | Notes |
|--------|------------|--------|-------|
| Unauthorized API Access | **LOW** (OIDC on) / **CRITICAL** (OIDC off) | ✅ Mitigated when OIDC enabled | RBAC with per-config access control |
| CSRF | **LOW** | ✅ Mitigated | SameSite=Lax + OIDC state parameter |
| SQL Injection | **LOW** | ✅ Mitigated | Parameterized queries throughout |
| XSS | **LOW** | ✅ Mitigated | Vue auto-escaping + CSP |
| Clickjacking | **LOW** | ✅ Mitigated | `X-Frame-Options: DENY` + `frame-ancestors 'none'` |
| DoS / Abuse | **LOW** | ✅ Mitigated | Global (100/min) + auth (10/min) rate limiting, 1MB body limit, server timeouts |
| Session Hijacking | **LOW** | ✅ Mitigated | HttpOnly+Secure cookies, HSTS, server-side revocation via `jti` |
| Credential Exposure (DNS) | **LOW** | ✅ Mitigated | DNS provider credentials encrypted at rest (AES-256-GCM) |
| Session not revoked after logout | **LOW** | ✅ Mitigated | `RevokeSession(jti)` on logout; validated on every authenticated request |
| Man-in-the-Middle | **LOW** | ✅ Mitigated by deployment | Intended for reverse proxy; HSTS header enforces HTTPS downstream |
| JWT Secret Weakness | **LOW** | ✅ Mitigated | Minimum 32-byte secret enforced at startup |
| Stale Role Claims | **LOW** | ⚠️ Accepted trade-off | Roles cached in JWT; changes take effect at next login (within expiry window) |
| OIDC Code Injection | **LOW** | ✅ Mitigated | PKCE S256 challenge/verifier in auth flow |
| Information Leakage (errors) | **LOW** | ✅ Mitigated | Generic error messages to clients; raw errors logged server-side only |

---

## Open Issues & Hardening Recommendations

### Critical (Must Address Before Production)

- [x] ~~**HTTPS/TLS**~~: Designed to run behind a TLS-terminating reverse proxy (nginx, Caddy, cloud LB). HSTS header enforces HTTPS in browsers.
- [x] ~~**Encrypt DNS Provider Credentials**~~: `dns_providers.config_json` encrypted at rest with AES-256-GCM. Key sourced from `GSLB_ENCRYPTION_KEY` env var. Transparent migration: existing plaintext values are read as-is until re-saved.
- [x] ~~**JWT Secret Validation**~~: `config.Validate()` enforces >= 32-byte JWT secret plus required OIDC fields at startup.

### High Priority

- [x] ~~**Server-Side Session Revocation**~~: Each JWT carries a UUID `jti` claim. A `sessions` table records issued tokens; `ValidateSession()` checks revocation on every authenticated request. Logout calls `RevokeSession(jti)` immediately. Expired records cleaned up every 15 minutes.
- [x] ~~**HTTP Server Hardening**~~: `ReadTimeout` (15s), `WriteTimeout` (15s), `ReadHeaderTimeout` (5s), `IdleTimeout` (60s), `MaxHeaderBytes` (1MB) on `http.Server`. Request body limited to 1MB via `MaxBodySize` middleware.
- [x] ~~**HSTS Header**~~: `Strict-Transport-Security: max-age=63072000; includeSubDomains` on all responses.
- [x] ~~**PKCE (Proof Key for Code Exchange)**~~: OIDC login uses S256 PKCE challenge/verifier in HttpOnly cookie; provides defense-in-depth against authorization code injection.
- [x] ~~**Stricter Rate Limits for Auth Endpoints**~~: `/api/v1/auth/*` has a dedicated 10 req/min rate limiter separate from the global 100 req/min limit.
- [ ] **Distributed Rate Limiting**: In-memory rate limiter resets on restart and is not shared across instances. For multi-instance deployments, use Redis-backed rate limiting.

### Medium Priority

- [x] ~~**Structured Logging**~~: All logging uses `log/slog` with structured key-value fields. JSON format available via `--log-json` flag. Sensitive fields are excluded from log output.
- [x] ~~**Error Message Information Leakage**~~: `SyncUser` returns generic `"authentication failed"` for both disabled accounts and unknown users. `handleStoreError` logs raw errors server-side only.
- [x] ~~**OIDC Logout**~~: Logout returns the provider's `end_session_endpoint` URL so the frontend can complete IdP-side logout.
- [x] ~~**Audit Log: Record User Identity**~~: `logAudit` records `session.UserID` in the `user_id` column of `audit_logs`.
- [x] ~~**Backend Endpoints Missing Config-Level Access Control**~~: `RequireBackendConfigAccess` middleware applied to `GET/PUT/DELETE /backends/:id` and `GET /backends/:id/health`.
- [x] ~~**Request Body Size Limits**~~: `MaxBodySize(1MB)` applied globally via `http.MaxBytesReader`.
- [ ] **Dependency Updates**: Regularly run `npm audit fix` and `go get -u` to patch known vulnerabilities.

### Low Priority

- [x] ~~**Source Maps**~~: Source maps are disabled in production builds (`sourcemap: false` in Vite config) to prevent source code disclosure.
- [ ] **Database File Permissions**: Ensure SQLite file has permissions `0600` to prevent unauthorized local access.
- [ ] **Cookie Domain Restriction**: Session cookie domain is empty (current host). For multi-subdomain deployments, set explicitly to avoid leakage to sibling domains.
- [x] ~~**`SameSite` from Config Applied**~~: `handlers/auth.go` maps the `CookieSameSite` config string to the appropriate `http.SameSite` constant.
- [x] ~~**Rate Limiter Memory Growth**~~: Background cleanup goroutine prunes stale entries every 5 minutes.

---

## DNS Provider Security

### Cloudflare

**Required Permissions**:
- Zone:Edit (to create/update DNS records)
- Zone:Read (to list zones)

**Security Best Practices**:
- Use zone-specific API tokens — never use the global API key
- Rotate tokens regularly and monitor usage in the Cloudflare dashboard
- Once credential encryption is implemented, store tokens encrypted at rest

### Mock Provider

- No real DNS changes — safe for testing
- No credentials required

---

## Secure Deployment Checklist

Before deploying to production:

- [ ] Deploy behind a TLS-terminating reverse proxy (nginx, Caddy, cloud LB)
- [ ] Set `oidc.enabled: true` and configure a trusted OIDC provider
- [ ] Set a strong JWT secret (>= 32 random bytes): `GSLB_JWT_SECRET`
- [ ] Set `cookie_secure: true` (default) — only disable for local HTTP development
- [ ] Encrypt DNS provider credentials at rest
- [ ] Configure firewall rules (allow only necessary ports)
- [ ] Set SQLite file permissions to `0600`
- [ ] Set up log monitoring and alerting
- [ ] Run `go test -race ./...` to check for race conditions
- [ ] Run `gosec ./...` for static security analysis
- [ ] Run `npm audit` and fix vulnerabilities
- [ ] Remove development/test data from database
- [ ] Configure automated backups for the SQLite database
- [ ] Document incident response procedures

---

## Security Contacts

If you discover a security vulnerability:

1. **DO NOT** create a public issue
2. Email security concerns to: [your-email@example.com]
3. Include detailed description and reproduction steps
4. Allow reasonable time for response before public disclosure

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [Go Security Guidelines](https://golang.org/security)
- [Vue.js Security](https://vuejs.org/guide/best-practices/security.html)
- [OAuth 2.0 Security Best Current Practice (RFC 9700)](https://datatracker.ietf.org/doc/html/rfc9700)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [Cloudflare API Security](https://developers.cloudflare.com/api/)

---

**Security is an ongoing process, not a one-time setup. Regularly review and update security measures.**
