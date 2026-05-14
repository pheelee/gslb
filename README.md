# GSLB — Global Server Load Balancer

A self-hosted Global Server Load Balancing application that monitors backend health and automatically updates DNS records based on backend availability. Built as a single binary with an embedded Vue 3 web UI.

## Features

- **Automated DNS failover** via Cloudflare DNS API
- **Health checking** — ICMP ping, TCP connect
- **Load balancing modes** — Round-robin (all healthy backends in DNS) and weighted failover (primary/standby)
- **OIDC/OAuth2 authentication** with PKCE — compatible with Keycloak, Authentik, Entra ID, Okta, etc.
- **Role-based access control** — admin, operator, viewer with per-config permissions
- **Audit logging** — full action trail for all write operations
- **Single binary** — Go backend with embedded Vue 3 + TypeScript frontend
- **No CGO** — pure Go SQLite via `modernc.org/sqlite`

---

## Table of Contents

- [Quick Start](#quick-start)
- [Building from Source](#building-from-source)
- [Configuration](#configuration)
- [Running in Production](#running-in-production)
- [Authentication](#authentication)
- [Roles and Permissions](#roles-and-permissions)
- [DNS Provider Setup](#dns-provider-setup)
- [Health Checks](#health-checks)
- [Load Balancing Modes](#load-balancing-modes)
- [API Reference](#api-reference)
- [Systemd Service](#systemd-service)
- [Docker](#docker)
- [Reverse Proxy](#reverse-proxy)

---

## Quick Start

```bash
# Download a release binary (Linux amd64)
chmod +x gslb
./gslb --config config.yaml
```

The web UI is available at `http://localhost:8090`.

---

## Building from Source

### Prerequisites

- Go 1.26+
- Node.js 22+
- npm

### Build

```bash
# Clone the repository
git clone https://github.com/pheelee/gslb
cd gslb

# Build frontend
cd web
npm install
npm run build
cd ..

# Build Go binary (production)
go build -ldflags="-s -w" -o gslb ./cmd/gslb
```

The resulting `gslb` binary contains the embedded frontend — no separate static file serving is needed.

### Development

```bash
# Backend (with hot-reload via air or plain go run)
go run ./cmd/gslb --config config.yaml

# Frontend dev server (proxies API to backend)
cd web && npm run dev
```

---

## Configuration

GSLB is configured via a YAML file. All values can be overridden with environment variables.

### config.yaml

```yaml
# Path to SQLite database file
database_path: gslb.db

# Address and port to listen on
listen_addr: ":8090"

# Number of concurrent health check workers
health_check_workers: 10

# How often the DNS reconciler runs
reconcile_interval: 30s

# Number of days to retain audit log entries
audit_log_retention_days: 30

# AES-256 encryption key for DNS provider credentials (64-char hex string)
# Generate with: openssl rand -hex 32
encryption_key: ""

oidc:
  enabled: false
  issuer: ""
  client_id: ""
  client_secret: ""
  redirect_url: ""
  roles_claim: "roles"
  default_role: "viewer"

jwt:
  secret: ""         # Min 32 characters. Generate with: openssl rand -base64 32
  expiry: 24h
  cookie_secure: true
  cookie_name: "gslb_session"
```

### Environment Variable Overrides

All configuration fields can be overridden via environment variables with the `GSLB_` prefix:

| Environment Variable | Config Field |
|---|---|
| `GSLB_DATABASE_PATH` | `database_path` |
| `GSLB_LISTEN_ADDR` | `listen_addr` |
| `GSLB_HEALTH_CHECK_WORKERS` | `health_check_workers` |
| `GSLB_RECONCILE_INTERVAL` | `reconcile_interval` |
| `GSLB_AUDIT_LOG_RETENTION_DAYS` | `audit_log_retention_days` |
| `GSLB_ENCRYPTION_KEY` | `encryption_key` |
| `GSLB_OIDC_ENABLED` | `oidc.enabled` |
| `GSLB_OIDC_ISSUER` | `oidc.issuer` |
| `GSLB_OIDC_CLIENT_ID` | `oidc.client_id` |
| `GSLB_OIDC_CLIENT_SECRET` | `oidc.client_secret` |
| `GSLB_OIDC_REDIRECT_URL` | `oidc.redirect_url` |
| `GSLB_OIDC_ROLES_CLAIM` | `oidc.roles_claim` |
| `GSLB_OIDC_DEFAULT_ROLE` | `oidc.default_role` |
| `GSLB_JWT_SECRET` | `jwt.secret` |
| `GSLB_JWT_EXPIRY` | `jwt.expiry` |
| `GSLB_JWT_COOKIE_SECURE` | `jwt.cookie_secure` |
| `GSLB_JWT_COOKIE_NAME` | `jwt.cookie_name` |

### Generating Secrets

```bash
# Encryption key (AES-256, 64 hex chars)
openssl rand -hex 32

# JWT secret (min 32 chars)
openssl rand -base64 32
```

---

## Running in Production

### Minimal production config

```yaml
database_path: /var/lib/gslb/gslb.db
listen_addr: ":8090"
health_check_workers: 10
reconcile_interval: 30s
audit_log_retention_days: 90
encryption_key: "<64-char hex string>"

oidc:
  enabled: true
  issuer: "https://auth.example.com/realms/myrealm"
  client_id: "gslb"
  client_secret: "<client-secret>"
  redirect_url: "https://gslb.example.com/api/v1/auth/callback"
  roles_claim: "roles"
  default_role: "viewer"

jwt:
  secret: "<min-32-char-secret>"
  expiry: 8h
  cookie_secure: true
  cookie_name: "gslb_session"
```

### Running without OIDC

When OIDC is disabled, the API has no authentication. All endpoints are accessible without a token. Do not expose the application to untrusted networks in this mode.

---

## Authentication

GSLB supports two modes:

### No authentication (OIDC disabled)

When `oidc.enabled: false`, **all API endpoints are fully open with no authentication or access control**. The `/api/v1/auth/*` endpoints return `501 Not Implemented`. This mode is only appropriate for trusted internal networks or local development.

### OIDC / OAuth2 (recommended for production)

GSLB implements the Authorization Code flow with PKCE. Any OIDC-compliant provider works.

**Required scopes**: `openid`, `profile`, `email`

**Roles from OIDC**: Configure `roles_claim` to match the JWT claim your provider uses (e.g., `roles`, `groups`). Claim values should match GSLB role names: `admin`, `operator`, `viewer`.

**User provisioning**: Any authenticated OIDC user receives a local account on first login with the configured `default_role` assigned.

#### Keycloak setup

1. Create a client with `confidential` access type
2. Set redirect URI to `https://gslb.example.com/api/v1/auth/callback`
3. Add a client role mapper that maps client roles to the `roles` claim
4. Set `roles_claim: "roles"` in config

#### Authentik setup

1. Create an OAuth2 Provider with the redirect URI above
2. Use a scope mapping to include group membership as `roles` in the token
3. Set `roles_claim: "roles"` in config

---

## Roles and Permissions

| Action | admin | operator | viewer |
|---|---|---|---|
| View GSLB configs | yes | yes | yes (assigned only) |
| Create/edit/delete configs | yes | yes | no |
| Manage backends | yes | yes | no |
| Configure health checks | yes | yes | no |
| Configure DNS providers | yes | yes | no |
| View audit logs | yes | yes | no |
| Manage users | yes | no | no |
| Assign config roles | yes | no | no |

**Per-config viewer access**: Admins can grant specific viewers read access to individual configs via **Config → Roles**.

---

## DNS Provider Setup

GSLB currently supports **Cloudflare** as the DNS provider.

### Cloudflare

1. Go to **Cloudflare Dashboard → My Profile → API Tokens**
2. Create a token with **Zone → DNS → Edit** permission scoped to your zone
3. In GSLB, navigate to a config and open **DNS Provider**
4. Enter your Zone ID and API token

The token is stored AES-256 encrypted in the database using the `encryption_key` from config.

---

## Health Checks

Each GSLB config can have one health check configuration that applies to all its backends.

### Check types

| Type | Description |
|---|---|
| `icmp` | ICMP ping (requires `CAP_NET_RAW` — see below) |
| `tcp` | TCP connection to `ip:port` |

### Configuration

| Field | Description | Default |
|---|---|---|
| `type` | `icmp` or `tcp` | required |
| `timeout_seconds` | Per-check timeout | 5 |
| `threshold_healthy` | Consecutive successes to mark healthy | 2 |
| `threshold_unhealthy` | Consecutive failures to mark unhealthy | 2 |

### ICMP permissions

The ICMP checker first attempts unprivileged ICMP (`udp4 / SOCK_DGRAM`), which works when the process GID falls within `/proc/sys/net/ipv4/ping_group_range`. If that fails it falls back to a raw socket (`ip4:icmp / SOCK_RAW`), which requires `CAP_NET_RAW`.

**Bare metal / systemd** — grant the capability directly on the binary:

```bash
sudo setcap cap_net_raw+ep /usr/local/bin/gslb
```

The Docker image already has this xattr set at build time (see [Docker](#docker)).

**Docker** — when `no-new-privileges` is active the kernel ignores file capabilities, so the capability must be granted at the container level instead:

```yaml
cap_drop:
  - ALL
cap_add:
  - NET_RAW
```

---

## Load Balancing Modes

### Round-robin

All healthy backends are published to DNS. DNS resolvers distribute traffic across them. Best for active/active setups.

### Weighted (failover)

Only **one backend** is active in DNS at a time — the healthy backend with the lowest weight value. When that backend fails, the next healthy backend (by weight) takes over. This is an active/standby failover mechanism, not true weighted distribution.

To configure priorities, assign weights to backends (lower = higher priority). Reorder them in the UI for display purposes; the actual failover order is determined by weight value.

---

## API Reference

All endpoints require authentication except where noted.

| Method | Path | Description | Auth required |
|---|---|---|---|
| GET | `/health` | Liveness probe | No |
| GET | `/ready` | Readiness probe | No |
| GET | `/api/v1/status` | Application status | Yes |
| GET | `/api/v1/configs` | List GSLB configs | Yes |
| POST | `/api/v1/configs` | Create config | Yes (operator+) |
| GET | `/api/v1/configs/:id` | Get config | Yes |
| PUT | `/api/v1/configs/:id` | Update config | Yes (operator+) |
| DELETE | `/api/v1/configs/:id` | Delete config | Yes (operator+) |
| GET | `/api/v1/configs/:id/backends` | List backends | Yes |
| POST | `/api/v1/configs/:id/backends` | Add backend | Yes (operator+) |
| GET | `/api/v1/backends/:id` | Get backend | Yes |
| PUT | `/api/v1/backends/:id` | Update backend | Yes (operator+) |
| DELETE | `/api/v1/backends/:id` | Delete backend | Yes (operator+) |
| GET | `/api/v1/backends/:id/health` | Get backend health state | Yes |
| GET | `/api/v1/backends/:id/history` | Get backend health history | Yes |
| GET | `/api/v1/configs/:id/health-check` | Get health check config | Yes |
| POST | `/api/v1/configs/:id/health-check` | Configure health check | Yes (operator+) |
| DELETE | `/api/v1/configs/:id/health-check` | Remove health check | Yes (operator+) |
| GET | `/api/v1/configs/:id/dns-provider` | Get DNS provider | Yes |
| POST | `/api/v1/configs/:id/dns-provider` | Configure DNS provider | Yes (operator+) |
| DELETE | `/api/v1/configs/:id/dns-provider` | Remove DNS provider | Yes (operator+) |
| GET | `/api/v1/configs/:id/status` | Get config DNS status | Yes |
| GET | `/api/v1/configs/:id/roles` | Get config role assignments | Yes (admin) |
| PUT | `/api/v1/configs/:id/roles` | Update config role assignments | Yes (admin) |
| GET | `/api/v1/audit-logs` | List audit log entries | Yes (operator+) |
| GET | `/api/v1/roles` | List available roles | Yes |
| GET | `/api/v1/admin/users` | List users | Yes (admin) |
| POST | `/api/v1/admin/users` | Create user | Yes (admin) |
| PUT | `/api/v1/admin/users/:id` | Update user | Yes (admin) |
| DELETE | `/api/v1/admin/users/:id` | Delete user | Yes (admin) |
| GET | `/api/v1/auth/user` | Current user info | Yes |
| GET | `/api/v1/auth/login` | Initiate OIDC login | No |
| GET | `/api/v1/auth/callback` | OIDC callback | No |
| POST | `/api/v1/auth/logout` | Logout / revoke session | Yes |

---

## Systemd Service

Create `/etc/systemd/system/gslb.service`:

```ini
[Unit]
Description=GSLB - Global Server Load Balancer
After=network.target

[Service]
Type=simple
User=gslb
Group=gslb
ExecStart=/usr/local/bin/gslb --config /etc/gslb/config.yaml
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/gslb
PrivateTmp=yes

# Uncomment if ICMP health checks are used.
# setcap cap_net_raw+ep on the binary is the preferred alternative —
# it avoids granting the capability to the entire service unit.
# AmbientCapabilities=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
```

```bash
# Create user and directories
sudo useradd -r -s /sbin/nologin gslb
sudo mkdir -p /etc/gslb /var/lib/gslb
sudo chown gslb:gslb /var/lib/gslb

# Copy binary and config
sudo cp gslb /usr/local/bin/gslb
sudo cp config.yaml /etc/gslb/config.yaml
sudo chmod 640 /etc/gslb/config.yaml
sudo chown root:gslb /etc/gslb/config.yaml

# Grant ICMP capability on the binary (preferred over AmbientCapabilities)
sudo setcap cap_net_raw+ep /usr/local/bin/gslb

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now gslb
```

---

## Docker

The repository includes a production-ready multi-stage `Dockerfile` and a `docker-compose.yml`.

### Image hardening

The image is hardened by default:

- **Non-root user** — runs as `gslb` (UID 1000); `/` is owned by root and not writable by the runtime user
- **File capability** — `cap_net_raw+ep` is set on the binary at build time via `setcap`, enabling ICMP health checks without running as root (bare metal / Kubernetes without `no-new-privileges`)
- **Minimal base** — Alpine 3.21 with only CA certificates and timezone data added
- **Health check** — built-in `HEALTHCHECK` instruction

### docker-compose

```bash
docker compose up -d
```

The bundled `docker-compose.yml` includes:

| Setting | Value |
|---|---|
| `cap_drop` | `ALL` — every capability dropped |
| `cap_add` | `NET_RAW` (commented out by default) |
| `security_opt` | `no-new-privileges:true` |
| `read_only` | `true` — root filesystem is read-only |
| `tmpfs` | `/tmp` (64 MB) for transient files |

> **ICMP health checks in Docker**: with `no-new-privileges:true` the kernel ignores file capabilities, so uncomment `cap_add: NET_RAW` in `docker-compose.yml` if any config uses ICMP health checks.

### Environment variables

All configuration options are exposed as environment variables in `docker-compose.yml`. The most important ones to set for production:

```bash
GSLB_ENCRYPTION_KEY=<openssl rand -hex 32>
GSLB_JWT_SECRET=<openssl rand -base64 32>
GSLB_OIDC_ENABLED=true
GSLB_OIDC_ISSUER=https://auth.example.com/realms/myrealm
GSLB_OIDC_CLIENT_ID=gslb
GSLB_OIDC_CLIENT_SECRET=<client-secret>
GSLB_OIDC_REDIRECT_URL=https://gslb.example.com/api/v1/auth/callback
GSLB_JWT_COOKIE_SECURE=true
```

### Building the image

```bash
docker build -t gslb:latest .
```

---

## Reverse Proxy

GSLB should run behind a reverse proxy that handles TLS termination. Set `jwt.cookie_secure: true` (the default) when serving over HTTPS.

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name gslb.example.com;

    ssl_certificate     /etc/letsencrypt/live/gslb.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/gslb.example.com/privkey.pem;

    location / {
        proxy_pass         http://127.0.0.1:8090;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```
gslb.example.com {
    reverse_proxy localhost:8090
}
```

---

## License

MIT — see [LICENSE](LICENSE).
