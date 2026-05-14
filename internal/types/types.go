package types

import "time"

// HealthStatus represents backend health state
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// LBMethod represents load balancing algorithm
type LBMethod string

const (
	RoundRobin LBMethod = "round_robin"
	Weighted   LBMethod = "weighted"
)

// Config represents a GSLB configuration
type Config struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	DNSName            string    `json:"dns_name"`
	DNSTTL             int       `json:"dns_ttl"`
	LBMethod           LBMethod  `json:"lb_method"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	LastReconcileError string    `json:"last_reconcile_error,omitempty"`
}

// Backend represents a backend server
type Backend struct {
	ID       string `json:"id"`
	ConfigID string `json:"config_id"`
	IP       string `json:"ip"`
	Port     *int   `json:"port,omitempty"`
	Weight   int    `json:"weight"`
	Enabled  bool   `json:"enabled"`
}

// HealthCheck represents health check configuration
type HealthCheck struct {
	ID                 string `json:"id"`
	ConfigID           string `json:"config_id"`
	Type               string `json:"type"` // "icmp", "tcp"
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	ThresholdHealthy   int    `json:"threshold_healthy"`
	ThresholdUnhealthy int    `json:"threshold_unhealthy"`
}

// HealthState represents current health status of a backend
type HealthState struct {
	BackendID            string       `json:"backend_id"`
	Status               HealthStatus `json:"status"`
	ConsecutiveSuccesses int          `json:"consecutive_successes"`
	ConsecutiveFailures  int          `json:"consecutive_failures"`
	LastCheckAt          *time.Time   `json:"last_check_at,omitempty"`
	LastHealthyAt        *time.Time   `json:"last_healthy_at,omitempty"`
	LastError            string       `json:"last_error,omitempty"`
}

// DNSProviderConfig represents DNS provider configuration
type DNSProviderConfig struct {
	ID           string `json:"id"`
	ConfigID     string `json:"config_id"`
	ProviderType string `json:"provider_type"` // "cloudflare", "mock"
	ConfigJSON   string `json:"config_json"`   // Provider-specific config as JSON
}

// HealthHistoryBucket stores aggregated 5-minute probe data for a backend.
type HealthHistoryBucket struct {
	ID           int64     `json:"id,omitempty"`
	BackendID    string    `json:"backend_id"`
	BucketStart  time.Time `json:"bucket_start"`
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	Selected     int       `json:"selected"` // 1 if backend was selected for DNS in this bucket
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// BackendHistoryResponse is returned by GET /api/v1/backends/:id/history.
type BackendHistoryResponse struct {
	Buckets []HealthHistoryBucket `json:"buckets"`
	Uptime  map[string]float64    `json:"uptime"` // keys: "24h", "7d", "30d"
}

// AuditLog represents a user action audit log entry
type AuditLog struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`      // e.g., "config:create", "backend:delete"
	EntityType string    `json:"entity_type"` // e.g., "config", "backend"
	EntityID   string    `json:"entity_id"`   // ID of the affected entity
	EntityName string    `json:"entity_name"` // Human-readable name of the affected entity
	ConfigID   string    `json:"config_id"`   // ID of the parent config (same as EntityID for config entities)
	Details    string    `json:"details"`     // JSON with field-level changes
	UserID     string    `json:"user_id"`     // Authenticated user ID (empty if unauthenticated)
	IPAddress  string    `json:"ip_address"`  // User's IP address
	UserAgent  string    `json:"user_agent"`  // User's browser/client
	CreatedAt  time.Time `json:"created_at"`
}
