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

	if !node.Ready {
		return nil, fmt.Errorf("agent on node %s is not ready", nodeName)
	}

	// Execute agent in pod with MCP request as stdin
	stdin := bytes.NewReader(mcpRequest)
	response, err := r.k8sClient.ExecInPod(ctx, node.PodName, "agent", stdin)
	if err != nil {
		return nil, fmt.Errorf("exec failed on node %s: %w", nodeName, err)
	}

	log.Printf(`{"level":"debug","msg":"node response received",`+
		`"node":"%s","response_size":%d}`, nodeName, len(response))

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

			response, err := r.RouteToNode(ctx, n.Name, mcpRequest)

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
