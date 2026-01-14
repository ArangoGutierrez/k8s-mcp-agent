// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package metrics provides Prometheus metrics for the MCP server.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total MCP requests by tool and status.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_requests_total",
			Help: "Total MCP requests processed",
		},
		[]string{"tool", "status"},
	)

	// RequestDuration tracks MCP request latency.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_request_duration_seconds",
			Help:    "MCP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool"},
	)

	// NodeHealth tracks per-node health status.
	NodeHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_node_health",
			Help: "Node health status (1=healthy, 0=unhealthy)",
		},
		[]string{"node"},
	)

	// CircuitBreakerState tracks circuit breaker state per node.
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"node"},
	)

	// ActiveRequests tracks in-flight requests.
	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_active_requests",
			Help: "Number of active MCP requests",
		},
	)

	// GatewayRequestDuration tracks gateway-to-agent request latency by node,
	// transport method (http/exec), and status (success/error).
	// This provides granular visibility into per-node performance and enables
	// detection of slow nodes or transport issues.
	GatewayRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "mcp_gateway_request_duration_seconds",
			Help: "Gateway to agent request duration in seconds",
			// Custom buckets optimized for gateway-to-agent latency.
			// HTTP mode typically: 50-500ms
			// Exec mode typically: 500ms-5s
			// Timeouts: 30-60s
			Buckets: []float64{
				0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
			},
		},
		[]string{"node", "transport", "status"},
	)
)

// RecordRequest records metrics for a completed request.
func RecordRequest(tool, status string, durationSeconds float64) {
	RequestsTotal.WithLabelValues(tool, status).Inc()
	RequestDuration.WithLabelValues(tool).Observe(durationSeconds)
}

// SetNodeHealth sets the health status for a node.
func SetNodeHealth(node string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	NodeHealth.WithLabelValues(node).Set(value)
}

// SetCircuitState sets the circuit breaker state for a node.
func SetCircuitState(node string, state int) {
	CircuitBreakerState.WithLabelValues(node).Set(float64(state))
}

// RecordGatewayRequest records latency metrics for a gateway-to-agent request.
// Parameters:
//   - node: Target node name (e.g., "ip-10-0-1-123")
//   - transport: "http" or "exec"
//   - status: "success" or "error"
//   - durationSeconds: Request duration in seconds
func RecordGatewayRequest(node, transport, status string, durationSeconds float64) {
	GatewayRequestDuration.WithLabelValues(node, transport, status).Observe(durationSeconds)
}
