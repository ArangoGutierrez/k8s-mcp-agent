// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// testRequestID is a fixed ID used for test JSON-RPC requests.
	// The specific value doesn't matter for testing, but using a constant
	// makes it clear this is intentional and not a forgotten variable.
	testRequestID = 100
)

// callTool invokes an MCP tool with the given name and arguments.
func callTool(t *testing.T, toolName string, args map[string]interface{}) *JSONRPCResponse {
	t.Helper()

	params := map[string]interface{}{
		"name": toolName,
	}
	if args != nil {
		params["arguments"] = args
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      testRequestID,
		"method":  "tools/call",
		"params":  params,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(gatewayURL+"/mcp", "application/json",
		bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var rpcResp JSONRPCResponse
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	require.NoError(t, err)

	return &rpcResp
}

func TestTool_GetGPUInventory(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := callTool(t, "get_gpu_inventory", nil)

	assert.Nil(t, resp.Error, "get_gpu_inventory should not error")
	require.NotNil(t, resp.Result, "get_gpu_inventory should return result")

	// Verify content structure
	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Should have content array")
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	assert.Equal(t, "text", firstItem["type"])
	assert.Contains(t, firstItem, "text")

	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("get_gpu_inventory output:\n%s", text)
}

func TestTool_GetGPUHealth(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := callTool(t, "get_gpu_health", nil)

	assert.Nil(t, resp.Error, "get_gpu_health should not error")
	require.NotNil(t, resp.Result, "get_gpu_health should return result")

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Result should contain content array")
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("get_gpu_health output:\n%s", text)

	// Should contain health-related information
	assert.Contains(t, text, "health",
		"Output should mention health")
}

func TestTool_AnalyzeXIDErrors(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := callTool(t, "analyze_xid_errors", nil)

	assert.Nil(t, resp.Error, "analyze_xid_errors should not error")
	require.NotNil(t, resp.Result, "analyze_xid_errors should return result")

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Result should contain content array")
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("analyze_xid_errors output:\n%s", text)
}

func TestTool_DescribeGPUNode(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	// Call without node_name to test default behavior
	resp := callTool(t, "describe_gpu_node", nil)

	// This tool may require node_name, accept error or success
	assert.Equal(t, "2.0", resp.JSONRPC)

	if resp.Error != nil {
		t.Logf("describe_gpu_node returned error (may be expected): code=%d, msg=%s",
			resp.Error.Code, resp.Error.Message)
	} else {
		require.NotNil(t, resp.Result)
		var result map[string]interface{}
		err := json.Unmarshal(resp.Result, &result)
		require.NoError(t, err)

		content, ok := result["content"].([]interface{})
		require.True(t, ok, "Result should contain content array")
		require.NotEmpty(t, content)

		firstItem, ok := content[0].(map[string]interface{})
		require.True(t, ok, "Content item should be object")
		text, ok := firstItem["text"].(string)
		require.True(t, ok, "Content should have text string")
		t.Logf("describe_gpu_node output:\n%s", text)
	}
}

func TestTool_GetPodGPUAllocation(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := callTool(t, "get_pod_gpu_allocation", nil)

	assert.Nil(t, resp.Error, "get_pod_gpu_allocation should not error")
	require.NotNil(t, resp.Result, "get_pod_gpu_allocation should return result")

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Result should contain content array")
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("get_pod_gpu_allocation output:\n%s", text)
}

func TestTool_UnknownTool(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := callTool(t, "nonexistent_tool", nil)

	require.NotNil(t, resp.Error, "Unknown tool should return error")
	t.Logf("Unknown tool error: code=%d, msg=%s",
		resp.Error.Code, resp.Error.Message)

	// Should be a proper JSON-RPC error with a non-zero code.
	// JSON-RPC 2.0 reserves -32768 to -32000 for predefined errors,
	// but MCP servers typically return negative codes for tool errors.
	assert.NotZero(t, resp.Error.Code, "Error should have a non-zero code")
}

func TestTool_AllToolsReturnValidContentStructure(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	tools := []string{
		"get_gpu_inventory",
		"get_gpu_health",
		"analyze_xid_errors",
		"get_pod_gpu_allocation",
	}

	for _, toolName := range tools {
		t.Run(toolName, func(t *testing.T) {
			resp := callTool(t, toolName, nil)

			// Skip tools that error (may require specific arguments)
			if resp.Error != nil {
				t.Logf("%s returned error: %s", toolName, resp.Error.Message)
				return
			}

			require.NotNil(t, resp.Result,
				"%s should return result", toolName)

			var result map[string]interface{}
			err := json.Unmarshal(resp.Result, &result)
			require.NoError(t, err, "%s result should be valid JSON", toolName)

			// MCP tools must return content array
			content, ok := result["content"].([]interface{})
			require.True(t, ok,
				"%s should have content array", toolName)
			require.NotEmpty(t, content,
				"%s content should not be empty", toolName)

			// Each content item should have type
			for i, item := range content {
				itemMap, ok := item.(map[string]interface{})
				require.True(t, ok,
					"%s content[%d] should be object", toolName, i)
				_, hasType := itemMap["type"]
				assert.True(t, hasType,
					"%s content[%d] should have type", toolName, i)
			}
		})
	}
}

func TestTool_ResponsePerformance(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	// Test that tools respond within reasonable time
	tools := []string{
		"get_gpu_inventory",
		"get_gpu_health",
	}

	for _, toolName := range tools {
		t.Run(toolName, func(t *testing.T) {
			start := time.Now()
			resp := callTool(t, toolName, nil)
			elapsed := time.Since(start)

			t.Logf("%s response time: %v", toolName, elapsed)

			if resp.Error == nil {
				// Tools should respond within 10 seconds in mock mode
				assert.Less(t, elapsed, 10*time.Second,
					"%s should respond within 10 seconds", toolName)
			}
		})
	}
}
