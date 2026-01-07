# Gateway Mode Implementation

## Issue Reference

- **Issue:** [#72 - feat: Gateway mode with node routing](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/72)
- **Priority:** P0-Blocker
- **Labels:** kind/feature, area/mcp-protocol, area/k8s-ephemeral
- **Milestone:** M3: The Ephemeral Tunnel
- **Depends on:** #71 (HTTP transport) ✅ Merged

## Background

Currently, querying GPU status requires connecting directly to a specific node's
pod via `kubectl exec`. Users must know which node has the GPU they want to
query. This is cumbersome for multi-node GPU clusters.

Gateway mode provides a single MCP entry point that:
- Routes queries to per-node DaemonSet agents
- Supports node-specific queries ("GPU on node-5")
- Supports aggregated queries ("all GPUs across all nodes")
- Uses the K8s pod exec API for routing

### Architecture

```
┌─────────────┐     HTTP/stdio     ┌─────────────────┐     pod exec API    ┌──────────────┐
│   Cursor    │ ─────────────────▶ │     Gateway     │ ─────────────────▶  │ Node Agent   │
│   Claude    │                    │   (Deployment)  │                     │ (DaemonSet)  │
└─────────────┘                    └─────────────────┘                     └──────────────┘
```

---

## Objective

Implement a Gateway mode that provides a single MCP entry point for querying
GPUs across all nodes in a Kubernetes cluster, with optional node targeting.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gateway-mode
```

Verify:
```bash
git branch --show-current
# Should output: feat/gateway-mode
```

---

## Implementation Tasks

### Task 1: Add Kubernetes Client Package

Create a K8s client wrapper for pod discovery and exec.

**Files to create:**
- `pkg/k8s/client.go`
- `pkg/k8s/client_test.go`

**Implementation:**

```go
// pkg/k8s/client.go
package k8s

import (
    "context"
    "fmt"
    "io"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/tools/remotecommand"
)

// Client wraps the Kubernetes clientset for GPU agent operations.
type Client struct {
    clientset  *kubernetes.Clientset
    restConfig *rest.Config
    namespace  string
}

// GPUNode represents a node with GPU agents.
type GPUNode struct {
    Name       string `json:"name"`
    PodName    string `json:"pod_name"`
    PodIP      string `json:"pod_ip"`
    Ready      bool   `json:"ready"`
}

// NewClient creates a new Kubernetes client.
// Uses in-cluster config if available, falls back to kubeconfig.
func NewClient(namespace string) (*Client, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        // Fall back to kubeconfig
        config, err = clientcmd.BuildConfigFromFlags("", 
            clientcmd.RecommendedHomeFile)
        if err != nil {
            return nil, fmt.Errorf("failed to get k8s config: %w", err)
        }
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create clientset: %w", err)
    }
    
    return &Client{
        clientset:  clientset,
        restConfig: config,
        namespace:  namespace,
    }, nil
}

// ListGPUNodes returns all nodes running the GPU agent DaemonSet.
func (c *Client) ListGPUNodes(ctx context.Context) ([]GPUNode, error) {
    // List pods with the GPU agent label
    pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, 
        metav1.ListOptions{
            LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server",
        })
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }
    
    nodes := make([]GPUNode, 0, len(pods.Items))
    for _, pod := range pods.Items {
        ready := false
        for _, cond := range pod.Status.Conditions {
            if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
                ready = true
                break
            }
        }
        
        nodes = append(nodes, GPUNode{
            Name:    pod.Spec.NodeName,
            PodName: pod.Name,
            PodIP:   pod.Status.PodIP,
            Ready:   ready,
        })
    }
    
    return nodes, nil
}

// ExecInPod executes a command in a pod and returns the output.
func (c *Client) ExecInPod(ctx context.Context, podName, container string, 
    stdin io.Reader, stdout, stderr io.Writer) error {
    
    req := c.clientset.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(podName).
        Namespace(c.namespace).
        SubResource("exec").
        VersionedParams(&corev1.PodExecOptions{
            Container: container,
            Command:   []string{"/agent", "--nvml-mode=real"},
            Stdin:     stdin != nil,
            Stdout:    true,
            Stderr:    true,
        }, scheme.ParameterCodec)
    
    exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
    if err != nil {
        return fmt.Errorf("failed to create executor: %w", err)
    }
    
    return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
        Stdin:  stdin,
        Stdout: stdout,
        Stderr: stderr,
    })
}

// GetPodForNode returns the GPU agent pod running on a specific node.
func (c *Client) GetPodForNode(ctx context.Context, nodeName string) (*GPUNode, error) {
    nodes, err := c.ListGPUNodes(ctx)
    if err != nil {
        return nil, err
    }
    
    for _, node := range nodes {
        if node.Name == nodeName {
            return &node, nil
        }
    }
    
    return nil, fmt.Errorf("no GPU agent found on node %s", nodeName)
}
```

**Acceptance criteria:**
- [ ] K8s client with in-cluster and kubeconfig support
- [ ] `ListGPUNodes()` returns all nodes with GPU agents
- [ ] `ExecInPod()` executes commands in agent pods
- [ ] `GetPodForNode()` finds agent pod for specific node

---

### Task 2: Add `list_gpu_nodes` Tool

Create a new tool to list all GPU nodes in the cluster.

**Files to create:**
- `pkg/tools/list_gpu_nodes.go`
- `pkg/tools/list_gpu_nodes_test.go`

**Implementation:**

```go
// pkg/tools/list_gpu_nodes.go
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
    
    response := map[string]interface{}{
        "status":     "success",
        "node_count": len(nodes),
        "nodes":      nodes,
    }
    
    jsonBytes, err := json.MarshalIndent(response, "", "  ")
    if err != nil {
        return mcp.NewToolResultError(
            fmt.Sprintf("failed to marshal response: %s", err)), nil
    }
    
    log.Printf(`{"level":"info","msg":"list_gpu_nodes completed",`+
        `"node_count":%d}`, len(nodes))
    
    return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetListGPUNodesTool returns the MCP tool definition.
func GetListGPUNodesTool() mcp.Tool {
    return mcp.NewTool("list_gpu_nodes",
        mcp.WithDescription(
            "Lists all Kubernetes nodes running the GPU MCP agent. "+
            "Returns node names, pod names, and readiness status. "+
            "Use this to discover which nodes have GPU agents before "+
            "querying specific nodes with other GPU tools.",
        ),
    )
}
```

**Acceptance criteria:**
- [ ] `list_gpu_nodes` tool registered
- [ ] Returns node name, pod name, IP, ready status
- [ ] Works in Gateway mode only
- [ ] Unit tests pass

---

### Task 3: Add `node` Parameter to GPU Tools

Modify existing tools to accept optional `node` parameter.

**Files to modify:**
- `pkg/tools/gpu_inventory.go`
- `pkg/tools/gpu_health.go`
- `pkg/tools/analyze_xid.go`

**Pattern for each tool:**

```go
// Update tool definition to include node parameter
func GetGPUInventoryTool() mcp.Tool {
    return mcp.NewTool("get_gpu_inventory",
        mcp.WithDescription(
            "Returns GPU hardware inventory. "+
            "In Gateway mode, specify 'node' parameter to query a "+
            "specific node, or omit to aggregate from all nodes.",
        ),
        mcp.WithString("node",
            mcp.Description("Target node name (optional in Gateway mode)"),
        ),
    )
}

// In handler, extract node parameter
func (h *GPUInventoryHandler) Handle(...) (*mcp.CallToolResult, error) {
    args, _ := request.Params.Arguments.(map[string]interface{})
    nodeName, _ := args["node"].(string)
    
    if h.gatewayMode && nodeName != "" {
        return h.handleGatewayRequest(ctx, nodeName, request)
    }
    
    // Existing local handling...
}
```

**Acceptance criteria:**
- [ ] All 3 GPU tools have optional `node` parameter
- [ ] Tools work in both local and gateway modes
- [ ] Node parameter documented in tool description

---

### Task 4: Implement Gateway Router

Create the gateway router that forwards requests to node agents.

**Files to create:**
- `pkg/gateway/router.go`
- `pkg/gateway/router_test.go`

**Implementation:**

```go
// pkg/gateway/router.go
package gateway

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "sync"
    
    "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
)

// Router forwards MCP requests to node agents.
type Router struct {
    k8sClient *k8s.Client
}

// NewRouter creates a new gateway router.
func NewRouter(k8sClient *k8s.Client) *Router {
    return &Router{k8sClient: k8sClient}
}

// RouteToNode sends an MCP request to a specific node's agent.
func (r *Router) RouteToNode(ctx context.Context, nodeName string, 
    mcpRequest []byte) ([]byte, error) {
    
    node, err := r.k8sClient.GetPodForNode(ctx, nodeName)
    if err != nil {
        return nil, fmt.Errorf("node not found: %w", err)
    }
    
    if !node.Ready {
        return nil, fmt.Errorf("agent on node %s is not ready", nodeName)
    }
    
    // Execute agent in pod with MCP request as stdin
    stdin := bytes.NewReader(mcpRequest)
    var stdout, stderr bytes.Buffer
    
    err = r.k8sClient.ExecInPod(ctx, node.PodName, "agent", 
        stdin, &stdout, &stderr)
    if err != nil {
        return nil, fmt.Errorf("exec failed: %w (stderr: %s)", 
            err, stderr.String())
    }
    
    return stdout.Bytes(), nil
}

// RouteToAllNodes sends an MCP request to all nodes and aggregates results.
func (r *Router) RouteToAllNodes(ctx context.Context, 
    mcpRequest []byte) ([]NodeResult, error) {
    
    nodes, err := r.k8sClient.ListGPUNodes(ctx)
    if err != nil {
        return nil, err
    }
    
    results := make([]NodeResult, 0, len(nodes))
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    for _, node := range nodes {
        if !node.Ready {
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
            } else {
                result.Response = response
            }
            results = append(results, result)
        }(node)
    }
    
    wg.Wait()
    return results, nil
}

// NodeResult holds the result from a single node.
type NodeResult struct {
    NodeName string          `json:"node_name"`
    PodName  string          `json:"pod_name"`
    Response json.RawMessage `json:"response,omitempty"`
    Error    string          `json:"error,omitempty"`
}
```

**Acceptance criteria:**
- [ ] Router can forward to single node
- [ ] Router can aggregate from all nodes
- [ ] Concurrent execution for multi-node queries
- [ ] Error handling per node

---

### Task 5: Add Gateway Mode to Server

Update server to support Gateway mode with K8s client.

**Files to modify:**
- `pkg/mcp/server.go`
- `cmd/agent/main.go`

**Config changes:**

```go
// Add to Config struct
type Config struct {
    // ... existing fields ...
    
    // GatewayMode enables routing to node agents
    GatewayMode bool
    // Namespace for GPU agent pods
    Namespace string
}
```

**CLI flag:**

```go
// In main.go
var (
    // ... existing flags ...
    gatewayMode = flag.Bool("gateway", false, 
        "Enable gateway mode (routes to node agents)")
    namespace = flag.String("namespace", "gpu-diagnostics",
        "Namespace for GPU agent pods (gateway mode)")
)
```

**Acceptance criteria:**
- [ ] `--gateway` flag enables gateway mode
- [ ] `--namespace` configures pod namespace
- [ ] K8s client initialized in gateway mode
- [ ] `list_gpu_nodes` tool only in gateway mode

---

### Task 6: Add Helm Templates for Gateway Mode

Create Gateway Deployment alongside DaemonSet.

**Files to create:**
- `deployment/helm/k8s-gpu-mcp-server/templates/gateway-deployment.yaml`
- `deployment/helm/k8s-gpu-mcp-server/templates/gateway-service.yaml`
- `deployment/helm/k8s-gpu-mcp-server/templates/gateway-rbac.yaml`

**Values additions:**

```yaml
# Add to values.yaml
gateway:
  # Enable gateway deployment
  enabled: false
  
  # Number of gateway replicas
  replicas: 1
  
  # Gateway HTTP port
  port: 8080
  
  # Resource limits
  resources:
    requests:
      cpu: 10m
      memory: 32Mi
    limits:
      cpu: 100m
      memory: 128Mi
```

**Gateway Deployment template:**

```yaml
{{- if .Values.gateway.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-gateway
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
  labels:
    {{- include "k8s-gpu-mcp-server.labels" . | nindent 4 }}
    app.kubernetes.io/component: gateway
spec:
  replicas: {{ .Values.gateway.replicas }}
  selector:
    matchLabels:
      {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 6 }}
      app.kubernetes.io/component: gateway
  template:
    metadata:
      labels:
        {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 8 }}
        app.kubernetes.io/component: gateway
    spec:
      serviceAccountName: {{ include "k8s-gpu-mcp-server.serviceAccountName" . }}-gateway
      containers:
      - name: gateway
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        command: ["/agent"]
        args:
        - "--gateway"
        - "--port={{ .Values.gateway.port }}"
        - "--namespace={{ include "k8s-gpu-mcp-server.namespace" . }}"
        ports:
        - name: http
          containerPort: {{ .Values.gateway.port }}
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
        readinessProbe:
          httpGet:
            path: /readyz
            port: http
        resources:
          {{- toYaml .Values.gateway.resources | nindent 10 }}
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
{{- end }}
```

**Gateway RBAC:**

```yaml
{{- if .Values.gateway.enabled }}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-gateway
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["list", "get"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-gateway
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-gateway
subjects:
- kind: ServiceAccount
  name: {{ include "k8s-gpu-mcp-server.serviceAccountName" . }}-gateway
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
{{- end }}
```

**Acceptance criteria:**
- [ ] Gateway Deployment template
- [ ] Gateway Service template
- [ ] RBAC for pod list/exec
- [ ] Gateway ServiceAccount
- [ ] Values.yaml gateway config

---

### Task 7: Add Dependencies to go.mod

Add Kubernetes client-go dependencies.

```bash
go get k8s.io/client-go@latest
go get k8s.io/api@latest
go get k8s.io/apimachinery@latest
```

**Acceptance criteria:**
- [ ] K8s client-go in go.mod
- [ ] `go mod tidy` clean
- [ ] Build succeeds

---

## Testing Requirements

### Local Testing (Mock Mode)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Test local mode (existing behavior)
./bin/agent --nvml-mode=mock

# Gateway mode requires K8s - use integration test
```

### Integration Testing

```bash
# Deploy DaemonSet
helm upgrade --install gpu-agent ./deployment/helm/k8s-gpu-mcp-server \
  --set gpu.runtimeClass.enabled=true

# Deploy Gateway
helm upgrade --install gpu-agent ./deployment/helm/k8s-gpu-mcp-server \
  --set gateway.enabled=true \
  --set gateway.port=8080

# Test list_gpu_nodes
kubectl port-forward svc/k8s-gpu-mcp-server-gateway 8080:8080 &
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call",
       "params":{"name":"list_gpu_nodes"}}'

# Test get_gpu_inventory on specific node
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call",
       "params":{"name":"get_gpu_inventory",
                 "arguments":{"node":"gpu-node-1"}}}'
```

---

## Pre-Commit Checklist

- [ ] `make fmt` - Code formatted
- [ ] `make lint` - Linter passes
- [ ] `make test` - All tests pass
- [ ] New tests for K8s client
- [ ] New tests for gateway router
- [ ] New tests for list_gpu_nodes tool
- [ ] Documentation updated

---

## Commit and Push

```bash
git add -A
git commit -s -S -m "feat(gateway): add gateway mode with node routing

- Add K8s client for pod discovery and exec
- Add list_gpu_nodes tool
- Add node parameter to GPU tools
- Implement gateway router for request forwarding
- Add Helm templates for Gateway Deployment
- Add RBAC for pod exec permissions

Fixes #72"

git push -u origin feat/gateway-mode
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(gateway): add gateway mode with node routing" \
  --body "Fixes #72

## Summary
Adds Gateway mode for multi-node GPU cluster management through a single
MCP entry point.

## Architecture
\`\`\`
Cursor/Claude ──▶ Gateway (HTTP) ──▶ Node Agents (DaemonSet)
                      │                    │
                      ▼                    ▼
               pod exec API         local NVML queries
\`\`\`

## Changes
- \`pkg/k8s/\` - Kubernetes client for pod discovery/exec
- \`pkg/gateway/\` - Request router to node agents
- \`list_gpu_nodes\` tool - Discover GPU nodes
- \`node\` parameter on all GPU tools
- Helm templates for Gateway Deployment
- RBAC for pod list/exec

## New Tools
| Tool | Description |
|------|-------------|
| \`list_gpu_nodes\` | Lists all nodes with GPU agents |

## Updated Tools
| Tool | New Parameter |
|------|---------------|
| \`get_gpu_inventory\` | \`node\` (optional) |
| \`get_gpu_health\` | \`node\` (optional) |
| \`analyze_xid_errors\` | \`node\` (optional) |

## Usage
\`\`\`bash
# Deploy with gateway
helm install gpu-agent ./deployment/helm/k8s-gpu-mcp-server \\
  --set gateway.enabled=true

# Query all nodes
{\"method\":\"tools/call\",\"params\":{\"name\":\"list_gpu_nodes\"}}

# Query specific node
{\"method\":\"tools/call\",\"params\":{
  \"name\":\"get_gpu_inventory\",
  \"arguments\":{\"node\":\"gpu-node-1\"}
}}
\`\`\`

## Testing
- [ ] Local tests pass (mock mode)
- [ ] Gateway mode starts
- [ ] list_gpu_nodes returns pods
- [ ] Node routing works
- [ ] Aggregation works" \
  --label "kind/feature" \
  --label "area/mcp-protocol" \
  --label "area/k8s-ephemeral" \
  --label "prio/p0-blocker" \
  --milestone "M3: The Ephemeral Tunnel"
```

---

## File Structure After Implementation

```
pkg/
├── k8s/
│   ├── client.go         # NEW: K8s client wrapper
│   └── client_test.go    # NEW: Client tests
├── gateway/
│   ├── router.go         # NEW: Request router
│   └── router_test.go    # NEW: Router tests
├── tools/
│   ├── gpu_inventory.go  # MODIFIED: add node param
│   ├── gpu_health.go     # MODIFIED: add node param
│   ├── analyze_xid.go    # MODIFIED: add node param
│   ├── list_gpu_nodes.go # NEW: list_gpu_nodes tool
│   └── ...
└── mcp/
    └── server.go         # MODIFIED: gateway mode support

deployment/helm/k8s-gpu-mcp-server/templates/
├── gateway-deployment.yaml  # NEW
├── gateway-service.yaml     # NEW
├── gateway-rbac.yaml        # NEW
└── gateway-serviceaccount.yaml  # NEW
```

---

## Acceptance Criteria

**Must Have:**
- [ ] `--gateway` flag enables gateway mode
- [ ] `list_gpu_nodes` tool lists all GPU nodes
- [ ] `node` parameter on all GPU tools
- [ ] Gateway routes requests to correct node agent
- [ ] Aggregation when no node specified
- [ ] Helm templates for Gateway Deployment
- [ ] RBAC permissions for pod exec

**Should Have:**
- [ ] Concurrent node queries for aggregation
- [ ] Per-node error handling in aggregation
- [ ] Health/ready checks for Gateway

**Nice to Have:**
- [ ] Node caching with TTL
- [ ] Connection pooling
- [ ] Metrics for gateway routing

---

## Related Issues

- **#71** - HTTP transport ✅ (Merged - prerequisite)
- **#73** - Multi-cluster support (can extend gateway)
- **#28** - K8s client (shared code)

---

## Quick Reference

```bash
# Branch
git checkout -b feat/gateway-mode

# Add K8s deps
go get k8s.io/client-go@latest

# Build
make agent

# Test local
./bin/agent --nvml-mode=mock

# Test gateway (in K8s)
./bin/agent --gateway --port 8080 --namespace gpu-diagnostics

# Commit
git commit -s -S -m "feat(gateway): add gateway mode with node routing"

# PR
gh pr create --title "..." --label "kind/feature" --milestone "M3"
```

---

**Reply "GO" when ready to start implementation.** 🚀

