package lb

import (
	"sync/atomic"

	"github.com/pheelee/gslb/internal/types"
)

// Selector chooses backends based on load balancing method
type Selector interface {
	Select(backends []types.Backend) *types.Backend
}

// RoundRobinSelector implements circular selection
type RoundRobinSelector struct {
	counter uint64
}

func (s *RoundRobinSelector) Select(backends []types.Backend) *types.Backend {
	if len(backends) == 0 {
		return nil
	}
	count := atomic.AddUint64(&s.counter, 1)
	return &backends[count%uint64(len(backends))]
}

// WeightedSelector implements weighted round robin
type WeightedSelector struct {
	counter uint64
}

func (s *WeightedSelector) Select(backends []types.Backend) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, b := range backends {
		if b.Weight > 0 {
			totalWeight += b.Weight
		}
	}

	// Fallback to round robin if all weights are 0
	if totalWeight == 0 {
		count := atomic.AddUint64(&s.counter, 1)
		return &backends[count%uint64(len(backends))]
	}

	// Use atomic counter % totalWeight to select
	count := atomic.AddUint64(&s.counter, 1)
	offset := count % uint64(totalWeight)

	// Find the backend at the offset
	current := uint64(0)
	for i := range backends {
		if backends[i].Weight <= 0 {
			continue
		}
		if offset < current+uint64(backends[i].Weight) {
			return &backends[i]
		}
		current += uint64(backends[i].Weight)
	}

	// Fallback (should not reach here)
	return &backends[0]
}

// GetHealthyBackends filters backends by health status
func GetHealthyBackends(backends []types.Backend, states map[string]types.HealthStatus) []types.Backend {
	var healthy []types.Backend
	for _, b := range backends {
		if !b.Enabled {
			continue
		}
		status, ok := states[b.ID]
		if !ok {
			// Unknown status - include it as healthy by default
			healthy = append(healthy, b)
			continue
		}
		if status == types.StatusHealthy {
			healthy = append(healthy, b)
		}
	}
	return healthy
}
