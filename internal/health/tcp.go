package health

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPChecker performs TCP port connectivity health checks.
type TCPChecker struct {
	target  string
	timeout time.Duration
}

// NewTCPChecker creates a new TCP checker.
// target: host:port (e.g., "192.168.1.1:80", "192.168.1.1:443")
// timeout: maximum time to wait for connection
func NewTCPChecker(target string, timeout time.Duration) *TCPChecker {
	return &TCPChecker{
		target:  target,
		timeout: timeout,
	}
}

// Target returns the target address.
func (c *TCPChecker) Target() string {
	return c.target
}

// Check attempts to establish a TCP connection to the target.
// Returns true if the connection is successful, false otherwise.
func (c *TCPChecker) Check(ctx context.Context) (bool, error) {
	// Create timeout context if not provided with one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Ensure target has a port
	host, port, err := net.SplitHostPort(c.target)
	if err != nil {
		// Try to parse as host only, use common ports
		host = c.target
		port = "80" // Default to port 80
		c.target = net.JoinHostPort(host, port)
	}

	// Validate port
	if port == "" {
		return false, fmt.Errorf("no port specified for TCP check: %s", c.target)
	}

	// Create dialer with timeout support
	dialer := &net.Dialer{
		Timeout:   c.timeout,
		KeepAlive: -1, // Disable keep-alive for health checks
	}

	// Attempt connection with context
	conn, err := dialer.DialContext(ctx, "tcp", c.target)
	if err != nil {
		return false, fmt.Errorf("TCP connection failed: %w", err)
	}
	defer conn.Close()

	// Set deadlines for read/write operations
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return false, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	return true, nil
}
