package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewDB(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() failed: %v", err)
	}
}

func TestMigrate(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	// Verify tables exist
	tables := []string{"configs", "backends", "health_checks", "health_states", "dns_providers"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %s was not created", table)
		} else if err != nil {
			t.Errorf("failed to check table %s: %v", table, err)
		}
	}
}

func TestForeignKeys(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	// Insert a config
	_, err = db.Exec(`
		INSERT INTO configs (id, name, dns_name, dns_ttl, lb_method)
		VALUES ('config-1', 'test-config', 'test.example.com', 30, 'round_robin')
	`)
	if err != nil {
		t.Fatalf("failed to insert config: %v", err)
	}

	// Insert a backend linked to the config
	_, err = db.Exec(`
		INSERT INTO backends (id, config_id, ip, port, weight, enabled)
		VALUES ('backend-1', 'config-1', '192.168.1.1', 8080, 1, true)
	`)
	if err != nil {
		t.Fatalf("failed to insert backend: %v", err)
	}

	// Insert health state linked to the backend
	_, err = db.Exec(`
		INSERT INTO health_states (backend_id, status, consecutive_successes, consecutive_failures)
		VALUES ('backend-1', 'healthy', 5, 0)
	`)
	if err != nil {
		t.Fatalf("failed to insert health_state: %v", err)
	}

	// Verify CASCADE delete works - delete config and verify backend and health_state are deleted
	_, err = db.Exec("DELETE FROM configs WHERE id = 'config-1'")
	if err != nil {
		t.Fatalf("failed to delete config: %v", err)
	}

	var backendCount, healthStateCount int
	err = db.QueryRow("SELECT COUNT(*) FROM backends WHERE id = 'backend-1'").Scan(&backendCount)
	if err != nil {
		t.Fatalf("failed to query backends: %v", err)
	}
	err = db.QueryRow("SELECT COUNT(*) FROM health_states WHERE backend_id = 'backend-1'").Scan(&healthStateCount)
	if err != nil {
		t.Fatalf("failed to query health_states: %v", err)
	}

	if backendCount != 0 {
		t.Error("backend was not deleted when config was deleted (foreign key CASCADE not working)")
	}
	if healthStateCount != 0 {
		t.Error("health_state was not deleted when backend was deleted (foreign key CASCADE not working)")
	}
}

func TestIndexes(t *testing.T) {
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	indexes := []struct {
		name string
		sql  string
	}{
		{"idx_backends_config_id", "SELECT 1 FROM backends WHERE config_id = 'test'"},
		{"idx_health_states_backend_id", "SELECT 1 FROM health_states WHERE backend_id = 'test'"},
	}

	for _, idx := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx.name).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("index %s was not created", idx.name)
		} else if err != nil {
			t.Errorf("failed to check index %s: %v", idx.name, err)
		}
	}
}
