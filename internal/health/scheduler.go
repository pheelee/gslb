// Package health provides health check scheduling and execution.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// Scheduler manages periodic health checks for all backends.
type Scheduler struct {
	store      store.Store
	engine     *Engine
	interval   time.Duration
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	aggregator *Aggregator // optional; may be nil
}

// NewScheduler creates a new health check scheduler.
func NewScheduler(s store.Store, engine *Engine, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Scheduler{
		store:    s,
		engine:   engine,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// SetAggregator attaches a history aggregator so probe results are recorded.
func (s *Scheduler) SetAggregator(a *Aggregator) {
	s.aggregator = a
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	s.running = true
	s.wg.Add(1)

	go s.run(ctx)
	slog.Info("health check scheduler started", "interval", s.interval)
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
	slog.Info("health check scheduler stopped")
}

// IsRunning returns true if the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// run is the main scheduler loop.
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	// Run initial check
	s.scheduleChecks(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler context cancelled")
			return
		case <-s.stopCh:
			slog.Info("scheduler stop signal received")
			return
		case <-ticker.C:
			s.scheduleChecks(ctx)
		}
	}
}

// scheduleChecks loads all backends and schedules health checks.
func (s *Scheduler) scheduleChecks(ctx context.Context) {
	// Get all configs
	configs, err := s.store.ListConfigs(ctx)
	if err != nil {
		slog.Error("failed to list configs for health checks", "error", err)
		return
	}

	for _, config := range configs {
		// Get health check config for this config
		hc, err := s.store.GetHealthCheck(ctx, config.ID)
		if err != nil {
			// No health check configured, skip
			continue
		}

		// Get backends for this config
		backends, err := s.store.ListBackends(ctx, config.ID)
		if err != nil {
			slog.Error("failed to list backends", "config_id", config.ID, "error", err)
			continue
		}

		for _, backend := range backends {
			if !backend.Enabled {
				continue
			}

			req := CheckRequest{
				BackendID: backend.ID,
				Type:      hc.Type,
				Target:    formatTarget(backend.IP, backend.Port),
				Timeout:   time.Duration(hc.TimeoutSeconds) * time.Second,
			}

			if err := s.engine.Submit(req); err != nil {
				slog.Error("failed to submit health check", "backend_id", backend.ID, "error", err)
			}
		}
	}
}

// formatTarget creates a target string from IP and optional port.
func formatTarget(ip string, port *int) string {
	if port != nil && *port > 0 {
		return fmt.Sprintf("%s:%d", ip, *port)
	}
	return ip
}

// StartResultProcessor starts a goroutine to process health check results.
func (s *Scheduler) StartResultProcessor(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.processResults(ctx)
	}()
}

// processResults processes health check results and updates states.
func (s *Scheduler) processResults(ctx context.Context) {
	results := s.engine.Results()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case result := <-results:
			s.handleResult(ctx, result)
		}
	}
}

// handleResult processes a single health check result.
func (s *Scheduler) handleResult(ctx context.Context, result CheckResult) {
	// Record probe in history aggregator (if attached).
	if s.aggregator != nil {
		s.aggregator.RecordProbe(result.BackendID, result.Healthy && result.Error == nil)
	}

	// Get current health state
	state, err := s.store.GetHealthState(ctx, result.BackendID)
	if err != nil {
		// No existing state, create new one
		state = &types.HealthState{
			BackendID: result.BackendID,
			Status:    types.StatusUnknown,
		}
	}

	// Update state based on result
	now := time.Now()
	state.LastCheckAt = &now

	if result.Error != nil {
		state.ConsecutiveFailures++
		state.ConsecutiveSuccesses = 0
		state.LastError = result.Error.Error()
	} else if result.Healthy {
		state.ConsecutiveSuccesses++
		state.ConsecutiveFailures = 0
		state.LastError = ""
		state.LastHealthyAt = &now
	} else {
		state.ConsecutiveFailures++
		state.ConsecutiveSuccesses = 0
	}

	// Look up configured thresholds from the health check for this backend's config.
	thresholdHealthy, thresholdUnhealthy := 2, 2
	if backend, err := s.store.GetBackend(ctx, result.BackendID); err == nil {
		if hc, err := s.store.GetHealthCheck(ctx, backend.ConfigID); err == nil {
			if hc.ThresholdHealthy > 0 {
				thresholdHealthy = hc.ThresholdHealthy
			}
			if hc.ThresholdUnhealthy > 0 {
				thresholdUnhealthy = hc.ThresholdUnhealthy
			}
		}
	}

	state.Status = determineHealthStatus(state, thresholdHealthy, thresholdUnhealthy)

	if err := s.store.UpdateHealthState(ctx, state); err != nil {
		slog.Error("failed to update health state", "backend_id", result.BackendID, "error", err)
	}
}

// determineHealthStatus determines the health status based on consecutive results
// and the configured thresholds.
func determineHealthStatus(state *types.HealthState, thresholdHealthy, thresholdUnhealthy int) types.HealthStatus {
	if state.ConsecutiveSuccesses >= thresholdHealthy {
		return types.StatusHealthy
	}
	if state.ConsecutiveFailures >= thresholdUnhealthy {
		return types.StatusUnhealthy
	}
	return types.StatusUnknown
}
