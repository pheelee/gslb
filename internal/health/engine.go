package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Engine orchestrates health checks using a worker pool.
// It manages concurrent execution of health checks and aggregates results.
type Engine struct {
	mu          sync.RWMutex
	workerCount int
	requestChan chan CheckRequest
	resultChan  chan CheckResult
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	factory     *CheckerFactory
	running     bool
}

// NewEngine creates a new health check engine with the specified number of workers.
// workerCount determines how many concurrent health checks can run simultaneously.
func NewEngine(workerCount int) *Engine {
	if workerCount <= 0 {
		workerCount = 4 // Default to 4 workers
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		workerCount: workerCount,
		requestChan: make(chan CheckRequest),
		resultChan:  make(chan CheckResult, workerCount*10), // Buffered to handle bursts
		ctx:         ctx,
		cancel:      cancel,
		factory:     &CheckerFactory{},
	}
}

// Start begins the worker pool.
// This method is safe to call multiple times - subsequent calls are no-ops.
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return
	}

	e.running = true

	// Start workers
	for i := 0; i < e.workerCount; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
}

// Stop gracefully shuts down the worker pool.
// It waits for all ongoing checks to complete before returning.
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	e.cancel()
	e.mu.Unlock()

	// Wait for all workers to finish
	e.wg.Wait()
}

// Submit adds a health check request to the work queue.
// Returns an error if the engine is not running.
func (e *Engine) Submit(req CheckRequest) error {
	e.mu.RLock()
	running := e.running
	e.mu.RUnlock()

	if !running {
		return fmt.Errorf("engine is not running")
	}

	select {
	case e.requestChan <- req:
		return nil
	case <-e.ctx.Done():
		return fmt.Errorf("engine is shutting down")
	}
}

// Results returns the result channel for receiving check results.
// Callers should read from this channel to process health check results.
func (e *Engine) Results() <-chan CheckResult {
	return e.resultChan
}

// worker is the goroutine that processes health check requests.
func (e *Engine) worker(id int) {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		case req, ok := <-e.requestChan:
			if !ok {
				return
			}
			e.executeCheck(req)
		}
	}
}

// executeCheck performs a single health check and sends the result.
func (e *Engine) executeCheck(req CheckRequest) {
	start := time.Now()

	// Create checker
	checker := e.factory.NewChecker(req)

	// Create timeout context for this check
	ctx, cancel := context.WithTimeout(e.ctx, req.Timeout)
	defer cancel()

	// Perform check
	healthy, err := checker.Check(ctx)

	result := CheckResult{
		BackendID: req.BackendID,
		Healthy:   healthy,
		Error:     err,
		Duration:  time.Since(start),
	}

	// Send result (non-blocking with timeout)
	select {
	case e.resultChan <- result:
	case <-e.ctx.Done():
		// Engine is shutting down, discard result
	case <-time.After(100 * time.Millisecond):
		// Result channel full, discard result to avoid blocking
	}
}

// IsRunning returns true if the engine is currently running.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// WorkerCount returns the number of workers in the pool.
func (e *Engine) WorkerCount() int {
	return e.workerCount
}

// EngineStats provides statistics about the engine's operation.
type EngineStats struct {
	WorkerCount int
	Running     bool
}

// Stats returns current engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return EngineStats{
		WorkerCount: e.workerCount,
		Running:     e.running,
	}
}

// CheckScheduler manages scheduled health checks for multiple backends.
type CheckScheduler struct {
	mu       sync.RWMutex
	engines  []*Engine
	requests map[string]CheckRequest // backendID -> request
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewCheckScheduler creates a new scheduler with the specified engine configuration.
// numEngines: number of engine instances to distribute load
// workersPerEngine: number of workers per engine
func NewCheckScheduler(numEngines, workersPerEngine int) *CheckScheduler {
	if numEngines <= 0 {
		numEngines = 1
	}
	if workersPerEngine <= 0 {
		workersPerEngine = 4
	}

	engines := make([]*Engine, numEngines)
	for i := 0; i < numEngines; i++ {
		engines[i] = NewEngine(workersPerEngine)
	}

	return &CheckScheduler{
		engines:  engines,
		requests: make(map[string]CheckRequest),
		stopChan: make(chan struct{}),
	}
}

// Start begins all engines and the scheduler.
func (s *CheckScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, engine := range s.engines {
		engine.Start()
	}
}

// Stop gracefully shuts down all engines and the scheduler.
func (s *CheckScheduler) Stop() {
	close(s.stopChan)
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, engine := range s.engines {
		engine.Stop()
	}
}

// Register adds a backend to the scheduler for periodic health checks.
func (s *CheckScheduler) Register(backendID string, req CheckRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req.BackendID = backendID
	s.requests[backendID] = req
}

// Unregister removes a backend from the scheduler.
func (s *CheckScheduler) Unregister(backendID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.requests, backendID)
}

// Run starts the scheduler loop that periodically submits checks.
// This method blocks until Stop is called.
func (s *CheckScheduler) Run() {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.submitScheduledChecks()
		}
	}
}

// submitScheduledChecks submits all registered checks to engines.
func (s *CheckScheduler) submitScheduledChecks() {
	s.mu.RLock()
	requests := make([]CheckRequest, 0, len(s.requests))
	for _, req := range s.requests {
		requests = append(requests, req)
	}
	s.mu.RUnlock()

	if len(requests) == 0 {
		return
	}

	// Distribute checks across engines using round-robin
	for i, req := range requests {
		engineIdx := i % len(s.engines)
		_ = s.engines[engineIdx].Submit(req)
	}
}

// Results aggregates results from all engines into a single channel.
// The returned channel must be consumed to prevent blocking.
func (s *CheckScheduler) Results() <-chan CheckResult {
	// Create a merged channel
	merged := make(chan CheckResult)

	var wg sync.WaitGroup
	for _, engine := range s.engines {
		wg.Add(1)
		go func(e *Engine) {
			defer wg.Done()
			for result := range e.Results() {
				select {
				case merged <- result:
				case <-s.stopChan:
					return
				}
			}
		}(engine)
	}

	// Close merged channel when all engines stop
	go func() {
		wg.Wait()
		close(merged)
	}()

	return merged
}

// GetEngine returns the engine at the given index.
func (s *CheckScheduler) GetEngine(idx int) *Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx < 0 || idx >= len(s.engines) {
		return nil
	}
	return s.engines[idx]
}

// EngineCount returns the number of engines.
func (s *CheckScheduler) EngineCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.engines)
}
