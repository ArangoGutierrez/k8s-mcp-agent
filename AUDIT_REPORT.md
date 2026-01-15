# Production Readiness Audit Report

**Project:** k8s-gpu-mcp-server  
**Audit Date:** 2026-01-15  
**Auditor:** Senior Go Reliability Engineer / Kubernetes Architect  
**Branch:** `audit/production-readiness`

---

## Executive Summary

This audit evaluates the k8s-gpu-mcp-server codebase against three pillars:
1. Effective Go & Best Practices
2. Defensive Programming Patterns
3. Kubernetes Production Readiness

**Overall Assessment:** The codebase demonstrates solid engineering practices with
good use of context propagation, error wrapping, and interface-based design.
However, several issues require attention before production deployment.

| Severity | Count |
|----------|-------|
| Critical | 3     |
| Major    | 8     |
| Minor    | 6     |

---

## [Critical] Immediate Action Required

### 1. Goroutine Leak in HTTP Server Startup

**File:** `pkg/mcp/http.go:77-84`

**Issue:** The goroutine that signals server readiness via `close(h.ready)` runs
immediately when the server starts, but if `ListenAndServe` fails synchronously
(e.g., port already in use), the error is sent to `errCh` after `close(h.ready)`
has already signaled readiness. This creates a race condition where consumers
of `h.ready` may believe the server is ready when it actually failed.

**Fix:**
```go
// Start server in goroutine
errCh := make(chan error, 1)
go func() {
    err := h.httpServer.ListenAndServe()
    if err != http.ErrServerClosed {
        errCh <- err
    }
    close(errCh)
}()

// Signal ready only after confirming server is listening
// Use a brief delay or check for actual bind
select {
case err := <-errCh:
    return err
case <-time.After(50 * time.Millisecond):
    close(h.ready)
}
```

---

### 2. Missing Response Body Close Error Handling

**File:** `pkg/gateway/http_client.go:112-115`

**Issue:** The response body close error is explicitly discarded with `_ = resp.Body.Close()`.
While this is common practice, in a production system handling GPU diagnostics,
a close failure could indicate connection issues or resource exhaustion that
should be logged.

```go
defer func() {
    _ = resp.Body.Close()  // Error silently discarded
}()
```

**Fix:**
```go
defer func() {
    if err := resp.Body.Close(); err != nil {
        klog.V(4).InfoS("failed to close response body", "error", err)
    }
}()
```

---

### 3. Race Condition in Real NVML State

**File:** `pkg/nvml/real.go:16-46`

**Issue:** The `Real` struct has an `initialized` bool field that is read and written
without synchronization. In concurrent access scenarios (unlikely but possible
if `Init` is called from multiple goroutines), this creates a data race.

```go
type Real struct {
    initialized bool  // Not protected by mutex
}

func (r *Real) Init(ctx context.Context) error {
    // ...
    if r.initialized {  // Read without lock
        return nil
    }
    // ...
    r.initialized = true  // Write without lock
    return nil
}
```

**Fix:**
```go
type Real struct {
    mu          sync.Mutex
    initialized bool
}

func (r *Real) Init(ctx context.Context) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if r.initialized {
        return nil
    }
    // ... initialization logic ...
    r.initialized = true
    return nil
}
```

---

## [Major] Production Risks

### 1. Missing Timeout on Stdio Transport Scanner

**File:** `pkg/mcp/oneshot.go:88-120`

**Issue:** The `bufio.Scanner` in oneshot transport has no inherent timeout. If stdin
blocks indefinitely (e.g., client hangs), the goroutine will never exit. While
context cancellation is checked, `scanner.Scan()` is a blocking call that won't
respect context cancellation.

**Fix:**
```go
// Wrap stdin with a deadline-aware reader
type deadlineReader struct {
    r       io.Reader
    timeout time.Duration
}

func (d *deadlineReader) Read(p []byte) (n int, err error) {
    // Use select with timeout channel
    done := make(chan struct{})
    var readN int
    var readErr error
    go func() {
        readN, readErr = d.r.Read(p)
        close(done)
    }()
    
    select {
    case <-done:
        return readN, readErr
    case <-time.After(d.timeout):
        return 0, context.DeadlineExceeded
    }
}
```

---

### 2. Unbounded Memory Growth in Gateway Results Aggregation

**File:** `pkg/gateway/router.go:276-328`

**Issue:** When routing to all nodes, results are collected in a slice that grows
unbounded. For clusters with many GPU nodes, this could cause memory pressure.
Additionally, results are appended under a mutex lock, which could cause
contention with many concurrent node responses.

**Fix:** Consider using a bounded worker pool pattern:
```go
// Use semaphore for bounded concurrency
sem := make(chan struct{}, 10) // Max 10 concurrent requests
results := make(chan NodeResult, len(nodes))

for _, node := range nodes {
    sem <- struct{}{} // Acquire
    go func(n k8s.GPUNode) {
        defer func() { <-sem }() // Release
        // ... process node ...
        results <- result
    }(node)
}
```

---

### 3. Hardcoded Service Name Limitation

**File:** `pkg/k8s/client.go:192`

**Issue:** The `DefaultServiceName` is hardcoded to `"gpu-mcp-k8s-gpu-mcp-server"`.
This will break DNS-based routing for deployments with custom release names or
`fullnameOverride`. The code comment acknowledges this limitation.

```go
const DefaultServiceName = "gpu-mcp-k8s-gpu-mcp-server"
```

**Fix:**
```go
// Configurable via environment variable
var DefaultServiceName = getEnvOrDefault(
    "GPU_MCP_SERVICE_NAME", 
    "gpu-mcp-k8s-gpu-mcp-server",
)

func getEnvOrDefault(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

---

### 4. Global Mutable State in Metrics Package

**File:** `pkg/metrics/metrics.go:12-76`

**Issue:** All metrics are registered globally via `promauto`. While this is standard
for Prometheus, it means:
1. Metrics cannot be reset between tests
2. Multiple instances in the same process will conflict
3. No way to disable metrics collection

The metrics are initialized at package load time, before any configuration
can be applied.

**Fix:** Consider using a registry pattern:
```go
type MetricsRegistry struct {
    RequestsTotal        *prometheus.CounterVec
    RequestDuration      *prometheus.HistogramVec
    // ...
}

func NewMetricsRegistry(reg prometheus.Registerer) *MetricsRegistry {
    m := &MetricsRegistry{}
    m.RequestsTotal = prometheus.NewCounterVec(...)
    reg.MustRegister(m.RequestsTotal)
    return m
}
```

---

### 5. Missing Input Validation on Node Name

**File:** `pkg/tools/describe_gpu_node.go:149-151`

**Issue:** The `node_name` argument is checked for empty string but not sanitized.
A malicious or malformed node name could potentially cause issues with
Kubernetes API calls or log injection.

```go
nodeName, ok := args["node_name"].(string)
if !ok || nodeName == "" {
    return mcp.NewToolResultError("node_name is required"), nil
}
// nodeName used directly without validation
```

**Fix:**
```go
nodeName, ok := args["node_name"].(string)
if !ok || nodeName == "" {
    return mcp.NewToolResultError("node_name is required"), nil
}

// Validate node name format (RFC 1123 subdomain)
if !isValidNodeName(nodeName) {
    return mcp.NewToolResultError(
        "invalid node_name: must be a valid DNS subdomain"), nil
}

func isValidNodeName(name string) bool {
    // Node names must be valid DNS subdomains
    const dns1123SubdomainRegex = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    matched, _ := regexp.MatchString(dns1123SubdomainRegex, name)
    return matched && len(name) <= 253
}
```

---

### 6. Potential Panic on Nil Device in GPU Health Collection

**File:** `pkg/tools/gpu_health.go:149-155`

**Issue:** If `GetDeviceByIndex` returns a nil device without an error (which
shouldn't happen with correct NVML implementation, but is possible with
custom mocks or future changes), the subsequent method calls will panic.

```go
device, err := h.nvmlClient.GetDeviceByIndex(ctx, i)
if err != nil {
    klog.ErrorS(err, "failed to get device", "index", i)
    continue
}
// device could theoretically be nil if implementation is buggy
health := h.collectGPUHealth(ctx, i, device)  // Would panic
```

**Fix:**
```go
device, err := h.nvmlClient.GetDeviceByIndex(ctx, i)
if err != nil {
    klog.ErrorS(err, "failed to get device", "index", i)
    continue
}
if device == nil {
    klog.ErrorS(nil, "GetDeviceByIndex returned nil device", "index", i)
    continue
}
```

---

### 7. Missing Graceful Shutdown for Stdio Server

**File:** `pkg/mcp/server.go:222-238`

**Issue:** In stdio mode (non-oneshot), the `server.ServeStdio` call runs in a
goroutine, but there's no mechanism to cleanly stop it. When context is
cancelled, `s.Shutdown()` is called but it only logs - it doesn't actually
stop the stdio server since mcp-go doesn't expose a shutdown method.

```go
go func() {
    if err := server.ServeStdio(s.mcpServer); err != nil {
        errCh <- fmt.Errorf("MCP server error: %w", err)
    }
}()
// ...
case <-ctx.Done():
    klog.InfoS("MCP server stopping", "reason", "context cancelled")
    return s.Shutdown()  // This doesn't actually stop ServeStdio
```

**Fix:** This requires coordination with the mcp-go library. Consider:
1. Using `os.Stdin` with a wrapper that can be closed
2. Setting read deadlines on stdin
3. Filing an issue/PR with mcp-go for graceful shutdown support

---

### 8. File Handle Leak Risk in KmsgReader

**File:** `pkg/xid/kmsg.go:58-66`

**Issue:** If the `file.Seek` call fails, the deferred close will still run, but
there's no guarantee the file descriptor is in a valid state. More critically,
the goroutine spawned for reading (lines 82-101) could potentially hold
references to the file after the function returns if the timeout triggers.

```go
file, err := os.OpenFile(r.path, os.O_RDONLY, 0)
if err != nil {
    // ...
}
defer func() { _ = file.Close() }()
// ...
go func() {
    defer close(done)
    for scanner.Scan() {  // Still reading from file
        // ...
    }
}()

select {
case <-done:
case <-readCtx.Done():
    // Returns, file is closed by defer, but goroutine may still be reading!
}
```

**Fix:**
```go
// Ensure goroutine cleanup before returning
select {
case <-done:
    // Normal completion
case <-readCtx.Done():
    // Timeout - wait briefly for goroutine to notice
    select {
    case <-done:
    case <-time.After(100 * time.Millisecond):
        klog.V(2).InfoS("kmsg reader goroutine didn't exit cleanly")
    }
}
```

---

## [Minor] Code Hygiene & Idiomatic Go

### 1. Inconsistent Error Wrapping Style

**Files:** Various

**Issue:** Some errors use `fmt.Errorf("message: %w", err)` while others use
`fmt.Errorf("message: %s", err)`. The `%w` verb should be used consistently
to preserve error chains.

**Locations:**
- `pkg/xid/parser.go:141` uses `%w`
- `pkg/k8s/client.go:289` uses `%w`
- Consistent usage throughout (GOOD)

---

### 2. Magic Numbers in Health Score Calculation

**File:** `pkg/tools/gpu_health.go:569-678`

**Issue:** Health score penalties are hardcoded as magic numbers throughout the
function. While there are some constants defined (lines 237-246), the score
deductions (30, 20, 10, etc.) are inline.

**Fix:** Define constants for all score penalties:
```go
const (
    scorePenaltyTempCritical    = 30
    scorePenaltyTempHigh        = 20
    scorePenaltyTempElevated    = 10
    scorePenaltyMemoryCritical  = 20
    scorePenaltyMemoryHigh      = 10
    // ... etc
)
```

---

### 3. TODO Comments Without Issue References

**Files:**
- `pkg/tools/gpu_health.go:237` - `TODO(#68)` - Good, has reference
- `pkg/tools/gpu_health.go:345` - `TODO(#69)` - Good, has reference
- `pkg/mcp/http.go:131` - `TODO: Check NVML initialization status` - Missing reference

**Issue:** The TODO at `pkg/mcp/http.go:131` doesn't have an issue reference,
making it easy to forget.

**Fix:** Create a tracking issue and reference it:
```go
// TODO(#XX): Check NVML initialization status for more accurate readiness
```

---

### 4. Exported Type Without Documentation

**File:** `pkg/gateway/router.go:82-87`

**Issue:** `NodeResult` is an exported type but lacks a package-level comment
explaining its purpose in the context of the gateway routing system.

```go
// NodeResult holds the result from a single node.
type NodeResult struct {
```

**Fix:** Add more context:
```go
// NodeResult holds the result from a single node agent request.
// It is returned by RouteToNode and RouteToAllNodes, containing either
// the raw MCP response bytes or an error message if the request failed.
type NodeResult struct {
```

---

### 5. Unused ready Channel Field

**File:** `pkg/mcp/http.go:23`

**Issue:** The `ready` channel in `HTTPServer` is created but never exposed or
used externally. If it's meant for testing, it should be accessible.

```go
type HTTPServer struct {
    // ...
    ready      chan struct{}  // Created but not exposed
}
```

**Fix:** Either remove if unused, or expose via method:
```go
// Ready returns a channel that is closed when the server is ready.
func (h *HTTPServer) Ready() <-chan struct{} {
    return h.ready
}
```

---

### 6. Inconsistent Context Parameter Naming

**Files:** Various

**Issue:** Most functions use `ctx` for context, but some inner scopes shadow
it with `readCtx`, `execCtx`, etc. While not incorrect, it can be confusing.
Consider using `parentCtx` for the outer context when creating child contexts.

**Example from** `pkg/xid/kmsg.go:77`:
```go
func (r *KmsgReader) ReadMessages(ctx context.Context) ([]string, error) {
    // ...
    readCtx, cancel := context.WithTimeout(ctx, kmsgReadTimeout)
```

This pattern is acceptable but should be consistent across the codebase.

---

## Kubernetes Production Readiness Checklist

| Requirement | Status | Notes |
|-------------|--------|-------|
| Graceful Shutdown (SIGTERM) | ✅ | Implemented in `cmd/agent/main.go:181-183` |
| Liveness Probe | ✅ | `/healthz` endpoint in `pkg/mcp/http.go:51` |
| Readiness Probe | ⚠️ | `/readyz` exists but doesn't check NVML status |
| Structured Logging | ✅ | Using klog/v2 with structured logging |
| Prometheus Metrics | ✅ | `/metrics` endpoint with custom metrics |
| No Hardcoded Secrets | ✅ | Configuration via env vars and flags |
| Resource Limits | N/A | Defined in Helm chart |
| Network Policies | ✅ | Defined in `deployment/helm/.../networkpolicy.yaml` |

---

## Recommendations Summary

### Immediate (Before Production)
1. Fix NVML state race condition
2. Add logging for response body close errors
3. Fix HTTP server ready signal race

### Short-term (Next Sprint)
1. Add proper stdio server shutdown mechanism
2. Implement bounded concurrency for multi-node routing
3. Add node name input validation
4. Make service name configurable

### Long-term (Technical Debt)
1. Consider metrics registry pattern for testability
2. Add issue references to all TODOs
3. Standardize health score penalty constants
4. Improve readiness probe to check NVML status

---

## Positive Observations

The codebase demonstrates several excellent practices:

1. **Interface-based Design:** The `nvml.Interface` abstraction enables clean
   testing and mock implementations.

2. **Context Propagation:** Consistent use of `context.Context` for cancellation
   and timeout handling throughout the codebase.

3. **Error Wrapping:** Proper use of `%w` verb in most error returns, preserving
   error chains for debugging.

4. **Defensive nil Checks:** Good defensive programming with nil checks in
   critical paths like `flattenGPUInfo()`.

5. **Circuit Breaker Pattern:** Well-implemented circuit breaker for resilient
   gateway-to-agent communication.

6. **Comprehensive Metrics:** Good observability with per-node, per-transport
   latency metrics.

7. **Clear Code Organization:** Clean separation between transport (stdio/HTTP),
   business logic (tools), and infrastructure (k8s, nvml).

---

*Report generated by production readiness audit automation.*
