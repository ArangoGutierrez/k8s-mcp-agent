// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/mark3labs/mcp-go/mcp"
)

// ProxyHandler forwards tool calls to node agents and aggregates responses.
type ProxyHandler struct {
	router   *Router
	toolName string
}

// NewProxyHandler creates a handler that proxies a specific tool to agents.
func NewProxyHandler(k8sClient *k8s.Client, toolName string) *ProxyHandler {
	return &ProxyHandler{
		router:   NewRouter(k8sClient),
		toolName: toolName,
	}
}

// Handle proxies the tool call to all node agents and aggregates results.
func (p *ProxyHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf(`{"level":"info","msg":"proxy_tool invoked","tool":"%s"}`,
		p.toolName)

	// Build MCP request to send to agents
	mcpRequest := buildMCPRequest(p.toolName, request.Params.Arguments)

	// Route to all nodes
	results, err := p.router.RouteToAllNodes(ctx, mcpRequest)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to route to nodes: %v", err)), nil
	}

	// Aggregate results
	aggregated := p.aggregateResults(results)

	jsonBytes, err := json.MarshalIndent(aggregated, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	log.Printf(`{"level":"info","msg":"proxy_tool completed","tool":"%s",`+
		`"node_count":%d}`, p.toolName, len(results))

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// buildMCPRequest creates the JSON-RPC request to send to node agents.
func buildMCPRequest(toolName string, arguments interface{}) []byte {
	// First send initialize, then the tool call
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "gateway-proxy",
				"version": "1.0",
			},
		},
		"id": 0,
	}

	toolReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
		"id": 1,
	}

	initBytes, err := json.Marshal(initReq)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal init request",`+
			`"error":"%v"}`, err)
		return nil
	}

	toolBytes, err := json.Marshal(toolReq)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal tool request",`+
			`"error":"%v"}`, err)
		return nil
	}

	// Concatenate with newline (stdio protocol)
	return append(append(initBytes, '\n'), toolBytes...)
}

// aggregateResults combines results from multiple nodes.
func (p *ProxyHandler) aggregateResults(results []NodeResult) interface{} {
	// For tools that return per-node data, aggregate into a cluster view
	aggregated := map[string]interface{}{
		"status":     "success",
		"node_count": len(results),
		"nodes":      []interface{}{},
	}

	successCount := 0
	errorCount := 0
	nodeResults := make([]interface{}, 0, len(results))

	for _, result := range results {
		nodeData := map[string]interface{}{
			"node_name": result.NodeName,
			"pod_name":  result.PodName,
		}

		if result.Error != "" {
			nodeData["error"] = result.Error
			errorCount++
		} else {
			// Parse the tool response from the agent
			parsed := parseToolResponse(result.Response)
			nodeData["data"] = parsed
			successCount++
		}

		nodeResults = append(nodeResults, nodeData)
	}

	aggregated["nodes"] = nodeResults
	aggregated["success_count"] = successCount
	aggregated["error_count"] = errorCount

	if errorCount > 0 && successCount == 0 {
		aggregated["status"] = "error"
	} else if errorCount > 0 {
		aggregated["status"] = "partial"
	}

	return aggregated
}

// parseToolResponse extracts the tool result from the MCP response.
func parseToolResponse(response []byte) interface{} {
	// The response contains initialize response + tool response
	// Split by newline and parse the last JSON object
	lines := splitJSONLines(response)
	if len(lines) == 0 {
		return map[string]interface{}{"error": "empty response"}
	}

	// Parse the last response (tool call result)
	lastLine := lines[len(lines)-1]
	var mcpResponse struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(lastLine, &mcpResponse); err != nil {
		return map[string]interface{}{"error": "failed to parse response"}
	}

	if mcpResponse.Error != nil {
		return map[string]interface{}{"error": mcpResponse.Error.Message}
	}

	if mcpResponse.Result.IsError {
		if len(mcpResponse.Result.Content) > 0 {
			return map[string]interface{}{
				"error": mcpResponse.Result.Content[0].Text,
			}
		}
		return map[string]interface{}{"error": "unknown error"}
	}

	// Parse the text content as JSON
	if len(mcpResponse.Result.Content) > 0 {
		var data interface{}
		if err := json.Unmarshal(
			[]byte(mcpResponse.Result.Content[0].Text), &data); err != nil {
			// Return as string if not JSON
			return mcpResponse.Result.Content[0].Text
		}
		return data
	}

	return nil
}

// splitJSONLines splits response into individual JSON objects.
//
// Note: This is a simple brace-counting parser that works for well-formed
// MCP JSON-RPC responses. It does not handle escaped braces within string
// values (e.g., "error": "expected '}' but got '{'"). This is acceptable
// because MCP responses are machine-generated and don't contain such patterns.
// For more complex scenarios, consider using a streaming JSON decoder.
func splitJSONLines(data []byte) [][]byte {
	var lines [][]byte
	var current []byte
	depth := 0

	for _, b := range data {
		current = append(current, b)
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				lines = append(lines, current)
				current = nil
			}
		}
	}

	return lines
}
