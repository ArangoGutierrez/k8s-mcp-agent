// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sendMCPRequest sends an MCP request from a fixture file.
func sendMCPRequest(t *testing.T, fixture string) *JSONRPCResponse {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("fixtures", "requests", fixture))
	require.NoError(t, err, "Failed to read fixture %s", fixture)

	return sendMCPRequestData(t, data)
}

// sendMCPRequestData sends raw MCP request data.
func sendMCPRequestData(t *testing.T, data []byte) *JSONRPCResponse {
	t.Helper()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(gatewayURL+"/mcp", "application/json",
		bytes.NewReader(data))
	require.NoError(t, err, "Failed to send MCP request")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read MCP response")

	t.Logf("MCP Response: %s", string(body))

	var rpcResp JSONRPCResponse
	err = json.Unmarshal(body, &rpcResp)
	require.NoError(t, err, "Failed to parse JSON-RPC response: %s", string(body))

	return &rpcResp
}

func TestMCP_Initialize(t *testing.T) {
	resp := sendMCPRequest(t, "initialize.json")

	assert.Equal(t, "2.0", resp.JSONRPC, "JSONRPC version should be 2.0")
	assert.Nil(t, resp.Error, "Initialize should not return error")
	require.NotNil(t, resp.Result, "Initialize should return result")

	// Verify result contains server info
	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err, "Failed to parse initialize result")

	assert.Contains(t, result, "protocolVersion",
		"Initialize result should contain protocolVersion")
	assert.Contains(t, result, "serverInfo",
		"Initialize result should contain serverInfo")
	assert.Contains(t, result, "capabilities",
		"Initialize result should contain capabilities")

	// Verify server info
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	require.True(t, ok, "serverInfo should be an object")
	assert.Contains(t, serverInfo, "name", "serverInfo should contain name")
}

func TestMCP_ToolsList(t *testing.T) {
	// Initialize first (required by MCP protocol)
	sendMCPRequest(t, "initialize.json")

	resp := sendMCPRequest(t, "tools_list.json")

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error, "tools/list should not return error")
	require.NotNil(t, resp.Result, "tools/list should return result")

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err, "Failed to parse tools/list result")

	tools, ok := result["tools"].([]interface{})
	require.True(t, ok, "Result should contain tools array")

	t.Logf("Found %d tools", len(tools))

	// Should have 5 tools
	assert.Len(t, tools, 5, "Expected 5 MCP tools")

	// Collect tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		require.True(t, ok, "Tool should be an object")
		name, ok := toolMap["name"].(string)
		require.True(t, ok, "Tool should have name string")
		toolNames[i] = name
		t.Logf("Tool %d: %s", i, name)
	}

	// Verify all expected tools
	expectedTools := []string{
		"get_gpu_inventory",
		"get_gpu_health",
		"analyze_xid_errors",
		"describe_gpu_node",
		"get_pod_gpu_allocation",
	}
	for _, expected := range expectedTools {
		assert.Contains(t, toolNames, expected,
			"Should have tool %s", expected)
	}
}

func TestMCP_ToolsHaveSchemas(t *testing.T) {
	sendMCPRequest(t, "initialize.json")
	resp := sendMCPRequest(t, "tools_list.json")

	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err)

	tools, ok := result["tools"].([]interface{})
	require.True(t, ok, "Result should contain tools array")
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		require.True(t, ok, "Tool should be an object")
		name, ok := toolMap["name"].(string)
		require.True(t, ok, "Tool should have name string")

		// Each tool should have description and inputSchema
		assert.Contains(t, toolMap, "description",
			"Tool %s should have description", name)
		assert.Contains(t, toolMap, "inputSchema",
			"Tool %s should have inputSchema", name)

		// Verify inputSchema is valid
		schema, ok := toolMap["inputSchema"].(map[string]interface{})
		require.True(t, ok, "inputSchema for %s should be object", name)
		assert.Equal(t, "object", schema["type"],
			"inputSchema type for %s should be 'object'", name)
	}
}

func TestMCP_InvalidMethod(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":99,"method":"invalid/method"}`
	resp := sendMCPRequestData(t, []byte(req))

	assert.Equal(t, "2.0", resp.JSONRPC)
	require.NotNil(t, resp.Error, "Invalid method should return error")
	assert.Equal(t, -32601, resp.Error.Code,
		"Invalid method should return -32601 (method not found)")
	t.Logf("Error message: %s", resp.Error.Message)
}

func TestMCP_MalformedJSON(t *testing.T) {
	req := `{not valid json`

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(gatewayURL+"/mcp", "application/json",
		bytes.NewBufferString(req))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Malformed JSON response: %s", string(body))

	var rpcResp JSONRPCResponse
	err = json.Unmarshal(body, &rpcResp)
	require.NoError(t, err, "Server should return valid JSON-RPC error")

	require.NotNil(t, rpcResp.Error, "Malformed JSON should return error")
	assert.Equal(t, -32700, rpcResp.Error.Code,
		"Malformed JSON should return -32700 (parse error)")
}

func TestMCP_MissingID(t *testing.T) {
	// Notification without ID (should not return response per JSON-RPC spec,
	// but many servers do respond)
	req := `{"jsonrpc":"2.0","method":"tools/list"}`
	resp := sendMCPRequestData(t, []byte(req))

	// The server may return null ID or error - both are acceptable
	t.Logf("Response for request without ID: JSONRPC=%s, ID=%v, Error=%v",
		resp.JSONRPC, resp.ID, resp.Error)
}

func TestMCP_InvalidParams(t *testing.T) {
	// Call tool with invalid params
	req := `{
		"jsonrpc": "2.0",
		"id": 100,
		"method": "tools/call",
		"params": {
			"name": "describe_gpu_node",
			"arguments": {
				"invalid_param": "value"
			}
		}
	}`
	resp := sendMCPRequestData(t, []byte(req))

	assert.Equal(t, "2.0", resp.JSONRPC)
	// The response could be an error or success depending on tool implementation
	// Most tools will ignore unknown params
	t.Logf("Invalid params response - Error: %v, Result: %s",
		resp.Error, string(resp.Result))
}
