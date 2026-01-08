# Gateway Tool Proxy: Expose ALL GPU Tools via Gateway

## Issue Reference

- **Issue:** [#98 - Gateway mode should expose ALL GPU tools by proxying to node agents](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/98)
- **Priority:** P0-Blocker
- **Labels:** kind/feature, area/gateway, area/mcp-protocol
- **Milestone:** M3: The Ephemeral Tunnel

## Background

The gateway currently only exposes `list_gpu_nodes` tool. Users cannot access the core
GPU tools (`get_gpu_inventory`, `get_gpu_health`, `analyze_xid_errors`) without
connecting directly to node agents.

**Current architecture (broken UX):**
```
User → Gateway → list_gpu_nodes only
User → Agent (direct) → get_gpu_inventory, get_gpu_health, analyze_xid_errors
```

**Desired architecture:**
```
User → Gateway → ALL tools (proxied to node agents automatically)
```

### Existing Infrastructure

The codebase already has most pieces:

1. **Router** (`pkg/gateway/router.go`):
   - `RouteToNode()` - Send request to specific node
   - `RouteToAllNodes()` - Fan-out to all nodes, aggregate results
   - Uses `k8sClient.ExecInPod()` to run stdio agent

2. **K8s Client** (`pkg/k8s/client.go`):
   - `ListGPUNodes()` - Find all agent pods
   - `ExecInPod()` - Execute MCP request via kubectl exec

3. **Tool Definitions** (`pkg/tools/*.go`):
   - `GetGPUInventoryTool()` - Tool schema
   - `GetGPUHealthTool()` - Tool schema
   - `GetAnalyzeXIDTool()` - Tool schema

**What's missing:** Proxy handlers that register tools in gateway mode and route to agents.

---

## Objective

Register ALL GPU tools in gateway mode with proxy handlers that forward requests to
node agents via the existing Router infrastructure.

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gateway-tool-proxy
```

---

## Implementation Tasks

### Task 1: Create Proxy Tool Handler

Create a generic proxy handler that forwards any tool call to node agents.

**File to create:** `pkg/gateway/proxy.go`

```go
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
            fmt.Sprintf("failed to route to nodes: %s", err)), nil
    }

    // Aggregate results
    aggregated := p.aggregateResults(results)

    jsonBytes, err := json.MarshalIndent(aggregated, "", "  ")
    if err != nil {
        return mcp.NewToolResultError(
            fmt.Sprintf("failed to marshal response: %s", err)), nil
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

    initBytes, _ := json.Marshal(initReq)
    toolBytes, _ := json.Marshal(toolReq)

    // Concatenate with newline (stdio protocol)
    return append(append(initBytes, '\n'), toolBytes...)
}

// aggregateResults combines results from multiple nodes.
func (p *ProxyHandler) aggregateResults(results []NodeResult) interface{} {
    // For tools that return per-node data, aggregate into a cluster view
    aggregated := map[string]interface{}{
        "status":      "success",
        "node_count":  len(results),
        "nodes":       []interface{}{},
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
            return map[string]interface{}{"error": mcpResponse.Result.Content[0].Text}
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
func splitJSONLines(data []byte) [][]byte {
    var lines [][]byte
    var current []byte
    depth := 0

    for _, b := range data {
        current = append(current, b)
        if b == '{' {
            depth++
        } else if b == '}' {
            depth--
            if depth == 0 {
                lines = append(lines, current)
                current = nil
            }
        }
    }

    return lines
}
```

**Acceptance criteria:**
- [ ] `ProxyHandler` forwards tool calls to agents
- [ ] `buildMCPRequest` creates valid MCP JSON-RPC
- [ ] `aggregateResults` combines multi-node responses
- [ ] `parseToolResponse` extracts tool results from MCP responses

---

### Task 2: Register Proxy Tools in Gateway Mode

Modify `pkg/mcp/server.go` to register GPU tools with proxy handlers in gateway mode.

**File to modify:** `pkg/mcp/server.go`

**Changes:**

```go
// In the imports, add:
import (
    // ... existing imports ...
    "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/gateway"
)

// Replace the gateway mode section (lines 120-128) with:
if cfg.GatewayMode {
    // Gateway mode: register all tools with proxy handlers
    
    // list_gpu_nodes - handled directly by gateway (no proxy needed)
    listNodesHandler := tools.NewListGPUNodesHandler(cfg.K8sClient)
    mcpServer.AddTool(tools.GetListGPUNodesTool(), listNodesHandler.Handle)

    // GPU tools - proxied to node agents
    inventoryProxy := gateway.NewProxyHandler(cfg.K8sClient, "get_gpu_inventory")
    mcpServer.AddTool(tools.GetGPUInventoryTool(), inventoryProxy.Handle)

    healthProxy := gateway.NewProxyHandler(cfg.K8sClient, "get_gpu_health")
    mcpServer.AddTool(tools.GetGPUHealthTool(), healthProxy.Handle)

    xidProxy := gateway.NewProxyHandler(cfg.K8sClient, "analyze_xid_errors")
    mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidProxy.Handle)

    log.Printf(`{"level":"info","msg":"MCP server initialized",`+
        `"mode":"%s","gateway":true,"namespace":"%s",`+
        `"tools":["list_gpu_nodes","get_gpu_inventory","get_gpu_health","analyze_xid_errors"],`+
        `"version":"%s","commit":"%s"}`,
        cfg.Mode, cfg.Namespace, cfg.Version, cfg.GitCommit)
}
```

**Acceptance criteria:**
- [ ] Gateway registers `get_gpu_inventory` with proxy handler
- [ ] Gateway registers `get_gpu_health` with proxy handler
- [ ] Gateway registers `analyze_xid_errors` with proxy handler
- [ ] `list_gpu_nodes` still uses direct handler (no proxy needed)

---

### Task 3: Add Proxy Handler Tests

**File to create:** `pkg/gateway/proxy_test.go`

```go
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
}

func TestParseToolResponse(t *testing.T) {
    tests := []struct {
        name     string
        response string
        wantErr  bool
    }{
        {
            name: "valid response",
            response: `{"jsonrpc":"2.0","id":0,"result":{}}
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"status\":\"healthy\"}"}]}}`,
            wantErr: false,
        },
        {
            name: "error response",
            response: `{"jsonrpc":"2.0","id":0,"result":{}}
{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"tool failed"}}`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := parseToolResponse([]byte(tt.response))
            resultMap, ok := result.(map[string]interface{})
            if ok && resultMap["error"] != nil && !tt.wantErr {
                t.Errorf("unexpected error: %v", resultMap["error"])
            }
        })
    }
}

func TestSplitJSONLines(t *testing.T) {
    input := `{"a":1}{"b":2}{"c":{"nested":3}}`
    lines := splitJSONLines([]byte(input))

    if len(lines) != 3 {
        t.Errorf("expected 3 lines, got %d", len(lines))
    }
}

func TestAggregateResults(t *testing.T) {
    handler := &ProxyHandler{toolName: "test"}

    results := []NodeResult{
        {
            NodeName: "node-1",
            PodName:  "pod-1",
            Response: []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"gpus\":1}"}]}}`),
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
}
```

**Acceptance criteria:**
- [ ] `TestBuildMCPRequest` validates request format
- [ ] `TestParseToolResponse` handles success and error cases
- [ ] `TestSplitJSONLines` correctly splits JSON objects
- [ ] `TestAggregateResults` combines multi-node results

---

### Task 4: Fix Label Selector to Exclude Gateway Pod

While implementing, also fix issue #96 - exclude gateway pod from node listing.

**File to modify:** `pkg/k8s/client.go`

**Change in `ListGPUNodes` function:**

```go
// List pods with the GPU agent label, excluding gateway
pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx,
    metav1.ListOptions{
        LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server,app.kubernetes.io/component!=gateway",
    })
```

**Acceptance criteria:**
- [ ] Gateway pod excluded from `ListGPUNodes` results
- [ ] Only DaemonSet agent pods returned

---

## Testing Requirements

### Local Testing (Mock Mode)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Run specific tests
go test -v ./pkg/gateway/...

# Build
make agent
```

### Integration Testing (Real Cluster)

Deploy gateway and test all tools:

```bash
# Deploy with gateway
helm upgrade gpu-mcp ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --set gateway.enabled=true \
  --set image.tag=<your-tag>

# Port forward
kubectl port-forward -n gpu-diagnostics \
  svc/gpu-mcp-k8s-gpu-mcp-server-gateway 8080:8080

# Test (from curl pod or local)
curl -si -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize",...}'

# Get session ID from Mcp-Session-Id header, then:
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}'
```

**Expected response:**
```json
{
  "status": "success",
  "node_count": 1,
  "success_count": 1,
  "error_count": 0,
  "nodes": [
    {
      "node_name": "ip-10-0-0-153",
      "pod_name": "gpu-mcp-k8s-gpu-mcp-server-xxxxx",
      "data": {
        "device_count": 1,
        "devices": [{"name": "Tesla T4", ...}]
      }
    }
  ]
}
```

---

## Pre-Commit Checklist

```bash
make fmt
make lint
make test
make all
```

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass
- [ ] `go test ./... -race` - No race conditions

---

## Commit and Push

```bash
git add -A
git commit -s -S -m "feat(gateway): proxy all GPU tools to node agents

- Add ProxyHandler for forwarding tool calls to agents
- Register get_gpu_inventory, get_gpu_health, analyze_xid_errors in gateway
- Aggregate multi-node responses with success/error counts
- Fix label selector to exclude gateway pod from node listing

Fixes #98
Fixes #96"

git push -u origin feat/gateway-tool-proxy
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(gateway): proxy all GPU tools to node agents" \
  --body "Fixes #98
Fixes #96

## Summary
Gateway now exposes ALL GPU tools by proxying requests to node agents.
Users can access \`get_gpu_inventory\`, \`get_gpu_health\`, and \`analyze_xid_errors\`
through the gateway without connecting directly to agents.

## Changes
- Add \`pkg/gateway/proxy.go\` with \`ProxyHandler\`
- Register all GPU tools in gateway mode
- Aggregate multi-node responses
- Fix label selector to exclude gateway pod (#96)

## Testing
- [ ] Unit tests pass
- [ ] Integration tests on real cluster
- [ ] All tools accessible via gateway

## Response Format
\`\`\`json
{
  \"status\": \"success\",
  \"node_count\": 2,
  \"success_count\": 2,
  \"error_count\": 0,
  \"nodes\": [
    {\"node_name\": \"node-1\", \"data\": {...}},
    {\"node_name\": \"node-2\", \"data\": {...}}
  ]
}
\`\`\`" \
  --label "kind/feature" \
  --label "area/gateway" \
  --label "P0-Blocker"
```

---

## Related Issues

- **#96** - list_gpu_nodes includes gateway pod (fixed in Task 4)
- **#99** - Consolidate list_gpu_nodes into get_gpu_inventory (future work)

---

## Quick Reference

```
1. Create branch ──────────► feat/gateway-tool-proxy
2. Create pkg/gateway/proxy.go
3. Modify pkg/mcp/server.go (gateway mode)
4. Fix pkg/k8s/client.go (label selector)
5. Add tests ──────────────► pkg/gateway/proxy_test.go
6. make all ───────────────► Verify all checks pass
7. git commit -s -S ───────► Signed commit
8. gh pr create ───────────► PR with labels
9. Test on real cluster ───► Verify all tools work
```

---

**Reply "GO" when ready to start implementation.** 🚀

