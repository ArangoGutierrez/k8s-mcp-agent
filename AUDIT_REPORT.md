# Audit Report: Last 4 Commits

**Scope**: `aad2d6e..45ada53` (feat/tools, fix/gateway, docs/prompts, feat/rbac)  
**Date**: 2026-01-16  
**Auditor**: Go Reliability Engineer + K8s Architect

---

## [Critical] Immediate Action Required

*None identified.*

---

## [Major] Production Risk

### 1. Missing nil check in PodGPUAllocationHandler.Handle

- **File**: `pkg/tools/pod_gpu_allocation.go:106`
- **Issue**: `h.clientset.CoreV1().Pods().List()` called without checking if `clientset` is nil. The constructor at line 33 accepts nil clientset, but Handle() assumes it's always set. Will panic if clientset is nil.
- **Verification**: ✓ confirmed - grep shows direct call without nil guard (unlike `describe_gpu_node.go` which has `if h.clientset != nil` at lines 181, 216)
- **Risk**: Panic in production if handler is created without K8s client
- **Fix**:

```go
// Add nil check at start of Handle()
func (h *PodGPUAllocationHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    klog.InfoS("get_pod_gpu_allocation invoked")

    // Guard against nil clientset
    if h.clientset == nil {
        return mcp.NewToolResultError(
            "K8s client not configured - this tool requires cluster access"), nil
    }
    // ... rest of function
```

### 2. k8sError captured but not exposed in response

- **File**: `pkg/tools/describe_gpu_node.go:178,189,210,249`
- **Issue**: `k8sError` string is populated when K8s access fails (lines 189, 210) and used to set `status = "partial"` (line 249), but the error message itself is not included in the response JSON. Callers see `status: "partial"` but don't know *why*.
- **Verification**: ✓ confirmed - `GPUNodeDescription` struct (lines 37-45) has no field for error details; k8sError is only logged, not returned
- **Risk**: Operators cannot diagnose RBAC issues without checking agent logs
- **Fix**: Add error field to `GPUNodeDescription` or `NodeInfo`:

```go
// Option A: Add to GPUNodeDescription
type GPUNodeDescription struct {
    Status      string           `json:"status"`
    StatusError string           `json:"status_error,omitempty"` // NEW
    // ...
}

// Then at line 254:
response := GPUNodeDescription{
    Status:      status,
    StatusError: k8sError, // Include error detail
    // ...
}
```

---

## [Minor] Code Hygiene

### 1. Dead code: NodeInfoPartial struct never used

- **File**: `pkg/tools/describe_gpu_node.go:47-52`
- **Issue**: `NodeInfoPartial` struct is defined but never instantiated or referenced in the codebase
- **Verification**: ✓ confirmed - grep shows only definition at lines 47-48, no usage
- **Fix**: Remove unused type:

```go
// DELETE lines 47-52
// NodeInfoPartial is used when K8s API access fails but NVML data is available.
type NodeInfoPartial struct {
    Name           string `json:"name"`
    K8sUnavailable bool   `json:"k8s_unavailable,omitempty"`
    K8sError       string `json:"k8s_error,omitempty"`
}
```

### 2. Inconsistent graceful error handling pattern

- **File**: `pkg/tools/pod_gpu_allocation.go:106-124` vs `pkg/tools/describe_gpu_node.go:181-211`
- **Issue**: `describe_gpu_node.go` gracefully handles nil clientset and K8s errors by returning partial data. `pod_gpu_allocation.go` returns a structured error JSON but would panic if clientset is nil (see Major #1).
- **Verification**: ✓ confirmed - different patterns between the two handlers added in same feature
- **Fix**: Align patterns - add nil clientset guard to `pod_gpu_allocation.go`

---

## Positive Observations

✅ **Context propagation**: All I/O operations properly accept and check `ctx.Context`  
✅ **Error wrapping**: Consistent use of `fmt.Errorf("%w", err)` for error chains  
✅ **Concurrency safety**: `router.go:293-359` uses channels and WaitGroup correctly  
✅ **Graceful shutdown**: `main.go:177-185` properly handles SIGINT/SIGTERM  
✅ **Structured logging**: klog/v2 used throughout, no `fmt.Print` in production code  
✅ **Input validation**: Node names validated with `isValidNodeName()` at handler entry  
✅ **Nil safety in proxy.go**: `flattenGPUInfo()` has proper nil check at line 275-277  
✅ **Division-by-zero guard**: `calculateOverallHealth()` returns early if `len(gpus) == 0`  

---

## Verification Summary

| Metric | Count |
|--------|-------|
| Findings generated | 6 |
| ✓ Confirmed | 4 |
| ✗ Dropped | 1 (division-by-zero was already handled) |
| ? Manual-review | 1 (whether k8sError should be in response is a design decision) |

---

## Recommendations

1. **Immediate**: Add nil clientset guard to `pod_gpu_allocation.go` Handle() to prevent panic
2. **Short-term**: Either use `NodeInfoPartial` or delete it; current state is confusing
3. **Consider**: Expose k8sError in response JSON for operational visibility (may be intentional omission for security)
