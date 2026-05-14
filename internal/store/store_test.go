package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/db"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	database, err := db.NewDB(":memory:")
	require.NoError(t, err)
	err = db.Migrate(database)
	require.NoError(t, err)
	return database
}

func TestConfigCRUD(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}

	t.Run("CreateConfig", func(t *testing.T) {
		err := s.CreateConfig(ctx, cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, cfg.ID)
		assert.False(t, cfg.CreatedAt.IsZero())
		assert.False(t, cfg.UpdatedAt.IsZero())
	})

	t.Run("GetConfig", func(t *testing.T) {
		got, err := s.GetConfig(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, cfg.ID, got.ID)
		assert.Equal(t, cfg.Name, got.Name)
		assert.Equal(t, cfg.DNSName, got.DNSName)
		assert.Equal(t, cfg.DNSTTL, got.DNSTTL)
		assert.Equal(t, cfg.LBMethod, got.LBMethod)
	})

	t.Run("GetConfig_NotFound", func(t *testing.T) {
		_, err := s.GetConfig(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("UpdateConfig", func(t *testing.T) {
		cfg.Name = "updated-config"
		cfg.DNSTTL = 600
		err := s.UpdateConfig(ctx, cfg)
		require.NoError(t, err)

		got, err := s.GetConfig(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated-config", got.Name)
		assert.Equal(t, 600, got.DNSTTL)
	})

	t.Run("ListConfigs", func(t *testing.T) {
		configs, err := s.ListConfigs(ctx)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, cfg.ID, configs[0].ID)
	})

	t.Run("DeleteConfig", func(t *testing.T) {
		err := s.DeleteConfig(ctx, cfg.ID)
		require.NoError(t, err)

		_, err = s.GetConfig(ctx, cfg.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestBackendCRUD(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	port := 8080
	backend := &types.Backend{
		ConfigID: cfg.ID,
		IP:       "192.168.1.1",
		Port:     &port,
		Weight:   10,
		Enabled:  true,
	}

	t.Run("CreateBackend", func(t *testing.T) {
		err := s.CreateBackend(ctx, backend)
		require.NoError(t, err)
		assert.NotEmpty(t, backend.ID)
	})

	t.Run("GetBackend", func(t *testing.T) {
		got, err := s.GetBackend(ctx, backend.ID)
		require.NoError(t, err)
		assert.Equal(t, backend.ID, got.ID)
		assert.Equal(t, backend.ConfigID, got.ConfigID)
		assert.Equal(t, backend.IP, got.IP)
		assert.Equal(t, *backend.Port, *got.Port)
		assert.Equal(t, backend.Weight, got.Weight)
		assert.Equal(t, backend.Enabled, got.Enabled)
	})

	t.Run("GetBackend_NotFound", func(t *testing.T) {
		_, err := s.GetBackend(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("UpdateBackend", func(t *testing.T) {
		backend.IP = "192.168.1.2"
		backend.Weight = 20
		err := s.UpdateBackend(ctx, backend)
		require.NoError(t, err)

		got, err := s.GetBackend(ctx, backend.ID)
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.2", got.IP)
		assert.Equal(t, 20, got.Weight)
	})

	t.Run("ListBackends", func(t *testing.T) {
		backends, err := s.ListBackends(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Len(t, backends, 1)
		assert.Equal(t, backend.ID, backends[0].ID)
	})

	t.Run("DeleteBackend", func(t *testing.T) {
		err := s.DeleteBackend(ctx, backend.ID)
		require.NoError(t, err)

		_, err = s.GetBackend(ctx, backend.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestHealthCheckCRUD(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	hc := &types.HealthCheck{
		ConfigID:           cfg.ID,
		Type:               "tcp",
		IntervalSeconds:    10,
		TimeoutSeconds:     5,
		ThresholdHealthy:   2,
		ThresholdUnhealthy: 3,
	}

	t.Run("CreateHealthCheck", func(t *testing.T) {
		err := s.CreateHealthCheck(ctx, hc)
		require.NoError(t, err)
		assert.NotEmpty(t, hc.ID)
	})

	t.Run("GetHealthCheck", func(t *testing.T) {
		got, err := s.GetHealthCheck(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, hc.ID, got.ID)
		assert.Equal(t, hc.Type, got.Type)
	})

	t.Run("GetHealthCheck_NotFound", func(t *testing.T) {
		_, err := s.GetHealthCheck(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("UpdateHealthCheck", func(t *testing.T) {
		hc.Type = "tcp"
		hc.IntervalSeconds = 20
		err := s.UpdateHealthCheck(ctx, hc)
		require.NoError(t, err)

		got, err := s.GetHealthCheck(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, "tcp", got.Type)
		assert.Equal(t, 20, got.IntervalSeconds)
	})

	t.Run("DeleteHealthCheck", func(t *testing.T) {
		err := s.DeleteHealthCheck(ctx, cfg.ID)
		require.NoError(t, err)

		_, err = s.GetHealthCheck(ctx, cfg.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestHealthStateOps(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	port := 8080
	backend := &types.Backend{
		ConfigID: cfg.ID,
		IP:       "192.168.1.1",
		Port:     &port,
		Weight:   10,
		Enabled:  true,
	}
	err = s.CreateBackend(ctx, backend)
	require.NoError(t, err)

	now := time.Now()
	state := &types.HealthState{
		BackendID:            backend.ID,
		Status:               types.StatusHealthy,
		ConsecutiveSuccesses: 5,
		ConsecutiveFailures:  0,
		LastCheckAt:          &now,
		LastHealthyAt:        &now,
		LastError:            "",
	}

	t.Run("UpdateHealthState", func(t *testing.T) {
		err := s.UpdateHealthState(ctx, state)
		require.NoError(t, err)
	})

	t.Run("GetHealthState", func(t *testing.T) {
		got, err := s.GetHealthState(ctx, backend.ID)
		require.NoError(t, err)
		assert.Equal(t, state.BackendID, got.BackendID)
		assert.Equal(t, state.Status, got.Status)
		assert.Equal(t, state.ConsecutiveSuccesses, got.ConsecutiveSuccesses)
	})

	t.Run("GetHealthState_NotFound", func(t *testing.T) {
		_, err := s.GetHealthState(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("GetHealthStates", func(t *testing.T) {
		states, err := s.GetHealthStates(ctx, cfg.ID)
		require.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, backend.ID, states[backend.ID].BackendID)
	})
}

func TestForeignKeyConstraints(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	t.Run("Backend_ForeignKeyToConfig", func(t *testing.T) {
		port := 8080
		backend := &types.Backend{
			ConfigID: "nonexistent-config",
			IP:       "192.168.1.1",
			Port:     &port,
			Weight:   10,
			Enabled:  true,
		}
		err := s.CreateBackend(ctx, backend)
		assert.ErrorIs(t, err, ErrForeignKeyViolation)
	})

	t.Run("HealthCheck_ForeignKeyToConfig", func(t *testing.T) {
		hc := &types.HealthCheck{
			ConfigID:           "nonexistent-config",
			Type:               "tcp",
			IntervalSeconds:    10,
			TimeoutSeconds:     5,
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 3,
		}
		err := s.CreateHealthCheck(ctx, hc)
		assert.ErrorIs(t, err, ErrForeignKeyViolation)
	})
}

func TestContextCancellation(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx, cancel := context.WithCancel(context.Background())

	cfg := &types.Config{
		Name:     "test-config",
		DNSName:  "test.example.com",
		DNSTTL:   300,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	cancel()

	err = s.CreateConfig(ctx, cfg)
	assert.Error(t, err)
}

func TestTransactions(t *testing.T) {
	s := New(newTestDB(t), nil)
	ctx := context.Background()

	t.Run("WithTx_Success", func(t *testing.T) {
		err := s.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO configs (id, name, dns_name, dns_ttl, lb_method, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				"tx-id", "tx-config", "tx.example.com", 300, "round_robin", time.Now(), time.Now())
			return err
		})
		require.NoError(t, err)

		configs, err := s.ListConfigs(ctx)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
	})

	t.Run("WithTx_Rollback", func(t *testing.T) {
		err := s.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO configs (id, name) VALUES (?, ?)", "dup", "dup")
			return err
		})
		assert.Error(t, err)

		configs, err := s.ListConfigs(ctx)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
	})
}
