package health

import (
	"sync"
	"testing"
	"time"

	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
)

// TestStateMachine tests the state machine's core functionality.
func TestStateMachine(t *testing.T) {
	t.Run("initial_state_is_unknown", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 3,
		}
		sm := NewStateMachine(config)

		assert.Equal(t, types.StatusUnknown, sm.Status())
		assert.Equal(t, 0, sm.ConsecutiveSuccesses())
		assert.Equal(t, 0, sm.ConsecutiveFailures())
	})

	t.Run("transition_to_healthy", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 3,
		}
		sm := NewStateMachine(config)

		// First success - still unknown
		status := sm.ProcessResult(true)
		assert.Equal(t, types.StatusUnknown, status)
		assert.Equal(t, 1, sm.ConsecutiveSuccesses())

		// Second success - transition to healthy
		status = sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, status)
		assert.Equal(t, 2, sm.ConsecutiveSuccesses())
	})

	t.Run("transition_to_unhealthy", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 2,
		}
		sm := NewStateMachine(config)

		// First failure - still unknown
		status := sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnknown, status)
		assert.Equal(t, 1, sm.ConsecutiveFailures())

		// Second failure - transition to unhealthy
		status = sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnhealthy, status)
		assert.Equal(t, 2, sm.ConsecutiveFailures())
	})

	t.Run("healthy_to_unhealthy_transition", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   1,
			ThresholdUnhealthy: 2,
		}
		sm := NewStateMachine(config)

		// Become healthy
		sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, sm.Status())

		// First failure - still healthy
		status := sm.ProcessResult(false)
		assert.Equal(t, types.StatusHealthy, status)
		assert.Equal(t, 0, sm.ConsecutiveSuccesses())
		assert.Equal(t, 1, sm.ConsecutiveFailures())

		// Second failure - transition to unhealthy
		status = sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnhealthy, status)
		assert.Equal(t, 2, sm.ConsecutiveFailures())
	})

	t.Run("unhealthy_to_healthy_transition", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   3,
			ThresholdUnhealthy: 1,
		}
		sm := NewStateMachine(config)

		// Become unhealthy
		sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnhealthy, sm.Status())

		// First success - still unhealthy
		status := sm.ProcessResult(true)
		assert.Equal(t, types.StatusUnhealthy, status)
		assert.Equal(t, 1, sm.ConsecutiveSuccesses())
		assert.Equal(t, 0, sm.ConsecutiveFailures())

		// Second success - still unhealthy
		status = sm.ProcessResult(true)
		assert.Equal(t, types.StatusUnhealthy, status)
		assert.Equal(t, 2, sm.ConsecutiveSuccesses())

		// Third success - transition to healthy
		status = sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, status)
		assert.Equal(t, 3, sm.ConsecutiveSuccesses())
	})

	t.Run("counter_reset_on_state_change", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 2,
		}
		sm := NewStateMachine(config)

		// Build up successes
		sm.ProcessResult(true)
		sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, sm.Status())
		assert.Equal(t, 2, sm.ConsecutiveSuccesses())
		assert.Equal(t, 0, sm.ConsecutiveFailures())

		// Failure resets successes
		sm.ProcessResult(false)
		assert.Equal(t, 0, sm.ConsecutiveSuccesses())
		assert.Equal(t, 1, sm.ConsecutiveFailures())

		// Build up failures
		sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnhealthy, sm.Status())
		assert.Equal(t, 0, sm.ConsecutiveSuccesses())
		assert.Equal(t, 2, sm.ConsecutiveFailures())

		// Success resets failures
		sm.ProcessResult(true)
		assert.Equal(t, 1, sm.ConsecutiveSuccesses())
		assert.Equal(t, 0, sm.ConsecutiveFailures())
	})

	t.Run("default_thresholds", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   0, // Should default to 1
			ThresholdUnhealthy: 0, // Should default to 1
		}
		sm := NewStateMachine(config)

		// Single success should be enough with default threshold
		status := sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, status)

		// Reset
		sm.Reset()

		// Single failure should be enough with default threshold
		status = sm.ProcessResult(false)
		assert.Equal(t, types.StatusUnhealthy, status)
	})

	t.Run("reset_clears_state", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   1,
			ThresholdUnhealthy: 1,
		}
		sm := NewStateMachine(config)

		// Set some state
		sm.ProcessResult(true)
		assert.Equal(t, types.StatusHealthy, sm.Status())
		assert.NotZero(t, sm.LastCheck())

		// Reset
		sm.Reset()

		assert.Equal(t, types.StatusUnknown, sm.Status())
		assert.Equal(t, 0, sm.ConsecutiveSuccesses())
		assert.Equal(t, 0, sm.ConsecutiveFailures())
		assert.True(t, sm.LastCheck().IsZero())
	})

	t.Run("last_check_updated", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   1,
			ThresholdUnhealthy: 1,
		}
		sm := NewStateMachine(config)

		before := time.Now()
		sm.ProcessResult(true)
		after := time.Now()

		lastCheck := sm.LastCheck()
		assert.True(t, lastCheck.After(before) || lastCheck.Equal(before))
		assert.True(t, lastCheck.Before(after) || lastCheck.Equal(after))
	})

	t.Run("state_snapshot", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   2,
			ThresholdUnhealthy: 3,
		}
		sm := NewStateMachine(config)

		sm.ProcessResult(true)
		sm.ProcessResult(true)

		snapshot := sm.StateSnapshot()
		assert.Equal(t, types.StatusHealthy, snapshot.Status)
		assert.Equal(t, 2, snapshot.ConsecutiveSuccesses)
		assert.Equal(t, 0, snapshot.ConsecutiveFailures)
		assert.False(t, snapshot.LastCheck.IsZero())
	})

	t.Run("concurrent_access", func(t *testing.T) {
		config := types.HealthCheck{
			ThresholdHealthy:   5,
			ThresholdUnhealthy: 5,
		}
		sm := NewStateMachine(config)

		var wg sync.WaitGroup
		numGoroutines := 100
		numOperations := 100

		// Concurrent writes
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					sm.ProcessResult(id%2 == 0)
				}
			}(i)
		}

		// Concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					_ = sm.Status()
					_ = sm.ConsecutiveSuccesses()
					_ = sm.ConsecutiveFailures()
					_ = sm.LastCheck()
					_ = sm.StateSnapshot()
				}
			}()
		}

		wg.Wait()

		// State should be valid after all operations
		status := sm.Status()
		assert.Contains(t, []types.HealthStatus{types.StatusUnknown, types.StatusHealthy, types.StatusUnhealthy}, status)
	})
}

// TestStateTransitions comprehensively tests all state transitions.
func TestStateTransitions(t *testing.T) {
	tests := []struct {
		name           string
		thresholdGood  int
		thresholdBad   int
		sequence       []bool // true = healthy, false = unhealthy
		expectedStatus types.HealthStatus
		expectedSucc   int
		expectedFail   int
	}{
		{
			name:           "unknown_to_healthy",
			thresholdGood:  3,
			thresholdBad:   3,
			sequence:       []bool{true, true, true},
			expectedStatus: types.StatusHealthy,
			expectedSucc:   3,
			expectedFail:   0,
		},
		{
			name:           "unknown_to_unhealthy",
			thresholdGood:  3,
			thresholdBad:   2,
			sequence:       []bool{false, false},
			expectedStatus: types.StatusUnhealthy,
			expectedSucc:   0,
			expectedFail:   2,
		},
		{
			name:           "healthy_persists",
			thresholdGood:  1,
			thresholdBad:   3,
			sequence:       []bool{true, true, true},
			expectedStatus: types.StatusHealthy,
			expectedSucc:   3,
			expectedFail:   0,
		},
		{
			name:           "unhealthy_persists",
			thresholdGood:  3,
			thresholdBad:   1,
			sequence:       []bool{false, false, false},
			expectedStatus: types.StatusUnhealthy,
			expectedSucc:   0,
			expectedFail:   3,
		},
		{
			name:           "flapping_recovery",
			thresholdGood:  2,
			thresholdBad:   2,
			sequence:       []bool{true, false, true, false, true, true},
			expectedStatus: types.StatusHealthy,
			expectedSucc:   2,
			expectedFail:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.HealthCheck{
				ThresholdHealthy:   tt.thresholdGood,
				ThresholdUnhealthy: tt.thresholdBad,
			}
			sm := NewStateMachine(config)

			var finalStatus types.HealthStatus
			for _, result := range tt.sequence {
				finalStatus = sm.ProcessResult(result)
			}

			assert.Equal(t, tt.expectedStatus, finalStatus)
			assert.Equal(t, tt.expectedStatus, sm.Status())
			assert.Equal(t, tt.expectedSucc, sm.ConsecutiveSuccesses())
			assert.Equal(t, tt.expectedFail, sm.ConsecutiveFailures())
		})
	}
}

// BenchmarkStateMachine benchmarks the state machine operations.
func BenchmarkStateMachine_ProcessResult(b *testing.B) {
	config := types.HealthCheck{
		ThresholdHealthy:   3,
		ThresholdUnhealthy: 3,
	}
	sm := NewStateMachine(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.ProcessResult(i%2 == 0)
	}
}

func BenchmarkStateMachine_Concurrent(b *testing.B) {
	config := types.HealthCheck{
		ThresholdHealthy:   3,
		ThresholdUnhealthy: 3,
	}

	b.RunParallel(func(pb *testing.PB) {
		sm := NewStateMachine(config)
		i := 0
		for pb.Next() {
			sm.ProcessResult(i%2 == 0)
			i++
		}
	})
}
