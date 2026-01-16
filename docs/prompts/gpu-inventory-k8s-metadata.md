# Enhance get_gpu_inventory with K8s Node Metadata

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/gpu-inventory-k8s-metadata.md`, the agent MUST
> continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/gpu-inventory-k8s-metadata.md`

### Iteration Rules (For the Agent)

1. **NEVER STOP EARLY** - If any task is `[TODO]` or `[WIP]`, keep working
2. **UPDATE STATUS** - Edit this file: mark tasks `[WIP]` → `[DONE]` as you go
3. **COMMIT PROGRESS** - Commit and push after each completed task
4. **SELF-CHECK** - Before ending your turn, verify ALL tasks show `[DONE]`
5. **REPORT STATUS** - End each turn with a status summary of remaining tasks
6. **⚠️ MERGE REQUIRES HUMAN APPROVAL** - When ready to merge, STOP and ask for confirmation. Do NOT merge autonomously.

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[DONE]` | `feat/gpu-inventory-k8s-metadata` |
| 1 | Add K8s node metadata to proxy aggregation | `[DONE]` | Labels, conditions, capacity |
| 2 | Add `include_k8s_metadata` parameter | `[DONE]` | Optional, defaults to true in gateway |
| 3 | Calculate allocatable vs allocated counts | `[DONE]` | From K8s node status |
| 4 | Add unit tests for metadata enrichment | `[DONE]` | |
| 5 | Update gateway proxy tests | `[DONE]` | |
| 6 | Run full test suite | `[DONE]` | `make all` |
| 7 | Test in real cluster | `[DONE]` | No cluster available - skipped |
| 8 | Create pull request | `[DONE]` | [#134](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/pull/134) |
| 9 | Wait for Copilot review | `[WIP]` | ⏳ Takes 1-2 min |
| 10 | Address review comments | `[TODO]` | |
| 11 | **Merge after reviews** | `[WAIT]` | ⚠️ **Requires human approval** |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[WAIT]` (human approval) | `[BLOCKED:reason]`

---

## Issue Reference

- **Closes:** [#29 - [K8s] Implement list_gpu_nodes tool](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/29)
- **Related:** #99 (consolidated list_gpu_nodes into get_gpu_inventory)
- **Priority:** P1-High
- **Labels:** `kind/feature`, `area/k8s-ephemeral`, `area/tools`
- **Milestone:** M3: The Ephemeral Tunnel

## Background

### History

Issue #29 originally requested a separate `list_gpu_nodes` tool. Issue #99 consolidated
this into `get_gpu_inventory` to reduce tool count and simplify the API. However, the
consolidation focused on GPU hardware data and **did not include K8s node metadata**.

### Current State

**What `get_gpu_inventory` returns (gateway mode):**
```json
{
  "status": "success",
  "cluster_summary": {
    "total_nodes": 2,
    "total_gpus": 4,
    "gpu_types": ["Tesla T4", "A100"]
  },
  "nodes": [
    {
      "node_name": "ip-10-0-0-153",
      "pod_name": "gpu-mcp-agent-abc123",
      "status": "ready",
      "gpus": [
        {
          "index": 0,
          "name": "Tesla T4",
          "uuid": "GPU-xxx",
          "memory_total": 16106127360,
          "temperature": 29,
          "utilization": 0
        }
      ]
    }
  ]
}
```

### What's Missing (from #29 requirements)

- **Node labels** - GPU type, topology zone, instance type
- **Node conditions** - Ready, MemoryPressure, DiskPressure, etc.
- **GPU capacity vs allocated** - allocatable, capacity, allocated counts

### Target State

```json
{
  "status": "success",
  "cluster_summary": {
    "total_nodes": 2,
    "total_gpus": 4,
    "gpus_allocatable": 4,
    "gpus_allocated": 2,
    "gpus_available": 2,
    "gpu_types": ["Tesla T4", "A100"]
  },
  "nodes": [
    {
      "node_name": "ip-10-0-0-153",
      "pod_name": "gpu-mcp-agent-abc123",
      "status": "ready",
      "kubernetes": {
        "labels": {
          "nvidia.com/gpu.product": "Tesla-T4",
          "topology.kubernetes.io/zone": "us-west-2a",
          "node.kubernetes.io/instance-type": "g4dn.xlarge"
        },
        "conditions": {
          "Ready": true,
          "MemoryPressure": false,
          "DiskPressure": false,
          "PIDPressure": false
        },
        "gpu_resources": {
          "capacity": 1,
          "allocatable": 1,
          "allocated": 1
        }
      },
      "gpus": [
        {
          "index": 0,
          "name": "Tesla T4",
          "uuid": "GPU-xxx",
          "memory_total": 16106127360,
          "temperature": 29,
          "utilization": 0
        }
      ]
    }
  ]
}
```

---

## Objective

Enhance `get_gpu_inventory` in gateway mode to include K8s node metadata (labels,
conditions, GPU capacity/allocation), completing the original #29 requirements.

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gpu-inventory-k8s-metadata
```

---

## Task 1: Add K8s Node Metadata to Proxy Aggregation

**File:** `pkg/gateway/proxy.go`

Modify `aggregateGPUInventory` to fetch and include K8s node metadata:

```go
// NodeK8sMetadata contains Kubernetes node information.
type NodeK8sMetadata struct {
    Labels       map[string]string `json:"labels,omitempty"`
    Conditions   map[string]bool   `json:"conditions,omitempty"`
    GPUResources *GPUResourceInfo  `json:"gpu_resources,omitempty"`
}

// GPUResourceInfo contains GPU resource capacity and allocation.
type GPUResourceInfo struct {
    Capacity    int64 `json:"capacity"`
    Allocatable int64 `json:"allocatable"`
    Allocated   int64 `json:"allocated"`
}

// aggregateGPUInventory creates a cluster-wide GPU inventory with summary.
func (p *ProxyHandler) aggregateGPUInventory(
    ctx context.Context,
    results []NodeResult,
) interface{} {
    // ... existing aggregation logic ...
    
    // Enrich with K8s metadata if client available
    if p.router.k8sClient != nil {
        for i := range nodeResults {
            nodeName := nodeResults[i]["node_name"].(string)
            metadata, err := p.getNodeK8sMetadata(ctx, nodeName)
            if err != nil {
                klog.V(4).InfoS("failed to get K8s metadata",
                    "node", nodeName, "error", err)
                continue
            }
            nodeResults[i]["kubernetes"] = metadata
        }
    }
    
    // ... rest of aggregation ...
}

// getNodeK8sMetadata fetches K8s node information.
func (p *ProxyHandler) getNodeK8sMetadata(
    ctx context.Context,
    nodeName string,
) (*NodeK8sMetadata, error) {
    node, err := p.router.k8sClient.GetNode(ctx, nodeName)
    if err != nil {
        return nil, err
    }
    
    // Filter to GPU-relevant labels
    labels := filterGPULabels(node.Labels)
    
    // Extract conditions as bool map
    conditions := make(map[string]bool)
    for _, cond := range node.Status.Conditions {
        conditions[string(cond.Type)] = cond.Status == corev1.ConditionTrue
    }
    
    // Get GPU resource info
    gpuResources := &GPUResourceInfo{}
    if qty, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
        gpuResources.Capacity = qty.Value()
    }
    if qty, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
        gpuResources.Allocatable = qty.Value()
    }
    // Allocated = Capacity - Allocatable (simplified)
    // More accurate: sum GPU requests from pods on this node
    gpuResources.Allocated = gpuResources.Capacity - gpuResources.Allocatable
    
    return &NodeK8sMetadata{
        Labels:       labels,
        Conditions:   conditions,
        GPUResources: gpuResources,
    }, nil
}

// filterGPULabels returns labels relevant to GPU operations.
func filterGPULabels(labels map[string]string) map[string]string {
    relevantPrefixes := []string{
        "nvidia.com/",
        "topology.kubernetes.io/",
        "node.kubernetes.io/instance-type",
        "kubernetes.io/arch",
        "kubernetes.io/os",
        "gpu-type",
        "accelerator",
    }
    
    filtered := make(map[string]string)
    for k, v := range labels {
        for _, prefix := range relevantPrefixes {
            if strings.HasPrefix(k, prefix) || k == prefix {
                filtered[k] = v
                break
            }
        }
    }
    return filtered
}
```

### Acceptance Criteria
- [ ] Node labels included (filtered to GPU-relevant)
- [ ] Node conditions included as bool map
- [ ] GPU capacity/allocatable from node status
- [ ] Graceful degradation if K8s client unavailable

---

## Task 2: Add `include_k8s_metadata` Parameter

**File:** `pkg/tools/gpu_inventory.go`

Add optional parameter to control metadata inclusion:

```go
// GetGPUInventoryTool returns the MCP tool definition.
func GetGPUInventoryTool() mcp.Tool {
    return mcp.NewTool("get_gpu_inventory",
        mcp.WithDescription(
            "Returns complete GPU hardware inventory. In gateway mode, "+
            "aggregates from all nodes with optional K8s metadata.",
        ),
        mcp.WithBoolean("include_k8s_metadata",
            mcp.Description(
                "Include Kubernetes node metadata (labels, conditions, "+
                "GPU capacity). Default: true in gateway mode, ignored in agent mode.",
            ),
        ),
    )
}
```

**File:** `pkg/gateway/proxy.go`

Honor the parameter in aggregation:

```go
func (p *ProxyHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // Extract include_k8s_metadata parameter
    includeK8s := true // Default for gateway mode
    if args := request.GetArguments(); args != nil {
        if v, ok := args["include_k8s_metadata"].(bool); ok {
            includeK8s = v
        }
    }
    
    // ... route to nodes ...
    
    // Aggregate with optional K8s metadata
    aggregated := p.aggregateResults(ctx, results, includeK8s)
    // ...
}
```

### Acceptance Criteria
- [ ] `include_k8s_metadata` parameter in tool schema
- [ ] Defaults to `true` in gateway mode
- [ ] Can be disabled via `{"include_k8s_metadata": false}`
- [ ] Parameter ignored in agent mode (no K8s client)

---

## Task 3: Calculate Accurate GPU Allocation

**File:** `pkg/gateway/proxy.go`

For accurate allocation counts, query pods on each node:

```go
// getNodeGPUAllocation returns the number of GPUs allocated on a node.
func (p *ProxyHandler) getNodeGPUAllocation(
    ctx context.Context,
    nodeName string,
) (int64, error) {
    // List pods on this node
    pods, err := p.router.k8sClient.ListPods(ctx, "",
        "", // all namespaces
        fmt.Sprintf("spec.nodeName=%s", nodeName))
    if err != nil {
        return 0, err
    }
    
    var totalAllocated int64
    for _, pod := range pods {
        // Skip completed/failed pods
        if pod.Status.Phase == corev1.PodSucceeded ||
            pod.Status.Phase == corev1.PodFailed {
            continue
        }
        
        for _, container := range pod.Spec.Containers {
            if req, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
                totalAllocated += req.Value()
            }
        }
    }
    
    return totalAllocated, nil
}
```

Update `getNodeK8sMetadata` to use accurate allocation:

```go
func (p *ProxyHandler) getNodeK8sMetadata(
    ctx context.Context,
    nodeName string,
) (*NodeK8sMetadata, error) {
    // ... existing code ...
    
    // Get accurate GPU allocation from pods
    allocated, err := p.getNodeGPUAllocation(ctx, nodeName)
    if err != nil {
        klog.V(4).InfoS("failed to get GPU allocation",
            "node", nodeName, "error", err)
        // Fall back to capacity - allocatable
        allocated = gpuResources.Capacity - gpuResources.Allocatable
    }
    gpuResources.Allocated = allocated
    
    // ...
}
```

### Acceptance Criteria
- [ ] Allocated count from actual pod GPU requests
- [ ] Handles pods in all namespaces
- [ ] Skips completed/failed pods
- [ ] Falls back to simple calculation on error

---

## Task 4: Add Unit Tests for Metadata Enrichment

**File:** `pkg/gateway/proxy_test.go`

```go
func TestAggregateGPUInventory_WithK8sMetadata(t *testing.T) {
    // Create fake K8s client with node
    node := &corev1.Node{
        ObjectMeta: metav1.ObjectMeta{
            Name: "gpu-node-1",
            Labels: map[string]string{
                "nvidia.com/gpu.product":           "Tesla-T4",
                "topology.kubernetes.io/zone":     "us-west-2a",
                "node.kubernetes.io/instance-type": "g4dn.xlarge",
                "unrelated-label":                 "should-be-filtered",
            },
        },
        Status: corev1.NodeStatus{
            Conditions: []corev1.NodeCondition{
                {Type: corev1.NodeReady, Status: corev1.ConditionTrue},
                {Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
            },
            Capacity: corev1.ResourceList{
                "nvidia.com/gpu": resource.MustParse("4"),
            },
            Allocatable: corev1.ResourceList{
                "nvidia.com/gpu": resource.MustParse("4"),
            },
        },
    }
    
    clientset := fake.NewSimpleClientset(node)
    k8sClient := k8s.NewClientWithConfig(clientset, nil, "default")
    
    // Create proxy handler
    handler := NewProxyHandler(k8sClient, "get_gpu_inventory")
    
    // Mock node results
    results := []NodeResult{
        {
            NodeName: "gpu-node-1",
            PodName:  "agent-pod-1",
            Response: `{"status":"success","devices":[{"name":"Tesla T4"}]}`,
        },
    }
    
    // Aggregate with K8s metadata
    aggregated := handler.aggregateGPUInventory(
        context.Background(), results, true)
    
    // Verify K8s metadata present
    agg := aggregated.(map[string]interface{})
    nodes := agg["nodes"].([]interface{})
    require.Len(t, nodes, 1)
    
    nodeData := nodes[0].(map[string]interface{})
    k8sMeta := nodeData["kubernetes"].(map[string]interface{})
    
    // Check labels filtered correctly
    labels := k8sMeta["labels"].(map[string]string)
    assert.Contains(t, labels, "nvidia.com/gpu.product")
    assert.NotContains(t, labels, "unrelated-label")
    
    // Check conditions
    conditions := k8sMeta["conditions"].(map[string]bool)
    assert.True(t, conditions["Ready"])
    assert.False(t, conditions["MemoryPressure"])
    
    // Check GPU resources
    gpuRes := k8sMeta["gpu_resources"].(map[string]interface{})
    assert.Equal(t, int64(4), gpuRes["capacity"])
}

func TestAggregateGPUInventory_WithoutK8sMetadata(t *testing.T) {
    // Test with include_k8s_metadata=false
    // ...
}

func TestFilterGPULabels(t *testing.T) {
    tests := []struct {
        name   string
        input  map[string]string
        want   []string
        notWant []string
    }{
        {
            name: "filters to GPU-relevant labels",
            input: map[string]string{
                "nvidia.com/gpu.product":           "Tesla-T4",
                "nvidia.com/gpu.memory":            "16GB",
                "topology.kubernetes.io/zone":     "us-west-2a",
                "node.kubernetes.io/instance-type": "g4dn.xlarge",
                "kubernetes.io/arch":              "amd64",
                "app.kubernetes.io/name":          "my-app",
                "random-label":                    "value",
            },
            want:    []string{"nvidia.com/gpu.product", "topology.kubernetes.io/zone"},
            notWant: []string{"app.kubernetes.io/name", "random-label"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := filterGPULabels(tt.input)
            for _, k := range tt.want {
                assert.Contains(t, result, k)
            }
            for _, k := range tt.notWant {
                assert.NotContains(t, result, k)
            }
        })
    }
}

func TestGetNodeGPUAllocation(t *testing.T) {
    // Create fake client with pods requesting GPUs
    pod1 := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "training-job-1",
            Namespace: "ml-workloads",
        },
        Spec: corev1.PodSpec{
            NodeName: "gpu-node-1",
            Containers: []corev1.Container{
                {
                    Name: "trainer",
                    Resources: corev1.ResourceRequirements{
                        Requests: corev1.ResourceList{
                            "nvidia.com/gpu": resource.MustParse("2"),
                        },
                    },
                },
            },
        },
        Status: corev1.PodStatus{
            Phase: corev1.PodRunning,
        },
    }
    
    clientset := fake.NewSimpleClientset(pod1)
    k8sClient := k8s.NewClientWithConfig(clientset, nil, "default")
    
    handler := NewProxyHandler(k8sClient, "get_gpu_inventory")
    
    allocated, err := handler.getNodeGPUAllocation(
        context.Background(), "gpu-node-1")
    
    require.NoError(t, err)
    assert.Equal(t, int64(2), allocated)
}
```

### Acceptance Criteria
- [ ] Test K8s metadata inclusion
- [ ] Test K8s metadata exclusion
- [ ] Test label filtering
- [ ] Test GPU allocation calculation
- [ ] All tests pass with race detector

---

## Task 5: Update Gateway Proxy Tests

**File:** `pkg/gateway/proxy_test.go`

Update existing tests to handle new metadata fields:

```go
func TestProxyHandler_GetGPUInventory_Integration(t *testing.T) {
    // Ensure existing tests still pass with new fields
    // ...
}
```

### Acceptance Criteria
- [ ] Existing proxy tests still pass
- [ ] New fields don't break backward compatibility
- [ ] Response structure validated

---

## Task 6: Run Full Test Suite

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Format code
gofmt -s -w .

# Run all checks
make all

# Test gateway package specifically
go test -v ./pkg/gateway/... -count=1

# Test with race detector
go test -race ./pkg/gateway/...
```

### Acceptance Criteria
- [ ] `gofmt` produces no changes
- [ ] `go vet` passes
- [ ] `golangci-lint run` passes
- [ ] All tests pass
- [ ] No race conditions detected

---

## Task 7: Test in Real Cluster

**Conditional:** Only if KUBECONFIG is available.

```bash
# Verify cluster access
kubectl cluster-info
kubectl get nodes -l nvidia.com/gpu.present=true

# Port-forward to gateway
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &

# Test get_gpu_inventory with K8s metadata
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"tools/call",
    "params":{
      "name":"get_gpu_inventory",
      "arguments":{}
    }
  }' | jq '.result.content[0].text | fromjson'

# Verify K8s metadata present
# Should see: nodes[].kubernetes.labels, conditions, gpu_resources

# Test without K8s metadata
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"get_gpu_inventory",
      "arguments":{"include_k8s_metadata": false}
    }
  }' | jq '.result.content[0].text | fromjson'

# Should NOT have kubernetes field

# Cleanup
kill %1
```

### Acceptance Criteria
- [ ] K8s metadata appears in response
- [ ] Labels filtered to GPU-relevant
- [ ] Conditions show Ready status
- [ ] GPU resources show capacity/allocatable/allocated
- [ ] `include_k8s_metadata=false` omits kubernetes field

---

## Task 8: Create Pull Request

```bash
git push -u origin feat/gpu-inventory-k8s-metadata

gh pr create \
  --title "feat(tools): add K8s node metadata to get_gpu_inventory" \
  --body "## Summary

Enhances \`get_gpu_inventory\` in gateway mode to include Kubernetes node
metadata, completing the functionality originally requested in #29.

## Changes

### Gateway Aggregation (\`pkg/gateway/proxy.go\`)
- Add \`NodeK8sMetadata\` struct with labels, conditions, GPU resources
- Add \`getNodeK8sMetadata()\` to fetch K8s node info
- Add \`getNodeGPUAllocation()\` for accurate allocation counts
- Add \`filterGPULabels()\` to filter GPU-relevant labels
- Honor \`include_k8s_metadata\` parameter (default: true)

### Tool Schema (\`pkg/tools/gpu_inventory.go\`)
- Add \`include_k8s_metadata\` optional parameter

### Tests
- Unit tests for metadata enrichment
- Label filtering tests
- GPU allocation calculation tests

## Example Output

\`\`\`json
{
  \"cluster_summary\": {
    \"total_nodes\": 2,
    \"gpus_allocatable\": 4,
    \"gpus_allocated\": 2
  },
  \"nodes\": [
    {
      \"node_name\": \"gpu-node-1\",
      \"kubernetes\": {
        \"labels\": {
          \"nvidia.com/gpu.product\": \"Tesla-T4\"
        },
        \"conditions\": {
          \"Ready\": true
        },
        \"gpu_resources\": {
          \"capacity\": 4,
          \"allocatable\": 4,
          \"allocated\": 2
        }
      },
      \"gpus\": [...]
    }
  ]
}
\`\`\`

## Testing

- [x] Unit tests pass
- [x] \`make all\` succeeds
- [x] Real cluster verification (if applicable)
- [x] Race detector clean

## Closes

Closes #29 - Completes K8s node metadata functionality via enhancement to
existing tool (per #99 consolidation decision)." \
  --label "kind/feature" \
  --label "area/k8s-ephemeral" \
  --label "area/tools" \
  --milestone "M3: The Ephemeral Tunnel"
```

---

## Quick Reference

### Files to Modify

| File | Changes |
|------|---------|
| `pkg/gateway/proxy.go` | Add K8s metadata enrichment, allocation calc |
| `pkg/gateway/proxy_test.go` | Tests for new functionality |
| `pkg/tools/gpu_inventory.go` | Add `include_k8s_metadata` parameter |

### New Types

```go
type NodeK8sMetadata struct {
    Labels       map[string]string `json:"labels,omitempty"`
    Conditions   map[string]bool   `json:"conditions,omitempty"`
    GPUResources *GPUResourceInfo  `json:"gpu_resources,omitempty"`
}

type GPUResourceInfo struct {
    Capacity    int64 `json:"capacity"`
    Allocatable int64 `json:"allocatable"`
    Allocated   int64 `json:"allocated"`
}
```

### Relevant Labels to Filter

```go
relevantPrefixes := []string{
    "nvidia.com/",
    "topology.kubernetes.io/",
    "node.kubernetes.io/instance-type",
    "kubernetes.io/arch",
    "kubernetes.io/os",
    "gpu-type",
    "accelerator",
}
```

---

**Reply "GO" when ready to start implementation.** 🚀

<!-- 
COMPLETION MARKER - Do not output until ALL tasks are [DONE]:
<completion>ALL_TASKS_DONE</completion>
-->
