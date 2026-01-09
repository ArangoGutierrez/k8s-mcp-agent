// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

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

	// Build MCP request to send to agents using framing utilities
	mcpRequest, err := BuildMCPRequest(p.toolName, request.GetArguments())
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to build MCP request",`+
			`"tool":"%s","error":"%v"}`, p.toolName, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to build request: %v", err)), nil
	}

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

// aggregateResults combines results from multiple nodes.
func (p *ProxyHandler) aggregateResults(results []NodeResult) interface{} {
	// Special handling for get_gpu_inventory - create cluster summary
	if p.toolName == "get_gpu_inventory" {
		return p.aggregateGPUInventory(results)
	}

	// Default aggregation for other tools
	return p.aggregateDefault(results)
}

// aggregateDefault provides the standard aggregation for most tools.
func (p *ProxyHandler) aggregateDefault(results []NodeResult) interface{} {
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

// aggregateGPUInventory creates a cluster-wide GPU inventory with summary.
func (p *ProxyHandler) aggregateGPUInventory(results []NodeResult) interface{} {
	totalGPUs := 0
	readyNodes := 0
	gpuTypes := make(map[string]bool)
	nodes := make([]interface{}, 0, len(results))

	for _, result := range results {
		nodeData := map[string]interface{}{
			"name": result.NodeName,
		}

		if result.Error != "" {
			nodeData["status"] = "error"
			nodeData["error"] = result.Error
		} else {
			nodeData["status"] = "ready"
			readyNodes++

			// Parse the inventory response
			parsed := parseToolResponse(result.Response)
			if inv, ok := parsed.(map[string]interface{}); ok {
				// Extract driver/cuda versions
				if v, ok := inv["driver_version"]; ok {
					nodeData["driver_version"] = v
				}
				if v, ok := inv["cuda_version"]; ok {
					nodeData["cuda_version"] = v
				}

				// Extract and flatten GPU list
				if devices, ok := inv["devices"].([]interface{}); ok {
					totalGPUs += len(devices)
					gpus := make([]interface{}, 0, len(devices))
					for _, d := range devices {
						if dev, ok := d.(map[string]interface{}); ok {
							// Collect GPU types
							if name, ok := dev["name"].(string); ok {
								gpuTypes[name] = true
							}
							// Flatten memory to memory_total_gb
							gpu := flattenGPUInfo(dev)
							gpus = append(gpus, gpu)
						}
					}
					nodeData["gpus"] = gpus
				}
			}
		}

		nodes = append(nodes, nodeData)
	}

	// Build GPU types list (sorted for deterministic output)
	types := make([]string, 0, len(gpuTypes))
	for t := range gpuTypes {
		types = append(types, t)
	}
	sort.Strings(types)

	return map[string]interface{}{
		"status": "success",
		"cluster_summary": map[string]interface{}{
			"total_nodes": len(results),
			"ready_nodes": readyNodes,
			"total_gpus":  totalGPUs,
			"gpu_types":   types,
		},
		"nodes": nodes,
	}
}

// flattenGPUInfo simplifies GPU info for cluster view.
// Returns a flattened GPU info map with proper nil handling.
func flattenGPUInfo(dev map[string]interface{}) map[string]interface{} {
	if dev == nil {
		return map[string]interface{}{"error": "nil device data"}
	}

	gpu := make(map[string]interface{})

	// Copy basic fields with nil checks
	if v, ok := dev["index"]; ok {
		gpu["index"] = v
	}
	if v, ok := dev["name"]; ok {
		gpu["name"] = v
	}
	if v, ok := dev["uuid"]; ok {
		gpu["uuid"] = v
	}

	// Flatten memory with proper type checking
	if mem, ok := dev["memory"].(map[string]interface{}); ok && mem != nil {
		if total, ok := mem["total_bytes"].(float64); ok {
			gpu["memory_total_gb"] = total / (1024 * 1024 * 1024)
		}
	}

	// Flatten temperature with proper type checking
	if temp, ok := dev["temperature"].(map[string]interface{}); ok && temp != nil {
		if curr, ok := temp["current_celsius"].(float64); ok {
			gpu["temperature_c"] = int(curr)
		}
	}

	// Flatten utilization with proper type checking
	if util, ok := dev["utilization"].(map[string]interface{}); ok && util != nil {
		if gpuPct, ok := util["gpu_percent"].(float64); ok {
			gpu["utilization_percent"] = int(gpuPct)
		}
	}

	return gpu
}

// parseToolResponse extracts the tool result from the MCP response.
// Uses the framing utilities for robust JSON parsing.
func parseToolResponse(response []byte) interface{} {
	// Use ParseStdioResponse for proper error handling
	data, err := ParseStdioResponse(response)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	if data == nil {
		return nil
	}

	return data
}
