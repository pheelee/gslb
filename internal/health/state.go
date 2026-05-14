package health

import (
	"sync"
	"time"

	"github.com/pheelee/gslb/internal/types"
)

// StateMachine tracks health transitions for a backend using configurable thresholds.
// It implements a hysteresis pattern to avoid flapping between states.
type StateMachine struct {
	mu                   sync.RWMutex
	config               types.HealthCheck
	currentStatus        types.HealthStatus
	consecutiveSuccesses int
	consecutiveFailures  int
	lastCheck            time.Time
}

// NewStateMachine creates a new StateMachine with the given configuration.
func NewStateMachine(config types.HealthCheck) *StateMachine {
	return &StateMachine{
		config:        config,
		currentStatus: types.StatusUnknown,
	}
}

// ProcessResult processes a health check result and returns the new status.
// It implements the following state transition logic:
//
//   - Unknown + threshold_healthy consecutive successes → Healthy
//   - Unknown + threshold_unhealthy consecutive failures → Unhealthy
//   - Healthy + threshold_unhealthy consecutive failures → Unhealthy
//   - Unhealthy + threshold_healthy consecutive successes → Healthy
//
// This hysteresis pattern prevents flapping when a backend is intermittently
// reachable.
func (sm *StateMachine) ProcessResult(healthy bool) types.HealthStatus {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.lastCheck = time.Now()

	if healthy {
		sm.consecutiveSuccesses++
		sm.consecutiveFailures = 0

		// Check if we should transition to healthy
		threshold := sm.config.ThresholdHealthy
		if threshold <= 0 {
			threshold = 1 // Default to 1
		}

		if sm.consecutiveSuccesses >= threshold {
			if sm.currentStatus != types.StatusHealthy {
				sm.currentStatus = types.StatusHealthy
			}
		}
	} else {
		sm.consecutiveFailures++
		sm.consecutiveSuccesses = 0

		// Check if we should transition to unhealthy
		threshold := sm.config.ThresholdUnhealthy
		if threshold <= 0 {
			threshold = 1 // Default to 1
		}

		if sm.consecutiveFailures >= threshold {
			if sm.currentStatus != types.StatusUnhealthy {
				sm.currentStatus = types.StatusUnhealthy
			}
		}
	}

	return sm.currentStatus
}

// Status returns the current health status.
func (sm *StateMachine) Status() types.HealthStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentStatus
}

// ConsecutiveSuccesses returns the number of consecutive successful checks.
func (sm *StateMachine) ConsecutiveSuccesses() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.consecutiveSuccesses
}

// ConsecutiveFailures returns the number of consecutive failed checks.
func (sm *StateMachine) ConsecutiveFailures() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.consecutiveFailures
}

// LastCheck returns the time of the last check.
func (sm *StateMachine) LastCheck() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastCheck
}

// Reset resets the state machine to unknown state.
func (sm *StateMachine) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentStatus = types.StatusUnknown
	sm.consecutiveSuccesses = 0
	sm.consecutiveFailures = 0
	sm.lastCheck = time.Time{}
}

// StateSnapshot returns a snapshot of the current state.
func (sm *StateMachine) StateSnapshot() StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return StateSnapshot{
		Status:               sm.currentStatus,
		ConsecutiveSuccesses: sm.consecutiveSuccesses,
		ConsecutiveFailures:  sm.consecutiveFailures,
		LastCheck:            sm.lastCheck,
	}
}

// StateSnapshot represents a point-in-time snapshot of the state machine.
type StateSnapshot struct {
	Status               types.HealthStatus
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
	LastCheck            time.Time
}
