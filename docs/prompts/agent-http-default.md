# Enable HTTP Transport as Default for DaemonSet Agents

## Issue Reference

- **Issue:** [#114 - feat(agent): Enable HTTP transport as default for DaemonSet agents](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/114)
- **Priority:** P1-High
- **Labels:** kind/feature, prio/p1-high
- **Parent Epic:** #112 - HTTP transport refactor

## Background

The current oneshot pattern has significant per-request overhead:

| Operation | Latency |
|-----------|---------|
| Agent process spawn | 10-20ms |
| Go runtime startup | 5-10ms |
| NVML Init() | 50-100ms |
| NVML query | 10-50ms |
| Process teardown | 5-10ms |
| **Total per request** | **130-290ms** |

With HTTP mode, NVML is initialized **once** at pod startup, reducing per-request
overhead to **12-57ms** (an 80% improvement).

### Current Architecture (stdio mode)

```
DaemonSet Pod
┌─────────────────────────────────────┐
│  command: ["sleep", "infinity"]     │
│                                     │
│  kubectl exec → /agent --oneshot=2  │──► NVML Init() ──► Query ──► Exit
│  kubectl exec → /agent --oneshot=2  │──► NVML Init() ──► Query ──► Exit
│  kubectl exec → /agent --oneshot=2  │──► NVML Init() ──► Query ──► Exit
└─────────────────────────────────────┘
     ↑ NVML initialized per request (expensive!)
```

### Target Architecture (http mode)

```
DaemonSet Pod
┌─────────────────────────────────────┐
│  command: ["/agent", "--port=8080"] │
│                                     │
│  NVML Init() (once at startup)      │
│           ↓                         │
│  HTTP :8080 ──► Query ──► Response  │
│  HTTP :8080 ──► Query ──► Response  │
│  HTTP :8080 ──► Query ──► Response  │
└─────────────────────────────────────┘
     ↑ NVML stays warm, constant memory
```

---

## Objective

Change the default transport mode from `stdio` to `http` so DaemonSet agents run
as persistent HTTP servers with NVML initialized once at startup.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/agent-http-default
```

---

## Implementation Tasks

### Task 1: Change Transport Mode Default

Change the default transport mode from `stdio` to `http` in Helm values.

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Current code (line ~149):**
```yaml
transport:
  # -- Transport mode: stdio or http
  # stdio: Uses kubectl exec for interaction (default)
  # http: Exposes HTTP endpoint for MCP access
  mode: stdio
```

**Change to:**
```yaml
transport:
  # -- Transport mode: stdio or http
  # http: Runs persistent HTTP server (recommended for production)
  # stdio: Uses kubectl exec for interaction (legacy, for debugging)
  mode: http
```

**Acceptance criteria:**
- [ ] Default transport mode is `http`
- [ ] Comment updated to reflect http as recommended

> 💡 **Commit:** `feat(helm): change default transport mode to http`

---

### Task 2: Enable Service by Default for HTTP Mode

When HTTP mode is enabled, the service should also be enabled by default.

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Current code (line ~159):**
```yaml
service:
  # -- Enable service (required for HTTP mode)
  enabled: false
```

**Change to:**
```yaml
service:
  # -- Enable service for agent pods (auto-enabled when transport.mode=http)
  # Required for gateway to discover agent endpoints
  enabled: true
```

**Acceptance criteria:**
- [ ] Service enabled by default
- [ ] Comment updated to explain relationship with transport mode

> 💡 **Commit:** `feat(helm): enable service by default for HTTP mode`

---

### Task 3: Add GetAgentHTTPEndpoint Method to K8s Client

Add a helper method to construct agent HTTP endpoint URLs from GPUNode data.

**File:** `pkg/k8s/client.go`

**Add after GPUNode struct definition (~line 53):**

```go
// AgentHTTPPort is the default port agents listen on in HTTP mode.
const AgentHTTPPort = 8080

// GetAgentHTTPEndpoint returns the HTTP endpoint for an agent pod.
// Returns empty string if the pod has no IP assigned.
func (n GPUNode) GetAgentHTTPEndpoint() string {
    if n.PodIP == "" {
        return ""
    }
    return fmt.Sprintf("http://%s:%d", n.PodIP, AgentHTTPPort)
}
```

**Acceptance criteria:**
- [ ] `AgentHTTPPort` constant defined (8080)
- [ ] `GetAgentHTTPEndpoint()` method on GPUNode struct
- [ ] Returns empty string for pods without IP

> 💡 **Commit:** `feat(k8s): add GetAgentHTTPEndpoint helper method`

---

### Task 4: Add Unit Tests for GetAgentHTTPEndpoint

**File:** `pkg/k8s/client_test.go`

**Add test:**

```go
func TestGPUNode_GetAgentHTTPEndpoint(t *testing.T) {
    tests := []struct {
        name     string
        node     GPUNode
        expected string
    }{
        {
            name: "pod with IP",
            node: GPUNode{
                Name:    "gpu-node-1",
                PodName: "gpu-agent-abc123",
                PodIP:   "10.0.0.5",
                Ready:   true,
            },
            expected: "http://10.0.0.5:8080",
        },
        {
            name: "pod without IP",
            node: GPUNode{
                Name:    "gpu-node-2",
                PodName: "gpu-agent-pending",
                PodIP:   "",
                Ready:   false,
            },
            expected: "",
        },
        {
            name: "pod with IPv6",
            node: GPUNode{
                Name:    "gpu-node-3",
                PodName: "gpu-agent-ipv6",
                PodIP:   "fd00::1",
                Ready:   true,
            },
            expected: "http://fd00::1:8080",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := tt.node.GetAgentHTTPEndpoint()
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Acceptance criteria:**
- [ ] Test for pod with IP
- [ ] Test for pod without IP
- [ ] Test for IPv6 address

> 💡 **Commit:** `test(k8s): add GetAgentHTTPEndpoint tests`

---

### Task 5: Increase Default Resource Limits for HTTP Mode

HTTP mode requires more baseline memory since the agent process stays resident.

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Current code (line ~121):**
```yaml
resources:
  requests:
    cpu: 1m
    memory: 10Mi
  limits:
    cpu: 100m
    memory: 50Mi
```

**Change to:**
```yaml
# Resource limits
# Note: HTTP mode requires more baseline memory since agent stays resident
# Oneshot/stdio mode has lower baseline but higher peak during requests
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

**Rationale:**
- HTTP mode: ~15-20MB constant memory (NVML stays warm)
- Stdio mode: ~5MB idle, 100-200MB peak (NVML init spike per request)
- 128Mi limit provides headroom for concurrent requests

**Acceptance criteria:**
- [ ] Memory request increased to 32Mi
- [ ] Memory limit increased to 128Mi
- [ ] CPU request increased to 10m
- [ ] Comment explains HTTP mode memory requirements

> 💡 **Commit:** `chore(helm): increase resource limits for HTTP mode`

---

### Task 6: Verify DaemonSet Template Supports HTTP Mode

The DaemonSet template already supports HTTP mode. Verify it's working correctly.

**File:** `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml`

**Verify these sections exist (lines ~70-100):**

```yaml
{{- if eq .Values.transport.mode "http" }}
command: ["/agent"]
args:
- "--nvml-mode=real"
- "--port={{ .Values.transport.http.port }}"
- "--addr={{ .Values.transport.http.addr }}"
- "--mode={{ default "read-only" .Values.agent.mode }}"
ports:
- name: http
  containerPort: {{ .Values.transport.http.port }}
  protocol: TCP
livenessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /readyz
    port: http
  initialDelaySeconds: 5
  periodSeconds: 10
{{- else }}
{{- /* Stdio mode: Sleep until kubectl exec invokes the agent */}}
command: ["sleep", "infinity"]
stdin: true
tty: true
{{- end }}
```

**Acceptance criteria:**
- [ ] HTTP mode uses `/agent` with `--port` flag
- [ ] Liveness probe configured for `/healthz`
- [ ] Readiness probe configured for `/readyz`
- [ ] Stdio mode uses `sleep infinity` as fallback

> 💡 **No code changes needed** - just verification

---

### Task 7: Update Documentation Comment in values.yaml

Add a section explaining the transport modes and their trade-offs.

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Update the transport section comment:**

```yaml
# Transport configuration
# 
# HTTP Mode (default, recommended for production):
#   - Agent runs as persistent HTTP server
#   - NVML initialized once at startup (~50-100ms)
#   - Per-request latency: 12-57ms
#   - Memory: constant ~15-20MB
#   - Use case: Gateway routing, high-frequency queries
#
# Stdio Mode (legacy, for debugging):
#   - Agent spawned per kubectl exec
#   - NVML initialized per request
#   - Per-request latency: 130-290ms
#   - Memory: spiky (5MB idle → 200MB peak)
#   - Use case: Direct kubectl debugging, SRE access
transport:
  # -- Transport mode: http (recommended) or stdio (legacy)
  mode: http
```

**Acceptance criteria:**
- [ ] Transport section has detailed mode comparison
- [ ] Trade-offs documented (latency, memory, use case)

> 💡 **Commit:** `docs(helm): document transport mode trade-offs`

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Verify Helm template renders correctly
helm template gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --set transport.mode=http | grep -A 20 "containers:"

# Verify stdio mode still works
helm template gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --set transport.mode=stdio | grep -A 5 "command:"
```

### Integration Testing (Real Cluster)

```bash
# Deploy with HTTP mode (now default)
helm upgrade --install gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics --create-namespace

# Verify pods are running with HTTP server
kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/name=k8s-gpu-mcp-server

# Check agent logs for NVML initialization
kubectl logs -n gpu-diagnostics -l app.kubernetes.io/component=gpu-diagnostics \
  | grep -i "nvml\|http\|listening"

# Verify health endpoint works
kubectl exec -n gpu-diagnostics <agent-pod> -- \
  wget -qO- http://localhost:8080/healthz

# Test MCP endpoint
kubectl exec -n gpu-diagnostics <agent-pod> -- \
  wget -qO- --post-data='{"jsonrpc":"2.0","method":"initialize","id":1}' \
  http://localhost:8080/mcp
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
- [ ] Helm template renders correctly for both modes

---

## Commit Summary

| Order | Commit Message |
|-------|----------------|
| 1 | `feat(helm): change default transport mode to http` |
| 2 | `feat(helm): enable service by default for HTTP mode` |
| 3 | `feat(k8s): add GetAgentHTTPEndpoint helper method` |
| 4 | `test(k8s): add GetAgentHTTPEndpoint tests` |
| 5 | `chore(helm): increase resource limits for HTTP mode` |
| 6 | `docs(helm): document transport mode trade-offs` |

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(agent): enable HTTP transport as default for DaemonSet agents" \
  --body "Fixes #114

## Summary

Changes the default transport mode from \`stdio\` to \`http\` so DaemonSet
agents run as persistent HTTP servers with NVML initialized once at startup.

## Changes

- Change default \`transport.mode\` from \`stdio\` to \`http\`
- Enable service by default for agent pods
- Add \`GetAgentHTTPEndpoint()\` helper method to GPUNode
- Increase default resource limits for HTTP mode (128Mi memory)
- Document transport mode trade-offs in values.yaml

## Performance Impact

| Metric | Stdio (before) | HTTP (after) | Improvement |
|--------|----------------|--------------|-------------|
| Per-request latency | 130-290ms | 12-57ms | **80%** |
| NVML init | Per request | Once at startup | **Eliminated** |
| Memory pattern | Spiky | Constant | Stable |

## Testing

- [ ] Unit tests pass
- [ ] Helm template renders correctly
- [ ] Manual E2E test with real cluster
- [ ] Verified stdio fallback still works

## Backward Compatibility

Stdio mode remains available via \`transport.mode: stdio\` for debugging
and SRE access via kubectl exec.

## Related

- Parent epic: #112
- Blocks: #115 (Gateway HTTP routing)" \
  --label "kind/feature" \
  --label "prio/p1-high"
```

---

## Success Criteria

| Metric | Before | After |
|--------|--------|-------|
| Default transport | stdio | http |
| NVML init frequency | Per request | Once at startup |
| Per-request latency | 130-290ms | 12-57ms |
| Memory pattern | Spiky | Constant |

---

## Related Files

- `deployment/helm/k8s-gpu-mcp-server/values.yaml` - Helm values
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml` - DaemonSet template
- `pkg/k8s/client.go` - K8s client with GPUNode struct
- `pkg/mcp/http.go` - HTTP server implementation

---

## Notes

- This change is **backward compatible** - stdio mode remains available
- After this change, #115 can implement gateway HTTP routing using pod IPs
- The DaemonSet template already supports both modes - just changing the default

---

**Reply "GO" when ready to start implementation.** 🚀
