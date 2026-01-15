# Audit Remediation Plan

**Based on:** `AUDIT_REPORT.md`  
**Created:** 2026-01-15  
**Target Completion:** TBD  

---

## Overview

This plan breaks down each audit finding into atomic, actionable tasks.
Tasks are organized by priority and include acceptance criteria.

**Task Notation:**
- `[ ]` - Not started
- `[~]` - In progress
- `[x]` - Completed
- `[!]` - Blocked

---

## Phase 1: Critical Issues (P0)

### C1: Race Condition in Real NVML State

**File:** `pkg/nvml/real.go`  
**Risk:** Data corruption, unpredictable behavior under concurrent access

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| C1.1 | Add `sync.Mutex` field to `Real` struct | Struct has `mu sync.Mutex` field |
| C1.2 | Add mutex lock/unlock in `Init()` method | `Init()` acquires lock before reading `initialized` |
| C1.3 | Add mutex lock/unlock in `Shutdown()` method | `Shutdown()` acquires lock before reading `initialized` |
| C1.4 | Add unit test for concurrent Init calls | Test passes with `-race` flag |
| C1.5 | Add unit test for concurrent Shutdown calls | Test passes with `-race` flag |

**Steps:**
```
[ ] C1.1 - Add sync.Mutex to Real struct
[ ] C1.2 - Protect Init() with mutex
[ ] C1.3 - Protect Shutdown() with mutex
[ ] C1.4 - Write test: TestReal_ConcurrentInit
[ ] C1.5 - Write test: TestReal_ConcurrentShutdown
[ ] C1.6 - Run `go test -race ./pkg/nvml/...`
```

---

### C2: HTTP Server Ready Signal Race

**File:** `pkg/mcp/http.go`  
**Risk:** Consumers may believe server is ready when it actually failed to start

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| C2.1 | Delay ready signal until server confirms listening | `ready` channel only closes after successful bind |
| C2.2 | Handle synchronous ListenAndServe failures | Errors before listen return immediately |
| C2.3 | Add test for port-in-use scenario | Test verifies ready is not signaled on bind failure |

**Steps:**
```
[ ] C2.1 - Refactor ListenAndServe to delay ready signal
[ ] C2.2 - Add select with short timeout before signaling ready
[ ] C2.3 - Write test: TestHTTPServer_ReadyOnBindFailure
[ ] C2.4 - Verify existing tests still pass
```

---

### C3: Missing Response Body Close Error Handling

**File:** `pkg/gateway/http_client.go`  
**Risk:** Silent failures mask connection issues

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| C3.1 | Replace `_ = resp.Body.Close()` with logged close | Close errors logged at V(4) level |

**Steps:**
```
[ ] C3.1 - Update defer block in doRequest()
[ ] C3.2 - Verify no test regressions
```

---

## Phase 2: Major Issues (P1)

### M1: Missing Timeout on Stdio Transport Scanner

**File:** `pkg/mcp/oneshot.go`  
**Risk:** Goroutine hangs indefinitely if stdin blocks

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M1.1 | Create deadline-aware reader wrapper | Reader returns error on timeout |
| M1.2 | Wrap stdin with deadline reader | Scanner respects context timeout |
| M1.3 | Add configuration for read timeout | Timeout configurable via OneshotConfig |
| M1.4 | Add test for stdin timeout scenario | Test verifies timeout behavior |

**Steps:**
```
[ ] M1.1 - Define deadlineReader type in oneshot.go
[ ] M1.2 - Implement Read() with timeout select
[ ] M1.3 - Add ReadTimeout field to OneshotConfig
[ ] M1.4 - Update NewOneshotTransport to wrap reader
[ ] M1.5 - Write test: TestOneshotTransport_StdinTimeout
[ ] M1.6 - Update documentation
```

---

### M2: Unbounded Memory Growth in Gateway Results

**File:** `pkg/gateway/router.go`  
**Risk:** Memory pressure in large clusters

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M2.1 | Add MaxConcurrentRequests config to Router | Configurable concurrency limit |
| M2.2 | Implement semaphore-based worker pool | Concurrent requests bounded |
| M2.3 | Use buffered channel for results | Results collected via channel |
| M2.4 | Add metrics for concurrent request count | Prometheus gauge tracks active requests |

**Steps:**
```
[ ] M2.1 - Add MaxConcurrentRequests to RouterOption
[ ] M2.2 - Create semaphore channel in RouteToAllNodes
[ ] M2.3 - Refactor goroutine spawning to use semaphore
[ ] M2.4 - Replace mutex-protected slice with results channel
[ ] M2.5 - Add mcp_gateway_concurrent_requests gauge
[ ] M2.6 - Write test: TestRouter_BoundedConcurrency
[ ] M2.7 - Write benchmark: BenchmarkRouteToAllNodes
```

---

### M3: Hardcoded Service Name

**File:** `pkg/k8s/client.go`  
**Risk:** DNS routing fails with custom Helm release names

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M3.1 | Add env var lookup for service name | `GPU_MCP_SERVICE_NAME` env var supported |
| M3.2 | Update GPUNode to use configurable service name | ServiceName populated from config |
| M3.3 | Document the environment variable | README and quickstart updated |

**Steps:**
```
[ ] M3.1 - Create getEnvOrDefault helper function
[ ] M3.2 - Update DefaultServiceName initialization
[ ] M3.3 - Update NewClient to accept service name option
[ ] M3.4 - Add WithServiceName ClientOption
[ ] M3.5 - Write test: TestClient_CustomServiceName
[ ] M3.6 - Update docs/quickstart.md
[ ] M3.7 - Update Helm chart values.yaml with note
```

---

### M4: Global Mutable State in Metrics

**File:** `pkg/metrics/metrics.go`  
**Risk:** Tests cannot reset metrics, multiple instances conflict

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M4.1 | Create MetricsRegistry struct | All metrics encapsulated in struct |
| M4.2 | Add NewMetricsRegistry constructor | Accepts prometheus.Registerer |
| M4.3 | Update all metric consumers | Use registry instead of globals |
| M4.4 | Keep global registry for backward compat | Default registry for simple use |

**Steps:**
```
[ ] M4.1 - Define MetricsRegistry struct
[ ] M4.2 - Move metric definitions into struct fields
[ ] M4.3 - Create NewMetricsRegistry(reg prometheus.Registerer)
[ ] M4.4 - Create DefaultRegistry using promauto
[ ] M4.5 - Update RecordRequest to use registry method
[ ] M4.6 - Update SetNodeHealth to use registry method
[ ] M4.7 - Update SetCircuitState to use registry method
[ ] M4.8 - Update RecordGatewayRequest to use registry method
[ ] M4.9 - Update HTTP server to use configurable registry
[ ] M4.10 - Write test with custom registry
```

---

### M5: Missing Input Validation on Node Name

**File:** `pkg/tools/describe_gpu_node.go`  
**Risk:** Malformed input could cause API issues or log injection

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M5.1 | Create isValidNodeName validator | Validates RFC 1123 subdomain format |
| M5.2 | Add validation in describe_gpu_node handler | Invalid names return error |
| M5.3 | Add validation in pod_gpu_allocation handler | Consistent validation across tools |
| M5.4 | Add validation tests | Edge cases covered |

**Steps:**
```
[ ] M5.1 - Create pkg/tools/validation.go
[ ] M5.2 - Implement isValidNodeName with regex
[ ] M5.3 - Add length check (max 253 chars)
[ ] M5.4 - Update describe_gpu_node.go Handle()
[ ] M5.5 - Update pod_gpu_allocation.go Handle()
[ ] M5.6 - Write test: TestValidation_NodeName
[ ] M5.7 - Write test: TestDescribeGPUNode_InvalidNodeName
```

---

### M6: Potential Panic on Nil Device

**File:** `pkg/tools/gpu_health.go`  
**Risk:** Nil pointer dereference causes panic

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M6.1 | Add nil check after GetDeviceByIndex | Nil device logged and skipped |
| M6.2 | Add same check in gpu_inventory.go | Consistent nil handling |
| M6.3 | Add same check in describe_gpu_node.go | Consistent nil handling |

**Steps:**
```
[ ] M6.1 - Add nil check in gpu_health.go collectGPUHealth loop
[ ] M6.2 - Add nil check in gpu_inventory.go Handle loop
[ ] M6.3 - Add nil check in describe_gpu_node.go collectGPUInfo loop
[ ] M6.4 - Add nil check in analyze_xid.go findGPUByPCI loop
[ ] M6.5 - Write test with mock returning nil device
```

---

### M7: Missing Graceful Shutdown for Stdio Server

**File:** `pkg/mcp/server.go`  
**Risk:** Stdio server cannot be cleanly stopped

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M7.1 | Research mcp-go library shutdown support | Document findings |
| M7.2 | Implement stdin wrapper with close capability | Wrapper can signal EOF |
| M7.3 | Update runStdio to use closeable stdin | Context cancel closes stdin |

**Steps:**
```
[ ] M7.1 - Review mcp-go ServeStdio implementation
[ ] M7.2 - Check for shutdown hooks or context support
[ ] M7.3 - If no support, create closeable reader wrapper
[ ] M7.4 - Implement Close() to signal EOF
[ ] M7.5 - Update runStdio to close wrapper on context done
[ ] M7.6 - Write test: TestServer_StdioGracefulShutdown
[ ] M7.7 - If needed, file issue with mcp-go
```

---

### M8: File Handle Leak Risk in KmsgReader

**File:** `pkg/xid/kmsg.go`  
**Risk:** File handle leak if goroutine outlives function

| Task | Description | Acceptance Criteria |
|------|-------------|---------------------|
| M8.1 | Add goroutine cleanup wait | Function waits for goroutine before returning |
| M8.2 | Add timeout for goroutine cleanup | Max 100ms wait to avoid blocking |

**Steps:**
```
[ ] M8.1 - Update select in ReadMessages to wait for done
[ ] M8.2 - Add secondary timeout select for cleanup
[ ] M8.3 - Log warning if goroutine doesn't exit cleanly
[ ] M8.4 - Write test: TestKmsgReader_GoroutineCleanup
```

---

## Phase 3: Minor Issues (P2)

### N1: Magic Numbers in Health Score Calculation

**File:** `pkg/tools/gpu_health.go`

**Steps:**
```
[ ] N1.1 - Define constants for all score penalties
[ ] N1.2 - Replace inline numbers with constants
[ ] N1.3 - Add godoc comments explaining penalty rationale
```

---

### N2: TODO Without Issue Reference

**File:** `pkg/mcp/http.go:131`

**Steps:**
```
[ ] N2.1 - Create GitHub issue for NVML readiness check
[ ] N2.2 - Update TODO comment with issue number
```

---

### N3: Exported Type Without Documentation

**File:** `pkg/gateway/router.go`

**Steps:**
```
[ ] N3.1 - Enhance NodeResult godoc comment
[ ] N3.2 - Review other exported types for missing docs
[ ] N3.3 - Run godoc linter to find gaps
```

---

### N4: Unused ready Channel Field

**File:** `pkg/mcp/http.go`

**Steps:**
```
[ ] N4.1 - Add Ready() method to expose channel
[ ] N4.2 - Or remove if truly unused after C2 fix
[ ] N4.3 - Add test using Ready() for synchronization
```

---

### N5: Inconsistent Context Parameter Naming

**Files:** Various

**Steps:**
```
[ ] N5.1 - Audit all files for context naming patterns
[ ] N5.2 - Document preferred pattern in .cursor/rules
[ ] N5.3 - Optionally standardize (low priority)
```

---

### N6: Error Wrapping Consistency Check

**Files:** Various

**Steps:**
```
[ ] N6.1 - Grep for `fmt.Errorf.*%s.*err`
[ ] N6.2 - Replace %s with %w where appropriate
[ ] N6.3 - Verify error chain preservation
```

---

## Verification Checklist

After all fixes:

```
[ ] All unit tests pass: `go test ./... -count=1`
[ ] Race detector clean: `go test ./... -race`
[ ] Linter clean: `golangci-lint run`
[ ] Build succeeds: `make build`
[ ] Integration tests pass (if available)
[ ] Manual smoke test in cluster
```

---

## Progress Tracking

| Phase | Total | Done | Remaining |
|-------|-------|------|-----------|
| P0 Critical | 3 | 0 | 3 |
| P1 Major | 8 | 0 | 8 |
| P2 Minor | 6 | 0 | 6 |
| **Total** | **17** | **0** | **17** |

---

## Notes

- Each task should be a separate commit
- Use conventional commit format: `fix(pkg): description`
- Reference this plan in PR description
- Update progress tracking as tasks complete
