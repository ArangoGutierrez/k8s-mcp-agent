# E2E Failure Analysis & Architecture Refactor Plan

**Date:** January 9, 2026  
**Status:** Draft Analysis  
**Author:** Engineering Team

## Executive Summary

After extensive E2E testing, we have identified fundamental architectural issues
that cause intermittent failures in the gateway-to-agent communication path. 
This document provides a root cause analysis and proposes architectural changes.

---

## 1. Observed Failures

### 1.1 Symptoms

| Symptom | Frequency | Impact |
|---------|-----------|--------|
| `exec timeout after 30s` | ~50% of calls | Gateway cannot reach agents |
| `OOMKilled` (exit 137) | Consistent (before fix) | Agent process killed |
| `socket hang up` from NPM bridge | 100% of tools/call | No GPU data returned |
| Direct `kubectl exec` works | 100% success | Proves agent itself works |

### 1.2 Key Observation

**Direct kubectl exec works reliably, but gateway's client-go exec fails.**

```bash
# This works (100% of the time):
kubectl exec -i <pod> -- /agent --nvml-mode=real --oneshot=2 < request.json

# This fails (intermittently):
# Gateway → client-go exec → same pod → timeout
```

---

## 2. Root Cause Analysis

### 2.1 Architecture Overview (Current)

```
┌─────────────────┐      ┌───────────────────────┐      ┌─────────────────┐
│  Cursor/Claude  │      │       Gateway         │      │   Agent Pod     │
│  (MCP Client)   │─────▶│  (HTTP Server)        │─────▶│  (sleep inf)    │
│                 │ HTTP │                       │ EXEC │                 │
│                 │      │  ┌─────────────────┐  │      │  ┌───────────┐  │
│                 │      │  │ K8s Client      │──┼──────┼─▶│ /agent    │  │
│                 │      │  │ (client-go)     │  │ SPDY │  │ --oneshot │  │
│                 │      │  └─────────────────┘  │      │  └───────────┘  │
└─────────────────┘      └───────────────────────┘      └─────────────────┘
        │                          │                            │
        │                          │                            │
        ▼                          ▼                            ▼
   NPM Bridge             HTTP → Exec → Stdio           NVML → GPU
   (port-forward)         (3 protocol layers)           (hardware)
```

### 2.2 Identified Problems

#### Problem 1: Excessive Protocol Layering

The current architecture has **5 protocol layers** between Claude and the GPU:

1. **JSON-RPC** (Claude → NPM Bridge)
2. **HTTP** (NPM Bridge → Gateway via port-forward)
3. **MCP Streamable HTTP** (Gateway session management)
4. **SPDY/exec** (Gateway → Agent pod via client-go)
5. **JSON-RPC** (Agent stdin/stdout)

Each layer adds latency, potential failure points, and timeout complexity.

#### Problem 2: Parallel Exec Race Conditions

The gateway executes parallel `kubectl exec` calls to all 4 nodes:

```go
// pkg/gateway/router.go:108-138
for _, node := range nodes {
    wg.Add(1)
    go func(n k8s.GPUNode) {
        response, err := r.routeToGPUNode(ctx, n, mcpRequest)
        // ...
    }(node)
}
wg.Wait()
```

**Issues:**
- 4 parallel SPDY connections compete for resources
- Single slow node blocks entire response (30s timeout)
- No retry logic for transient failures
- Context propagation doesn't handle partial success

#### Problem 3: HTTP WriteTimeout Mismatch

```go
// pkg/mcp/http.go:38
WriteTimeout: 30 * time.Second,

// pkg/k8s/client.go:27
const DefaultExecTimeout = 30 * time.Second
```

The HTTP write timeout equals the exec timeout. When exec takes 30s, the HTTP
response cannot be written before the connection times out.

**Timeline:**
```
T+0s:    HTTP request received
T+0.1s:  Start parallel exec to 4 nodes
T+30s:   Exec timeout fires on all nodes
T+30s:   HTTP write timeout fires simultaneously
T+30s:   "socket hang up" - connection closed before response
```

#### Problem 4: Oneshot Transport Not Being Used in Deployed Pods

Looking at the Helm template:

```yaml
# deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml:94-99
{{- else }}
{{- /* Stdio mode: Sleep until kubectl exec invokes the agent */}}
command: ["sleep", "infinity"]
stdin: true
tty: true
{{- end }}
```

The pods run `sleep infinity` and we exec `/agent --oneshot=2` into them.
This works, but:
- Each exec spawns a new agent process (100-200ms startup)
- NVML initialization happens on every call (~50ms)
- No connection pooling or reuse

#### Problem 5: Cluster Infrastructure Issues

During testing, we discovered:
- Calico CNI was unhealthy (kube-proxy DNS misconfiguration)
- Some nodes had networking issues
- These were masked by the architecture complexity

---

## 3. Architectural Options

### Option A: Keep Exec Pattern, Fix Timeouts

**Minimal changes - fix the immediate issues**

Changes:
1. Increase HTTP WriteTimeout to 60s
2. Increase exec timeout to 45s
3. Add retry logic with exponential backoff
4. Implement partial success responses

Pros:
- Minimal code changes
- Preserves "on-demand" design philosophy

Cons:
- Still has 5 protocol layers
- Each call has 150-250ms overhead
- Fundamentally fragile

### Option B: Long-Running Agent HTTP Servers

**Agents run HTTP servers, gateway forwards requests**

```
┌─────────────────┐      ┌───────────────────────┐      ┌─────────────────┐
│  Cursor/Claude  │      │       Gateway         │      │   Agent Pod     │
│  (MCP Client)   │─────▶│  (HTTP Proxy)         │─────▶│  (HTTP Server)  │
│                 │ HTTP │                       │ HTTP │                 │
│                 │      │  Routes by nodeName   │      │  Port 8080      │
└─────────────────┘      └───────────────────────┘      └─────────────────┘
```

Changes:
1. Agents run in HTTP mode (`--port=8080`)
2. Gateway becomes an HTTP reverse proxy
3. Pod-to-pod networking instead of exec

Pros:
- Eliminates SPDY/exec complexity
- Agents stay warm (no startup overhead)
- Standard HTTP load balancing patterns
- Better observability (HTTP metrics)

Cons:
- Agents consume resources continuously (~15MB each)
- Requires network policy configuration
- Exposes ports (though internal only)

### Option C: Hybrid - HTTP for Gateway, Stdio for Direct

**Best of both worlds**

```
Direct Access (SRE/local):        Gateway Access (Cursor):
kubectl exec ... /agent           NPM Bridge → Gateway → HTTP → Agent

┌─────────────────┐               ┌─────────────────┐
│  kubectl exec   │               │  Agent Pod      │
│  -it <pod> --   │               │  ┌───────────┐  │
│  /agent (stdio) │               │  │ HTTP:8080 │  │
└─────────────────┘               │  └───────────┘  │
                                  └─────────────────┘
```

The agent supports BOTH modes:
- HTTP server for gateway communication
- Stdio for direct kubectl exec access

Pros:
- Gateway path is reliable (HTTP)
- Direct path still works for debugging
- Maintains "on-demand" philosophy for humans

Cons:
- Slightly more complex agent
- Agents still consume resources when idle

### Option D: Gateway Runs on GPU Nodes (DaemonSet)

**Eliminate the gateway-agent split entirely**

```
┌─────────────────┐      ┌──────────────────────────────────────────┐
│  Cursor/Claude  │      │              GPU Node                    │
│  (MCP Client)   │─────▶│  ┌────────────────────────────────────┐  │
│                 │ HTTP │  │  k8s-gpu-mcp-server (HTTP+NVML)    │  │
│                 │      │  │  - MCP HTTP Server                  │  │
│                 │      │  │  - Direct NVML access               │  │
│                 │      │  └────────────────────────────────────┘  │
└─────────────────┘      └──────────────────────────────────────────┘
```

Changes:
1. Single DaemonSet runs on GPU nodes only
2. Each pod is both HTTP server AND NVML client
3. NPM bridge connects to specific node or round-robins

Pros:
- Simplest architecture (2 protocol layers)
- No inter-pod communication needed
- Lower latency

Cons:
- No centralized cluster view
- Client must handle multi-node aggregation
- NPM bridge becomes more complex

---

## 4. Recommendation: Option C (Hybrid)

**Rationale:**

1. **Reliability**: HTTP is proven technology, exec is fragile
2. **Compatibility**: Direct kubectl exec still works for SREs
3. **Philosophy**: Maintains "minimal footprint" goal
4. **Incremental**: Can implement without breaking existing deploys

### Implementation Plan

#### Phase 1: Fix Immediate Issues (1-2 days)

1. [ ] Increase HTTP WriteTimeout to 90s
2. [ ] Increase exec timeout to 60s (environment variable)
3. [ ] Add detailed timing logs to identify bottlenecks
4. [ ] Add health endpoints to agent pods

#### Phase 2: Agent HTTP Mode (3-5 days)

1. [ ] Agent runs HTTP server when deployed as DaemonSet
2. [ ] Gateway routes to agent HTTP endpoints (pod IP:port)
3. [ ] Keep stdio support for direct kubectl exec
4. [ ] Update Helm chart for HTTP mode by default

#### Phase 3: Observability & Resilience (2-3 days)

1. [ ] Add Prometheus metrics to gateway and agents
2. [ ] Implement circuit breaker for node failures
3. [ ] Add retry with exponential backoff
4. [ ] Partial success responses (return data from healthy nodes)

#### Phase 4: Documentation & Testing (2-3 days)

1. [ ] Update architecture.md
2. [ ] Create integration test suite
3. [ ] Load testing with multiple concurrent requests
4. [ ] Chaos testing (node failures, network partitions)

---

## 5. Detailed Technical Changes

### 5.1 Agent Changes

```go
// cmd/agent/main.go - Support both HTTP and stdio

if *port > 0 {
    // HTTP mode: long-running server
    return runHTTPServer(ctx, *port)
} else {
    // Stdio mode: for kubectl exec
    return runStdio(ctx)
}
```

### 5.2 Gateway Changes

```go
// pkg/gateway/router.go - HTTP routing instead of exec

func (r *Router) routeToGPUNode(ctx context.Context, node k8s.GPUNode, 
    mcpRequest []byte) ([]byte, error) {
    
    // HTTP request to agent's pod IP
    url := fmt.Sprintf("http://%s:8080/mcp", node.PodIP)
    resp, err := r.httpClient.Post(url, "application/json", 
        bytes.NewReader(mcpRequest))
    // ...
}
```

### 5.3 Helm Chart Changes

```yaml
# values.yaml
agent:
  mode: http  # or stdio for direct-only deployments
  port: 8080

# daemonset.yaml
containers:
- name: agent
  command: ["/agent"]
  args:
  - "--nvml-mode=real"
  - "--port=8080"  # HTTP mode
  ports:
  - containerPort: 8080
    protocol: TCP
```

### 5.4 Service Discovery

Gateway discovers agent pods via K8s API (already implemented):

```go
pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx,
    metav1.ListOptions{
        LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server," +
            "app.kubernetes.io/component!=gateway",
    })
```

Then connects to each pod's IP directly (pod-to-pod networking).

---

## 6. Testing Strategy

### 6.1 Unit Tests

- [ ] HTTP client with mock server
- [ ] Retry logic with simulated failures
- [ ] Timeout handling edge cases

### 6.2 Integration Tests

- [ ] Gateway → Agent HTTP communication
- [ ] Multi-node aggregation
- [ ] Partial failure scenarios

### 6.3 E2E Tests

- [ ] Full path: NPM Bridge → Gateway → Agents → GPUs
- [ ] Concurrent request handling
- [ ] Node failure recovery

---

## 7. Rollback Plan

If HTTP mode causes issues:

1. Set `agent.mode: stdio` in Helm values
2. Redeploy with `helm upgrade`
3. Gateway falls back to exec mode

---

## 8. Success Criteria

| Metric | Current | Target |
|--------|---------|--------|
| E2E success rate | ~10% | >99% |
| Request latency (p50) | 30s+ (timeout) | <500ms |
| Request latency (p99) | 30s (timeout) | <2s |
| Agent memory usage | 256Mi | <64Mi |

---

## 9. Open Questions

1. **Network Policy**: Do we need explicit policies for pod-to-pod HTTP?
2. **Service Mesh**: Should we use Istio/Linkerd for retries/observability?
3. **Graceful Degradation**: How to handle single-node failures in aggregation?
4. **Caching**: Should gateway cache GPU inventory briefly?

---

## 10. References

- [Current Architecture Doc](../architecture.md)
- [MCP Protocol Spec](https://modelcontextprotocol.io/)
- [Kubernetes Pod Networking](https://kubernetes.io/docs/concepts/cluster-administration/networking/)
- [client-go exec issues](https://github.com/kubernetes/client-go/issues)

