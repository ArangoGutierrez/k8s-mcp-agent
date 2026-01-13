// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordRequest(t *testing.T) {
	// Reset metrics for clean test
	RequestsTotal.Reset()
	RequestDuration.Reset()

	RecordRequest("get_gpu_inventory", "success", 0.123)
	RecordRequest("get_gpu_health", "error", 0.456)
	RecordRequest("get_gpu_inventory", "success", 0.100)

	// Check counter values
	assert.Equal(t, 2.0, testutil.ToFloat64(
		RequestsTotal.WithLabelValues("get_gpu_inventory", "success")))
	assert.Equal(t, 1.0, testutil.ToFloat64(
		RequestsTotal.WithLabelValues("get_gpu_health", "error")))
}

func TestSetNodeHealth(t *testing.T) {
	NodeHealth.Reset()

	SetNodeHealth("node-1", true)
	SetNodeHealth("node-2", false)

	assert.Equal(t, 1.0, testutil.ToFloat64(
		NodeHealth.WithLabelValues("node-1")))
	assert.Equal(t, 0.0, testutil.ToFloat64(
		NodeHealth.WithLabelValues("node-2")))
}

func TestSetCircuitState(t *testing.T) {
	CircuitBreakerState.Reset()

	SetCircuitState("node-1", 0) // closed
	SetCircuitState("node-2", 1) // open
	SetCircuitState("node-3", 2) // half-open

	assert.Equal(t, 0.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-1")))
	assert.Equal(t, 1.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-2")))
	assert.Equal(t, 2.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-3")))
}

func TestActiveRequests(t *testing.T) {
	ActiveRequests.Set(0)

	// Simulate request start
	ActiveRequests.Inc()
	assert.Equal(t, 1.0, testutil.ToFloat64(ActiveRequests))

	ActiveRequests.Inc()
	assert.Equal(t, 2.0, testutil.ToFloat64(ActiveRequests))

	// Simulate request end
	ActiveRequests.Dec()
	assert.Equal(t, 1.0, testutil.ToFloat64(ActiveRequests))
}
