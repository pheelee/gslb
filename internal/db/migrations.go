package db

import (
	"database/sql"
	"fmt"
)

// Migrate runs all database migrations to create the schema.
func Migrate(db *sql.DB) error {
	migrations := []string{
		// configs table
		`CREATE TABLE IF NOT EXISTS configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			dns_name TEXT NOT NULL UNIQUE,
			dns_ttl INTEGER NOT NULL DEFAULT 30,
			lb_method TEXT NOT NULL DEFAULT 'round_robin',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// backends table
		`CREATE TABLE IF NOT EXISTS backends (
			id TEXT PRIMARY KEY,
			config_id TEXT NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER,
			weight INTEGER DEFAULT 1,
			enabled BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE
		);`,

		// health_checks table
		`CREATE TABLE IF NOT EXISTS health_checks (
			id TEXT PRIMARY KEY,
			config_id TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			interval_seconds INTEGER DEFAULT 10,
			timeout_seconds INTEGER DEFAULT 5,
			threshold_healthy INTEGER DEFAULT 2,
			threshold_unhealthy INTEGER DEFAULT 3,
			http_path TEXT,
			http_expected_status INTEGER,
			FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE
		);`,

		// health_states table
		`CREATE TABLE IF NOT EXISTS health_states (
			backend_id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'unknown',
			consecutive_successes INTEGER DEFAULT 0,
			consecutive_failures INTEGER DEFAULT 0,
			last_check_at TIMESTAMP,
			last_healthy_at TIMESTAMP,
			last_error TEXT,
			FOREIGN KEY (backend_id) REFERENCES backends(id) ON DELETE CASCADE
		);`,

		// dns_providers table
		`CREATE TABLE IF NOT EXISTS dns_providers (
			id TEXT PRIMARY KEY,
			config_id TEXT NOT NULL UNIQUE,
			provider_type TEXT NOT NULL,
			config_json TEXT NOT NULL,
			FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE
		);`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_backends_config_id ON backends(config_id);`,
		`CREATE INDEX IF NOT EXISTS idx_health_states_backend_id ON health_states(backend_id);`,

		// audit_logs table
		`CREATE TABLE IF NOT EXISTS audit_logs (
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
		);`,

		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);`,

		// users table
		`CREATE TABLE IF NOT EXISTS users (
			id           TEXT PRIMARY KEY,
			subject      TEXT UNIQUE NOT NULL,
			email        TEXT NOT NULL,
			name         TEXT,
			created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login_at TIMESTAMP,
			is_active    BOOLEAN DEFAULT true
		);`,
		`CREATE INDEX IF NOT EXISTS idx_users_subject ON users(subject);`,
		`CREATE INDEX IF NOT EXISTS idx_users_email   ON users(email);`,

		// roles table
		`CREATE TABLE IF NOT EXISTS roles (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,

		// seed default roles (ignore if already present)
		`INSERT OR IGNORE INTO roles (id, name, description) VALUES
			('00000000-0000-0000-0000-000000000001', 'admin',    'Full access to all configurations'),
			('00000000-0000-0000-0000-000000000002', 'operator', 'Can manage assigned configurations'),
			('00000000-0000-0000-0000-000000000003', 'viewer',   'Read-only access to assigned configurations');`,

		// user_roles join table
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id     TEXT NOT NULL,
			role_id     TEXT NOT NULL,
			assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			assigned_by TEXT,
			PRIMARY KEY (user_id, role_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
		);`,

		// config_roles join table
		`CREATE TABLE IF NOT EXISTS config_roles (
			config_id TEXT NOT NULL,
			role_id   TEXT NOT NULL,
			PRIMARY KEY (config_id, role_id),
			FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id)   REFERENCES roles(id)   ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_config_roles_config ON config_roles(config_id);`,
		`CREATE INDEX IF NOT EXISTS idx_config_roles_role   ON config_roles(role_id);`,

		// sessions table (server-side session revocation)
		`CREATE TABLE IF NOT EXISTS sessions (
			jti        TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			revoked_at TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`,

		// health_history table — aggregated 5-minute probe buckets
		`CREATE TABLE IF NOT EXISTS health_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			backend_id TEXT NOT NULL,
			bucket_start TIMESTAMP NOT NULL,
			success_count INTEGER NOT NULL DEFAULT 0,
			failure_count INTEGER NOT NULL DEFAULT 0,
			selected INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(backend_id, bucket_start)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_health_history_backend_bucket ON health_history(backend_id, bucket_start);`,
	}

	for i, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	// Incremental column migrations (safe to re-run; ignore "duplicate column" errors)
	optionalMigrations := []string{
		`ALTER TABLE audit_logs ADD COLUMN user_id TEXT`,
		`ALTER TABLE audit_logs ADD COLUMN config_id TEXT`,
		`ALTER TABLE configs ADD COLUMN last_reconcile_error TEXT`,
	}
	for _, m := range optionalMigrations {
		_, _ = db.Exec(m) // ignore error if column already exists
	}

	return nil
}
