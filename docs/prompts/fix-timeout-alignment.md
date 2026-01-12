# Fix Timeout Alignment for Gateway-Agent Communication

## Issue Reference

- **Issue:** [#113 - fix(gateway): Align HTTP and exec timeouts to prevent race conditions](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/113)
- **Priority:** P1-High
- **Labels:** kind/bug, prio/p1-high
- **Parent Epic:** #112 - HTTP transport refactor

## Background

The gateway's HTTP WriteTimeout (30s) equals the K8s client exec timeout (30s),
creating a guaranteed race condition. When an exec operation takes exactly 30s
(timeout), the HTTP response cannot be written before the connection closes.

**Timeline of failure:**
```
T+0s:    HTTP request received
T+0.1s:  Start parallel exec to 4 nodes
T+30s:   Exec timeout fires on all nodes
T+30s:   HTTP write timeout fires simultaneously
T+30s:   "socket hang up" - connection closed before response
```

**Root Cause Analysis:** [docs/reports/e2e-failure-analysis.md](../reports/e2e-failure-analysis.md)

---

## Objective

Align HTTP and exec timeouts with proper buffer to prevent race conditions and
add observability for timeout debugging.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b fix/timeout-alignment
```

---

## Implementation Tasks

### Task 1: Increase HTTP WriteTimeout

The HTTP server's WriteTimeout must be significantly larger than the exec
timeout to allow time for response marshaling and writing.

**File:** `pkg/mcp/http.go`

**Current code (line ~38):**
```go
WriteTimeout: 30 * time.Second,
```

**Change to:**
```go
WriteTimeout: 90 * time.Second,
```

**Rationale:**
- Exec timeout: 60s (new value)
- Response marshaling: ~5s buffer
- Network latency: ~5s buffer
- Total: 90s gives 20s safety margin

**Acceptance criteria:**
- [ ] WriteTimeout increased to 90s
- [ ] Comment added explaining the relationship to exec timeout

> 💡 **Commit:** `fix(mcp): increase HTTP WriteTimeout to 90s for exec buffer`

---

### Task 2: Make Exec Timeout Configurable

The exec timeout should be configurable via environment variable for
operational flexibility without recompilation.

**File:** `pkg/k8s/client.go`

**Current code (line ~27):**
```go
const DefaultExecTimeout = 30 * time.Second
```

**Changes:**

1. Change constant to variable with env override:

```go
// DefaultExecTimeout is the default timeout for kubectl exec operations.
// Can be overridden via EXEC_TIMEOUT environment variable.
var DefaultExecTimeout = 60 * time.Second

func init() {
    if envTimeout := os.Getenv("EXEC_TIMEOUT"); envTimeout != "" {
        if d, err := time.ParseDuration(envTimeout); err == nil {
            DefaultExecTimeout = d
            log.Printf(`{"level":"info","msg":"exec timeout configured",`+
                `"timeout":"%s","source":"env"}`, d)
        } else {
            log.Printf(`{"level":"warn","msg":"invalid EXEC_TIMEOUT",`+
                `"value":"%s","error":"%v","using_default":"%s"}`,
                envTimeout, err, DefaultExecTimeout)
        }
    }
}
```

2. Add import for `os` package if not present.

**Acceptance criteria:**
- [ ] Default exec timeout increased from 30s to 60s
- [ ] EXEC_TIMEOUT environment variable support added
- [ ] Log message when env override is used
- [ ] Warning logged for invalid duration format

> 💡 **Commit:** `fix(k8s): make exec timeout configurable via EXEC_TIMEOUT env`

---

### Task 3: Add Timing Telemetry to Router

Add detailed timing logs to identify which phase of exec takes longest.

**File:** `pkg/gateway/router.go`

**Add timing instrumentation to `routeToGPUNode`:**

```go
func (r *Router) routeToGPUNode(ctx context.Context, node k8s.GPUNode,
    mcpRequest []byte) ([]byte, error) {
    
    startTime := time.Now()
    
    log.Printf(`{"level":"debug","msg":"exec starting","node":"%s",`+
        `"pod":"%s"}`, node.Name, node.PodName)
    
    response, err := r.k8sClient.ExecInPod(ctx, node.PodName,
        "agent", bytes.NewReader(mcpRequest))
    
    duration := time.Since(startTime)
    
    if err != nil {
        log.Printf(`{"level":"error","msg":"exec failed","node":"%s",`+
            `"duration_ms":%d,"error":"%v"}`,
            node.Name, duration.Milliseconds(), err)
        return nil, err
    }
    
    log.Printf(`{"level":"info","msg":"exec completed","node":"%s",`+
        `"duration_ms":%d,"response_bytes":%d}`,
        node.Name, duration.Milliseconds(), len(response))
    
    return response, nil
}
```

**Also update `RouteToAllNodes` with aggregate timing:**

```go
func (r *Router) RouteToAllNodes(ctx context.Context,
    mcpRequest []byte) ([]NodeResult, error) {
    
    startTime := time.Now()
    
    // ... existing node listing code ...
    
    // After all goroutines complete:
    wg.Wait()
    close(resultsCh)
    
    totalDuration := time.Since(startTime)
    
    log.Printf(`{"level":"info","msg":"routing complete",`+
        `"total_nodes":%d,"success":%d,"failed":%d,"duration_ms":%d}`,
        len(nodes), successCount, failCount, totalDuration.Milliseconds())
    
    // ... rest of function ...
}
```

**Acceptance criteria:**
- [ ] Per-node exec timing logged
- [ ] Aggregate routing timing logged
- [ ] Duration in milliseconds for easy parsing

> 💡 **Commit:** `feat(gateway): add timing telemetry to router`

---

### Task 4: Update Helm Chart with Exec Timeout

Add exec timeout configuration to Helm values.

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Add under gateway section:**

```yaml
gateway:
  enabled: true
  # ... existing config ...
  
  # Timeout for kubectl exec operations to agent pods
  execTimeout: "60s"
```

**File:** `deployment/helm/k8s-gpu-mcp-server/templates/gateway-deployment.yaml`

**Add environment variable:**

```yaml
env:
  - name: EXEC_TIMEOUT
    value: {{ .Values.gateway.execTimeout | default "60s" | quote }}
```

**Acceptance criteria:**
- [ ] `gateway.execTimeout` value added to values.yaml
- [ ] Environment variable passed to gateway deployment
- [ ] Default value is "60s"

> 💡 **Commit:** `chore(helm): add execTimeout configuration for gateway`

---

### Task 5: Add Unit Tests

**File:** `pkg/k8s/client_test.go`

Add tests for timeout configuration:

```go
func TestExecTimeoutFromEnv(t *testing.T) {
    tests := []struct {
        name     string
        envValue string
        wantLog  string
    }{
        {
            name:     "valid duration",
            envValue: "45s",
            wantLog:  "exec timeout configured",
        },
        {
            name:     "invalid duration",
            envValue: "not-a-duration",
            wantLog:  "invalid EXEC_TIMEOUT",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

**Note:** Testing init() functions with env vars requires careful setup.
Consider refactoring to a testable function if needed.

**Acceptance criteria:**
- [ ] Test for valid EXEC_TIMEOUT parsing
- [ ] Test for invalid EXEC_TIMEOUT handling
- [ ] Test for default value when env not set

> 💡 **Commit:** `test(k8s): add exec timeout configuration tests`

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Verify timeout values
grep -r "WriteTimeout" pkg/mcp/
grep -r "DefaultExecTimeout" pkg/k8s/
grep -r "EXEC_TIMEOUT" pkg/ deployment/
```

### Integration Testing

```bash
# Deploy with custom timeout
helm upgrade gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --set gateway.execTimeout="45s" \
  -n gpu-diagnostics

# Verify env var is set
kubectl exec -n gpu-diagnostics <gateway-pod> -- env | grep EXEC

# Monitor logs during tool call
kubectl logs -n gpu-diagnostics <gateway-pod> -f | jq .
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

## Commit Summary

| Order | Commit Message |
|-------|----------------|
| 1 | `fix(mcp): increase HTTP WriteTimeout to 90s for exec buffer` |
| 2 | `fix(k8s): make exec timeout configurable via EXEC_TIMEOUT env` |
| 3 | `feat(gateway): add timing telemetry to router` |
| 4 | `chore(helm): add execTimeout configuration for gateway` |
| 5 | `test(k8s): add exec timeout configuration tests` |

---

## Create Pull Request

```bash
gh pr create \
  --title "fix(gateway): align HTTP and exec timeouts to prevent race conditions" \
  --body "Fixes #113

## Summary

Aligns HTTP WriteTimeout (90s) with exec timeout (60s) to prevent race
conditions that cause 'socket hang up' errors. Makes exec timeout
configurable via EXEC_TIMEOUT environment variable.

## Changes

- Increase HTTP WriteTimeout from 30s to 90s
- Increase default exec timeout from 30s to 60s
- Add EXEC_TIMEOUT env var support for runtime configuration
- Add timing telemetry for debugging slow exec operations
- Update Helm chart with gateway.execTimeout value

## Testing

- [ ] Unit tests pass
- [ ] Manual E2E test with real cluster
- [ ] Verified no 'socket hang up' errors under load

## Related

- Parent epic: #112
- Analysis: docs/reports/e2e-failure-analysis.md" \
  --label "kind/bug" \
  --label "prio/p1-high"
```

---

## Success Criteria

| Metric | Before | After |
|--------|--------|-------|
| Race condition window | 0s (guaranteed race) | 30s buffer |
| Exec timeout | 30s (often too short) | 60s (configurable) |
| Timeout visibility | None | Full timing logs |

---

## Related Files

- `pkg/mcp/http.go` - HTTP server configuration
- `pkg/k8s/client.go` - Kubernetes exec client
- `pkg/gateway/router.go` - Request routing
- `deployment/helm/k8s-gpu-mcp-server/values.yaml` - Helm values

---

**Reply "GO" when ready to start implementation.** 🚀
