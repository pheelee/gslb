// Package dns provides DNS provider abstractions and implementations
package dns

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// Reconciler watches health states and updates DNS records
type Reconciler struct {
	provider          DNSProvider
	store             store.Store
	interval          time.Duration
	mu                sync.RWMutex
	lastRecords       map[string][]string // config_id -> IPs (to detect changes)
	running           bool
	stopCh            chan struct{}
	OnBackendSelected func(backendID string) // optional; called for each backend selected for DNS
}

// NewReconciler creates a new DNS reconciler
func NewReconciler(provider DNSProvider, store store.Store, interval time.Duration) *Reconciler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Reconciler{
		provider:    provider,
		store:       store,
		interval:    interval,
		lastRecords: make(map[string][]string),
		stopCh:      make(chan struct{}),
	}
}

// Start begins the reconciliation loop
func (r *Reconciler) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("reconciler already running")
	}
	r.running = true
	r.stopCh = make(chan struct{})
	r.mu.Unlock()

	// Run initial reconciliation outside the lock — ReconcileOnce acquires its own locks.
	if err := r.ReconcileOnce(ctx); err != nil {
		slog.Error("initial reconciliation failed", "error", err)
	}

	// Start background loop
	go r.run(ctx)

	slog.Info("reconciler started", "interval", r.interval)
	return nil
}

// Stop stops the reconciliation loop
func (r *Reconciler) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false
	close(r.stopCh)

	slog.Info("reconciler stopped")
	return nil
}

// run is the background reconciliation loop
func (r *Reconciler) run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("reconciler context cancelled, stopping")
			return
		case <-r.stopCh:
			slog.Info("reconciler stop signal received")
			return
		case <-ticker.C:
			if err := r.ReconcileOnce(ctx); err != nil {
				slog.Error("reconciler reconciliation failed", "error", err)
			}
		}
	}
}

// ReconcileOnce performs a single reconciliation pass
func (r *Reconciler) ReconcileOnce(ctx context.Context) error {
	// Fetch all configs
	configs, err := r.store.ListConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list configs: %w", err)
	}

	slog.Info("reconciler reconciling configs", "count", len(configs))

	for _, cfg := range configs {
		if err := r.reconcileConfig(ctx, &cfg); err != nil {
			slog.Error("reconciler failed to reconcile config", "id", cfg.ID, "name", cfg.Name, "error", err)
			if setErr := r.store.SetConfigReconcileError(ctx, cfg.ID, err.Error()); setErr != nil {
				slog.Warn("failed to persist reconcile error", "config_id", cfg.ID, "error", setErr)
			}
		} else {
			if setErr := r.store.SetConfigReconcileError(ctx, cfg.ID, ""); setErr != nil {
				slog.Warn("failed to clear reconcile error", "config_id", cfg.ID, "error", setErr)
			}
		}
	}

	return nil
}

// reconcileConfig reconciles a single config
func (r *Reconciler) reconcileConfig(ctx context.Context, cfg *types.Config) error {
	// Get healthy backends for this config
	healthy, err := r.getHealthyBackends(ctx, cfg.ID)
	if err != nil {
		return fmt.Errorf("failed to get healthy backends: %w", err)
	}

	// Determine which backends to publish to DNS based on the LB method.
	var backendsForDNS []types.Backend
	switch cfg.LBMethod {
	case types.Weighted:
		// Publish only one backend at a time, selected by weighted algorithm.
		// Stick with the current active backend as long as it remains healthy;
		// only rotate when it is no longer in the healthy set.
		if b := r.selectWeighted(cfg.ID, healthy); b != nil {
			backendsForDNS = []types.Backend{*b}
		}
	default: // round_robin and anything else: publish all healthy backends
		backendsForDNS = healthy
	}

	// Notify selection tracker for each selected backend (regardless of DNS change).
	if r.OnBackendSelected != nil {
		for _, b := range backendsForDNS {
			r.OnBackendSelected(b.ID)
		}
	}

	// Extract IPs
	ips := extractIPs(backendsForDNS)

	// Check if update is needed
	if !r.needsUpdate(cfg.ID, ips) {
		return nil
	}

	// Perform DNS update
	if err := r.updateDNS(ctx, cfg, backendsForDNS); err != nil {
		return fmt.Errorf("failed to update DNS: %w", err)
	}

	return nil
}

// selectWeighted picks the highest-priority healthy backend for a weighted config.
// Priority is determined by weight: the backend with the lowest weight value wins
// (weight=1 is primary, weight=2 is secondary, etc.). On a tie the first in the
// slice is used. Called on every reconcile cycle; needsUpdate suppresses a DNS
// write when the same backend is already active.
func (r *Reconciler) selectWeighted(_ string, healthy []types.Backend) *types.Backend {
	if len(healthy) == 0 {
		return nil
	}
	best := &healthy[0]
	for i := 1; i < len(healthy); i++ {
		bw := healthy[i].Weight
		bestw := best.Weight
		if bw <= 0 {
			bw = 1
		}
		if bestw <= 0 {
			bestw = 1
		}
		if bw < bestw {
			best = &healthy[i]
		}
	}
	return best
}

// getHealthyBackends returns healthy backends for a config
func (r *Reconciler) getHealthyBackends(ctx context.Context, configID string) ([]types.Backend, error) {
	// Get all backends for this config
	backends, err := r.store.ListBackends(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("failed to list backends: %w", err)
	}

	// Get health states for backends in this config
	states, err := r.store.GetHealthStates(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("failed to get health states: %w", err)
	}

	// Filter to healthy, enabled backends
	var healthy []types.Backend
	for _, backend := range backends {
		// Skip disabled backends
		if !backend.Enabled {
			continue
		}

		// Check health state
		state, ok := states[backend.ID]
		if !ok {
			// No health state yet, skip (unknown health)
			continue
		}

		if state.Status == types.StatusHealthy {
			healthy = append(healthy, backend)
		}
	}

	return healthy, nil
}

// extractIPs extracts IP addresses from backends
func extractIPs(backends []types.Backend) []string {
	ips := make([]string, 0, len(backends))
	for _, b := range backends {
		if b.IP != "" {
			ips = append(ips, b.IP)
		}
	}
	// Sort for consistent comparison
	sort.Strings(ips)
	return ips
}

// needsUpdate checks if DNS update is needed
func (r *Reconciler) needsUpdate(configID string, newIPs []string) bool {
	r.mu.RLock()
	lastIPs, exists := r.lastRecords[configID]
	r.mu.RUnlock()

	if !exists {
		// First time seeing this config
		return true
	}

	if len(lastIPs) != len(newIPs) {
		return true
	}

	// Compare sorted slices
	for i := range lastIPs {
		if lastIPs[i] != newIPs[i] {
			return true
		}
	}

	return false
}

// resolveProvider returns the DNS provider for a config.
// It first tries to load a per-config provider from the store.
// If none is configured, it falls back to r.provider (may be nil).
func (r *Reconciler) resolveProvider(ctx context.Context, configID string) DNSProvider {
	dbCfg, err := r.store.GetDNSProvider(ctx, configID)
	if err == nil {
		p, err := CreateProvider(dbCfg.ProviderType, dbCfg.ConfigJSON)
		if err == nil {
			return p
		}
		slog.Warn("reconciler failed to instantiate per-config provider, using fallback",
			"config_id", configID, "error", err)
	}
	return r.provider // may be nil
}

// updateDNS performs the DNS update
func (r *Reconciler) updateDNS(ctx context.Context, cfg *types.Config, backends []types.Backend) error {
	provider := r.resolveProvider(ctx, cfg.ID)
	if provider == nil {
		slog.Debug("reconciler skipping config: no DNS provider configured", "config_id", cfg.ID, "dns_name", cfg.DNSName)
		return nil
	}

	ips := extractIPs(backends)

	slog.Info("reconciler updating DNS", "dns_name", cfg.DNSName, "id", cfg.ID, "ip_count", len(ips), "ips", ips)

	if len(ips) == 0 {
		// No healthy backends — delete the DNS record so clients get an honest NXDOMAIN
		// rather than sending traffic to unhealthy endpoints.
		delReq := DeleteRequest{Name: cfg.DNSName}
		if err := provider.DeleteRecord(ctx, delReq); err != nil {
			// "not found" means there is nothing to delete — that is fine.
			slog.Debug("reconciler delete DNS (no-op or already absent)", "dns_name", cfg.DNSName, "error", err)
		}
	} else {
		req := UpsertRequest{
			ZoneID:  "", // Let provider use its configured zone ID
			Name:    cfg.DNSName,
			TTL:     cfg.DNSTTL,
			Records: ips,
		}
		if err := provider.UpsertRecord(ctx, req); err != nil {
			return fmt.Errorf("failed to upsert DNS record: %w", err)
		}
	}

	// Update last known state
	r.mu.Lock()
	r.lastRecords[cfg.ID] = ips
	r.mu.Unlock()

	slog.Info("reconciler successfully updated DNS", "dns_name", cfg.DNSName, "id", cfg.ID)
	return nil
}

// ReconcileConfig triggers an immediate reconciliation for a single config by ID.
// Safe to call from any goroutine.
func (r *Reconciler) ReconcileConfig(ctx context.Context, configID string) error {
	cfg, err := r.store.GetConfig(ctx, configID)
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	if err := r.reconcileConfig(ctx, cfg); err != nil {
		if setErr := r.store.SetConfigReconcileError(ctx, configID, err.Error()); setErr != nil {
			slog.Warn("failed to persist reconcile error", "config_id", configID, "error", setErr)
		}
		return err
	}
	if setErr := r.store.SetConfigReconcileError(ctx, configID, ""); setErr != nil {
		slog.Warn("failed to clear reconcile error", "config_id", configID, "error", setErr)
	}
	return nil
}

// GetLastRecords returns the last known DNS records for a config (for testing)
func (r *Reconciler) GetLastRecords(configID string) ([]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ips, ok := r.lastRecords[configID]
	return ips, ok
}
