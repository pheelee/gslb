package lb

import (
	"sync"
	"testing"

	"github.com/pheelee/gslb/internal/types"
	"golang.org/x/sync/errgroup"
)

func TestRoundRobinSelector_Select(t *testing.T) {
	selector := &RoundRobinSelector{}

	tests := []struct {
		name     string
		backends []types.Backend
		wantNil  bool
	}{
		{
			name:     "empty backends",
			backends: []types.Backend{},
			wantNil:  true,
		},
		{
			name: "single backend",
			backends: []types.Backend{
				{ID: "b1", Weight: 1},
			},
			wantNil: false,
		},
		{
			name: "multiple backends",
			backends: []types.Backend{
				{ID: "b1", Weight: 1},
				{ID: "b2", Weight: 1},
				{ID: "b3", Weight: 1},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selector.Select(tt.backends)
			if tt.wantNil && got != nil {
				t.Errorf("Select() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Select() = nil, want non-nil")
			}
		})
	}
}

func TestRoundRobinSelector_Distribution(t *testing.T) {
	selector := &RoundRobinSelector{}

	backends := []types.Backend{
		{ID: "b1"},
		{ID: "b2"},
		{ID: "b3"},
	}

	// Track selection counts
	counts := make(map[string]int)
	for i := 0; i < 1000; i++ {
		b := selector.Select(backends)
		counts[b.ID]++
	}

	// Each backend should be selected roughly 333 times (1000/3)
	// Allow 15% deviation for statistical variance
	for _, b := range backends {
		count := counts[b.ID]
		expected := 1000 / len(backends)
		minExpected := int(float64(expected) * 0.85)
		maxExpected := int(float64(expected) * 1.15)

		if count < minExpected || count > maxExpected {
			t.Errorf("Backend %s selected %d times, expected between %d and %d",
				b.ID, count, minExpected, maxExpected)
		}
	}
}

func TestRoundRobinSelector_ThreadSafety(t *testing.T) {
	selector := &RoundRobinSelector{}

	backends := []types.Backend{
		{ID: "b1"},
		{ID: "b2"},
		{ID: "b3"},
	}

	var g errgroup.Group
	for i := 0; i < 100; i++ {
		g.Go(func() error {
			for j := 0; j < 100; j++ {
				b := selector.Select(backends)
				if b == nil {
					t.Errorf("Select() returned nil unexpectedly")
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("Concurrent access failed: %v", err)
	}
}

func TestWeightedSelector_Select(t *testing.T) {
	selector := &WeightedSelector{}

	tests := []struct {
		name     string
		backends []types.Backend
		wantNil  bool
	}{
		{
			name:     "empty backends",
			backends: []types.Backend{},
			wantNil:  true,
		},
		{
			name: "all zero weights - fallback to round robin",
			backends: []types.Backend{
				{ID: "b1", Weight: 0},
				{ID: "b2", Weight: 0},
			},
			wantNil: false,
		},
		{
			name: "single backend with weight",
			backends: []types.Backend{
				{ID: "b1", Weight: 10},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selector.Select(tt.backends)
			if tt.wantNil && got != nil {
				t.Errorf("Select() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Select() = nil, want non-nil")
			}
		})
	}
}

func TestWeightedSelector_Distribution(t *testing.T) {
	selector := &WeightedSelector{}

	backends := []types.Backend{
		{ID: "b1", Weight: 1},
		{ID: "b2", Weight: 2},
		{ID: "b3", Weight: 3},
	}

	// Track selection counts
	counts := make(map[string]int)
	for i := 0; i < 1200; i++ {
		b := selector.Select(backends)
		counts[b.ID]++
	}

	// b1=1/6, b2=2/6, b3=3/6 of selections
	// With 1200 selections: b1~=200, b2~=400, b3~=600
	// Allow 15% deviation

	tests := []struct {
		id       string
		expected int
		min      int
		max      int
	}{
		{"b1", 200, 170, 230},
		{"b2", 400, 340, 460},
		{"b3", 600, 510, 690},
	}

	for _, tt := range tests {
		count := counts[tt.id]
		if count < tt.min || count > tt.max {
			t.Errorf("Backend %s selected %d times, expected between %d and %d",
				tt.id, count, tt.min, tt.max)
		}
	}
}

func TestWeightedSelector_ThreadSafety(t *testing.T) {
	selector := &WeightedSelector{}

	backends := []types.Backend{
		{ID: "b1", Weight: 1},
		{ID: "b2", Weight: 2},
		{ID: "b3", Weight: 3},
	}

	var g errgroup.Group
	for i := 0; i < 100; i++ {
		g.Go(func() error {
			for j := 0; j < 100; j++ {
				b := selector.Select(backends)
				if b == nil {
					t.Errorf("Select() returned nil unexpectedly")
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Errorf("Concurrent access failed: %v", err)
	}
}

func TestWeightedSelector_LargeWeightRatio(t *testing.T) {
	selector := &WeightedSelector{}

	backends := []types.Backend{
		{ID: "b1", Weight: 1},
		{ID: "b2", Weight: 100},
	}

	counts := make(map[string]int)
	for i := 0; i < 1010; i++ {
		b := selector.Select(backends)
		counts[b.ID]++
	}

	// b1 should be selected ~10 times, b2 ~1000 times
	b1Count := counts["b1"]
	b2Count := counts["b2"]

	// b1 should be roughly 1% of selections (allow 0.5% to 2%)
	if b1Count < 5 || b1Count > 25 {
		t.Errorf("Backend b1 selected %d times, expected between 5 and 25", b1Count)
	}

	// b2 should get the vast majority
	if b2Count < 985 {
		t.Errorf("Backend b2 selected %d times, expected at least 985", b2Count)
	}
}

func TestGetHealthyBackends(t *testing.T) {
	backends := []types.Backend{
		{ID: "b1", Enabled: true},
		{ID: "b2", Enabled: false},
		{ID: "b3", Enabled: true},
		{ID: "b4", Enabled: true},
	}

	states := map[string]types.HealthStatus{
		"b1": types.StatusHealthy,
		"b3": types.StatusUnhealthy,
		// b4 is not in states (unknown)
	}

	healthy := GetHealthyBackends(backends, states)

	if len(healthy) != 2 {
		t.Errorf("Expected 2 healthy backends, got %d", len(healthy))
	}

	// Check that b1 is healthy
	found := false
	for _, b := range healthy {
		if b.ID == "b1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected b1 to be in healthy list")
	}

	// Check that b3 (unhealthy) is not in the list
	for _, b := range healthy {
		if b.ID == "b3" {
			t.Error("b3 is unhealthy but was included")
		}
	}
}

func TestGetHealthyBackends_AllUnhealthy(t *testing.T) {
	backends := []types.Backend{
		{ID: "b1", Enabled: true},
		{ID: "b2", Enabled: true},
	}

	states := map[string]types.HealthStatus{
		"b1": types.StatusUnhealthy,
		"b2": types.StatusUnhealthy,
	}

	healthy := GetHealthyBackends(backends, states)

	if len(healthy) != 0 {
		t.Errorf("Expected 0 healthy backends, got %d", len(healthy))
	}
}

func TestGetHealthyBackends_EmptyBackends(t *testing.T) {
	backends := []types.Backend{}
	states := map[string]types.HealthStatus{}

	healthy := GetHealthyBackends(backends, states)

	if len(healthy) != 0 {
		t.Errorf("Expected 0 healthy backends, got %d", len(healthy))
	}
}

func TestGetHealthyBackends_UnknownStatus(t *testing.T) {
	backends := []types.Backend{
		{ID: "b1", Enabled: true},
	}

	states := map[string]types.HealthStatus{}

	healthy := GetHealthyBackends(backends, states)

	if len(healthy) != 1 {
		t.Errorf("Expected 1 healthy backend (unknown status defaults to healthy), got %d", len(healthy))
	}
}

func TestConcurrentSelection(t *testing.T) {
	rrSelector := &RoundRobinSelector{}
	wSelector := &WeightedSelector{}

	backends := []types.Backend{
		{ID: "b1", Weight: 1},
		{ID: "b2", Weight: 2},
		{ID: "b3", Weight: 3},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 200)

	// Run 100 concurrent round-robin selectors
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if b := rrSelector.Select(backends); b == nil {
					errCh <- nil
				}
				if b := wSelector.Select(backends); b == nil {
					errCh <- nil
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("Concurrent selection error: %v", err)
		}
	}
}
