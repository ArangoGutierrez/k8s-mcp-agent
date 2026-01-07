// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package gateway provides the MCP gateway router for multi-node GPU clusters.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
)

// Router forwards MCP requests to node agents via pod exec.
type Router struct {
	k8sClient *k8s.Client
}

// NewRouter creates a new gateway router.
func NewRouter(k8sClient *k8s.Client) *Router {
	return &Router{k8sClient: k8sClient}
}

// NodeResult holds the result from a single node.
type NodeResult struct {
	NodeName string          `json:"node_name"`
	PodName  string          `json:"pod_name"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// RouteToNode sends an MCP request to a specific node's agent.
// This performs a pod lookup by node name first.
func (r *Router) RouteToNode(
	ctx context.Context,
	nodeName string,
	mcpRequest []byte,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing to node","node":"%s"}`,
		nodeName)

	node, err := r.k8sClient.GetPodForNode(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	return r.routeToGPUNode(ctx, *node, mcpRequest)
}

// routeToGPUNode sends an MCP request to a known GPU node's agent.
// This is more efficient when the GPUNode is already known (e.g., from
// ListGPUNodes) as it avoids an extra API call.
func (r *Router) routeToGPUNode(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
) ([]byte, error) {
	if !node.Ready {
		return nil, fmt.Errorf("agent on node %s is not ready", node.Name)
	}

	// Execute agent in pod with MCP request as stdin
	stdin := bytes.NewReader(mcpRequest)
	response, err := r.k8sClient.ExecInPod(ctx, node.PodName, "agent", stdin)
	if err != nil {
		return nil, fmt.Errorf("exec failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"debug","msg":"node response received",`+
		`"node":"%s","response_size":%d}`, node.Name, len(response))

	return response, nil
}

// RouteToAllNodes sends an MCP request to all nodes and aggregates results.
func (r *Router) RouteToAllNodes(
	ctx context.Context,
	mcpRequest []byte,
) ([]NodeResult, error) {
	nodes, err := r.k8sClient.ListGPUNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list GPU nodes: %w", err)
	}

	log.Printf(`{"level":"debug","msg":"routing to all nodes",`+
		`"node_count":%d}`, len(nodes))

	results := make([]NodeResult, 0, len(nodes))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, node := range nodes {
		if !node.Ready {
			log.Printf(`{"level":"warn","msg":"skipping unready node",`+
				`"node":"%s"}`, node.Name)
			continue
		}

		wg.Add(1)
		go func(n k8s.GPUNode) {
			defer wg.Done()

			// Use routeToGPUNode directly to avoid redundant API call
			response, err := r.routeToGPUNode(ctx, n, mcpRequest)

			mu.Lock()
			defer mu.Unlock()

			result := NodeResult{
				NodeName: n.Name,
				PodName:  n.PodName,
			}
			if err != nil {
				result.Error = err.Error()
				log.Printf(`{"level":"error","msg":"node routing failed",`+
					`"node":"%s","error":"%s"}`, n.Name, err)
			} else {
				result.Response = response
			}
			results = append(results, result)
		}(node)
	}

	wg.Wait()

	log.Printf(`{"level":"info","msg":"aggregation complete",`+
		`"total_nodes":%d,"results":%d}`, len(nodes), len(results))

	return results, nil
}
