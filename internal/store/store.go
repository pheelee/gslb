// Package store provides the data access layer for GSLB.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pheelee/gslb/internal/crypto"
	"github.com/pheelee/gslb/internal/types"
)

var (
	// ErrNotFound is returned when a record is not found.
	ErrNotFound = errors.New("record not found")

	// ErrAlreadyExists is returned when trying to create a duplicate record.
	ErrAlreadyExists = errors.New("record already exists")

	// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
	ErrForeignKeyViolation = errors.New("foreign key violation")
)

// SQLRunner is an interface that both *sql.DB and *sql.Tx satisfy.
type SQLRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// TxFunc is a function that runs within a transaction.
type TxFunc func(ctx context.Context, tx *sql.Tx) error

// DNSProviderStore defines operations for DNS provider configuration.
type DNSProviderStore interface {
	CreateDNSProvider(ctx context.Context, provider *types.DNSProviderConfig) error
	GetDNSProvider(ctx context.Context, configID string) (*types.DNSProviderConfig, error)
	UpdateDNSProvider(ctx context.Context, provider *types.DNSProviderConfig) error
	DeleteDNSProvider(ctx context.Context, configID string) error
}

// UserStore defines user CRUD and role-assignment operations.
type UserStore interface {
	CreateUser(ctx context.Context, user *types.User) error
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	GetUserBySubject(ctx context.Context, subject string) (*types.User, error)
	UpdateUser(ctx context.Context, user *types.User) error
	ListUsers(ctx context.Context) ([]types.User, error)
	AssignRole(ctx context.Context, userID, roleID, assignedBy string) error
	RemoveRole(ctx context.Context, userID, roleID string) error
	GetUserRoles(ctx context.Context, userID string) ([]types.Role, error)
}

// RoleStore defines role lookup operations.
type RoleStore interface {
	GetRoleByName(ctx context.Context, name string) (*types.Role, error)
	GetRoleByID(ctx context.Context, id string) (*types.Role, error)
	ListRoles(ctx context.Context) ([]types.Role, error)
}

// SessionStore defines session record operations for server-side revocation.
type SessionStore interface {
	CreateSessionRecord(ctx context.Context, sess *types.SessionRecord) error
	GetSessionRecord(ctx context.Context, jti string) (*types.SessionRecord, error)
	RevokeSession(ctx context.Context, jti string) error
	RevokeUserSessions(ctx context.Context, userID string) error
	CleanupExpiredSessions(ctx context.Context) (int64, error)
}

// ConfigRoleStore defines config-level access control operations.
type ConfigRoleStore interface {
	AssignConfigRole(ctx context.Context, configID, roleID string) error
	RemoveConfigRole(ctx context.Context, configID, roleID string) error
	GetConfigRoles(ctx context.Context, configID string) ([]types.Role, error)
	GetConfigsForRole(ctx context.Context, roleID string) ([]string, error)
	CanAccess(ctx context.Context, userID, configID string) (bool, error)
	ListConfigsForUser(ctx context.Context, userID string) ([]string, error)
}

// Store combines all sub-stores.
type Store interface {
	ConfigStore
	BackendStore
	HealthStore
	HealthHistoryStore
	DNSProviderStore
	AuditLogStore
	UserStore
	RoleStore
	ConfigRoleStore
	SessionStore
	Transactioner
}

// Transactioner supports transaction operations.
type Transactioner interface {
	WithTx(ctx context.Context, fn TxFunc) error
}

// configStore implements ConfigStore interface.
type configStore struct {
	db *sql.DB
}

// backendStore implements BackendStore interface.
type backendStore struct {
	db *sql.DB
}

// healthStore implements HealthStore interface.
type healthStore struct {
	db *sql.DB
}

// healthHistoryStore implements HealthHistoryStore interface.
type healthHistoryStore struct {
	db *sql.DB
}

// New creates a new Store with the given database connection.
// enc may be nil to disable DNS credential encryption.
func New(db *sql.DB, enc *crypto.Encryptor) Store {
	s := &store{}
	s.configStore = &configStore{db: db}
	s.backendStore = &backendStore{db: db}
	s.healthStore = &healthStore{db: db}
	s.healthHistoryStore = &healthHistoryStore{db: db}
	s.dnsProviderStore = &dnsProviderStore{db: db, enc: enc}
	s.auditLogStore = &auditLogStore{db: db}
	s.userStore = &userStore{db: db}
	s.sessionStore = &sessionStore{db: db}
	return s
}

type store struct {
	*configStore
	*backendStore
	*healthStore
	*healthHistoryStore
	*dnsProviderStore
	*auditLogStore
	*userStore
	*sessionStore
}

// ConfigStore operations
type ConfigStore interface {
	CreateConfig(ctx context.Context, cfg *types.Config) error
	GetConfig(ctx context.Context, id string) (*types.Config, error)
	UpdateConfig(ctx context.Context, cfg *types.Config) error
	DeleteConfig(ctx context.Context, id string) error
	ListConfigs(ctx context.Context) ([]types.Config, error)
	// SetConfigReconcileError persists the last reconciler error (empty string clears it).
	SetConfigReconcileError(ctx context.Context, id string, errMsg string) error
}

// BackendStore operations
type BackendStore interface {
	CreateBackend(ctx context.Context, backend *types.Backend) error
	GetBackend(ctx context.Context, id string) (*types.Backend, error)
	UpdateBackend(ctx context.Context, backend *types.Backend) error
	DeleteBackend(ctx context.Context, id string) error
	ListBackends(ctx context.Context, configID string) ([]types.Backend, error)
}

// HealthStore operations
type HealthStore interface {
	CreateHealthCheck(ctx context.Context, hc *types.HealthCheck) error
	GetHealthCheck(ctx context.Context, configID string) (*types.HealthCheck, error)
	UpdateHealthCheck(ctx context.Context, hc *types.HealthCheck) error
	DeleteHealthCheck(ctx context.Context, configID string) error

	UpdateHealthState(ctx context.Context, state *types.HealthState) error
	GetHealthState(ctx context.Context, backendID string) (*types.HealthState, error)
	GetHealthStates(ctx context.Context, configID string) (map[string]types.HealthState, error)
}

// HealthHistoryStore defines probe-history aggregation operations.
type HealthHistoryStore interface {
	UpsertHealthHistoryBucket(ctx context.Context, bucket *types.HealthHistoryBucket) error
	GetHealthHistory(ctx context.Context, backendID string, since time.Time) ([]types.HealthHistoryBucket, error)
	DeleteOldHealthHistory(ctx context.Context, olderThan time.Duration) (int64, error)
}

// AuditLogFilter holds optional query parameters for listing audit logs.
type AuditLogFilter struct {
	UserID     string     // Filter by authenticated user ID
	EntityType string     // Filter by entity type (e.g. "config", "backend")
	Action     string     // Filter by action (e.g. "config:create")
	From       *time.Time // Inclusive lower bound on created_at
	To         *time.Time // Inclusive upper bound on created_at
}

// AuditLogStore operations
type AuditLogStore interface {
	CreateAuditLog(ctx context.Context, log *types.AuditLog) error
	ListAuditLogs(ctx context.Context, filter AuditLogFilter, limit int, offset int) ([]types.AuditLog, error)
	GetAuditLog(ctx context.Context, id string) (*types.AuditLog, error)
	GetAuditLogsForEntity(ctx context.Context, entityType string, entityID string, limit int) ([]types.AuditLog, error)
	DeleteOldAuditLogs(ctx context.Context, olderThan time.Duration) (int64, error)
	CountAuditLogs(ctx context.Context, filter AuditLogFilter) (int64, error)
}

// BeginTx starts a new transaction.
func (s *store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.configStore.db.BeginTx(ctx, nil)
}

// Commit commits a transaction.
func Commit(tx *sql.Tx) error {
	return tx.Commit()
}

// Rollback rolls back a transaction.
func Rollback(tx *sql.Tx) error {
	return tx.Rollback()
}

// WithTx runs a function within a transaction.
func (s *store) WithTx(ctx context.Context, fn TxFunc) error {
	tx, err := s.configStore.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}
