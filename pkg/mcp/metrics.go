// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/metrics"
)

// Re-export metrics from the metrics package for backwards compatibility.
// The metrics are defined in pkg/metrics to avoid import cycles.

var (
	// RequestsTotal counts total MCP requests by tool and status.
	RequestsTotal = metrics.RequestsTotal

	// RequestDuration tracks MCP request latency.
	RequestDuration = metrics.RequestDuration

	// NodeHealth tracks per-node health status.
	NodeHealth = metrics.NodeHealth

	// CircuitBreakerState tracks circuit breaker state per node.
	CircuitBreakerState = metrics.CircuitBreakerState

	// ActiveRequests tracks in-flight requests.
	ActiveRequests = metrics.ActiveRequests

	// GatewayRequestDuration tracks gateway-to-agent request latency.
	GatewayRequestDuration = metrics.GatewayRequestDuration
)

// RecordRequest records metrics for a completed request.
func RecordRequest(tool, status string, durationSeconds float64) {
	metrics.RecordRequest(tool, status, durationSeconds)
}

// SetNodeHealth sets the health status for a node.
func SetNodeHealth(node string, healthy bool) {
	metrics.SetNodeHealth(node, healthy)
}

// SetCircuitState sets the circuit breaker state for a node.
func SetCircuitState(node string, state int) {
	metrics.SetCircuitState(node, state)
}

// RecordGatewayRequest records latency metrics for a gateway-to-agent request.
func RecordGatewayRequest(node, transport, status string, durationSeconds float64) {
	metrics.RecordGatewayRequest(node, transport, status, durationSeconds)
}
