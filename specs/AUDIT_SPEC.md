# Audit System Specification

## Overview

The GSLB audit system provides comprehensive tracking of all user actions and system changes. It records who made changes, when they were made, what entity was affected, and exactly what properties changed. This enables security auditing, compliance reporting, and operational troubleshooting.

## Goals

1. **Complete Accountability**: Record every configuration change with user identity and timestamp
2. **Detailed Change Tracking**: Capture field-level changes showing before/after values
3. **Non-Repudiation**: Immutable audit trail with IP address and user agent
4. **Queryable History**: Filter and search audit logs by entity, user, or time range
5. **Compliance Ready**: Support for retention policies and export capabilities

## Data Model

### AuditLog Entry

```go
type AuditLog struct {
    ID         string    `json:"id"`          // UUID v4
    Action     string    `json:"action"`      // Format: "{entity}:{operation}"
    EntityType string    `json:"entity_type"` // "config", "backend", "health_check", etc.
    EntityID   string    `json:"entity_id"`   // UUID of affected entity
    EntityName string    `json:"entity_name"` // Human-readable name
    Details    string    `json:"details"`     // JSON with changed properties
    UserID     string    `json:"user_id"`     // Authenticated user ID (OIDC sub)
    IPAddress  string    `json:"ip_address"`  // Client IP address
    UserAgent  string    `json:"user_agent"`  // Browser/client identifier
    CreatedAt  time.Time `json:"created_at"`  // UTC timestamp
}
```

### Action Format

Actions follow the pattern `{entity_type}:{operation}`:

| Entity Type | Operations | Example |
|-------------|------------|---------|
| config | create, update, delete | `config:create` |
| backend | create, update, delete | `backend:update` |
| health_check | create, update, delete | `health_check:delete` |
| dns_provider | create, update, delete | `dns_provider:update` |

### Details JSON Schema

```json
{
  "changes": [
    {
      "field": "dns_name",
      "old": "old.example.com",
      "new": "new.example.com"
    },
    {
      "field": "dns_ttl",
      "old": "300",
      "new": "60"
    }
  ]
}
```

**Special Cases:**
- **Create**: All fields have empty `old` values
- **Delete**: All fields have empty `new` values
- **No Changes**: Empty `changes` array or `{}`

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    entity_name TEXT,
    details TEXT,
    user_id TEXT,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
```

## API Endpoints

### List Audit Logs
```
GET /api/v1/audit-logs?limit={n}&offset={n}
```

**Response:**
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "action": "config:update",
      "entity_type": "config",
      "entity_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "entity_name": "production-api",
      "details": "{\"changes\":[{\"field\":\"dns_ttl\",\"old\":\"300\",\"new\":\"60\"}]}",
      "user_id": "auth0|123456789",
      "ip_address": "203.0.113.42",
      "user_agent": "Mozilla/5.0...",
      "created_at": "2024-01-15T09:30:00Z"
    }
  ]
}
```

### Get Single Audit Log
```
GET /api/v1/audit-logs/:id
```

### Get Audit Logs for Entity
```
GET /api/v1/audit-logs/entity/:type/:id?limit={n}
```

Returns all audit log entries for a specific entity (useful for viewing history of a single config).

### Get Audit Log Count
```
GET /api/v1/audit-logs/count
```

**Response:**
```json
{
  "data": {"count": 1542}
}
```

## User Identity

### Authenticated Mode (OIDC Enabled)

When OIDC authentication is enabled:
- `user_id` is populated from the OIDC subject claim
- Users table links subject to internal user ID
- All audit entries include authenticated user identity

### Unauthenticated Mode (OIDC Disabled)

When running without authentication:
- `user_id` is empty string
- Audit still captures IP address and user agent
- Suitable for single-user or development deployments

### User Context Extraction

```go
func (h *Handlers) logAudit(ctx context.Context, action string, entityType string, 
    entityID string, entityName string, details string, c *gin.Context) {
    
    logEntry := &types.AuditLog{
        Action:     action,
        EntityType: entityType,
        EntityID:   entityID,
        EntityName: entityName,
        Details:    details,
    }

    if c != nil {
        logEntry.IPAddress = c.ClientIP()
        logEntry.UserAgent = c.Request.UserAgent()
        
        // Extract authenticated user from context
        if session, ok := auth.SessionFromContext(c); ok {
            logEntry.UserID = session.UserID
        }
    }
    
    // Persist audit log
    if err := h.store.CreateAuditLog(ctx, logEntry); err != nil {
        slog.Error("failed to create audit log", "error", err)
    }
}
```

## Change Tracking Implementation

### Field Comparison Logic

For updates, compare each field between old and new values:

```go
var changes []ChangeEntry
if existing.Name != cfg.Name {
    changes = append(changes, ChangeEntry{
        Field: "name", 
        Old:   existing.Name, 
        New:   cfg.Name,
    })
}
if existing.DNSTTL != cfg.DNSTTL {
    changes = append(changes, ChangeEntry{
        Field: "dns_ttl",
        Old:   strconv.Itoa(existing.DNSTTL),
        New:   strconv.Itoa(cfg.DNSTTL),
    })
}
```

### Nullable Fields

For nullable fields (e.g., `port`), handle nil values:

```go
func formatPort(port *int) string {
    if port == nil {
        return ""
    }
    return strconv.Itoa(*port)
}
```

### Boolean Fields

Convert booleans to strings for consistent storage:

```go
func formatBool(b bool) string {
    if b {
        return "true"
    }
    return "false"
}
```

## Retention and Cleanup

### Automatic Cleanup

Old audit logs can be purged to manage database size:

```go
// Delete audit logs older than 90 days
deleted, err := store.DeleteOldAuditLogs(ctx, 90*24*time.Hour)
```

### Retention Recommendations

| Environment | Retention Period | Rationale |
|-------------|------------------|-----------|
| Production | 1-2 years | Compliance and security forensics |
| Staging | 30-90 days | Debugging and testing |
| Development | 7-30 days | Limited storage needs |

## Frontend Display

### Audit Log Page

The AuditLog.vue component provides:

1. **Table View**: Timestamp, Action badge, Entity type, Name, Changes count, IP
2. **Expandable Rows**: Click to view detailed field-level changes
3. **Diff Visualization**: Old values in red (strikethrough), new values in green
4. **Pagination**: Load more entries on demand
5. **Filtering**: By entity type, action type, or date range (future enhancement)

### Action Badges

Color-coded badges indicate operation type:
- **Green**: Create operations
- **Yellow**: Update operations  
- **Red**: Delete operations
- **Blue**: Other operations

## Security Considerations

1. **Immutable Logs**: Audit logs should be append-only; no update or delete operations through API
2. **Admin-Only Access**: Audit log endpoints require admin role
3. **Sensitive Data**: The `details` field may contain sensitive values; ensure proper access controls
4. **IP Privacy**: Consider GDPR/privacy laws when storing IP addresses
5. **Log Injection**: Validate and sanitize all user-controlled fields before storage

## Performance Considerations

1. **Async Logging**: Audit logging should not block the main request
2. **Batch Inserts**: For bulk operations, consider batching audit log inserts
3. **Partitioning**: For high-volume systems, partition by `created_at` date
4. **Indexing**: Ensure indexes support common query patterns (by entity, by date)
5. **Cleanup Scheduling**: Run cleanup during off-peak hours

## Future Enhancements

1. **Advanced Filtering**: Filter by date range, user, action type, or entity
2. **Export**: CSV/JSON export for compliance reporting
3. **Real-time Stream**: WebSocket feed of audit events
4. **Aggregations**: Dashboard showing activity trends
5. **Alerting**: Trigger alerts on suspicious patterns (e.g., mass deletions)
6. **Data Integrity**: Cryptographic signing of audit entries
7. **Archival**: Move old logs to cold storage (S3, etc.)

## Testing

### Unit Tests

```go
func TestCreateAuditLog(t *testing.T) {
    store := newTestStore()
    log := &types.AuditLog{
        Action:     "config:create",
        EntityType: "config",
        EntityID:   "test-id",
        EntityName: "test-config",
        UserID:     "user-123",
    }
    
    err := store.CreateAuditLog(ctx, log)
    require.NoError(t, err)
    assert.NotEmpty(t, log.ID)
    assert.WithinDuration(t, time.Now(), log.CreatedAt, time.Second)
}
```

### Integration Tests

1. Create config → Verify audit log created
2. Update config → Verify changes captured correctly
3. Delete config → Verify delete audit entry
4. Verify user ID captured when authenticated
5. Verify empty user ID when unauthenticated
6. Test pagination and filtering

## Migration Guide

### Adding Audit to New Entity Types

1. **Add handler calls** in create/update/delete endpoints:
```go
changes := []ChangeEntry{
    {Field: "field_name", Old: "", New: value},
}
h.logAudit(c.Request.Context(), "entity:create", "entity", id, name, buildDetails(changes), c)
```

2. **Update frontend** (if new entity type):
Add CSS class in AuditLog.vue:
```css
.entity-newtype {
  /* styling */
}
```

3. **Add tests** for new audit events

## Current Implementation Status

✅ **Implemented:**
- Database schema and migrations
- AuditLog type and store methods
- API endpoints (list, get, count, by entity)
- User identity extraction from OIDC context
- Frontend AuditLog.vue with diff view
- Config entity audit logging (create, update, delete)

🔄 **Pending:**
- Backend entity audit logging
- Health check entity audit logging
- DNS provider entity audit logging
- Automatic retention cleanup
- Advanced filtering API
- Export functionality
