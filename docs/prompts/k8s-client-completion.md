# K8s Client Interface Completion (Issue #28)

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/k8s-client-completion.md`, the agent MUST
> continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/k8s-client-completion.md`

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
| 0 | Create feature branch | `[DONE]` | `feat/k8s-client-interface` |
| 1 | Add `ListNodes` method | `[DONE]` | Query K8s nodes with GPU resources |
| 2 | Add `GetNode` method | `[DONE]` | Get single node by name |
| 3 | Add `ListPods` method | `[DONE]` | Generic pod listing with selectors |
| 4 | Add `GetPod` method | `[DONE]` | Get single pod by namespace/name |
| 5 | Add unit tests for new methods | `[DONE]` | Use fake clientset |
| 6 | Add `--kubeconfig` CLI flag | `[DONE]` | Skipped - KUBECONFIG env var already supported |
| 7 | Run full test suite | `[DONE]` | All tests pass with race detector |
| 8 | Create pull request | `[DONE]` | PR #133 |
| 9 | Wait for Copilot review | `[DONE]` | ✅ No comments - code approved |
| 10 | Address review comments | `[DONE]` | N/A - No comments to address |
| 11 | **Merge after reviews** | `[WAIT]` | ⚠️ **Requires human approval** |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[WAIT]` (human approval) | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** [#28 - [K8s] Implement Kubernetes client initialization](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/28)
- **Priority:** P1-High
- **Labels:** `kind/feature`, `area/k8s-ephemeral`
- **Milestone:** M3: The Ephemeral Tunnel
- **Blocks:** #29, #31, #37, #38, #39

## Background

### Current State Analysis

The `pkg/k8s/client.go` already exists with substantial implementation:

**Already Implemented ✅:**
- In-cluster config detection (ServiceAccount)
- Out-of-cluster config (`~/.kube/config` fallback, `KUBECONFIG` env var)
- `NewClient(namespace)` constructor with options pattern
- `NewClientWithConfig()` for testing
- `ListGPUNodes()` - lists GPU agent pods (DaemonSet-specific)
- `GetPodForNode()` - gets GPU agent pod on a specific node
- `Namespace()` - returns configured namespace
- `Clientset()` - exposes raw K8s clientset
- Unit tests with fake clientset

**Missing from Original Issue Spec ❌:**

The issue proposed this interface:
```go
type Client interface {
    GetCurrentNamespace() string
    ListNodes(ctx context.Context, labelSelector string) ([]v1.Node, error)
    GetNode(ctx context.Context, name string) (*v1.Node, error)
    ListPods(ctx context.Context, namespace, labelSelector string) ([]v1.Pod, error)
    GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error)
}
```

Current gaps:
1. `ListNodes()` - **NOT implemented** (needed by #29 `list_gpu_nodes`)
2. `GetNode()` - **NOT implemented** (needed by `describe_gpu_node`, #40)
3. `ListPods()` - **NOT directly** (users must use `Clientset()`)
4. `GetPod()` - **NOT directly** (users must use `Clientset()`)
5. `--kubeconfig` CLI flag - **NOT implemented** (uses env var only)

### Why This Matters

Issue #28 is marked as "foundational for all K8s-aware tools." Multiple M3 issues depend on it:

| Dependent Issue | Needs |
|-----------------|-------|
| #29 `list_gpu_nodes` | `ListNodes()` with label selector |
| #31 `correlate_gpu_workload` | `ListPods()`, `GetPod()` |
| #37 K8s context in NVML tools | `GetNode()` for metadata |
| #38 `get_node_events` | Node queries |
| #39 MIG discovery | Node metadata |

---

## Objective

Complete the K8s client interface by adding the missing `ListNodes`, `GetNode`,
`ListPods`, and `GetPod` methods. Optionally add a `--kubeconfig` CLI flag.

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/k8s-client-interface
```

---

## Task 1: Add `ListNodes` Method

**File:** `pkg/k8s/client.go`

Add method to list Kubernetes nodes with optional label selector:

```go
// ListNodes returns nodes matching the optional label selector.
// An empty labelSelector returns all nodes.
// Common GPU-related selectors:
//   - "nvidia.com/gpu.present=true"
//   - "node.kubernetes.io/instance-type=p4d.24xlarge"
func (c *Client) ListNodes(
    ctx context.Context,
    labelSelector string,
) ([]corev1.Node, error) {
    nodeList, err := c.clientset.CoreV1().Nodes().List(ctx,
        metav1.ListOptions{
            LabelSelector: labelSelector,
        })
    if err != nil {
        return nil, fmt.Errorf("failed to list nodes: %w", err)
    }
    return nodeList.Items, nil
}
```

### Acceptance Criteria
- [ ] Method returns `[]corev1.Node`
- [ ] Supports empty label selector (returns all nodes)
- [ ] Supports GPU-specific selectors
- [ ] Handles API errors gracefully

---

## Task 2: Add `GetNode` Method

**File:** `pkg/k8s/client.go`

Add method to get a single node by name:

```go
// GetNode returns a node by name.
// Returns an error if the node does not exist.
func (c *Client) GetNode(
    ctx context.Context,
    name string,
) (*corev1.Node, error) {
    node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get node %s: %w", name, err)
    }
    return node, nil
}
```

### Acceptance Criteria
- [ ] Method returns `*corev1.Node`
- [ ] Returns error for non-existent node
- [ ] Error message includes node name

---

## Task 3: Add `ListPods` Method

**File:** `pkg/k8s/client.go`

Add generic pod listing method:

```go
// ListPods returns pods in the specified namespace matching the selectors.
// If namespace is empty, uses the client's configured namespace.
// If labelSelector is empty, returns all pods in the namespace.
// If fieldSelector is empty, no field filtering is applied.
//
// Common selectors:
//   - labelSelector: "app.kubernetes.io/name=my-app"
//   - fieldSelector: "spec.nodeName=gpu-node-1"
func (c *Client) ListPods(
    ctx context.Context,
    namespace string,
    labelSelector string,
    fieldSelector string,
) ([]corev1.Pod, error) {
    if namespace == "" {
        namespace = c.namespace
    }

    podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx,
        metav1.ListOptions{
            LabelSelector: labelSelector,
            FieldSelector: fieldSelector,
        })
    if err != nil {
        return nil, fmt.Errorf("failed to list pods in %s: %w", namespace, err)
    }
    return podList.Items, nil
}
```

### Acceptance Criteria
- [ ] Method returns `[]corev1.Pod`
- [ ] Defaults to client's namespace when empty
- [ ] Supports both label and field selectors
- [ ] Can filter pods by node via fieldSelector

---

## Task 4: Add `GetPod` Method

**File:** `pkg/k8s/client.go`

Add method to get a single pod:

```go
// GetPod returns a pod by namespace and name.
// If namespace is empty, uses the client's configured namespace.
func (c *Client) GetPod(
    ctx context.Context,
    namespace string,
    name string,
) (*corev1.Pod, error) {
    if namespace == "" {
        namespace = c.namespace
    }

    pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name,
        metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get pod %s/%s: %w",
            namespace, name, err)
    }
    return pod, nil
}
```

### Acceptance Criteria
- [ ] Method returns `*corev1.Pod`
- [ ] Defaults to client's namespace when empty
- [ ] Error includes full namespace/name path

---

## Task 5: Add Unit Tests for New Methods

**File:** `pkg/k8s/client_test.go`

Add comprehensive tests for all new methods:

```go
func TestListNodes(t *testing.T) {
    tests := []struct {
        name          string
        nodes         []corev1.Node
        labelSelector string
        wantCount     int
        wantErr       bool
    }{
        {
            name:          "no nodes",
            nodes:         []corev1.Node{},
            labelSelector: "",
            wantCount:     0,
        },
        {
            name: "all nodes no selector",
            nodes: []corev1.Node{
                makeNode("node-1", nil),
                makeNode("node-2", nil),
            },
            labelSelector: "",
            wantCount:     2,
        },
        {
            name: "filter by GPU label",
            nodes: []corev1.Node{
                makeNode("gpu-node-1", map[string]string{
                    "nvidia.com/gpu.present": "true",
                }),
                makeNode("cpu-node-1", nil),
            },
            labelSelector: "nvidia.com/gpu.present=true",
            wantCount:     1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            clientset := fake.NewSimpleClientset()
            for _, node := range tt.nodes {
                _, err := clientset.CoreV1().Nodes().Create(
                    context.Background(), &node, metav1.CreateOptions{})
                require.NoError(t, err)
            }

            client := NewClientWithConfig(clientset, nil, "default")
            nodes, err := client.ListNodes(context.Background(), tt.labelSelector)

            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Len(t, nodes, tt.wantCount)
        })
    }
}

func TestGetNode(t *testing.T) {
    tests := []struct {
        name     string
        nodes    []corev1.Node
        nodeName string
        wantErr  bool
    }{
        {
            name:     "node not found",
            nodes:    []corev1.Node{},
            nodeName: "missing",
            wantErr:  true,
        },
        {
            name: "node found",
            nodes: []corev1.Node{
                makeNode("target-node", map[string]string{
                    "kubernetes.io/hostname": "target-node",
                }),
            },
            nodeName: "target-node",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            clientset := fake.NewSimpleClientset()
            for _, node := range tt.nodes {
                _, err := clientset.CoreV1().Nodes().Create(
                    context.Background(), &node, metav1.CreateOptions{})
                require.NoError(t, err)
            }

            client := NewClientWithConfig(clientset, nil, "default")
            node, err := client.GetNode(context.Background(), tt.nodeName)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.nodeName)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.nodeName, node.Name)
        })
    }
}

func TestListPods(t *testing.T) {
    tests := []struct {
        name          string
        pods          []corev1.Pod
        namespace     string
        labelSelector string
        fieldSelector string
        wantCount     int
    }{
        {
            name:      "empty namespace uses default",
            pods:      []corev1.Pod{makePod("pod-1", "test-ns", "node-1")},
            namespace: "",  // Should use client's namespace
            wantCount: 1,
        },
        {
            name: "filter by label",
            pods: []corev1.Pod{
                makePodWithLabels("gpu-pod", "test-ns", "node-1",
                    map[string]string{"gpu": "true"}),
                makePod("cpu-pod", "test-ns", "node-1"),
            },
            namespace:     "test-ns",
            labelSelector: "gpu=true",
            wantCount:     1,
        },
    }
    // ... implementation
}

func TestGetPod(t *testing.T) {
    // Similar pattern to TestGetNode
}

// Helper functions
func makeNode(name string, labels map[string]string) corev1.Node {
    return corev1.Node{
        ObjectMeta: metav1.ObjectMeta{
            Name:   name,
            Labels: labels,
        },
        Status: corev1.NodeStatus{
            Conditions: []corev1.NodeCondition{
                {
                    Type:   corev1.NodeReady,
                    Status: corev1.ConditionTrue,
                },
            },
        },
    }
}

func makePod(name, namespace, nodeName string) corev1.Pod {
    return corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
        Spec: corev1.PodSpec{
            NodeName: nodeName,
        },
    }
}

func makePodWithLabels(name, namespace, nodeName string,
    labels map[string]string) corev1.Pod {
    pod := makePod(name, namespace, nodeName)
    pod.Labels = labels
    return pod
}
```

### Acceptance Criteria
- [ ] All new methods have test coverage
- [ ] Tests cover success and error cases
- [ ] Tests verify label/field selector behavior
- [ ] Helper functions reduce test boilerplate
- [ ] `go test -race ./pkg/k8s/...` passes

---

## Task 6: Add `--kubeconfig` CLI Flag (Optional)

**File:** `cmd/agent/main.go`

Add explicit kubeconfig flag for better UX:

```go
// In flag definitions section
kubeconfig = flag.String("kubeconfig", "",
    "Path to kubeconfig file (optional, defaults to $KUBECONFIG or ~/.kube/config)")
```

Update K8s client initialization to use the flag:

```go
// In gateway/client initialization
if *kubeconfig != "" {
    os.Setenv("KUBECONFIG", *kubeconfig)
}
k8sClient, err := k8s.NewClient(*namespace)
```

**Alternative:** Document that `KUBECONFIG` environment variable is the preferred
method and close this task as "won't fix" with documentation update.

### Acceptance Criteria (if implementing)
- [ ] `--kubeconfig` flag added
- [ ] Flag takes precedence over env var
- [ ] Help text documents default behavior
- [ ] Works in both agent and gateway mode

### Alternative: Documentation Update
- [ ] Update README.md with KUBECONFIG usage
- [ ] Update `--help` output to mention KUBECONFIG
- [ ] Add example in docs/quickstart.md

---

## Task 7: Run Full Test Suite

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Format code
gofmt -s -w .

# Run all checks
make all

# Specifically test K8s package
go test -v ./pkg/k8s/... -count=1

# Test with race detector
go test -race ./pkg/k8s/...
```

### Acceptance Criteria
- [ ] `gofmt` produces no changes
- [ ] `go vet` passes
- [ ] `golangci-lint run` passes
- [ ] All tests pass
- [ ] No race conditions detected

---

## Task 8: Create Pull Request

```bash
git push -u origin feat/k8s-client-interface

gh pr create \
  --title "feat(k8s): complete client interface with node/pod methods" \
  --body "## Summary

Completes the K8s client interface by adding the missing methods specified in #28.

## Changes

### New Methods in \`pkg/k8s/client.go\`
- \`ListNodes(ctx, labelSelector)\` - List nodes with optional label filter
- \`GetNode(ctx, name)\` - Get single node by name
- \`ListPods(ctx, namespace, labelSelector, fieldSelector)\` - Generic pod listing
- \`GetPod(ctx, namespace, name)\` - Get single pod

### Tests
- Comprehensive unit tests for all new methods
- Helper functions for test fixtures

## Motivation

Issue #28 is foundational - these methods are needed by:
- #29 \`list_gpu_nodes\` - needs \`ListNodes()\`
- #31 \`correlate_gpu_workload\` - needs \`ListPods()\`, \`GetPod()\`
- #37 K8s context enhancement - needs \`GetNode()\`
- #38 \`get_node_events\` - needs node queries

## Testing

- [x] Unit tests pass
- [x] \`make all\` succeeds
- [x] Race detector clean

## Closes

Closes #28" \
  --label "kind/feature" \
  --label "area/k8s-ephemeral" \
  --milestone "M3: The Ephemeral Tunnel"
```

---

## Quick Reference

### Files to Modify

| File | Changes |
|------|---------|
| `pkg/k8s/client.go` | Add `ListNodes`, `GetNode`, `ListPods`, `GetPod` |
| `pkg/k8s/client_test.go` | Add tests for new methods |
| `cmd/agent/main.go` | Optional: add `--kubeconfig` flag |

### Key Imports

```go
import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes/fake"
)
```

### Method Signatures Summary

```go
// New methods to add
func (c *Client) ListNodes(ctx context.Context, labelSelector string) ([]corev1.Node, error)
func (c *Client) GetNode(ctx context.Context, name string) (*corev1.Node, error)
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector, fieldSelector string) ([]corev1.Pod, error)
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
```

---

## GPU-Specific Extensions (Future)

After the base methods are complete, consider adding GPU-specific convenience methods:

```go
// ListGPUEnabledNodes returns nodes with nvidia.com/gpu resources.
func (c *Client) ListGPUEnabledNodes(ctx context.Context) ([]corev1.Node, error) {
    // Filter nodes that have nvidia.com/gpu in their capacity
    nodes, err := c.ListNodes(ctx, "")
    if err != nil {
        return nil, err
    }
    
    gpuNodes := make([]corev1.Node, 0)
    for _, node := range nodes {
        if _, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
            gpuNodes = append(gpuNodes, node)
        }
    }
    return gpuNodes, nil
}

// GetNodeGPUCount returns the number of GPUs on a node.
func (c *Client) GetNodeGPUCount(ctx context.Context, nodeName string) (int64, error) {
    node, err := c.GetNode(ctx, nodeName)
    if err != nil {
        return 0, err
    }
    
    if gpuQty, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
        return gpuQty.Value(), nil
    }
    return 0, nil
}
```

These are **out of scope** for #28 but could be added in follow-up work.

---

**Reply "GO" when ready to start implementation.** 🚀

<!-- 
COMPLETION MARKER - Do not output until ALL tasks are [DONE]:
<completion>ALL_TASKS_DONE</completion>
-->
