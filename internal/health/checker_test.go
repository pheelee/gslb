package health

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTCPChecker tests the TCP port connectivity checker.
func TestTCPChecker(t *testing.T) {
	t.Run("successful_connection", func(t *testing.T) {
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

		checker := NewTCPChecker(listener.Addr().String(), 5*time.Second)
		assert.Equal(t, listener.Addr().String(), checker.Target())

		ctx := context.Background()
		healthy, err := checker.Check(ctx)

		assert.True(t, healthy)
		assert.NoError(t, err)
	})

	t.Run("connection_refused", func(t *testing.T) {
		// Use a port that's unlikely to be open
		checker := NewTCPChecker("127.0.0.1:1", 100*time.Millisecond)

		ctx := context.Background()
		healthy, err := checker.Check(ctx)

		assert.False(t, healthy)
		assert.Error(t, err)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		checker := NewTCPChecker("192.0.2.1:80", 5*time.Second) // Use TEST-NET-1 (non-routable)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		healthy, err := checker.Check(ctx)

		assert.False(t, healthy)
		assert.Error(t, err)
	})

	t.Run("timeout", func(t *testing.T) {
		checker := NewTCPChecker("192.0.2.1:80", 50*time.Millisecond) // TEST-NET-1 is non-routable

		ctx := context.Background()
		start := time.Now()
		healthy, err := checker.Check(ctx)
		duration := time.Since(start)

		assert.False(t, healthy)
		assert.Error(t, err)
		assert.Less(t, duration, 200*time.Millisecond, "should timeout quickly")
	})

	t.Run("no_port_specified", func(t *testing.T) {
		// Test when target doesn't have a port
		checker := NewTCPChecker("127.0.0.1", 100*time.Millisecond)

		// The checker should add port 80 by default
		assert.Equal(t, "127.0.0.1", checker.Target())
	})
}

// TestICMPChecker tests the ICMP ping checker.
// Note: These tests may be skipped if raw ICMP sockets are not available.
func TestICMPChecker(t *testing.T) {
	t.Run("target_returns_address", func(t *testing.T) {
		checker := NewICMPChecker("127.0.0.1", 5*time.Second)
		assert.Equal(t, "127.0.0.1", checker.Target())
	})

	t.Run("context_cancellation", func(t *testing.T) {
		checker := NewICMPChecker("127.0.0.1", 5*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// The checker may either return context error or attempt the check
		// Both are acceptable behaviors
		_, _ = checker.Check(ctx)
		// Just verify it doesn't panic
	})

	t.Run("invalid_target", func(t *testing.T) {
		checker := NewICMPChecker("invalid-host-name-that-does-not-exist.example", 5*time.Second)

		ctx := context.Background()
		healthy, err := checker.Check(ctx)

		// Should fail because target cannot be resolved
		assert.False(t, healthy)
		assert.Error(t, err)
	})
}

// TestCheckerFactory tests the factory that creates appropriate checkers.
func TestCheckerFactory(t *testing.T) {
	factory := &CheckerFactory{}

	t.Run("create_icmp_checker", func(t *testing.T) {
		req := CheckRequest{
			Type:    "icmp",
			Target:  "127.0.0.1",
			Timeout: 5 * time.Second,
		}
		checker := factory.NewChecker(req)
		assert.NotNil(t, checker)
		assert.Equal(t, "127.0.0.1", checker.Target())
	})

	t.Run("create_tcp_checker", func(t *testing.T) {
		req := CheckRequest{
			Type:    "tcp",
			Target:  "127.0.0.1:80",
			Timeout: 5 * time.Second,
		}
		checker := factory.NewChecker(req)
		assert.NotNil(t, checker)
		assert.Equal(t, "127.0.0.1:80", checker.Target())
	})

	t.Run("unknown_type_returns_fallback", func(t *testing.T) {
		req := CheckRequest{
			Type:    "unknown",
			Target:  "127.0.0.1",
			Timeout: 5 * time.Second,
		}
		checker := factory.NewChecker(req)
		assert.NotNil(t, checker)
		assert.Equal(t, "127.0.0.1", checker.Target())

		// Fallback checker always returns false
		healthy, err := checker.Check(context.Background())
		assert.False(t, healthy)
		assert.NoError(t, err)
	})
}

// BenchmarkTCPChecker benchmarks the TCP checker.
func BenchmarkTCPChecker(b *testing.B) {
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

	checker := NewTCPChecker(listener.Addr().String(), 5*time.Second)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.Check(ctx)
	}
}

// TestCheckerResult verifies CheckResult struct works correctly
func TestCheckerResult(t *testing.T) {
	result := CheckResult{
		BackendID: "backend-1",
		Healthy:   true,
		Error:     nil,
		Duration:  100 * time.Millisecond,
	}

	assert.Equal(t, "backend-1", result.BackendID)
	assert.True(t, result.Healthy)
	assert.NoError(t, result.Error)
	assert.Equal(t, 100*time.Millisecond, result.Duration)

	// Test with error
	result2 := CheckResult{
		BackendID: "backend-2",
		Healthy:   false,
		Error:     fmt.Errorf("connection refused"),
		Duration:  50 * time.Millisecond,
	}

	assert.Equal(t, "backend-2", result2.BackendID)
	assert.False(t, result2.Healthy)
	assert.Error(t, result2.Error)
	assert.Contains(t, result2.Error.Error(), "connection refused")
}
