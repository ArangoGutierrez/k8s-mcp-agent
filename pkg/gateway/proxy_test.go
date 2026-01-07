// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMCPRequest(t *testing.T) {
	args := map[string]interface{}{"filter": "healthy"}
	request := buildMCPRequest("get_gpu_health", args)

	// Should contain two JSON objects
	lines := splitJSONLines(request)
	require.Len(t, lines, 2, "expected 2 JSON objects")

	// First should be initialize
	var init map[string]interface{}
	err := json.Unmarshal(lines[0], &init)
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

func TestBuildMCPRequest_NilArguments(t *testing.T) {
	request := buildMCPRequest("get_gpu_inventory", nil)

	lines := splitJSONLines(request)
	require.Len(t, lines, 2)

	var tool map[string]interface{}
	err := json.Unmarshal(lines[1], &tool)
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

func TestSplitJSONLines(t *testing.T) {
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
			lines := splitJSONLines([]byte(tt.input))
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
