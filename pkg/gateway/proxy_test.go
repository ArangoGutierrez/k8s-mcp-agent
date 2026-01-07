// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"
)

func TestBuildMCPRequest(t *testing.T) {
	args := map[string]interface{}{"filter": "healthy"}
	request := buildMCPRequest("get_gpu_health", args)

	// Should contain two JSON objects
	lines := splitJSONLines(request)
	if len(lines) != 2 {
		t.Errorf("expected 2 JSON objects, got %d", len(lines))
	}

	// First should be initialize
	var init map[string]interface{}
	if err := json.Unmarshal(lines[0], &init); err != nil {
		t.Fatalf("failed to parse init request: %v", err)
	}
	if init["method"] != "initialize" {
		t.Errorf("expected initialize method, got %v", init["method"])
	}

	// Second should be tools/call
	var tool map[string]interface{}
	if err := json.Unmarshal(lines[1], &tool); err != nil {
		t.Fatalf("failed to parse tool request: %v", err)
	}
	if tool["method"] != "tools/call" {
		t.Errorf("expected tools/call method, got %v", tool["method"])
	}

	// Verify tool name in params
	params, ok := tool["params"].(map[string]interface{})
	if !ok {
		t.Fatal("params is not a map")
	}
	if params["name"] != "get_gpu_health" {
		t.Errorf("expected get_gpu_health, got %v", params["name"])
	}
}

func TestParseToolResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantErr     bool
		wantContent bool
	}{
		{
			name: "valid response with JSON content",
			response: `{"jsonrpc":"2.0","id":0,"result":{}}` +
				`{"jsonrpc":"2.0","id":1,"result":{"content":[` +
				`{"type":"text","text":"{\"status\":\"healthy\"}"}]}}`,
			wantErr:     false,
			wantContent: true,
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
				if !isMap || resultMap["error"] == nil {
					// For empty response case
					if tt.response == "" {
						if resultMap["error"] != "empty response" {
							t.Errorf("expected 'empty response' error")
						}
						return
					}
					t.Errorf("expected error in result, got %v", result)
				}
			} else if tt.wantContent {
				if isMap && resultMap["error"] != nil {
					t.Errorf("unexpected error: %v", resultMap["error"])
				}
				// Should have parsed the JSON content
				if isMap && resultMap["status"] != "healthy" {
					t.Errorf("expected status=healthy, got %v", result)
				}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitJSONLines([]byte(tt.input))
			if len(lines) != tt.expected {
				t.Errorf("expected %d lines, got %d", tt.expected, len(lines))
			}
		})
	}
}

func TestAggregateResults(t *testing.T) {
	handler := &ProxyHandler{toolName: "test"}

	t.Run("all success", func(t *testing.T) {
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

		if aggMap["status"] != "success" {
			t.Errorf("expected success status, got %v", aggMap["status"])
		}
		if aggMap["success_count"] != 2 {
			t.Errorf("expected 2 success, got %v", aggMap["success_count"])
		}
		if aggMap["error_count"] != 0 {
			t.Errorf("expected 0 errors, got %v", aggMap["error_count"])
		}
	})

	t.Run("partial success", func(t *testing.T) {
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

		if aggMap["status"] != "partial" {
			t.Errorf("expected partial status, got %v", aggMap["status"])
		}
		if aggMap["success_count"] != 1 {
			t.Errorf("expected 1 success, got %v", aggMap["success_count"])
		}
		if aggMap["error_count"] != 1 {
			t.Errorf("expected 1 error, got %v", aggMap["error_count"])
		}
	})

	t.Run("all error", func(t *testing.T) {
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

		if aggMap["status"] != "error" {
			t.Errorf("expected error status, got %v", aggMap["status"])
		}
		if aggMap["success_count"] != 0 {
			t.Errorf("expected 0 success, got %v", aggMap["success_count"])
		}
		if aggMap["error_count"] != 2 {
			t.Errorf("expected 2 errors, got %v", aggMap["error_count"])
		}
	})
}
