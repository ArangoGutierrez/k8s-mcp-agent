// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_Allow_ClosedCircuit(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// New node should be allowed (circuit closed)
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
}

func TestCircuitBreaker_RecordFailure_OpensCircuit(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 3
	cb := NewCircuitBreaker(cfg)

	// Record failures below threshold
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))

	// Record failure at threshold - circuit opens
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitOpen, cb.State("node-1"))
}

func TestCircuitBreaker_RecordSuccess_ClosesCircuit(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// Success resets the circuit
	cb.RecordSuccess("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
	assert.Equal(t, 0, cb.Failures("node-1"))
}

func TestCircuitBreaker_HalfOpen_AfterTimeout(t *testing.T) {
	cfg := CircuitBreakerConfig{
		Threshold:    2,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitOpen, cb.State("node-1"))

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow request
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitHalfOpen, cb.State("node-1"))
}

func TestCircuitBreaker_MultipleNodes(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open circuit for node-1
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// node-2 should still be allowed
	assert.True(t, cb.Allow("node-2"))

	// Success on node-2
	cb.RecordSuccess("node-2")
	assert.True(t, cb.Allow("node-2"))
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// Reset should clear state
	cb.Reset("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
	assert.Equal(t, 0, cb.Failures("node-1"))
}

func TestCircuitState_String(t *testing.T) {
	assert.Equal(t, "closed", CircuitClosed.String())
	assert.Equal(t, "open", CircuitOpen.String())
	assert.Equal(t, "half-open", CircuitHalfOpen.String())
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()

	assert.Equal(t, 3, cfg.Threshold)
	assert.Equal(t, 30*time.Second, cfg.ResetTimeout)
}

func TestCircuitBreaker_HalfOpen_FailureReopensCircuit(t *testing.T) {
	cfg := CircuitBreakerConfig{
		Threshold:    2,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.Equal(t, CircuitOpen, cb.State("node-1"))

	// Wait for reset timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitHalfOpen, cb.State("node-1"))

	// Another failure while half-open should re-open the circuit
	cb.RecordFailure("node-1")
	assert.Equal(t, CircuitOpen, cb.State("node-1"))
}

func TestCircuitBreaker_HalfOpen_SuccessClosesCircuit(t *testing.T) {
	cfg := CircuitBreakerConfig{
		Threshold:    2,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.Equal(t, CircuitOpen, cb.State("node-1"))

	// Wait for reset timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitHalfOpen, cb.State("node-1"))

	// Success while half-open should close the circuit
	cb.RecordSuccess("node-1")
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
	assert.Equal(t, 0, cb.Failures("node-1"))
}
