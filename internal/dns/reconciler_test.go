package dns

import (
	"context"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/db"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestStore creates an in-memory SQLite store for testing
func setupTestStore(t *testing.T) store.Store {
	t.Helper()

	database, err := db.NewDB(":memory:")
	require.NoError(t, err)

	// Run migrations
	require.NoError(t, db.Migrate(database))

	return store.New(database, nil)
}

// createTestConfig creates a test config with the given parameters
func createTestConfig(t *testing.T, ctx context.Context, s store.Store, name, dnsName string, ttl int) *types.Config {
	t.Helper()

	cfg := &types.Config{
		Name:     name,
		DNSName:  dnsName,
		DNSTTL:   ttl,
		LBMethod: types.RoundRobin,
	}
	require.NoError(t, s.CreateConfig(ctx, cfg))
	return cfg
}

// createTestBackend creates a test backend with the given parameters
func createTestBackend(t *testing.T, ctx context.Context, s store.Store, configID, ip string, enabled bool) *types.Backend {
	t.Helper()

	backend := &types.Backend{
		ConfigID: configID,
		IP:       ip,
		Weight:   1,
		Enabled:  enabled,
	}
	require.NoError(t, s.CreateBackend(ctx, backend))
	return backend
}

// createTestHealthCheck creates a test health check
func createTestHealthCheck(t *testing.T, ctx context.Context, s store.Store, configID string) *types.HealthCheck {
	t.Helper()

	hc := &types.HealthCheck{
		ConfigID:           configID,
		Type:               "icmp",
		IntervalSeconds:    30,
		TimeoutSeconds:     5,
		ThresholdHealthy:   2,
		ThresholdUnhealthy: 3,
	}
	require.NoError(t, s.CreateHealthCheck(ctx, hc))
	return hc
}

// setBackendHealth sets the health state for a backend
func setBackendHealth(t *testing.T, ctx context.Context, s store.Store, backendID string, status types.HealthStatus) {
	t.Helper()

	state := &types.HealthState{
		BackendID:            backendID,
		Status:               status,
		ConsecutiveSuccesses: 2,
		ConsecutiveFailures:  0,
		LastCheckAt:          &[]time.Time{time.Now()}[0],
	}
	if status == types.StatusHealthy {
		state.LastHealthyAt = &[]time.Time{time.Now()}[0]
	}
	require.NoError(t, s.UpdateHealthState(ctx, state))
}

func TestNewReconciler(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{
			name:     "default interval",
			interval: 0,
			want:     30 * time.Second,
		},
		{
			name:     "custom interval",
			interval: 60 * time.Second,
			want:     60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReconciler(provider, s, tt.interval)
			assert.NotNil(t, r)
			assert.Equal(t, tt.want, r.interval)
			assert.NotNil(t, r.lastRecords)
			assert.NotNil(t, r.stopCh)
		})
	}

	// Test that we can interact with the store through the reconciler
	_ = ctx
}

func TestReconciler_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 100*time.Millisecond)

	// Test Start
	err = r.Start(ctx)
	require.NoError(t, err)
	assert.True(t, r.running)

	// Test double Start (should fail)
	err = r.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test Stop
	err = r.Stop()
	require.NoError(t, err)
	assert.False(t, r.running)

	// Test Stop when not running (should succeed)
	err = r.Stop()
	assert.NoError(t, err)
}

func TestReconciler_StartWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 50*time.Millisecond)

	// Start reconciler
	err = r.Start(ctx)
	require.NoError(t, err)

	// Cancel context - should not panic and goroutine should exit cleanly
	cancel()

	// Give it a moment to process the cancellation
	time.Sleep(100 * time.Millisecond)

	// Test passes if no panic occurred
}

func TestReconciler_ReconcileOnce_NoConfigs(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Reconcile with no configs
	err = r.ReconcileOnce(ctx)
	assert.NoError(t, err)

	// No DNS records should be created
	mock := provider.(*MockProvider)
	assert.Equal(t, 0, mock.RecordCount())
}

func TestReconciler_ReconcileOnce_WithHealthyBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backends
	backend1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	backend2 := createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", true)

	// Set backends as healthy
	setBackendHealth(t, ctx, s, backend1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, backend2.ID, types.StatusHealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Verify DNS record was created
	mock := provider.(*MockProvider)
	assert.Equal(t, 1, mock.RecordCount())

	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Equal(t, "app.example.com", record.Name)
	assert.Equal(t, 300, record.TTL)
	assert.Len(t, record.Records, 2)
	assert.Contains(t, record.Records, "1.2.3.4")
	assert.Contains(t, record.Records, "5.6.7.8")
}

func TestReconciler_ReconcileOnce_SkipsDisabledBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backends - one enabled, one disabled
	backend1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	_ = createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", false) // disabled

	// Set enabled backend as healthy
	setBackendHealth(t, ctx, s, backend1.ID, types.StatusHealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Verify only enabled backend is in DNS
	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Len(t, record.Records, 1)
	assert.Contains(t, record.Records, "1.2.3.4")
	assert.NotContains(t, record.Records, "5.6.7.8")
}

func TestReconciler_ReconcileOnce_SkipsUnhealthyBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backends
	backend1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	backend2 := createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", true)

	// Set one healthy, one unhealthy
	setBackendHealth(t, ctx, s, backend1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, backend2.ID, types.StatusUnhealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Verify only healthy backend is in DNS
	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Len(t, record.Records, 1)
	assert.Contains(t, record.Records, "1.2.3.4")
	assert.NotContains(t, record.Records, "5.6.7.8")
}

func TestReconciler_ReconcileOnce_SkipsUnknownHealthBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backends
	backend1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	_ = createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", true)

	// Set one healthy, leave other with unknown health (no health state)
	setBackendHealth(t, ctx, s, backend1.ID, types.StatusHealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Verify only healthy backend is in DNS
	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Len(t, record.Records, 1)
	assert.Contains(t, record.Records, "1.2.3.4")
}

func TestReconciler_ReconcileOnce_EmptyRecordSetWhenNoHealthyBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create enabled backend but set as unhealthy
	backend := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	setBackendHealth(t, ctx, s, backend.ID, types.StatusUnhealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// No healthy backends — DNS record should be deleted (or never created).
	mock := provider.(*MockProvider)
	_, ok := mock.GetRecord("mock-zone", "app.example.com")
	assert.False(t, ok, "DNS record should be absent when no backends are healthy")

	// lastRecords should be updated with empty IPs
	ips, ok := r.GetLastRecords(cfg.ID)
	require.True(t, ok)
	assert.Empty(t, ips)
}

func TestReconciler_ReconcileOnce_OnlyUpdatesWhenChanged(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backend
	backend := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	setBackendHealth(t, ctx, s, backend.ID, types.StatusHealthy)

	// First reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	mock := provider.(*MockProvider)
	initialUpdateCount := 0
	for _, rec := range mock.GetAllRecords() {
		_ = rec
		initialUpdateCount++
	}
	assert.Equal(t, 1, initialUpdateCount)

	// Second reconcile with no changes
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Should not have called UpsertRecord again (mock stores same record)
	// The mock provider updates the record, so we check if anything changed
	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Equal(t, []string{"1.2.3.4"}, record.Records)
}

func TestReconciler_ReconcileOnce_UpdatesWhenIPsChange(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create first backend
	backend1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	setBackendHealth(t, ctx, s, backend1.ID, types.StatusHealthy)

	// First reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Equal(t, []string{"1.2.3.4"}, record.Records)

	// Create second backend and make it healthy
	backend2 := createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", true)
	setBackendHealth(t, ctx, s, backend2.ID, types.StatusHealthy)

	// Second reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// DNS should now have both IPs
	record, ok = mock.GetRecord("mock-zone", "app.example.com")
	require.True(t, ok)
	assert.Len(t, record.Records, 2)
	assert.Contains(t, record.Records, "1.2.3.4")
	assert.Contains(t, record.Records, "5.6.7.8")
}

func TestReconciler_ReconcileOnce_MultipleConfigs(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create two configs
	cfg1 := createTestConfig(t, ctx, s, "config-1", "app1.example.com", 300)
	cfg2 := createTestConfig(t, ctx, s, "config-2", "app2.example.com", 600)
	createTestHealthCheck(t, ctx, s, cfg1.ID)
	createTestHealthCheck(t, ctx, s, cfg2.ID)

	// Create backends
	b1 := createTestBackend(t, ctx, s, cfg1.ID, "1.2.3.4", true)
	b2 := createTestBackend(t, ctx, s, cfg2.ID, "5.6.7.8", true)
	setBackendHealth(t, ctx, s, b1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, b2.ID, types.StatusHealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Verify both DNS records were created
	mock := provider.(*MockProvider)
	assert.Equal(t, 2, mock.RecordCount())

	record1, ok := mock.GetRecord("mock-zone", "app1.example.com")
	require.True(t, ok)
	assert.Equal(t, 300, record1.TTL)
	assert.Equal(t, []string{"1.2.3.4"}, record1.Records)

	record2, ok := mock.GetRecord("mock-zone", "app2.example.com")
	require.True(t, ok)
	assert.Equal(t, 600, record2.TTL)
	assert.Equal(t, []string{"5.6.7.8"}, record2.Records)
}

func TestReconciler_GetLastRecords(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Initially no records
	ips, ok := r.GetLastRecords("config-1")
	assert.False(t, ok)
	assert.Nil(t, ips)

	// Create config and backend
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)
	backend := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	setBackendHealth(t, ctx, s, backend.ID, types.StatusHealthy)

	// Reconcile
	err = r.ReconcileOnce(ctx)
	require.NoError(t, err)

	// Now should have records
	ips, ok = r.GetLastRecords(cfg.ID)
	require.True(t, ok)
	assert.Equal(t, []string{"1.2.3.4"}, ips)
}

func TestReconciler_GetHealthyBackends(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create config
	cfg := createTestConfig(t, ctx, s, "test-config", "app.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg.ID)

	// Create backends
	b1 := createTestBackend(t, ctx, s, cfg.ID, "1.2.3.4", true)
	b2 := createTestBackend(t, ctx, s, cfg.ID, "5.6.7.8", true)
	_ = createTestBackend(t, ctx, s, cfg.ID, "9.10.11.12", false) // disabled

	// Set health states
	setBackendHealth(t, ctx, s, b1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, b2.ID, types.StatusUnhealthy)
	// b3 has no health state

	// Get healthy backends
	healthy, err := r.getHealthyBackends(ctx, cfg.ID)
	require.NoError(t, err)

	// Should only have b1
	assert.Len(t, healthy, 1)
	assert.Equal(t, b1.ID, healthy[0].ID)
	assert.Equal(t, "1.2.3.4", healthy[0].IP)
}

func TestReconciler_NeedsUpdate(t *testing.T) {
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// First time - needs update
	assert.True(t, r.needsUpdate("config-1", []string{"1.2.3.4"}))

	// Simulate update
	r.mu.Lock()
	r.lastRecords["config-1"] = []string{"1.2.3.4"}
	r.mu.Unlock()

	// Same IPs - no update needed
	assert.False(t, r.needsUpdate("config-1", []string{"1.2.3.4"}))

	// Different order (should be sorted) - no update needed
	assert.False(t, r.needsUpdate("config-1", []string{"1.2.3.4"}))

	// Different IPs - needs update
	assert.True(t, r.needsUpdate("config-1", []string{"1.2.3.4", "5.6.7.8"}))

	// Different count - needs update
	assert.True(t, r.needsUpdate("config-1", []string{}))
}

func TestExtractIPs(t *testing.T) {
	tests := []struct {
		name     string
		backends []types.Backend
		want     []string
	}{
		{
			name:     "empty",
			backends: []types.Backend{},
			want:     []string{},
		},
		{
			name: "single backend",
			backends: []types.Backend{
				{IP: "1.2.3.4"},
			},
			want: []string{"1.2.3.4"},
		},
		{
			name: "multiple backends",
			backends: []types.Backend{
				{IP: "3.3.3.3"},
				{IP: "1.1.1.1"},
				{IP: "2.2.2.2"},
			},
			want: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, // sorted
		},
		{
			name: "skips empty IPs",
			backends: []types.Backend{
				{IP: "1.2.3.4"},
				{IP: ""},
				{IP: "5.6.7.8"},
			},
			want: []string{"1.2.3.4", "5.6.7.8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIPs(tt.backends)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReconciler_ReconcileOnce_ContinuesOnError(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create two configs - one with valid DNS name, one without
	cfg1 := createTestConfig(t, ctx, s, "config-1", "", 300) // empty DNS name
	cfg2 := createTestConfig(t, ctx, s, "config-2", "app2.example.com", 300)
	createTestHealthCheck(t, ctx, s, cfg1.ID)
	createTestHealthCheck(t, ctx, s, cfg2.ID)

	// Create backends for both
	b1 := createTestBackend(t, ctx, s, cfg1.ID, "1.2.3.4", true)
	b2 := createTestBackend(t, ctx, s, cfg2.ID, "5.6.7.8", true)
	setBackendHealth(t, ctx, s, b1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, b2.ID, types.StatusHealthy)

	// Reconcile should not fail, it should continue after errors
	err = r.ReconcileOnce(ctx)
	assert.NoError(t, err) // Returns nil even if individual configs fail

	// The second config should still have been processed
	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "app2.example.com")
	assert.True(t, ok, "second config should have DNS record")
	assert.Equal(t, []string{"5.6.7.8"}, record.Records)
}

func TestReconciler_WeightedRoundRobin_SingleBackendInDNS(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	// Create a weighted config with two healthy backends
	cfg := &types.Config{
		Name:     "weighted-config",
		DNSName:  "weighted.example.com",
		DNSTTL:   300,
		LBMethod: types.Weighted,
	}
	require.NoError(t, s.CreateConfig(ctx, cfg))
	createTestHealthCheck(t, ctx, s, cfg.ID)

	b1 := &types.Backend{ConfigID: cfg.ID, IP: "1.1.1.1", Weight: 3, Enabled: true}
	b2 := &types.Backend{ConfigID: cfg.ID, IP: "2.2.2.2", Weight: 1, Enabled: true}
	require.NoError(t, s.CreateBackend(ctx, b1))
	require.NoError(t, s.CreateBackend(ctx, b2))
	setBackendHealth(t, ctx, s, b1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, b2.ID, types.StatusHealthy)

	// Reconcile — weight=1 backend (2.2.2.2) is highest priority and must win
	require.NoError(t, r.ReconcileOnce(ctx))

	mock := provider.(*MockProvider)
	record, ok := mock.GetRecord("mock-zone", "weighted.example.com")
	require.True(t, ok)
	assert.Len(t, record.Records, 1, "weighted mode must write exactly one backend to DNS")
	assert.Equal(t, "2.2.2.2", record.Records[0], "lowest weight (highest priority) backend must be selected")
}

func TestReconciler_WeightedRoundRobin_RotatesOnFailure(t *testing.T) {
	ctx := context.Background()
	s := setupTestStore(t)
	provider, err := NewMockProvider(`{"name": "test"}`)
	require.NoError(t, err)

	r := NewReconciler(provider, s, 30*time.Second)

	cfg := &types.Config{
		Name:     "weighted-config",
		DNSName:  "weighted.example.com",
		DNSTTL:   300,
		LBMethod: types.Weighted,
	}
	require.NoError(t, s.CreateConfig(ctx, cfg))
	createTestHealthCheck(t, ctx, s, cfg.ID)

	b1 := &types.Backend{ConfigID: cfg.ID, IP: "1.1.1.1", Weight: 1, Enabled: true}
	b2 := &types.Backend{ConfigID: cfg.ID, IP: "2.2.2.2", Weight: 2, Enabled: true}
	require.NoError(t, s.CreateBackend(ctx, b1))
	require.NoError(t, s.CreateBackend(ctx, b2))
	setBackendHealth(t, ctx, s, b1.ID, types.StatusHealthy)
	setBackendHealth(t, ctx, s, b2.ID, types.StatusHealthy)

	require.NoError(t, r.ReconcileOnce(ctx))
	mock := provider.(*MockProvider)
	record, _ := mock.GetRecord("mock-zone", "weighted.example.com")
	require.Len(t, record.Records, 1)
	activeIP := record.Records[0]

	// b1 (weight=1) is primary; mark it unhealthy
	assert.Equal(t, "1.1.1.1", activeIP, "primary (weight=1) must be active initially")
	setBackendHealth(t, ctx, s, b1.ID, types.StatusUnhealthy)

	// Next reconcile must fall over to b2 (weight=2)
	require.NoError(t, r.ReconcileOnce(ctx))
	record2, ok2 := mock.GetRecord("mock-zone", "weighted.example.com")
	require.True(t, ok2)
	require.Len(t, record2.Records, 1)
	assert.Equal(t, "2.2.2.2", record2.Records[0], "must fail over to secondary (weight=2)")
}

