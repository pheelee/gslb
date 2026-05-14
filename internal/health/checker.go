// Package health provides health check functionality for GSLB backends.
package health

import (
	"context"
	"time"
)

// HealthChecker is the interface for different types of health checks.
type HealthChecker interface {
	// Check performs the health check and returns true if healthy.
	// The context can be used for timeout and cancellation.
	Check(ctx context.Context) (bool, error)
	// Target returns the target being checked (IP or IP:port).
	Target() string
}

// CheckRequest represents a request to perform a health check.
type CheckRequest struct {
	BackendID string
	Type      string        // "icmp", "tcp"
	Target    string        // IP or IP:port
	Timeout   time.Duration // Overall timeout for the check
}

// CheckResult represents the result of a health check.
type CheckResult struct {
	BackendID string
	Healthy   bool
	Error     error
	Duration  time.Duration
}

// CheckerFactory creates appropriate HealthChecker implementations.
type CheckerFactory struct{}

// NewChecker creates a HealthChecker based on the request type.
func (f *CheckerFactory) NewChecker(req CheckRequest) HealthChecker {
	switch req.Type {
	case "icmp":
		return NewICMPChecker(req.Target, req.Timeout)
	case "tcp":
		return NewTCPChecker(req.Target, req.Timeout)
	default:
		// Return a checker that always fails for unknown types
		return &unknownChecker{target: req.Target, checkType: req.Type}
	}
}

// unknownChecker is a fallback for unsupported check types.
type unknownChecker struct {
	target    string
	checkType string
}

func (u *unknownChecker) Check(ctx context.Context) (bool, error) {
	return false, nil
}

func (u *unknownChecker) Target() string {
	return u.target
}
