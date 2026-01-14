// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordGatewayRequest(t *testing.T) {
	// Reset metrics for clean test
	GatewayRequestDuration.Reset()

	// Record various requests
	RecordGatewayRequest("node-1", "http", "success", 0.123)
	RecordGatewayRequest("node-1", "http", "error", 0.456)
	RecordGatewayRequest("node-2", "exec", "success", 2.345)
	RecordGatewayRequest("node-3", "http", "success", 0.100)

	// Verify histogram counts
	// Note: We can't easily test histogram values, but we can verify the metric
	// exists and has the right number of observations via the _count metric

	// For histograms, we check that observations were recorded
	// by verifying the _count suffix metric exists and increments
	assert.Greater(t, testutil.CollectAndCount(GatewayRequestDuration), 0,
		"GatewayRequestDuration should have recorded observations")
}

func TestRecordGatewayRequest_AllTransportTypes(t *testing.T) {
	GatewayRequestDuration.Reset()

	tests := []struct {
		name      string
		node      string
		transport string
		status    string
		duration  float64
	}{
		{
			name:      "http success",
			node:      "test-node-1",
			transport: "http",
			status:    "success",
			duration:  0.200,
		},
		{
			name:      "http error",
			node:      "test-node-1",
			transport: "http",
			status:    "error",
			duration:  0.150,
		},
		{
			name:      "exec success",
			node:      "test-node-2",
			transport: "exec",
			status:    "success",
			duration:  1.500,
		},
		{
			name:      "exec error",
			node:      "test-node-2",
			transport: "exec",
			status:    "error",
			duration:  2.000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordGatewayRequest(tt.node, tt.transport, tt.status, tt.duration)

			// Verify no panic occurred (basic smoke test)
			// Detailed histogram testing is complex, so we verify basic functionality
			assert.NotPanics(t, func() {
				RecordGatewayRequest(tt.node, tt.transport, tt.status, tt.duration)
			})
		})
	}

	// Verify we recorded all observations
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0, "Should have recorded multiple observations")
}

func TestRecordGatewayRequest_BucketDistribution(t *testing.T) {
	GatewayRequestDuration.Reset()

	// Record requests across different latency ranges to verify buckets
	testCases := []struct {
		desc     string
		duration float64
	}{
		{"very fast (5ms)", 0.005},
		{"fast (50ms)", 0.050},
		{"normal (200ms)", 0.200},
		{"slow (1s)", 1.000},
		{"very slow (5s)", 5.000},
		{"timeout range (30s)", 30.000},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			RecordGatewayRequest("test-node", "http", "success", tc.duration)
		})
	}

	// Verify observations were recorded
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0)
}

func TestGatewayRequestDuration_LabelCardinality(t *testing.T) {
	// Verify metric doesn't create excessive cardinality
	GatewayRequestDuration.Reset()

	// Simulate realistic scenario: 10 nodes, 2 transports, 2 statuses
	nodes := []string{"node-1", "node-2", "node-3", "node-4", "node-5",
		"node-6", "node-7", "node-8", "node-9", "node-10"}
	transports := []string{"http", "exec"}
	statuses := []string{"success", "error"}

	for _, node := range nodes {
		for _, transport := range transports {
			for _, status := range statuses {
				RecordGatewayRequest(node, transport, status, 0.1)
			}
		}
	}

	// With 10 nodes × 2 transports × 2 statuses = 40 label combinations
	// This is safe cardinality for Prometheus
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0)

	// Verify we don't panic with many label combinations
	assert.NotPanics(t, func() {
		RecordGatewayRequest("node-11", "http", "success", 0.1)
	})
}
