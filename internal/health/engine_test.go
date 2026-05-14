package health

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngine tests the health check engine's basic functionality.
func TestEngine(t *testing.T) {
	t.Run("create_engine", func(t *testing.T) {
		engine := NewEngine(4)
		assert.Equal(t, 4, engine.WorkerCount())
		assert.False(t, engine.IsRunning())
		assert.Equal(t, 4, engine.Stats().WorkerCount)
	})

	t.Run("default_worker_count", func(t *testing.T) {
		engine := NewEngine(0)
		assert.Equal(t, 4, engine.WorkerCount())
	})

	t.Run("start_and_stop", func(t *testing.T) {
		engine := NewEngine(2)
		assert.False(t, engine.IsRunning())

		engine.Start()
		assert.True(t, engine.IsRunning())

		engine.Stop()
		assert.False(t, engine.IsRunning())
	})

	t.Run("multiple_start_calls", func(t *testing.T) {
		engine := NewEngine(2)
		engine.Start()
		engine.Start() // Should be a no-op
		assert.True(t, engine.IsRunning())
		engine.Stop()
	})

	t.Run("stop_before_start", func(t *testing.T) {
		engine := NewEngine(2)
		engine.Stop() // Should not panic
		assert.False(t, engine.IsRunning())
	})

	t.Run("submit_to_running_engine", func(t *testing.T) {
		// Create a test TCP server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()

		engine := NewEngine(2)
		engine.Start()
		defer engine.Stop()

		req := CheckRequest{
			BackendID: "backend-1",
			Type:      "tcp",
			Target:    listener.Addr().String(),
			Timeout:   5 * time.Second,
		}

		err = engine.Submit(req)
		assert.NoError(t, err)

		// Wait for result
		select {
		case result := <-engine.Results():
			assert.Equal(t, "backend-1", result.BackendID)
			assert.True(t, result.Healthy)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for result")
		}
	})

	t.Run("submit_to_stopped_engine", func(t *testing.T) {
		engine := NewEngine(2)

		req := CheckRequest{
			BackendID: "backend-1",
			Type:      "tcp",
			Target:    "127.0.0.1:80",
			Timeout:   5 * time.Second,
		}

		err := engine.Submit(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("concurrent_submissions", func(t *testing.T) {
		// Create a test TCP server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()

		engine := NewEngine(4)
		engine.Start()
		defer engine.Stop()

		numRequests := 100

		results := make([]CheckResult, 0, numRequests)
		var resultMu sync.Mutex
		var collectorWg sync.WaitGroup
		stopCollector := make(chan struct{})

		collectorWg.Add(1)
		go func() {
			defer collectorWg.Done()
			for {
				select {
				case result := <-engine.Results():
					resultMu.Lock()
					results = append(results, result)
					resultMu.Unlock()
				case <-stopCollector:
					return
				}
			}
		}()

		var submitWg sync.WaitGroup
		submitWg.Add(numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				defer submitWg.Done()
				req := CheckRequest{
					BackendID: fmt.Sprintf("backend-%d", id%10),
					Type:      "tcp",
					Target:    listener.Addr().String(),
					Timeout:   5 * time.Second,
				}
				err := engine.Submit(req)
				assert.NoError(t, err)
			}(i)
		}

		submitWg.Wait()
		time.Sleep(500 * time.Millisecond)
		close(stopCollector)
		collectorWg.Wait()

		resultMu.Lock()
		resultCount := len(results)
		resultMu.Unlock()

		assert.Equal(t, numRequests, resultCount)
	})
}

// TestCheckScheduler tests the scheduler that manages multiple engines.
func TestCheckScheduler(t *testing.T) {
	t.Run("create_scheduler", func(t *testing.T) {
		scheduler := NewCheckScheduler(2, 4)
		assert.Equal(t, 2, scheduler.EngineCount())
		assert.NotNil(t, scheduler.GetEngine(0))
		assert.NotNil(t, scheduler.GetEngine(1))
		assert.Nil(t, scheduler.GetEngine(2))
		assert.Nil(t, scheduler.GetEngine(-1))
	})

	t.Run("default_scheduler_config", func(t *testing.T) {
		scheduler := NewCheckScheduler(0, 0)
		assert.Equal(t, 1, scheduler.EngineCount())
		assert.NotNil(t, scheduler.GetEngine(0))
	})

	t.Run("register_and_unregister", func(t *testing.T) {
		scheduler := NewCheckScheduler(1, 2)

		req := CheckRequest{
			Type:    "tcp",
			Target:  "127.0.0.1:80",
			Timeout: 5 * time.Second,
		}

		scheduler.Register("backend-1", req)
		scheduler.Register("backend-2", req)

		// Unregister
		scheduler.Unregister("backend-1")
	})

	t.Run("start_and_stop_scheduler", func(t *testing.T) {
		scheduler := NewCheckScheduler(1, 2)
		scheduler.Start()

		// Allow some time for engines to start
		time.Sleep(50 * time.Millisecond)

		scheduler.Stop()
	})
}

// TestWorkerPoolConcurrency tests concurrent behavior of the worker pool.
func TestWorkerPoolConcurrency(t *testing.T) {
	t.Run("multiple_workers", func(t *testing.T) {
		// Create multiple test servers
		listeners := make([]net.Listener, 5)
		for i := range listeners {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			listeners[i] = l
			defer l.Close()

			go func(ln net.Listener) {
				for {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					conn.Close()
				}
			}(l)
		}

		engine := NewEngine(5)
		engine.Start()
		defer engine.Stop()

		numRequests := 50
		results := make(map[string]int)
		var resultMu sync.Mutex
		var collectorWg sync.WaitGroup
		stopCollector := make(chan struct{})

		collectorWg.Add(1)
		go func() {
			defer collectorWg.Done()
			for {
				select {
				case result := <-engine.Results():
					resultMu.Lock()
					results[result.BackendID]++
					resultMu.Unlock()
				case <-stopCollector:
					return
				}
			}
		}()

		var submitWg sync.WaitGroup
		submitWg.Add(numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				defer submitWg.Done()
				req := CheckRequest{
					BackendID: fmt.Sprintf("backend-%d", id%5),
					Type:      "tcp",
					Target:    listeners[id%5].Addr().String(),
					Timeout:   5 * time.Second,
				}
				err := engine.Submit(req)
				assert.NoError(t, err)
			}(i)
		}

		submitWg.Wait()
		time.Sleep(500 * time.Millisecond)
		close(stopCollector)
		collectorWg.Wait()

		resultMu.Lock()
		totalResults := 0
		for _, count := range results {
			totalResults += count
		}
		resultMu.Unlock()

		assert.Equal(t, numRequests, totalResults)
	})
}

// TestEngineShutdown tests graceful shutdown behavior.
func TestEngineShutdown(t *testing.T) {
	t.Run("shutdown_discards_pending_checks", func(t *testing.T) {
		engine := NewEngine(2)
		engine.Start()

		// Submit many requests with short timeout
		for i := 0; i < 20; i++ {
			req := CheckRequest{
				BackendID: fmt.Sprintf("backend-%d", i%10),
				Type:      "tcp",
				Target:    "192.0.2.1:80",
				Timeout:   100 * time.Millisecond,
			}
			_ = engine.Submit(req)
		}

		done := make(chan struct{})
		go func() {
			engine.Stop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("engine shutdown timed out")
		}
	})
}

// TestCheckResultFlow tests the complete flow from submission to result.
func TestCheckResultFlow(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	engine := NewEngine(2)
	engine.Start()
	defer engine.Stop()

	// Test successful check
	t.Run("successful_check_flow", func(t *testing.T) {
		req := CheckRequest{
			BackendID: "test-backend",
			Type:      "tcp",
			Target:    listener.Addr().String(),
			Timeout:   5 * time.Second,
		}

		err := engine.Submit(req)
		require.NoError(t, err)

		result := <-engine.Results()
		assert.Equal(t, "test-backend", result.BackendID)
		assert.True(t, result.Healthy)
		assert.NoError(t, result.Error)
		assert.Greater(t, result.Duration, time.Duration(0))
	})

	// Test failed check
	t.Run("failed_check_flow", func(t *testing.T) {
		req := CheckRequest{
			BackendID: "failing-backend",
			Type:      "tcp",
			Target:    "127.0.0.1:1", // Unlikely to be open
			Timeout:   100 * time.Millisecond,
		}

		err := engine.Submit(req)
		require.NoError(t, err)

		result := <-engine.Results()
		assert.Equal(t, "failing-backend", result.BackendID)
		assert.False(t, result.Healthy)
		assert.Error(t, result.Error)
	})
}

// BenchmarkEngine benchmarks the engine's performance.
func BenchmarkEngine(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	engine := NewEngine(4)
	engine.Start()
	defer engine.Stop()

	req := CheckRequest{
		BackendID: "bench-backend",
		Type:      "tcp",
		Target:    listener.Addr().String(),
		Timeout:   5 * time.Second,
	}

	// Drain results in background
	go func() {
		for range engine.Results() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Submit(req)
	}
}

// BenchmarkEngineParallel benchmarks parallel engine usage.
func BenchmarkEngineParallel(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	engine := NewEngine(4)
	engine.Start()
	defer engine.Stop()

	// Drain results in background
	go func() {
		for range engine.Results() {
		}
	}()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := CheckRequest{
				BackendID: "bench-backend",
				Type:      "tcp",
				Target:    listener.Addr().String(),
				Timeout:   5 * time.Second,
			}
			engine.Submit(req)
			i++
		}
	})
}
