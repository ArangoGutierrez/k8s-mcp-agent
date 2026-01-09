// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMCPRequest_Proxy(t *testing.T) {
	args := map[string]interface{}{"filter": "healthy"}
	request, err := BuildMCPRequest("get_gpu_health", args)
	require.NoError(t, err, "BuildMCPRequest should not error")

	// Should contain two JSON objects
	lines := SplitJSONObjects(request)
	require.Len(t, lines, 2, "expected 2 JSON objects")

	// First should be initialize
	var init map[string]interface{}
	err = json.Unmarshal(lines[0], &init)
	require.NoError(t, err, "failed to parse init request")
	assert.Equal(t, "initialize", init["method"])

	// Second should be tools/call
	var tool map[string]interface{}
	err = json.Unmarshal(lines[1], &tool)
	require.NoError(t, err, "failed to parse tool request")
	assert.Equal(t, "tools/call", tool["method"])

	// Verify tool name in params
	params, ok := tool["params"].(map[string]interface{})
	require.True(t, ok, "params is not a map")
	assert.Equal(t, "get_gpu_health", params["name"])
}

func TestBuildMCPRequest_NilArgumentsProxy(t *testing.T) {
	request, err := BuildMCPRequest("get_gpu_inventory", nil)
	require.NoError(t, err)

	lines := SplitJSONObjects(request)
	require.Len(t, lines, 2)

	var tool map[string]interface{}
	err = json.Unmarshal(lines[1], &tool)
	require.NoError(t, err)

	params := tool["params"].(map[string]interface{})
	assert.Equal(t, "get_gpu_inventory", params["name"])
	assert.Nil(t, params["arguments"])
}

func TestParseToolResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantErr     bool
		wantContent bool
		wantStatus  string
	}{
		{
			name: "valid response with JSON content",
			response: `{"jsonrpc":"2.0","id":0,"result":{}}` +
				`{"jsonrpc":"2.0","id":1,"result":{"content":[` +
				`{"type":"text","text":"{\"status\":\"healthy\"}"}]}}`,
			wantErr:     false,
			wantContent: true,
			wantStatus:  "healthy",
		},
		{
			name: "error response",
			response: `{"jsonrpc":"2.0","id":0,"result":{}}` +
				`{"jsonrpc":"2.0","id":1,"error":{"code":-1,` +
				`"message":"tool failed"}}`,
			wantErr:     true,
			wantContent: false,
		},
		{
			name: "isError true response",
			response: `{"jsonrpc":"2.0","id":0,"result":{}}` +
				`{"jsonrpc":"2.0","id":1,"result":{"content":[` +
				`{"type":"text","text":"something went wrong"}],` +
				`"isError":true}}`,
			wantErr:     true,
			wantContent: false,
		},
		{
			name:        "empty response",
			response:    "",
			wantErr:     true,
			wantContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolResponse([]byte(tt.response))
			resultMap, isMap := result.(map[string]interface{})

			if tt.wantErr {
				require.True(t, isMap, "expected map result")
				assert.NotNil(t, resultMap["error"], "expected error in result")
			} else if tt.wantContent {
				require.True(t, isMap, "expected map result")
				assert.Nil(t, resultMap["error"], "unexpected error")
				assert.Equal(t, tt.wantStatus, resultMap["status"])
			}
		})
	}
}

func TestSplitJSONObjects_Proxy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "three simple objects",
			input:    `{"a":1}{"b":2}{"c":3}`,
			expected: 3,
		},
		{
			name:     "nested objects",
			input:    `{"a":{"nested":1}}{"b":2}`,
			expected: 2,
		},
		{
			name:     "with newlines",
			input:    "{\"a\":1}\n{\"b\":2}",
			expected: 2,
		},
		{
			name:     "empty",
			input:    "",
			expected: 0,
		},
		{
			name:     "single object",
			input:    `{"key":"value"}`,
			expected: 1,
		},
		{
			name:     "deeply nested",
			input:    `{"a":{"b":{"c":{"d":1}}}}`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := SplitJSONObjects([]byte(tt.input))
			assert.Len(t, lines, tt.expected)
		})
	}
}

func TestAggregateResults_AllSuccess(t *testing.T) {
	handler := &ProxyHandler{toolName: "test"}

	results := []NodeResult{
		{
			NodeName: "node-1",
			PodName:  "pod-1",
			Response: []byte(`{"jsonrpc":"2.0","id":1,"result":{` +
				`"content":[{"type":"text","text":"{\"gpus\":1}"}]}}`),
		},
		{
			NodeName: "node-2",
			PodName:  "pod-2",
			Response: []byte(`{"jsonrpc":"2.0","id":1,"result":{` +
				`"content":[{"type":"text","text":"{\"gpus\":2}"}]}}`),
		},
	}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "success", aggMap["status"])
	assert.Equal(t, 2, aggMap["success_count"])
	assert.Equal(t, 0, aggMap["error_count"])
	assert.Equal(t, 2, aggMap["node_count"])

	nodes := aggMap["nodes"].([]interface{})
	assert.Len(t, nodes, 2)
}

func TestAggregateResults_PartialSuccess(t *testing.T) {
	handler := &ProxyHandler{toolName: "test"}

	results := []NodeResult{
		{
			NodeName: "node-1",
			PodName:  "pod-1",
			Response: []byte(`{"jsonrpc":"2.0","id":1,"result":{` +
				`"content":[{"type":"text","text":"{\"gpus\":1}"}]}}`),
		},
		{
			NodeName: "node-2",
			PodName:  "pod-2",
			Error:    "connection failed",
		},
	}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "partial", aggMap["status"])
	assert.Equal(t, 1, aggMap["success_count"])
	assert.Equal(t, 1, aggMap["error_count"])
}

func TestAggregateResults_AllError(t *testing.T) {
	handler := &ProxyHandler{toolName: "test"}

	results := []NodeResult{
		{
			NodeName: "node-1",
			PodName:  "pod-1",
			Error:    "connection failed",
		},
		{
			NodeName: "node-2",
			PodName:  "pod-2",
			Error:    "timeout",
		},
	}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "error", aggMap["status"])
	assert.Equal(t, 0, aggMap["success_count"])
	assert.Equal(t, 2, aggMap["error_count"])
}

func TestAggregateResults_Empty(t *testing.T) {
	handler := &ProxyHandler{toolName: "test"}

	results := []NodeResult{}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "success", aggMap["status"])
	assert.Equal(t, 0, aggMap["success_count"])
	assert.Equal(t, 0, aggMap["error_count"])
	assert.Equal(t, 0, aggMap["node_count"])
}

func TestAggregateGPUInventory_ClusterSummary(t *testing.T) {
	handler := &ProxyHandler{toolName: "get_gpu_inventory"}

	// Mock response from two nodes with different GPU types
	node1Response := `{"jsonrpc":"2.0","id":0,"result":{}}` +
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text",` +
		`"text":"{\"driver_version\":\"575.57\",\"cuda_version\":\"12.9\",` +
		`\"device_count\":1,\"devices\":[{\"name\":\"Tesla T4\",` +
		`\"index\":0,\"uuid\":\"GPU-xxx\"}]}"}]}}`
	node2Response := `{"jsonrpc":"2.0","id":0,"result":{}}` +
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text",` +
		`"text":"{\"driver_version\":\"575.57\",\"cuda_version\":\"12.9\",` +
		`\"device_count\":2,\"devices\":[{\"name\":\"A100\",\"index\":0,` +
		`\"uuid\":\"GPU-yyy\"},{\"name\":\"A100\",\"index\":1,` +
		`\"uuid\":\"GPU-zzz\"}]}"}]}}`

	results := []NodeResult{
		{NodeName: "node1", PodName: "pod1", Response: []byte(node1Response)},
		{NodeName: "node2", PodName: "pod2", Response: []byte(node2Response)},
	}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "success", aggMap["status"])

	// Check cluster summary
	summary := aggMap["cluster_summary"].(map[string]interface{})
	assert.Equal(t, 2, summary["total_nodes"])
	assert.Equal(t, 2, summary["ready_nodes"])
	assert.Equal(t, 3, summary["total_gpus"])

	gpuTypes := summary["gpu_types"].([]string)
	assert.Len(t, gpuTypes, 2)
	assert.Contains(t, gpuTypes, "Tesla T4")
	assert.Contains(t, gpuTypes, "A100")

	// Check nodes array
	nodes := aggMap["nodes"].([]interface{})
	assert.Len(t, nodes, 2)

	node1Data := nodes[0].(map[string]interface{})
	assert.Equal(t, "node1", node1Data["name"])
	assert.Equal(t, "ready", node1Data["status"])
	assert.Equal(t, "575.57", node1Data["driver_version"])
}

func TestAggregateGPUInventory_WithErrors(t *testing.T) {
	handler := &ProxyHandler{toolName: "get_gpu_inventory"}

	node1Response := `{"jsonrpc":"2.0","id":0,"result":{}}` +
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text",` +
		`"text":"{\"device_count\":1,\"devices\":[{\"name\":\"Tesla T4\",` +
		`\"index\":0}]}"}]}}`

	results := []NodeResult{
		{NodeName: "node1", PodName: "pod1", Response: []byte(node1Response)},
		{NodeName: "node2", PodName: "pod2", Error: "connection refused"},
	}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "success", aggMap["status"])

	summary := aggMap["cluster_summary"].(map[string]interface{})
	assert.Equal(t, 2, summary["total_nodes"])
	assert.Equal(t, 1, summary["ready_nodes"])
	assert.Equal(t, 1, summary["total_gpus"])

	nodes := aggMap["nodes"].([]interface{})
	node2Data := nodes[1].(map[string]interface{})
	assert.Equal(t, "error", node2Data["status"])
	assert.Equal(t, "connection refused", node2Data["error"])
}

func TestAggregateGPUInventory_Empty(t *testing.T) {
	handler := &ProxyHandler{toolName: "get_gpu_inventory"}

	results := []NodeResult{}

	aggregated := handler.aggregateResults(results)
	aggMap := aggregated.(map[string]interface{})

	assert.Equal(t, "success", aggMap["status"])

	summary := aggMap["cluster_summary"].(map[string]interface{})
	assert.Equal(t, 0, summary["total_nodes"])
	assert.Equal(t, 0, summary["ready_nodes"])
	assert.Equal(t, 0, summary["total_gpus"])

	gpuTypes := summary["gpu_types"].([]string)
	assert.Len(t, gpuTypes, 0)
}

func TestFlattenGPUInfo(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]interface{}
		checkFn func(*testing.T, map[string]interface{})
	}{
		{
			name:  "nil input",
			input: nil,
			checkFn: func(t *testing.T, result map[string]interface{}) {
				assert.Contains(t, result, "error")
			},
		},
		{
			name: "basic fields",
			input: map[string]interface{}{
				"index": 0,
				"name":  "Tesla T4",
				"uuid":  "GPU-xxx",
			},
			checkFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, 0, result["index"])
				assert.Equal(t, "Tesla T4", result["name"])
				assert.Equal(t, "GPU-xxx", result["uuid"])
			},
		},
		{
			name: "with memory flattening",
			input: map[string]interface{}{
				"name": "Tesla T4",
				"memory": map[string]interface{}{
					"total_bytes": float64(16106127360),
				},
			},
			checkFn: func(t *testing.T, result map[string]interface{}) {
				memGB := result["memory_total_gb"].(float64)
				assert.InDelta(t, 15.0, memGB, 0.5)
			},
		},
		{
			name: "with temperature flattening",
			input: map[string]interface{}{
				"name": "Tesla T4",
				"temperature": map[string]interface{}{
					"current_celsius": float64(45),
				},
			},
			checkFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, 45, result["temperature_c"])
			},
		},
		{
			name: "with utilization flattening",
			input: map[string]interface{}{
				"name": "Tesla T4",
				"utilization": map[string]interface{}{
					"gpu_percent": float64(85),
				},
			},
			checkFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, 85, result["utilization_percent"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenGPUInfo(tt.input)
			tt.checkFn(t, result)
		})
	}
}
