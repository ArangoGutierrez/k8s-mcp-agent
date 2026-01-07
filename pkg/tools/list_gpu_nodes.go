// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/mark3labs/mcp-go/mcp"
)

// ListGPUNodesHandler handles the list_gpu_nodes tool.
type ListGPUNodesHandler struct {
	k8sClient *k8s.Client
}

// NewListGPUNodesHandler creates a new handler.
func NewListGPUNodesHandler(k8sClient *k8s.Client) *ListGPUNodesHandler {
	return &ListGPUNodesHandler{k8sClient: k8sClient}
}

// Handle processes the list_gpu_nodes tool request.
func (h *ListGPUNodesHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf(`{"level":"info","msg":"list_gpu_nodes invoked"}`)

	nodes, err := h.k8sClient.ListGPUNodes(ctx)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to list GPU nodes",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to list GPU nodes: %s", err)), nil
	}

	// Build response with summary
	readyCount := 0
	for _, node := range nodes {
		if node.Ready {
			readyCount++
		}
	}

	response := map[string]interface{}{
		"status":      "success",
		"node_count":  len(nodes),
		"ready_count": readyCount,
		"nodes":       nodes,
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal response",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to marshal response: %s", err)), nil
	}

	log.Printf(`{"level":"info","msg":"list_gpu_nodes completed",`+
		`"node_count":%d,"ready_count":%d}`, len(nodes), readyCount)

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetListGPUNodesTool returns the MCP tool definition.
func GetListGPUNodesTool() mcp.Tool {
	return mcp.NewTool("list_gpu_nodes",
		mcp.WithDescription(
			"Lists all Kubernetes nodes running the GPU MCP agent. "+
				"Returns node names, pod names, and readiness status. "+
				"Use this to discover which nodes have GPU agents before "+
				"querying specific nodes with other GPU tools. "+
				"Only available in Gateway mode.",
		),
	)
}
