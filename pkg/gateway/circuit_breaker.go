// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed allows requests through (normal operation).
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks requests (node is unhealthy).
	CircuitOpen
	// CircuitHalfOpen allows one test request through.
	CircuitHalfOpen
)

// CircuitBreaker tracks node health and prevents requests to failing nodes.
type CircuitBreaker struct {
	mu            sync.RWMutex
	failures      map[string]int
	lastFailure   map[string]time.Time
	state         map[string]CircuitState
	threshold     int
	resetTimeout  time.Duration
	onStateChange MetricsCallback
}

// MetricsCallback is called when circuit breaker state changes.
// Parameters: node name, circuit state (0=closed, 1=open, 2=half-open), healthy
type MetricsCallback func(node string, state int, healthy bool)

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// Threshold is the number of failures before opening the circuit.
	Threshold int
	// ResetTimeout is how long to wait before trying a half-open request.
	ResetTimeout time.Duration
	// OnStateChange is called when circuit state changes (optional).
	OnStateChange MetricsCallback
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Threshold:    3,
		ResetTimeout: 30 * time.Second,
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		failures:      make(map[string]int),
		lastFailure:   make(map[string]time.Time),
		state:         make(map[string]CircuitState),
		threshold:     cfg.Threshold,
		resetTimeout:  cfg.ResetTimeout,
		onStateChange: cfg.OnStateChange,
	}
}

// Allow checks if a request to the given node should be allowed.
// Returns true if the circuit is closed or half-open.
func (cb *CircuitBreaker) Allow(node string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.state[node]

	switch state {
	case CircuitOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailure[node]) > cb.resetTimeout {
			cb.state[node] = CircuitHalfOpen
			cb.notifyStateChange(node, CircuitHalfOpen, false)
			return true
		}
		return false

	case CircuitHalfOpen:
		// Allow one test request
		return true

	default: // CircuitClosed
		return true
	}
}

// RecordSuccess records a successful request to a node.
// Resets the failure count and closes the circuit.
func (cb *CircuitBreaker) RecordSuccess(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[node] = 0
	cb.state[node] = CircuitClosed
	cb.notifyStateChange(node, CircuitClosed, true)
}

// RecordFailure records a failed request to a node.
// Opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[node]++
	cb.lastFailure[node] = time.Now()

	if cb.failures[node] >= cb.threshold {
		cb.state[node] = CircuitOpen
		cb.notifyStateChange(node, CircuitOpen, false)
	}
}

// notifyStateChange calls the metrics callback if configured.
func (cb *CircuitBreaker) notifyStateChange(node string, state CircuitState, healthy bool) {
	if cb.onStateChange != nil {
		cb.onStateChange(node, int(state), healthy)
	}
}

// State returns the current state for a node.
func (cb *CircuitBreaker) State(node string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state[node]
}

// Failures returns the current failure count for a node.
func (cb *CircuitBreaker) Failures(node string) int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures[node]
}

// Reset resets the circuit breaker state for a node.
func (cb *CircuitBreaker) Reset(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failures, node)
	delete(cb.lastFailure, node)
	delete(cb.state, node)
}

// String returns a string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
