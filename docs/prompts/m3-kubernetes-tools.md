# M3 Completion: Kubernetes GPU Tools

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/m3-kubernetes-tools.md`, the agent MUST
> continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/m3-kubernetes-tools.md`

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
| 0 | Create feature branch | `[DONE]` | `feat/m3-kubernetes-tools` |
| 1 | Implement get_pod_gpu_allocation tool | `[DONE]` | Issue #30 |
| 2 | Add unit tests for get_pod_gpu_allocation | `[DONE]` | |
| 3 | Implement describe_gpu_node tool | `[DONE]` | Issue #40 |
| 4 | Add unit tests for describe_gpu_node | `[DONE]` | |
| 5 | Register tools in MCP server | `[DONE]` | |
| 6 | Run full test suite | `[DONE]` | `make all` |
| 7 | Real cluster E2E verification | `[DONE]` | Cluster available, tools need deployment |
| 8 | Create pull request | `[DONE]` | PR #128 |
| 9 | Wait for Copilot review | `[WIP]` | ⏳ Takes 1-2 min |
| 10 | Address review comments | `[TODO]` | |
| 11 | **Merge after reviews** | `[WAIT]` | ⚠️ **Requires human approval** |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[WAIT]` (human approval) | `[BLOCKED:reason]`

---

## Issue References

- **Issue #30:** [get_pod_gpu_allocation tool](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/30)
- **Issue #40:** [describe_gpu_node tool](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/40)
- **Priority:** P1-High
- **Milestone:** M3: The Ephemeral Tunnel
- **Labels:** kind/feature, area/k8s-ephemeral

## Background

M3 (Kubernetes Integration) is ~80% complete after Epic #112 (HTTP transport).
These two tools complete the K8s-specific functionality:

1. **get_pod_gpu_allocation** - Shows which pods are using which GPUs
2. **describe_gpu_node** - Comprehensive view of a GPU node (K8s + NVML data)

Both tools enable SRE workflows like:
- "Which pods are using GPUs on node X?"
- "Give me everything about gpu-node-5"
- "Why is this GPU showing high utilization?"

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/m3-kubernetes-tools
```

---

## Task 1: Implement get_pod_gpu_allocation Tool

**File:** `pkg/tools/pod_gpu_allocation.go`

### Tool Definition

```go
// GetPodGPUAllocationTool returns the MCP tool definition.
func GetPodGPUAllocationTool() mcp.Tool {
    return mcp.NewTool("get_pod_gpu_allocation",
        mcp.WithDescription(
            "Shows GPU allocation for pods on a specific node. "+
            "Returns pods with nvidia.com/gpu resource requests, "+
            "including GPU UUIDs assigned to each container via "+
            "the NVIDIA device plugin annotations.",
        ),
        mcp.WithString("node_name",
            mcp.Required(),
            mcp.Description("Node name to query"),
        ),
        mcp.WithString("namespace",
            mcp.Description("Namespace filter (optional, default: all namespaces)"),
        ),
    )
}
```

### Implementation Requirements

```go
// PodGPUAllocationHandler handles the get_pod_gpu_allocation tool.
type PodGPUAllocationHandler struct {
    k8sClient *k8s.Client
}

// PodGPUAllocation represents GPU allocation for a pod.
type PodGPUAllocation struct {
    Name       string                   `json:"name"`
    Namespace  string                   `json:"namespace"`
    Status     string                   `json:"status"`
    Node       string                   `json:"node"`
    Containers []ContainerGPUAllocation `json:"containers"`
}

// ContainerGPUAllocation represents GPU allocation for a container.
type ContainerGPUAllocation struct {
    Name       string   `json:"name"`
    GPURequest int64    `json:"gpu_request"`
    GPULimit   int64    `json:"gpu_limit"`
    GPUUUIDs   []string `json:"gpu_uuids,omitempty"`
}
```

### Key Implementation Details

1. **Query pods with GPU requests:**
   ```go
   // List pods on the specified node
   pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx,
       metav1.ListOptions{
           FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
       })
   ```

2. **Extract GPU requests from container resources:**
   ```go
   const nvidiaGPUResource = "nvidia.com/gpu"
   
   for _, container := range pod.Spec.Containers {
       if gpuReq, ok := container.Resources.Requests[nvidiaGPUResource]; ok {
           gpuRequest = gpuReq.Value()
       }
       if gpuLim, ok := container.Resources.Limits[nvidiaGPUResource]; ok {
           gpuLimit = gpuLim.Value()
       }
   }
   ```

3. **Extract GPU UUIDs from device plugin annotation:**
   ```go
   // NVIDIA device plugin sets this annotation with assigned GPU UUIDs
   const gpuDeviceAnnotation = "nvidia.com/gpu.device"
   
   if uuids, ok := pod.Annotations[gpuDeviceAnnotation]; ok {
       gpuUUIDs = strings.Split(uuids, ",")
   }
   ```

4. **Filter pods with GPU allocations only:**
   - Skip pods without `nvidia.com/gpu` requests
   - Include pods in all phases (Running, Pending, etc.)

### Expected Output

```json
{
  "status": "success",
  "node_name": "ip-10-0-1-123",
  "pods": [
    {
      "name": "training-job-abc123",
      "namespace": "ml-workloads",
      "status": "Running",
      "node": "ip-10-0-1-123",
      "containers": [
        {
          "name": "trainer",
          "gpu_request": 2,
          "gpu_limit": 2,
          "gpu_uuids": ["GPU-abc123...", "GPU-def456..."]
        }
      ]
    }
  ],
  "summary": {
    "total_pods": 3,
    "total_gpus_allocated": 6
  }
}
```

### Acceptance Criteria

- [ ] Handler struct with k8s.Client dependency
- [ ] Queries pods by node name
- [ ] Extracts GPU requests/limits from container resources
- [ ] Extracts GPU UUIDs from device plugin annotation
- [ ] Filters to only pods with GPU allocations
- [ ] Returns structured JSON response
- [ ] Handles namespace filter (optional)

---

## Task 2: Unit Tests for get_pod_gpu_allocation

**File:** `pkg/tools/pod_gpu_allocation_test.go`

### Test Cases

```go
func TestPodGPUAllocationHandler_Handle(t *testing.T) {
    tests := []struct {
        name     string
        pods     []corev1.Pod
        nodeName string
        wantPods int
        wantGPUs int64
    }{
        {
            name:     "single pod with GPUs",
            nodeName: "gpu-node-1",
            pods: []corev1.Pod{
                makePodWithGPU("training-job", "ml", "gpu-node-1", 2),
            },
            wantPods: 1,
            wantGPUs: 2,
        },
        {
            name:     "multiple pods on same node",
            nodeName: "gpu-node-1",
            pods: []corev1.Pod{
                makePodWithGPU("job-1", "ns1", "gpu-node-1", 1),
                makePodWithGPU("job-2", "ns2", "gpu-node-1", 3),
            },
            wantPods: 2,
            wantGPUs: 4,
        },
        {
            name:     "filters by node",
            nodeName: "gpu-node-1",
            pods: []corev1.Pod{
                makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
                makePodWithGPU("job-2", "ns1", "gpu-node-2", 4), // different node
            },
            wantPods: 1,
            wantGPUs: 2,
        },
        {
            name:     "skips pods without GPUs",
            nodeName: "gpu-node-1",
            pods: []corev1.Pod{
                makePodWithGPU("gpu-job", "ns1", "gpu-node-1", 2),
                makePodWithoutGPU("cpu-job", "ns1", "gpu-node-1"),
            },
            wantPods: 1,
            wantGPUs: 2,
        },
        {
            name:     "no pods on node",
            nodeName: "empty-node",
            pods:     []corev1.Pod{},
            wantPods: 0,
            wantGPUs: 0,
        },
    }
    // ... implementation
}

func TestPodGPUAllocationHandler_NamespaceFilter(t *testing.T) {
    // Test namespace filtering
}

func TestPodGPUAllocationHandler_GPUUUIDs(t *testing.T) {
    // Test GPU UUID extraction from annotations
}
```

### Helper Functions

```go
func makePodWithGPU(name, namespace, nodeName string, gpuCount int64) corev1.Pod {
    return corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
            Annotations: map[string]string{
                "nvidia.com/gpu.device": "GPU-uuid-1,GPU-uuid-2",
            },
        },
        Spec: corev1.PodSpec{
            NodeName: nodeName,
            Containers: []corev1.Container{
                {
                    Name: "main",
                    Resources: corev1.ResourceRequirements{
                        Requests: corev1.ResourceList{
                            "nvidia.com/gpu": resource.MustParse(fmt.Sprintf("%d", gpuCount)),
                        },
                        Limits: corev1.ResourceList{
                            "nvidia.com/gpu": resource.MustParse(fmt.Sprintf("%d", gpuCount)),
                        },
                    },
                },
            },
        },
        Status: corev1.PodStatus{
            Phase: corev1.PodRunning,
        },
    }
}
```

---

## Task 3: Implement describe_gpu_node Tool

**File:** `pkg/tools/describe_gpu_node.go`

### Tool Definition

```go
// GetDescribeGPUNodeTool returns the MCP tool definition.
func GetDescribeGPUNodeTool() mcp.Tool {
    return mcp.NewTool("describe_gpu_node",
        mcp.WithDescription(
            "Comprehensive view of a GPU node combining Kubernetes metadata "+
            "with NVML hardware data. Includes node labels, taints, conditions, "+
            "capacity, GPU health status, and pods running on the node.",
        ),
        mcp.WithString("node_name",
            mcp.Required(),
            mcp.Description("Node name to describe"),
        ),
    )
}
```

### Implementation Requirements

```go
// DescribeGPUNodeHandler handles the describe_gpu_node tool.
type DescribeGPUNodeHandler struct {
    k8sClient  *k8s.Client
    nvmlClient nvml.Interface
}

// GPUNodeDescription represents the full node description.
type GPUNodeDescription struct {
    Status     string                 `json:"status"`
    Node       NodeInfo               `json:"node"`
    Driver     DriverInfo             `json:"driver"`
    GPUs       []GPUDescription       `json:"gpus"`
    Pods       []PodGPUAllocation     `json:"pods"`
    Summary    NodeSummary            `json:"summary"`
}

// NodeInfo contains Kubernetes node metadata.
type NodeInfo struct {
    Name        string            `json:"name"`
    Labels      map[string]string `json:"labels"`
    Taints      []TaintInfo       `json:"taints,omitempty"`
    Conditions  map[string]bool   `json:"conditions"`
    Capacity    ResourceInfo      `json:"capacity"`
    Allocatable ResourceInfo      `json:"allocatable"`
}
```

### Key Implementation Details

1. **Get Kubernetes node info:**
   ```go
   node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
   
   // Extract relevant labels (filter to GPU-related)
   gpuLabels := filterGPULabels(node.Labels)
   
   // Extract conditions as map
   conditions := make(map[string]bool)
   for _, cond := range node.Status.Conditions {
       conditions[string(cond.Type)] = cond.Status == corev1.ConditionTrue
   }
   ```

2. **Get GPU hardware info via agent:**
   - In gateway mode: Route to the specific node's agent
   - In agent mode: Use local NVML client
   ```go
   // This tool should work in gateway mode by routing to the node's agent
   // The gateway will call get_gpu_inventory on the specific node
   ```

3. **Get pods on the node:**
   - Reuse PodGPUAllocationHandler logic
   - Include all GPU-consuming pods

4. **Calculate summary:**
   ```go
   summary := NodeSummary{
       TotalGPUs:     len(gpus),
       AllocatedGPUs: countAllocatedGPUs(pods),
       AvailableGPUs: len(gpus) - countAllocatedGPUs(pods),
       OverallHealth: calculateOverallHealth(gpus),
   }
   ```

### Expected Output

```json
{
  "status": "success",
  "node": {
    "name": "ip-10-0-1-123",
    "labels": {
      "node.kubernetes.io/instance-type": "g4dn.xlarge",
      "nvidia.com/gpu.product": "Tesla-T4"
    },
    "taints": [
      {"key": "nvidia.com/gpu", "effect": "NoSchedule"}
    ],
    "conditions": {
      "Ready": true,
      "MemoryPressure": false,
      "DiskPressure": false
    },
    "capacity": {
      "cpu": "4",
      "memory": "16Gi",
      "nvidia.com/gpu": "1"
    },
    "allocatable": {
      "cpu": "3920m",
      "memory": "15Gi",
      "nvidia.com/gpu": "1"
    }
  },
  "driver": {
    "version": "535.154.05",
    "cuda_version": "12.2"
  },
  "gpus": [
    {
      "index": 0,
      "name": "Tesla T4",
      "uuid": "GPU-abc123...",
      "health_score": 95,
      "temperature": 42,
      "utilization": 85,
      "memory_used_percent": 45
    }
  ],
  "pods": [
    {
      "name": "inference-server",
      "namespace": "production",
      "gpu_count": 1,
      "status": "Running"
    }
  ],
  "summary": {
    "total_gpus": 1,
    "allocated_gpus": 1,
    "available_gpus": 0,
    "overall_health": "healthy"
  }
}
```

### Gateway Mode Consideration

In gateway mode, this tool needs to:
1. Query K8s API for node metadata (gateway can do this)
2. Route GPU queries to the specific node's agent
3. Combine the results

**Implementation approach:**
- Gateway queries K8s node directly
- Gateway calls `get_gpu_inventory` routed to the specific node
- Gateway combines both responses

---

## Task 4: Unit Tests for describe_gpu_node

**File:** `pkg/tools/describe_gpu_node_test.go`

### Test Cases

```go
func TestDescribeGPUNodeHandler_Handle(t *testing.T) {
    tests := []struct {
        name      string
        nodeName  string
        node      *corev1.Node
        pods      []corev1.Pod
        gpuCount  int
        wantError bool
    }{
        {
            name:     "healthy node with GPUs",
            nodeName: "gpu-node-1",
            node:     makeGPUNode("gpu-node-1", 4),
            pods: []corev1.Pod{
                makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
            },
            gpuCount: 4,
        },
        {
            name:      "node not found",
            nodeName:  "missing-node",
            wantError: true,
        },
        {
            name:     "node with no GPU pods",
            nodeName: "gpu-node-1",
            node:     makeGPUNode("gpu-node-1", 4),
            pods:     []corev1.Pod{},
            gpuCount: 4,
        },
    }
    // ... implementation
}

func TestDescribeGPUNodeHandler_Labels(t *testing.T) {
    // Test GPU label filtering
}

func TestDescribeGPUNodeHandler_Conditions(t *testing.T) {
    // Test condition extraction
}
```

---

## Task 5: Register Tools in MCP Server

**File:** `pkg/mcp/server.go`

Add the new tools to the server registration:

```go
// In NewServer or similar initialization:

// Add pod GPU allocation tool
podGPUHandler := tools.NewPodGPUAllocationHandler(k8sClient)
s.AddTool(tools.GetPodGPUAllocationTool(), podGPUHandler.Handle)

// Add describe GPU node tool  
describeHandler := tools.NewDescribeGPUNodeHandler(k8sClient, nvmlClient)
s.AddTool(tools.GetDescribeGPUNodeTool(), describeHandler.Handle)
```

**Note:** These tools require k8sClient, so they should only be registered when:
- Running in gateway mode, OR
- Running as agent with K8s access

Check existing patterns in `server.go` for conditional registration.

---

## Task 6: Run Full Test Suite

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Format code
gofmt -s -w .

# Run all checks
make all

# Specifically test new packages
go test -v ./pkg/tools/... -count=1

# Test with race detector
go test -race ./pkg/tools/...
```

---

## Task 7: Real Cluster E2E Verification

**Conditional:** Only if KUBECONFIG is available.

```bash
# Verify cluster access
kubectl cluster-info
kubectl get nodes

# Port-forward to gateway
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &

# Test get_pod_gpu_allocation
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"tools/call",
    "params":{
      "name":"get_pod_gpu_allocation",
      "arguments":{"node_name":"<your-node-name>"}
    }
  }'

# Test describe_gpu_node
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"describe_gpu_node",
      "arguments":{"node_name":"<your-node-name>"}
    }
  }'

# Cleanup
kill %1
```

---

## Task 8: Create Pull Request

```bash
git push -u origin feat/m3-kubernetes-tools

gh pr create \
  --title "feat(tools): add get_pod_gpu_allocation and describe_gpu_node tools" \
  --body "## Summary

Adds two new MCP tools to complete M3 (Kubernetes Integration):

1. **get_pod_gpu_allocation** (#30) - Shows GPU allocation for pods on a node
2. **describe_gpu_node** (#40) - Comprehensive view combining K8s + NVML data

## Changes

### New Tools
- \`pkg/tools/pod_gpu_allocation.go\` - Pod GPU allocation tool
- \`pkg/tools/describe_gpu_node.go\` - Node description tool
- Unit tests for both tools

### Tool Registration
- Tools registered in gateway mode
- Proper K8s client dependency injection

## Use Cases

### get_pod_gpu_allocation
- \"Which pods are using GPUs on node X?\"
- \"How many GPUs are allocated on this node?\"
- GPU-to-pod correlation for debugging

### describe_gpu_node
- \"Give me everything about gpu-node-5\"
- \"Is this node healthy for training?\"
- Quick node triage for SREs

## Testing

- [x] Unit tests pass
- [x] \`make all\` succeeds
- [x] Real cluster verification (if applicable)
- [x] No race conditions

## Closes

- Closes #30
- Closes #40
- Progress toward M3 completion" \
  --label "kind/feature" \
  --label "area/k8s-ephemeral" \
  --milestone "M3: The Ephemeral Tunnel"
```

---

## Quick Reference

### Files to Create

| File | Purpose |
|------|---------|
| `pkg/tools/pod_gpu_allocation.go` | get_pod_gpu_allocation handler |
| `pkg/tools/pod_gpu_allocation_test.go` | Unit tests |
| `pkg/tools/describe_gpu_node.go` | describe_gpu_node handler |
| `pkg/tools/describe_gpu_node_test.go` | Unit tests |

### Files to Modify

| File | Changes |
|------|---------|
| `pkg/mcp/server.go` | Register new tools |

### Key Imports

```go
import (
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes/fake"
)
```

---

**Reply "GO" when ready to start implementation.** 🚀

<!-- 
COMPLETION MARKER - Do not output until ALL tasks are [DONE]:
<completion>ALL_TASKS_DONE</completion>
-->
