# Principal Engineer Analysis: The "Oneshot" Pattern in k8s-gpu-mcp-server

> Analysis Date: 2026-01-09
> Status: Discussion Document
> Author: AI Architecture Review

## Executive Summary

The `--oneshot=2` pattern in `pkg/k8s/client.go:173-174` represents a **process-per-request architecture** where each MCP tool invocation spawns a new agent process, processes exactly 2 JSON-RPC messages (initialize + tool call), and terminates. While functional, this design exhibits characteristics of a workaround rather than a first-principles solution.

---

## 1. Current Architecture Analysis

### 1.1 The Oneshot Flow

```
┌──────────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│  Gateway Pod         │     │   kubectl exec      │     │  Agent Pod       │
│  (HTTP/SSE listener) │     │   (SPDY tunnel)     │     │  (sleep ∞)       │
└──────────┬───────────┘     └──────────┬──────────┘     └────────┬─────────┘
           │                            │                         │
           │ 1. MCP tool request        │                         │
           │───────────────────────────►│ 2. spawn /agent         │
           │                            │ --oneshot=2             │
           │                            │────────────────────────►│
           │                            │                         │ 3. process init
           │                            │                         │ 4. process tool
           │                            │                         │ 5. exit(0)
           │                            │◄────────────────────────│
           │ 6. response                │                         │
           │◄───────────────────────────│                         │
           │                            │                         │
```

### 1.2 Key Observations

From the codebase:

**`pkg/k8s/client.go:170-177`:**
```go
execOpts := &corev1.PodExecOptions{
    Container: container,
    // Use --oneshot=2 to process exactly 2 requests (init + tool) then exit
    Command: []string{"/agent", "--nvml-mode=real", "--oneshot=2"},
    Stdin:   stdin != nil,
    Stdout:  true,
    Stderr:  true,
}
```

**`pkg/gateway/framing.go:68-126`:** The gateway builds a 2-message payload:
```go
// Build initialize request (ID: 0)
// Build tool call request (ID: 1)
// Concatenate with newlines
```

**`pkg/mcp/oneshot.go:17-25`:** The transport terminates after N requests.

### 1.3 Why This Pattern Was Chosen

Based on architecture docs:

1. **On-Demand Diagnostic Model**: Agent runs only during active sessions, sleeping otherwise
2. **No Network Exposure**: Stdio transport via `kubectl exec` avoids open ports
3. **kubectl Exec Compatibility**: SPDY tunneling requires process-based invocation
4. **DaemonSet Architecture**: Sleeping container provides instant access with near-zero idle cost

---

## 2. Problems with the Oneshot Approach

### 2.1 Architectural Debt

| Problem | Severity | Impact |
|---------|----------|--------|
| **Process spawn overhead** | Medium | ~50-200ms per request for exec setup + process init |
| **Repeated initialization** | High | NVML `Init()` called for every single tool invocation |
| **Session state loss** | Medium | No caching, no connection reuse, no telemetry aggregation |
| **Magic number coupling** | Medium | `--oneshot=2` hardcoded; fragile if protocol evolves |
| **Sequential bottleneck** | High | Gateway waits for all agents to spawn, run, exit |
| **No streaming support** | High | Can't support real-time telemetry or subscriptions |

### 2.2 Cost Breakdown Per Request

```
┌────────────────────────────────────────────────────────────────┐
│  Time Budget: Single Tool Call                                 │
├────────────────────────────────────────────────────────────────┤
│  kubectl exec SPDY negotiation      │  50-100ms                │
│  Container process spawn            │  10-20ms                 │
│  Go runtime startup                 │  5-10ms                  │
│  NVML Init()                        │  50-100ms                │
│  JSON-RPC parse (2 messages)        │  <1ms                    │
│  Actual GPU query                   │  10-50ms                 │
│  Process teardown                   │  5-10ms                  │
├────────────────────────────────────────────────────────────────┤
│  TOTAL                              │  130-290ms               │
│  (vs ~20-60ms if persistent)        │                          │
└────────────────────────────────────────────────────────────────┘
```

### 2.3 The Real Issue: MCP Protocol Impedance Mismatch

The MCP protocol is designed for **persistent, session-based communication**:

- `initialize` → establishes capabilities, negotiates protocol version
- `tools/list` → client discovers available tools
- `tools/call` → repeated invocations with session context
- Notifications, subscriptions, streaming (future)

The oneshot pattern **fights against** this by:
- Forcing session establishment per request
- Preventing stateful interactions
- Making MCP subscriptions/streaming impossible
- Duplicating initialization overhead

---

## 3. Alternative Architectures

### 3.1 Option A: Persistent HTTP/SSE Listener (Recommended)

**Pattern:** Each DaemonSet pod runs the agent as a persistent HTTP/SSE server on a local port.

```
┌────────────────────────────────────────────────────────────────┐
│  GPU Node (DaemonSet Pod)                                      │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  /agent --port=8080 (persistent)                         │  │
│  │  └── MCP Server (HTTP/SSE)                               │  │
│  │      └── NVML Client (initialized once)                  │  │
│  │      └── Session Manager                                 │  │
│  └───────────────────────────────────┬──────────────────────┘  │
│                                      │ :8080                   │
└──────────────────────────────────────┼─────────────────────────┘
                                       │
          Gateway routes via ClusterIP/Pod IP
```

**Pros:**
- NVML initialized once at pod startup
- Session state preserved across requests
- Supports streaming, subscriptions, real-time telemetry
- Latency: ~20-60ms vs 130-290ms
- Standard HTTP/SSE (MCP's recommended remote transport)

**Cons:**
- Requires network exposure (mitigated via NetworkPolicy)
- Slightly higher idle resource usage (still minimal for Go)
- Need health checks, restart logic

**Implementation:**
- Already supported: `--port=8080` flag exists in `cmd/agent/main.go`
- Gateway would call `http://pod-ip:8080/mcp/sse` instead of exec

### 3.2 Option B: Unix Socket with Sidecar Proxy

**Pattern:** Agent listens on a Unix domain socket, sidecar handles routing.

```
┌────────────────────────────────────────────────────────────────┐
│  GPU Node Pod                                                  │
│  ┌─────────────────────┐     ┌─────────────────────────────┐  │
│  │  Sidecar (envoy/    │     │  /agent                     │  │
│  │  custom proxy)      │◄───►│  (Unix socket listener)    │  │
│  │  :8080 HTTP         │     │  /var/run/mcp.sock          │  │
│  └─────────────────────┘     └─────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**Pros:**
- No network namespace changes needed
- Enhanced security (socket permissions)
- Proxy can handle retries, circuit breaking

**Cons:**
- Added complexity (sidecar container)
- More moving parts
- Overkill for current use case

### 3.3 Option C: gRPC Streaming (Future-Ready)

**Pattern:** Agent exposes gRPC service with bidirectional streaming.

```protobuf
service GPUAgent {
  rpc StreamMCP(stream MCPRequest) returns (stream MCPResponse);
  rpc GetInventory(InventoryRequest) returns (InventoryResponse);
  rpc WatchHealth(WatchRequest) returns (stream HealthUpdate);
}
```

**Pros:**
- Native streaming for real-time telemetry
- Strong typing, code generation
- Efficient binary protocol
- Bidirectional communication

**Cons:**
- Requires protocol translation (MCP is JSON-RPC)
- Heavier implementation
- Less ecosystem compatibility

### 3.4 Option D: Hybrid (Current + Persistent)

**Pattern:** Keep exec for one-off diagnostics, add HTTP for gateway.

```go
// Gateway uses HTTP transport for persistent agents
if agent.HasHTTPEndpoint() {
    return http.Post(agent.HTTPAddr + "/mcp", request)
}
// Fallback to exec for legacy/debugging
return exec.InPod(agent.PodName, request)
```

**Pros:**
- Backward compatible
- Graceful migration path
- Exec remains available for debugging

**Cons:**
- Two code paths to maintain
- Configuration complexity

---

## 4. Recommendation: Persistent HTTP/SSE Transport

### 4.1 Why HTTP/SSE?

1. **MCP Native:** HTTP/SSE is the recommended transport for remote MCP servers
2. **Already Implemented:** `pkg/mcp/http.go` exists with full SSE support
3. **Gateway Compatible:** Gateway already uses HTTP internally
4. **Minimal Change:** Mostly configuration, not architecture rewrite
5. **Production Patterns:** Aligns with Prometheus, node-exporter, cadvisor designs

### 4.2 Migration Path

```
Phase 1: Internal Refactor
├── Agent pods run with --port=8080 by default
├── Gateway calls Pod IPs directly
├── Keep exec as fallback/debug mode
└── Helm chart updated with NetworkPolicy

Phase 2: Enhanced Features
├── Connection pooling in gateway
├── Health check integration (liveness probe → /healthz)
├── Metrics endpoint (/metrics for Prometheus)
└── Session management for multi-request flows

Phase 3: Advanced
├── gRPC for streaming telemetry
├── MCP subscriptions (resources changed notifications)
└── Real-time XID alerting via Server-Sent Events
```

### 4.3 Helm Changes Required

```yaml
# values.yaml
agent:
  mode: "http"        # Instead of exec-triggered
  port: 8080
  
gateway:
  routingMode: "http" # Instead of kubectl-exec
  
networkPolicy:
  enabled: true       # Gateway → Agent only
```

### 4.4 Code Changes Scope

| File | Change |
|------|--------|
| `cmd/agent/main.go` | Default to `--port=8080` |
| `pkg/gateway/router.go` | HTTP client instead of `ExecInPod` |
| `pkg/k8s/client.go` | Remove oneshot exec code (or keep for debug) |
| `deployment/helm/.../daemonset.yaml` | Add readinessProbe, livenessProbe |
| `deployment/helm/.../gateway-deployment.yaml` | Remove exec RBAC if not needed |

---

## 5. Comparison Matrix

| Criterion | Oneshot (Current) | HTTP/SSE | gRPC | Unix Socket |
|-----------|-------------------|----------|------|-------------|
| **Latency** | 130-290ms | 20-60ms | 10-40ms | 20-50ms |
| **Resource Efficiency** | ❌ Process per request | ✅ Persistent | ✅ Persistent | ✅ Persistent |
| **Streaming Support** | ❌ No | ✅ SSE | ✅ Native | ⚠️ Custom |
| **Implementation Effort** | ✅ Done | ⚠️ Moderate | ❌ High | ⚠️ Moderate |
| **MCP Compliance** | ⚠️ Workaround | ✅ Standard | ⚠️ Translation | ⚠️ Custom |
| **Debugging** | ✅ Simple exec | ⚠️ Need tooling | ⚠️ Need tooling | ❌ Complex |
| **Security Surface** | ✅ No ports | ⚠️ NetworkPolicy | ⚠️ NetworkPolicy | ✅ Local only |

---

## 6. Technical Debt Assessment

### 6.1 Current State
The oneshot pattern is **acceptable for MVP** but becomes debt as the system scales:

- ✅ Works for single-request diagnostics
- ⚠️ Blocks streaming/subscription features
- ⚠️ Performance ceiling at scale
- ❌ Violates MCP session semantics

### 6.2 Recommended Prioritization

| Priority | Action | Effort | Impact |
|----------|--------|--------|--------|
| P1 | Implement HTTP routing in gateway | M | High - removes oneshot |
| P1 | Add NetworkPolicy for agent pods | S | Security |
| P2 | Add connection pooling | S | Performance |
| P2 | Add health probes to Helm chart | S | Reliability |
| P3 | gRPC for streaming telemetry | L | Future features |

---

## 7. Conclusion

The `--oneshot=2` pattern is a **clever workaround** that solved the immediate problem of invoking MCP tools through `kubectl exec`. However, from a principal engineer perspective, it represents:

1. **Protocol impedance mismatch** - Fighting MCP's session-based design
2. **Unnecessary overhead** - Process spawn + NVML init per request
3. **Feature ceiling** - Blocks streaming, subscriptions, real-time telemetry
4. **Coupling smell** - Magic number `2` couples framing layer to transport

**The recommended path forward is adopting HTTP/SSE as the primary transport**, keeping exec as a debug/fallback mechanism. This aligns with:
- MCP protocol design intent
- Kubernetes service patterns (like metrics exporters)
- The agent's existing HTTP transport implementation

---

## 8. Discussion Points

1. **Do we need backward compatibility with exec-only clusters?**
2. **What NetworkPolicy rules are acceptable for agent ↔ gateway communication?**
3. **Should we expose Prometheus metrics on the agent pods?**
4. **Timeline for deprecating oneshot mode?**

---

## References

- `pkg/k8s/client.go:170-177` - ExecInPod with oneshot invocation
- `pkg/mcp/oneshot.go` - OneshotTransport implementation
- `pkg/gateway/framing.go` - MCP request framing for exec
- `pkg/mcp/http.go` - Existing HTTP/SSE transport
- `docs/architecture.md` - System architecture documentation

---

*Report prepared for architectural review. No code modifications required.*
