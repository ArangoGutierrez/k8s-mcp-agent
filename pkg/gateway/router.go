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
	"time"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
)

// RoutingMode specifies how the gateway communicates with agents.
type RoutingMode string

const (
	// RoutingModeHTTP routes requests via HTTP to agent pods (recommended).
	RoutingModeHTTP RoutingMode = "http"
	// RoutingModeExec routes requests via kubectl exec (legacy).
	RoutingModeExec RoutingMode = "exec"
)

// Router forwards MCP requests to node agents.
type Router struct {
	k8sClient   *k8s.Client
	httpClient  *AgentHTTPClient
	routingMode RoutingMode
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithRoutingMode sets the routing mode.
func WithRoutingMode(mode RoutingMode) RouterOption {
	return func(r *Router) {
		r.routingMode = mode
	}
}

// NewRouter creates a new gateway router.
func NewRouter(k8sClient *k8s.Client, opts ...RouterOption) *Router {
	r := &Router{
		k8sClient:   k8sClient,
		httpClient:  NewAgentHTTPClient(),
		routingMode: RoutingModeHTTP, // Default to HTTP
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// NodeResult holds the result from a single node.
type NodeResult struct {
	NodeName string          `json:"node_name"`
	PodName  string          `json:"pod_name"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// RoutingMode returns the current routing mode.
func (r *Router) RoutingMode() RoutingMode {
	return r.routingMode
}

// RouteToNode sends an MCP request to a specific node's agent.
// This performs a pod lookup by node name first.
func (r *Router) RouteToNode(
	ctx context.Context,
	nodeName string,
	mcpRequest []byte,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing to node","node":"%s",`+
		`"routing_mode":"%s"}`, nodeName, r.routingMode)

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

	startTime := time.Now()

	// Try HTTP routing if enabled and pod has IP
	if r.routingMode == RoutingModeHTTP {
		endpoint := node.GetAgentHTTPEndpoint()
		if endpoint != "" {
			return r.routeViaHTTP(ctx, node, endpoint, mcpRequest, startTime)
		}
		log.Printf(`{"level":"warn","msg":"pod has no IP, falling back to exec",`+
			`"node":"%s","pod":"%s"}`, node.Name, node.PodName)
	}

	// Fall back to exec routing
	return r.routeViaExec(ctx, node, mcpRequest, startTime)
}

// routeViaHTTP sends request via HTTP to agent pod.
func (r *Router) routeViaHTTP(
	ctx context.Context,
	node k8s.GPUNode,
	endpoint string,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via HTTP","node":"%s",`+
		`"endpoint":"%s","request_size":%d}`,
		node.Name, endpoint, len(mcpRequest))

	// For HTTP mode, we send just the tool call - no init framing needed
	// The agent HTTP server handles the full MCP session
	response, err := r.httpClient.CallMCP(ctx, endpoint, mcpRequest)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf(`{"level":"error","msg":"HTTP request failed","node":"%s",`+
			`"endpoint":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, endpoint, duration.Milliseconds(), err)
		return nil, fmt.Errorf("HTTP request failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"HTTP request completed","node":"%s",`+
		`"endpoint":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, endpoint, duration.Milliseconds(), len(response))

	return response, nil
}

// routeViaExec sends request via kubectl exec to agent pod (legacy mode).
func (r *Router) routeViaExec(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via exec","node":"%s",`+
		`"pod":"%s","request_size":%d}`,
		node.Name, node.PodName, len(mcpRequest))

	stdin := bytes.NewReader(mcpRequest)
	response, err := r.k8sClient.ExecInPod(ctx, node.PodName, "agent", stdin)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf(`{"level":"error","msg":"exec failed","node":"%s",`+
			`"pod":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, node.PodName, duration.Milliseconds(), err)
		return nil, fmt.Errorf("exec failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"exec completed","node":"%s",`+
		`"pod":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, node.PodName, duration.Milliseconds(), len(response))

	return response, nil
}

// RouteToAllNodes sends an MCP request to all nodes and aggregates results.
func (r *Router) RouteToAllNodes(
	ctx context.Context,
	mcpRequest []byte,
) ([]NodeResult, error) {
	startTime := time.Now()

	nodes, err := r.k8sClient.ListGPUNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list GPU nodes: %w", err)
	}

	// Count ready nodes for logging
	readyCount := 0
	for _, n := range nodes {
		if n.Ready {
			readyCount++
		}
	}

	log.Printf(`{"level":"info","msg":"routing to nodes",`+
		`"total_nodes":%d,"ready_nodes":%d,"routing_mode":"%s"}`,
		len(nodes), readyCount, r.routingMode)

	results := make([]NodeResult, 0, len(nodes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	successCount := 0
	failCount := 0

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
				failCount++
			} else {
				result.Response = response
				successCount++
			}
			results = append(results, result)
		}(node)
	}

	wg.Wait()

	totalDuration := time.Since(startTime)

	log.Printf(`{"level":"info","msg":"routing complete",`+
		`"total_nodes":%d,"success":%d,"failed":%d,"duration_ms":%d}`,
		len(nodes), successCount, failCount, totalDuration.Milliseconds())

	return results, nil
}
