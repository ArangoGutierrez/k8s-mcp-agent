# Consolidate list_gpu_nodes into get_gpu_inventory

## Issue Reference

- **Issue:** [#99 - Consolidate list_gpu_nodes into get_gpu_inventory](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/99)
- **Priority:** P2-Medium
- **Labels:** enhancement, area/tools
- **Milestone:** M3: Kubernetes Integration

## Background

We currently have two overlapping tools:
- `list_gpu_nodes` - Returns K8s nodes with GPU agents (gateway only)
- `get_gpu_inventory` - Returns GPU hardware details

User feedback: "What is the diff between list_gpu_nodes and get_gpu_inventory?
I think we should simply have get_gpu_inventory that returns a list of nodes
with GPU, and the resource info."

**Benefits of consolidation:**
1. Fewer tools = less token overhead (each tool injected into prompts)
2. Better mental model - one tool for "what GPUs do I have"
3. Node context included - know which node has which GPU

---

## Objective

Consolidate `list_gpu_nodes` functionality into `get_gpu_inventory`, creating a
single tool that provides complete GPU visibility across the cluster.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/consolidate-gpu-inventory
```

---

## Current State Analysis

### Agent Mode (single node)
`get_gpu_inventory` returns:
```json
{
  "status": "success",
  "driver_version": "575.57.08",
  "cuda_version": "12.9",
  "device_count": 1,
  "devices": [{"name": "Tesla T4", "uuid": "...", ...}]
}
```

### Gateway Mode
`list_gpu_nodes` returns:
```json
{
  "status": "success",
  "node_count": 3,
  "ready_count": 3,
  "nodes": [{"name": "ip-10-0-0-153", "pod_name": "...", "ready": true}]
}
```

`get_gpu_inventory` (via proxy) returns:
```json
{
  "status": "success",
  "node_count": 3,
  "nodes": [
    {"node_name": "ip-10-0-0-153", "data": {"device_count": 1, "devices": [...]}}
  ]
}
```

---

## Desired State

### Agent Mode (unchanged)
Same as current - returns local GPU info.

### Gateway Mode (enhanced)
Single `get_gpu_inventory` returns cluster-wide view with summary:
```json
{
  "status": "success",
  "cluster_summary": {
    "total_nodes": 3,
    "ready_nodes": 3,
    "total_gpus": 3,
    "gpu_types": ["Tesla T4"]
  },
  "nodes": [
    {
      "name": "ip-10-0-0-153",
      "status": "ready",
      "driver_version": "575.57.08",
      "cuda_version": "12.9",
      "gpus": [
        {
          "index": 0,
          "name": "Tesla T4",
          "uuid": "GPU-xxx",
          "memory_total_gb": 15.1,
          "temperature_c": 26,
          "utilization_percent": 0
        }
      ]
    }
  ]
}
```

---

## Implementation Tasks

### Task 1: Enhance ProxyHandler Aggregation for GPU Inventory

Update the `aggregateResults` method to create a cluster summary when proxying
`get_gpu_inventory`.

**Files to modify:**
- `pkg/gateway/proxy.go` - Add cluster summary aggregation

**Code changes:**

```go
// aggregateResults combines results from multiple nodes.
func (p *ProxyHandler) aggregateResults(results []NodeResult) interface{} {
    // Special handling for get_gpu_inventory - create cluster summary
    if p.toolName == "get_gpu_inventory" {
        return p.aggregateGPUInventory(results)
    }
    
    // Default aggregation for other tools
    // ... existing code ...
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
    
    // Build GPU types list
    types := make([]string, 0, len(gpuTypes))
    for t := range gpuTypes {
        types = append(types, t)
    }
    
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
func flattenGPUInfo(dev map[string]interface{}) map[string]interface{} {
    gpu := map[string]interface{}{
        "index": dev["index"],
        "name":  dev["name"],
        "uuid":  dev["uuid"],
    }
    
    // Flatten memory
    if mem, ok := dev["memory"].(map[string]interface{}); ok {
        if total, ok := mem["total_bytes"].(float64); ok {
            gpu["memory_total_gb"] = total / (1024 * 1024 * 1024)
        }
    }
    
    // Flatten temperature
    if temp, ok := dev["temperature"].(map[string]interface{}); ok {
        if curr, ok := temp["current_celsius"].(float64); ok {
            gpu["temperature_c"] = int(curr)
        }
    }
    
    // Flatten utilization
    if util, ok := dev["utilization"].(map[string]interface{}); ok {
        if gpuPct, ok := util["gpu_percent"].(float64); ok {
            gpu["utilization_percent"] = int(gpuPct)
        }
    }
    
    return gpu
}
```

**Acceptance criteria:**
- [ ] Gateway returns cluster_summary with total_nodes, ready_nodes, total_gpus
- [ ] GPU types are collected across all nodes
- [ ] Node data includes status (ready/error)
- [ ] GPU info is flattened for readability

> 💡 **Commit after completing this task**

---

### Task 2: Remove list_gpu_nodes Tool Registration

Remove `list_gpu_nodes` from the gateway tool registration.

**Files to modify:**
- `pkg/mcp/server.go` - Remove list_gpu_nodes registration

**Current code to remove:**

```go
// In NewServer(), gateway mode section:
listNodesHandler := tools.NewListGPUNodesHandler(cfg.K8sClient)
mcpServer.AddTool(tools.GetListGPUNodesTool(), listNodesHandler.Handle)
```

**Acceptance criteria:**
- [ ] `list_gpu_nodes` no longer appears in tools list
- [ ] Gateway still registers `get_gpu_inventory`, `get_gpu_health`, `analyze_xid_errors`

> 💡 **Commit after completing this task**

---

### Task 3: Remove list_gpu_nodes Handler and Tool Definition

Delete the `list_gpu_nodes` tool files.

**Files to delete:**
- `pkg/tools/list_gpu_nodes.go`
- `pkg/tools/list_gpu_nodes_test.go`

**Commands:**

```bash
rm pkg/tools/list_gpu_nodes.go
rm pkg/tools/list_gpu_nodes_test.go
```

**Acceptance criteria:**
- [ ] Files deleted
- [ ] No compile errors
- [ ] No dead imports

> 💡 **Commit after completing this task**

---

### Task 4: Update get_gpu_inventory Tool Description

Update the tool description to reflect its new cluster-wide capability.

**Files to modify:**
- `pkg/tools/gpu_inventory.go` - Update `GetGPUInventoryTool()`

**New description:**

```go
func GetGPUInventoryTool() mcp.Tool {
    return mcp.NewTool("get_gpu_inventory",
        mcp.WithDescription(
            "Returns GPU inventory for all devices. "+
                "In agent mode: returns local GPU hardware details. "+
                "In gateway mode: returns cluster-wide inventory with "+
                "summary (total nodes, GPUs, types) and per-node GPU list. "+
                "Includes model, UUID, memory, temperature, and utilization.",
        ),
    )
}
```

**Acceptance criteria:**
- [ ] Description mentions both agent and gateway modes
- [ ] Description mentions cluster summary in gateway mode

> 💡 **Commit after completing this task**

---

### Task 5: Add Tests for Enhanced Aggregation

Add tests for the new cluster summary aggregation.

**Files to modify:**
- `pkg/gateway/proxy_test.go` - Add GPU inventory aggregation tests

**Test cases:**

```go
func TestAggregateGPUInventory_ClusterSummary(t *testing.T) {
    assert := assert.New(t)
    handler := NewProxyHandler(nil, "get_gpu_inventory")
    
    // Mock response from two nodes with different GPU types
    node1Response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"device_count\":1,\"devices\":[{\"name\":\"Tesla T4\",\"index\":0}]}"}]}}`
    node2Response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"device_count\":2,\"devices\":[{\"name\":\"A100\",\"index\":0},{\"name\":\"A100\",\"index\":1}]}"}]}}`
    
    results := []NodeResult{
        {NodeName: "node1", PodName: "pod1", Response: []byte(node1Response)},
        {NodeName: "node2", PodName: "pod2", Response: []byte(node2Response)},
    }
    
    aggregated := handler.aggregateResults(results).(map[string]interface{})
    
    // Check cluster summary
    summary := aggregated["cluster_summary"].(map[string]interface{})
    assert.Equal(2, summary["total_nodes"])
    assert.Equal(2, summary["ready_nodes"])
    assert.Equal(3, summary["total_gpus"])
    
    gpuTypes := summary["gpu_types"].([]string)
    assert.Contains(gpuTypes, "Tesla T4")
    assert.Contains(gpuTypes, "A100")
}
```

**Acceptance criteria:**
- [ ] Test cluster summary fields
- [ ] Test GPU type collection
- [ ] Test node error handling
- [ ] Test empty cluster case

> 💡 **Commit after completing this task**

---

### Task 6: Update Documentation

Update docs to reflect the consolidated tool.

**Files to modify:**
- `docs/mcp-usage.md` - Remove list_gpu_nodes references
- `README.md` - Update tool list if mentioned

**Acceptance criteria:**
- [ ] No references to list_gpu_nodes in docs
- [ ] get_gpu_inventory documented for both modes

> 💡 **Commit after completing this task**

---

## Testing Requirements

### Local Testing (Mock Mode)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Verify list_gpu_nodes is removed
./bin/agent --nvml-mode=mock < examples/initialize.json 2>/dev/null | \
  grep -c "list_gpu_nodes" && echo "❌ Still exists" || echo "✅ Removed"

# Test get_gpu_inventory still works
./bin/agent --nvml-mode=mock < examples/gpu_inventory.json
```

### Cluster Testing (if available)

```bash
# Deploy gateway and test cluster-wide inventory
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &

# Test get_gpu_inventory via gateway
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{...},"id":0}'
# Get session ID from response, then:
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory"},"id":1}'
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

---

## Commit and Push

### Commits (atomic)

```bash
git commit -s -S -m "feat(gateway): add cluster summary to gpu inventory aggregation"
git commit -s -S -m "chore(mcp): remove list_gpu_nodes tool registration"
git commit -s -S -m "chore(tools): delete list_gpu_nodes handler and tests"
git commit -s -S -m "docs(tools): update get_gpu_inventory description"
git commit -s -S -m "test(gateway): add gpu inventory aggregation tests"
git commit -s -S -m "docs: remove list_gpu_nodes references"
```

### Push

```bash
git push -u origin feat/consolidate-gpu-inventory
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(tools): consolidate list_gpu_nodes into get_gpu_inventory" \
  --body "Fixes #99

## Summary
Consolidates two overlapping tools into one, reducing token overhead and
improving UX.

## Changes
- Enhanced get_gpu_inventory in gateway mode with cluster_summary
- Removed list_gpu_nodes tool entirely
- Updated tool description for both modes
- Added aggregation tests

## Before (2 tools)
- list_gpu_nodes: returns nodes
- get_gpu_inventory: returns GPUs

## After (1 tool)
- get_gpu_inventory: returns cluster summary + nodes + GPUs

## Testing
- [x] make all passes
- [x] Unit tests for aggregation
- [x] list_gpu_nodes no longer in tool list" \
  --label "enhancement" \
  --label "area/tools"
```

---

## Quick Reference

**Estimated Time:** 1-2 hours

**Complexity:** Medium

**Files Changed:**
- `pkg/gateway/proxy.go` (modify)
- `pkg/gateway/proxy_test.go` (modify)
- `pkg/mcp/server.go` (modify)
- `pkg/tools/gpu_inventory.go` (modify)
- `pkg/tools/list_gpu_nodes.go` (delete)
- `pkg/tools/list_gpu_nodes_test.go` (delete)
- `docs/mcp-usage.md` (modify)

---

**Reply "GO" when ready to start implementation.** 🚀
