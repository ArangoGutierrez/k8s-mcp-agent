// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateway_GPUInventoryAggregation(t *testing.T) {
	// Initialize session
	sendMCPRequest(t, "initialize.json")

	// Call get_gpu_inventory
	resp := sendMCPRequest(t, "tools_call_inventory.json")

	assert.Nil(t, resp.Error, "get_gpu_inventory should not error")
	require.NotNil(t, resp.Result, "get_gpu_inventory should return result")

	var result map[string]interface{}
	err := json.Unmarshal(resp.Result, &result)
	require.NoError(t, err, "Failed to parse tool result")

	// Result should contain content array
	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Result should contain content array")
	require.NotEmpty(t, content, "Content array should not be empty")

	// First content item should be text type
	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	assert.Equal(t, "text", firstItem["type"], "Content type should be text")

	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("GPU Inventory response:\n%s", text)

	// In mock mode, should return mock GPU data
	assert.Contains(t, text, "GPU", "Response should mention GPU")
}

func TestGateway_AllNodesRespond(t *testing.T) {
	// Initialize session
	sendMCPRequest(t, "initialize.json")

	// Call get_gpu_health (aggregates from all nodes)
	req := `{
		"jsonrpc": "2.0",
		"id": 10,
		"method": "tools/call",
		"params": {
			"name": "get_gpu_health"
		}
	}`

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Post(gatewayURL+"/mcp", "application/json",
		bytes.NewBufferString(req))
	require.NoError(t, err)
	defer func() { _ = httpResp.Body.Close() }()

	var rpcResp JSONRPCResponse
	err = json.NewDecoder(httpResp.Body).Decode(&rpcResp)
	require.NoError(t, err)

	assert.Nil(t, rpcResp.Error, "get_gpu_health should succeed")
	require.NotNil(t, rpcResp.Result, "get_gpu_health should return result")

	var result map[string]interface{}
	err = json.Unmarshal(rpcResp.Result, &result)
	require.NoError(t, err)

	content, ok := result["content"].([]interface{})
	require.True(t, ok, "Result should contain content array")
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]interface{})
	require.True(t, ok, "Content item should be object")
	text, ok := firstItem["text"].(string)
	require.True(t, ok, "Content should have text string")
	t.Logf("GPU Health response:\n%s", text)

	// Should show health information
	assert.Contains(t, text, "health",
		"Response should contain health information")
}

func TestGateway_AnalyzeXIDErrors(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	req := `{
		"jsonrpc": "2.0",
		"id": 11,
		"method": "tools/call",
		"params": {
			"name": "analyze_xid_errors"
		}
	}`

	resp := sendMCPRequestData(t, []byte(req))

	assert.Nil(t, resp.Error, "analyze_xid_errors should succeed")
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
	t.Logf("XID Analysis response:\n%s", text)

	// Response should exist (may show no errors in mock mode)
	assert.NotEmpty(t, text, "Response should not be empty")
}

func TestGateway_DescribeGPUNode(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	// Without node_name, should return cluster-wide info or first node
	req := `{
		"jsonrpc": "2.0",
		"id": 12,
		"method": "tools/call",
		"params": {
			"name": "describe_gpu_node"
		}
	}`

	resp := sendMCPRequestData(t, []byte(req))

	// This tool may require node_name, so accept either success or error
	if resp.Error != nil {
		t.Logf("describe_gpu_node error (expected without node_name): %s",
			resp.Error.Message)
		// Should be a reasonable error, not a server crash
		assert.NotEqual(t, -32603, resp.Error.Code,
			"Should not be internal error")
		return
	}

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
	t.Logf("Describe GPU Node response:\n%s", text)
}

func TestGateway_GetPodGPUAllocation(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	req := `{
		"jsonrpc": "2.0",
		"id": 13,
		"method": "tools/call",
		"params": {
			"name": "get_pod_gpu_allocation"
		}
	}`

	resp := sendMCPRequestData(t, []byte(req))

	assert.Nil(t, resp.Error, "get_pod_gpu_allocation should succeed")
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
	t.Logf("Pod GPU Allocation response:\n%s", text)

	// May show no pods using GPUs in test environment
	assert.NotEmpty(t, text, "Response should not be empty")
}

func TestGateway_ConcurrentRequests(t *testing.T) {
	sendMCPRequest(t, "initialize.json")

	// Send multiple concurrent requests
	const numRequests = 5

	// Use a struct to capture both response and any error
	type result struct {
		resp *JSONRPCResponse
		err  error
	}
	results := make(chan result, numRequests)

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()

			req := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "get_gpu_inventory",
				},
			}
			data, err := json.Marshal(req)
			if err != nil {
				results <- result{err: err}
				return
			}

			// Make HTTP request directly to avoid race on t.Helper()
			client := &http.Client{Timeout: 30 * time.Second}
			httpResp, err := client.Post(gatewayURL+"/mcp", "application/json",
				bytes.NewReader(data))
			if err != nil {
				results <- result{err: err}
				return
			}
			defer func() { _ = httpResp.Body.Close() }()

			body, err := io.ReadAll(httpResp.Body)
			if err != nil {
				results <- result{err: err}
				return
			}

			var rpcResp JSONRPCResponse
			if err := json.Unmarshal(body, &rpcResp); err != nil {
				results <- result{err: err}
				return
			}

			results <- result{resp: &rpcResp}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect results
	successCount := 0
	for r := range results {
		if r.err == nil && r.resp != nil && r.resp.Error == nil && r.resp.Result != nil {
			successCount++
		}
	}

	t.Logf("Concurrent requests: %d/%d succeeded", successCount, numRequests)
	assert.GreaterOrEqual(t, successCount, numRequests-1,
		"Most concurrent requests should succeed")
}
