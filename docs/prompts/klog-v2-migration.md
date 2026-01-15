# klog/v2 Structured Logging Migration

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/klog-v2-migration.md`, the agent MUST continue
> working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke the prompt:**
> `@docs/prompts/klog-v2-migration.md`

### Iteration Rules (For the Agent)

1. **NEVER STOP EARLY** - If any task is `[TODO]` or `[WIP]`, keep working
2. **UPDATE STATUS** - Edit this file: mark tasks `[WIP]` → `[DONE]` as you go
3. **COMMIT PROGRESS** - Commit and push after each completed task
4. **SELF-CHECK** - Before ending your turn, verify ALL tasks show `[DONE]`
5. **REPORT STATUS** - End each turn with a status summary of remaining tasks
6. **⚠️ MERGE REQUIRES HUMAN APPROVAL** - When ready to merge, STOP and ask for
   confirmation. Do NOT merge autonomously.

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch `refactor/klog-v2-logging` | `[DONE]` | |
| 1 | Add klog/v2 dependency to go.mod | `[DONE]` | |
| 2 | Initialize klog in cmd/agent/main.go | `[DONE]` | |
| 3 | Migrate cmd/agent/main.go log calls | `[DONE]` | ~18 calls |
| 4 | Migrate pkg/mcp/server.go | `[DONE]` | ~9 calls |
| 5 | Migrate pkg/mcp/http.go | `[DONE]` | ~5 calls |
| 6 | Migrate pkg/mcp/oneshot.go | `[DONE]` | ~7 calls |
| 7 | Migrate pkg/tools/gpu_inventory.go | `[DONE]` | ~11 calls |
| 8 | Migrate pkg/tools/gpu_health.go | `[DONE]` | ~21 calls |
| 9 | Migrate pkg/tools/analyze_xid.go | `[DONE]` | ~9 calls |
| 10 | Migrate pkg/tools/describe_gpu_node.go | `[DONE]` | ~11 calls |
| 11 | Migrate pkg/tools/pod_gpu_allocation.go | `[DONE]` | ~6 calls |
| 12 | Migrate pkg/gateway/router.go | `[DONE]` | ~13 calls |
| 13 | Migrate pkg/gateway/proxy.go | `[DONE]` | ~3 calls |
| 14 | Migrate pkg/gateway/tracing.go + http_client.go | `[DONE]` | ~2 calls |
| 15 | Migrate pkg/k8s/client.go | `[DONE]` | ~5 calls |
| 16 | Migrate pkg/xid/parser.go | `[DONE]` | ~4 calls |
| 17 | Remove unused LogToStderr helper | `[DONE]` | pkg/mcp/server.go |
| 18 | Run `make all` - verify tests pass | `[DONE]` | |
| 19 | Create pull request | `[WIP]` | |
| 20 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 21 | Address review comments | `[TODO]` | |
| 22 | **Merge after approval** | `[WAIT]` | ⚠️ Requires human |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[WAIT]` | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** [#42 - Replace log.Printf with klog/v2 structured logging](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/42)
- **Priority:** P1-High
- **Labels:** `kind/refactor`, `area/logging`, `production-ready`
- **Milestone:** M4: Safety & Release
- **Autonomous Mode:** ✅ Enabled

---

## Background

The codebase currently uses manual JSON log formatting via `log.Printf`:

```go
log.Printf(`{"level":"info","msg":"MCP server starting","mode":"%s"}`, s.mode)
```

This approach has several issues:
- **No runtime verbosity control** - cannot adjust log levels at runtime
- **No error-first signatures** - errors not consistently passed as first arg
- **String escaping bugs** - manual JSON formatting is error-prone
- **Not ecosystem-consistent** - doesn't follow Kubernetes logging conventions

The solution is to adopt `k8s.io/klog/v2` structured logging, as used by
NVIDIA's `k8s-dra-driver-gpu` and other Kubernetes components.

---

## Objective

Replace all `log.Printf` calls with `klog/v2` structured logging, enabling
runtime verbosity control and following Kubernetes ecosystem conventions.

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b refactor/klog-v2-logging
```

---

## Implementation Tasks

### Task 1: Add klog/v2 Dependency `[TODO]`

Add `k8s.io/klog/v2` as a direct dependency. It's already an indirect
dependency via `k8s.io/client-go`.

```bash
go get k8s.io/klog/v2
```

Verify in `go.mod`:
```go
require (
    k8s.io/klog/v2 v2.130.1  // should move from indirect to direct
)
```

---

### Task 2: Initialize klog in main.go `[TODO]`

Update `cmd/agent/main.go` to initialize klog:

```go
import (
    "flag"
    "k8s.io/klog/v2"
)

func main() {
    // Initialize klog flags (adds -v, -logtostderr, etc.)
    klog.InitFlags(nil)
    
    // Parse flags (after adding custom flags)
    flag.Parse()
    
    // Flush logs on exit
    defer klog.Flush()
    
    // ... rest of main
}
```

**Important:** The existing `log.Printf` output goes to stderr by default,
and klog also defaults to stderr. This maintains compatibility.

---

### Task 3-16: Migrate Log Calls

#### Conversion Patterns

| Before (JSON-formatted) | After (klog structured) |
|------------------------|-------------------------|
| `log.Printf(`{"level":"info","msg":"X","k":"v"}`)` | `klog.InfoS("X", "k", "v")` |
| `log.Printf(`{"level":"error","msg":"X","error":"%s"}`, err)` | `klog.ErrorS(err, "X")` |
| `log.Printf(`{"level":"debug","msg":"X"}`)` | `klog.V(4).InfoS("X")` |
| `log.Printf(`{"level":"warn","msg":"X"}`)` | `klog.V(2).InfoS("X")` or use warning keys |
| `log.Fatalf(`{"level":"fatal"...}`)` | `klog.ErrorS(nil, "X"); os.Exit(1)` |

#### Verbosity Level Mapping

| Level | V() | Use Case |
|-------|-----|----------|
| Error | - | Always shown, use `ErrorS()` |
| Warning | V(2) | Important warnings |
| Info | - | Standard operational info |
| Debug | V(4) | Detailed debugging |
| Trace | V(6) | Very detailed tracing |

#### Key Naming Conventions

Use consistent keys across the codebase:
- `"err"` → use `ErrorS(err, ...)` instead
- `"mode"` → operation mode
- `"node"` → node name
- `"pod"` → pod name
- `"gpu"` or `"gpuIndex"` → GPU index
- `"duration"` → timing duration
- `"transport"` → stdio/http
- `"tool"` → tool name
- `"count"` → counts
- `"status"` → status strings

#### Example Migrations

**Info log:**
```go
// Before
log.Printf(`{"level":"info","msg":"MCP server starting",`+
    `"transport":"stdio","mode":"%s"}`, s.mode)

// After
klog.InfoS("MCP server starting", "transport", "stdio", "mode", s.mode)
```

**Error log:**
```go
// Before
log.Printf(`{"level":"error","msg":"failed to get device count",`+
    `"error":"%s"}`, err)

// After
klog.ErrorS(err, "failed to get device count")
```

**Error log with context:**
```go
// Before
log.Printf(`{"level":"error","msg":"failed to get device",`+
    `"index":%d,"error":"%s"}`, i, err)

// After
klog.ErrorS(err, "failed to get device", "index", i)
```

**Debug log:**
```go
// Before
log.Printf(`{"level":"debug","msg":"routing to node","node":"%s",`+
    `"routing_mode":"%s"}`, nodeName, r.routingMode)

// After
klog.V(4).InfoS("routing to node", "node", nodeName, 
    "routingMode", r.routingMode)
```

**Warning log:**
```go
// Before
log.Printf(`{"level":"warn","msg":"circuit open, skipping node",`+
    `"node":"%s","state":"%s"}`, node.Name, state)

// After
klog.V(2).InfoS("circuit open, skipping node", "node", node.Name,
    "state", state)
```

**Fatal log (special handling):**
```go
// Before
log.Fatalf(`{"level":"fatal","msg":"invalid mode","mode":"%s",`+
    `"valid":["read-only","operator"]}`, *mode)

// After
klog.ErrorS(nil, "invalid mode", "mode", *mode,
    "valid", []string{"read-only", "operator"})
klog.Flush()
os.Exit(1)
```

---

### Task 17: Remove LogToStderr Helper `[TODO]`

Remove the now-unused `LogToStderr` function from `pkg/mcp/server.go`:

```go
// DELETE this function - no longer needed with klog
func LogToStderr(level, msg string, fields map[string]interface{}) {
    // ...
}
```

---

### Task 18: Run Tests `[TODO]`

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Full check suite
make all

# If any tests check for exact log output, they may need updating
go test -v ./...
```

---

## Files to Update (Complete Inventory)

| File | log.Printf Count | Priority |
|------|-----------------|----------|
| `cmd/agent/main.go` | ~18 | High (init + startup) |
| `pkg/mcp/server.go` | ~9 | High |
| `pkg/mcp/http.go` | ~5 | Medium |
| `pkg/mcp/oneshot.go` | ~7 | Medium |
| `pkg/tools/gpu_inventory.go` | ~11 | Medium |
| `pkg/tools/gpu_health.go` | ~21 | Medium |
| `pkg/tools/analyze_xid.go` | ~9 | Medium |
| `pkg/tools/describe_gpu_node.go` | ~11 | Medium |
| `pkg/tools/pod_gpu_allocation.go` | ~6 | Medium |
| `pkg/gateway/router.go` | ~13 | Medium |
| `pkg/gateway/proxy.go` | ~3 | Low |
| `pkg/gateway/tracing.go` | ~1 | Low |
| `pkg/gateway/http_client.go` | ~1 | Low |
| `pkg/k8s/client.go` | ~5 | Medium |
| `pkg/xid/parser.go` | ~4 | Low |

**Total:** ~124 log.Printf calls across 15 Go files

---

## Testing Requirements

### Unit Testing

```bash
# Run all checks
make all

# Run tests with race detector
make test

# Verify verbosity works
./bin/agent --help | grep -E '^\s+-v'
# Should show: -v, --v Level   number for the log level verbosity
```

### Manual Verification

```bash
# Build and run with different verbosity levels
make agent

# Default (info only)
echo '{"jsonrpc":"2.0","method":"initialize","id":1}' | ./bin/agent

# Debug verbosity (-v=4)
echo '{"jsonrpc":"2.0","method":"initialize","id":1}' | ./bin/agent -v=4 2>&1

# High verbosity (-v=6)
echo '{"jsonrpc":"2.0","method":"initialize","id":1}' | ./bin/agent -v=6 2>&1
```

---

## Pre-Commit Checklist

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass
- [ ] No `log.Printf` calls remain in Go files
- [ ] No manual JSON log formatting remains

---

## Commit Strategy

Use atomic commits for each file/package:

```bash
git commit -s -S -m "refactor(mcp): migrate logging to klog/v2 in server.go"
git commit -s -S -m "refactor(tools): migrate logging to klog/v2 in gpu_inventory.go"
git commit -s -S -m "refactor(tools): migrate logging to klog/v2 in gpu_health.go"
# etc.
```

Or batch by logical group:
```bash
git commit -s -S -m "refactor(logging): add klog/v2 initialization in main.go"
git commit -s -S -m "refactor(logging): migrate pkg/mcp to klog/v2"
git commit -s -S -m "refactor(logging): migrate pkg/tools to klog/v2"
git commit -s -S -m "refactor(logging): migrate pkg/gateway to klog/v2"
git commit -s -S -m "refactor(logging): migrate pkg/k8s to klog/v2"
```

---

## PR Creation

```bash
gh pr create \
  --title "refactor(logging): replace log.Printf with klog/v2 structured logging" \
  --body "Fixes #42

## Summary
Replaces manual JSON log formatting with k8s.io/klog/v2 structured logging.

## Changes
- Add klog/v2 as direct dependency
- Initialize klog in main.go with flag support
- Replace ~124 log.Printf calls with klog equivalents
- Enable runtime verbosity control (-v flag)
- Use ErrorS(err, ...) for error-first logging

## Verbosity Levels
- Default: info level logs
- -v=2: warnings
- -v=4: debug
- -v=6: trace

## Testing
- [x] Unit tests pass
- [x] Manual verification with different -v levels
- [x] Linting passes" \
  --label "kind/refactor" \
  --label "area/logging" \
  --label "production-ready" \
  --milestone "M4: Safety & Release"
```

---

## References

- [klog/v2 Documentation](https://pkg.go.dev/k8s.io/klog/v2)
- [Kubernetes Structured Logging Blog](https://kubernetes.io/blog/2020/09/04/kubernetes-1-19-introducing-structured-logs/)
- [Contextual Logging in Kubernetes](https://kubernetes.io/blog/2022/05/25/contextual-logging/)
- [Issue #42](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/42)

---

## Quick Reference

### klog Initialization Pattern
```go
import "k8s.io/klog/v2"

func main() {
    klog.InitFlags(nil)
    flag.Parse()
    defer klog.Flush()
    // ...
}
```

### klog Structured Logging Patterns
```go
// Info
klog.InfoS("message", "key1", value1, "key2", value2)

// Error (error-first)
klog.ErrorS(err, "message", "key1", value1)

// Debug (verbosity 4)
klog.V(4).InfoS("debug message", "key1", value1)

// Warning (verbosity 2)
klog.V(2).InfoS("warning message", "key1", value1)
```

---

**Reply "GO" when ready to start implementation.** 🚀
