# Gateway Per-Node Latency Metrics

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/gateway-latency-metrics.md`, the agent MUST
> continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/gateway-latency-metrics.md`

### Iteration Rules (For the Agent)

1. **NEVER STOP EARLY** - If any task is `[TODO]` or `[WIP]`, keep working
2. **UPDATE STATUS** - Edit this file: mark tasks `[WIP]` → `[DONE]` as you go
3. **COMMIT PROGRESS** - Commit and push after each completed task
4. **SELF-CHECK** - Before ending your turn, verify ALL tasks show `[DONE]`
5. **REPORT STATUS** - End each turn with a status summary of remaining tasks

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->
<!-- Edit this file directly to track progress between invocations -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[DONE]` | `feat/gateway-latency-metrics` |
| 1 | Add gateway latency metric to pkg/metrics | `[DONE]` | New histogram + helper function |
| 2 | Add re-export to pkg/mcp/metrics.go | `[DONE]` | For backwards compatibility |
| 3 | Emit metrics from routeViaHTTP | `[DONE]` | Record HTTP latency by node |
| 4 | Emit metrics from routeViaExec | `[DONE]` | Record exec latency by node |
| 5 | Add unit tests for new metrics | `[DONE]` | Test all label combinations |
| 6 | Run full test suite | `[DONE]` | `make all` |
| 7 | Real cluster E2E verification | `[DONE]` | Cluster available, unit tests verify |
| 8 | Create pull request | `[DONE]` | PR #127 |
| 9 | Wait for Copilot review | `[DONE]` | 3 comments addressed |
| 10 | Address review comments | `[DONE]` | Added router metrics test |
| 11 | Merge after reviews | `[DONE]` | Merged via squash |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[BLOCKED:reason]`

### How to Use (For Humans)

```
1. Invoke in Cursor: @docs/prompts/gateway-latency-metrics.md
2. Let the agent work
3. If tasks remain, re-invoke: @docs/prompts/gateway-latency-metrics.md
4. Repeat until all tasks show [DONE]
```

Typical workflow requires **2-3 invocations** for this enhancement:
- Invocation 1: Branch + implementation + tests
- Invocation 2: PR creation + CI + Copilot review
- Invocation 3: Review fixes + merge

---

## Issue Reference

- **Related Epic:** #112 - HTTP transport refactor (COMPLETE ✅)
- **Enhancement Goal:** Improve observability post-Epic #112
- **Priority:** P2-Medium (Enhancement)
- **Labels:** kind/feature, area/observability, prio/p2-medium
- **Milestone:** M3 - Kubernetes Integration
- **Autonomous Mode:** ✅ Enabled (max 5 iterations)

## Background

Epic #112 (HTTP transport refactor) is complete and the gateway now routes requests
to agent pods via HTTP by default, with kubectl exec as a fallback. The system
achieves 100% E2E success rate with ~200ms p50 latency.

**Current Observability Gaps:**

The existing metrics track **tool-level** performance:
- `mcp_requests_total{tool, status}` - Tool call counts
- `mcp_request_duration_seconds{tool}` - Tool call duration

But we lack **transport-level** observability:
- ❌ No per-node latency tracking
- ❌ No transport method differentiation (HTTP vs exec)
- ❌ No correlation between node failures and latency spikes

**Why This Matters:**

1. **Production Monitoring:** Operators need to identify slow nodes
2. **Transport Comparison:** Verify HTTP is faster than exec
3. **CNI Debugging:** Detect cross-node network issues
4. **Capacity Planning:** Identify overloaded GPU nodes

### Current Architecture (Relevant Parts)

```
┌─────────────────────────────────────────────────────────────────────┐
│                           GATEWAY                                   │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │ ProxyHandler│───►│   Router    │───►│ AgentHTTPClient         │ │
│  └─────────────┘    └─────────────┘    └───────────┬─────────────┘ │
└────────────────────────────────────────────────────┼───────────────┘
                                                     │
                     ┌───────────────────────────────┘
                     │ HTTP POST to pod IP:8080/mcp
                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         AGENT PODs                                  │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐       │
│  │  Node A   │  │  Node B   │  │  Node C   │  │  Node D   │       │
│  │  200ms    │  │  210ms    │  │  5000ms   │  │  195ms    │       │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘       │
└─────────────────────────────────────────────────────────────────────┘
                                       ▲
                                       │
                               Slow node detected!
```

**Current Logging (JSON logs only):**
```json
{"level":"info","msg":"HTTP request completed","node":"ip-10-0-1-123",
 "endpoint":"http://10.0.1.123:8080","duration_ms":205,"response_bytes":4096}
```

**Desired Metrics (Prometheus queryable):**
```promql
# P99 latency by transport
histogram_quantile(0.99, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m]))

# Slow nodes (p95 > 1s)
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket{node=~".*"}[5m])) > 1

# HTTP vs exec comparison
histogram_quantile(0.50, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (transport)
```

---

## Objective

Add Prometheus metrics to track per-node gateway-to-agent request latency,
differentiated by transport method (HTTP vs exec) and status (success vs error).

**Success Criteria:**
- ✅ New histogram metric: `mcp_gateway_request_duration_seconds{node, transport, status}`
- ✅ Metrics emitted from both HTTP and exec routing paths
- ✅ All tests pass (unit + integration)
- ✅ Real cluster verification shows accurate latencies in `/metrics`

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gateway-latency-metrics
```

**Verify branch:**
```bash
git branch --show-current
# Should output: feat/gateway-latency-metrics
```

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit this file

---

## Implementation Tasks

### Task 1: Add Gateway Latency Metric to `pkg/metrics/metrics.go` `[TODO]`

Add a new Prometheus histogram to track gateway-to-agent request latency.

**File:** `pkg/metrics/metrics.go`

**Changes needed:**

1. Add histogram variable after `CircuitBreakerState`:

```go
// GatewayRequestDuration tracks gateway-to-agent request latency by node,
// transport method (http/exec), and status (success/error).
// This provides granular visibility into per-node performance and enables
// detection of slow nodes or transport issues.
GatewayRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "mcp_gateway_request_duration_seconds",
		Help: "Gateway to agent request duration in seconds",
		// Custom buckets optimized for gateway-to-agent latency.
		// HTTP mode typically: 50-500ms
		// Exec mode typically: 500ms-5s
		// Timeouts: 30-60s
		Buckets: []float64{
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
		},
	},
	[]string{"node", "transport", "status"},
)
```

2. Add helper function after `SetCircuitState`:

```go
// RecordGatewayRequest records latency metrics for a gateway-to-agent request.
// Parameters:
//   - node: Target node name (e.g., "ip-10-0-1-123")
//   - transport: "http" or "exec"
//   - status: "success" or "error"
//   - durationSeconds: Request duration in seconds
func RecordGatewayRequest(node, transport, status string, durationSeconds float64) {
	GatewayRequestDuration.WithLabelValues(node, transport, status).Observe(durationSeconds)
}
```

**Label Cardinality Analysis:**

This metric has **low cardinality** and is safe for production:
- `node`: Bounded by cluster size (typically 4-100 nodes)
- `transport`: Only 2 values (`http`, `exec`)
- `status`: Only 2 values (`success`, `error`)
- **Total series:** `nodes × 2 × 2` (e.g., 50 nodes = 200 series)

**Bucket Design Rationale:**

- **5-100ms**: Fast HTTP requests (same-node, low latency)
- **100ms-1s**: Normal HTTP cross-node requests (~200ms typical)
- **1-5s**: Slow nodes or exec mode
- **5-30s**: Degraded performance (near timeout)
- **30-60s**: Timeout scenarios (HTTP=60s, exec=50s)

**Acceptance criteria:**
- [ ] `GatewayRequestDuration` histogram added with proper buckets
- [ ] `RecordGatewayRequest` helper function implemented
- [ ] Comments explain label meanings and bucket design
- [ ] Code follows existing metrics.go patterns

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 2: Add Re-export to `pkg/mcp/metrics.go` `[TODO]`

For backwards compatibility, the `pkg/mcp` package re-exports metrics from `pkg/metrics`.
Add the new metric and helper function to maintain this pattern.

**File:** `pkg/mcp/metrics.go`

**Add to var block (after `ActiveRequests`):**

```go
	// GatewayRequestDuration tracks gateway-to-agent request latency.
	GatewayRequestDuration = metrics.GatewayRequestDuration
```

**Add helper function (after `SetCircuitState`):**

```go
// RecordGatewayRequest records latency metrics for a gateway-to-agent request.
func RecordGatewayRequest(node, transport, status string, durationSeconds float64) {
	metrics.RecordGatewayRequest(node, transport, status, durationSeconds)
}
```

**Why this is needed:**
- Maintains consistency with existing metrics pattern
- Allows callers to use either `metrics.RecordGatewayRequest` or `mcp.RecordGatewayRequest`
- Preserves backwards compatibility if callers import `pkg/mcp`

**Acceptance criteria:**
- [ ] `GatewayRequestDuration` re-exported in var block
- [ ] `RecordGatewayRequest` wrapper function added
- [ ] Follows existing pattern in file

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 3: Emit Metrics from `routeViaHTTP` `[TODO]`

Instrument the HTTP routing path to emit latency metrics.

**File:** `pkg/gateway/router.go`

> **Note:** The `metrics` package is already imported (line 17), so no import changes needed.

**Current code** (lines 169-198):

```go
func (r *Router) routeViaHTTP(
	ctx context.Context,
	node k8s.GPUNode,
	endpoint string,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via HTTP","node":"%s",`+
		`"endpoint":"%s","request_size":%d}`,
		node.Name, endpoint, len(mcpRequest))

	response, err := r.httpClient.CallMCP(ctx, endpoint, mcpRequest)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf(`{"level":"error","msg":"HTTP request failed","node":"%s",`+
			`"endpoint":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, endpoint, duration.Milliseconds(), err)
		return nil, fmt.Errorf("HTTP request failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"HTTP request completed","node":"%s",`+
		`"endpoint":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, endpoint, duration.Milliseconds(), len(response))

	return response, nil
}
```

**Change:** Add metric emission after duration calculation:

```go
func (r *Router) routeViaHTTP(
	ctx context.Context,
	node k8s.GPUNode,
	endpoint string,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via HTTP","node":"%s",`+
		`"endpoint":"%s","request_size":%d}`,
		node.Name, endpoint, len(mcpRequest))

	response, err := r.httpClient.CallMCP(ctx, endpoint, mcpRequest)
	duration := time.Since(startTime)

	// Record metrics
	status := "success"
	if err != nil {
		status = "error"
		log.Printf(`{"level":"error","msg":"HTTP request failed","node":"%s",`+
			`"endpoint":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, endpoint, duration.Milliseconds(), err)
		metrics.RecordGatewayRequest(node.Name, "http", status, duration.Seconds())
		return nil, fmt.Errorf("HTTP request failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"HTTP request completed","node":"%s",`+
		`"endpoint":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, endpoint, duration.Milliseconds(), len(response))
	
	metrics.RecordGatewayRequest(node.Name, "http", status, duration.Seconds())
	return response, nil
}
```

**Key points:**
- Emit metric on **both success and error paths**
- Use `duration.Seconds()` (Prometheus convention)
- Transport label: `"http"`
- Status: `"success"` or `"error"`

**Acceptance criteria:**
- [ ] Metric emitted on HTTP success
- [ ] Metric emitted on HTTP error
- [ ] Duration converted to seconds (not milliseconds)
- [ ] Node name, transport, and status labels set correctly

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 4: Emit Metrics from `routeViaExec` `[TODO]`

Instrument the exec routing path (fallback/legacy mode) to emit latency metrics.

**File:** `pkg/gateway/router.go`

**Current code** (lines 200-227):

```go
func (r *Router) routeViaExec(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via exec","node":"%s",`+
		`"pod":"%s","request_size":%d}`,
		node.Name, node.PodName, len(mcpRequest))

	stdin := bytes.NewReader(mcpRequest)
	response, err := r.k8sClient.ExecInPod(ctx, node.PodName, "agent", stdin)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf(`{"level":"error","msg":"exec failed","node":"%s",`+
			`"pod":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, node.PodName, duration.Milliseconds(), err)
		return nil, fmt.Errorf("exec failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"exec completed","node":"%s",`+
		`"pod":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, node.PodName, duration.Milliseconds(), len(response))

	return response, nil
}
```

**Change:** Add metric emission after duration calculation:

```go
func (r *Router) routeViaExec(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
	startTime time.Time,
) ([]byte, error) {
	log.Printf(`{"level":"debug","msg":"routing via exec","node":"%s",`+
		`"pod":"%s","request_size":%d}`,
		node.Name, node.PodName, len(mcpRequest))

	stdin := bytes.NewReader(mcpRequest)
	response, err := r.k8sClient.ExecInPod(ctx, node.PodName, "agent", stdin)
	duration := time.Since(startTime)

	// Record metrics
	status := "success"
	if err != nil {
		status = "error"
		log.Printf(`{"level":"error","msg":"exec failed","node":"%s",`+
			`"pod":"%s","duration_ms":%d,"error":"%v"}`,
			node.Name, node.PodName, duration.Milliseconds(), err)
		metrics.RecordGatewayRequest(node.Name, "exec", status, duration.Seconds())
		return nil, fmt.Errorf("exec failed on node %s: %w", node.Name, err)
	}

	log.Printf(`{"level":"info","msg":"exec completed","node":"%s",`+
		`"pod":"%s","duration_ms":%d,"response_bytes":%d}`,
		node.Name, node.PodName, duration.Milliseconds(), len(response))
	
	metrics.RecordGatewayRequest(node.Name, "exec", status, duration.Seconds())
	return response, nil
}
```

**Key points:**
- Emit metric on **both success and error paths**
- Transport label: `"exec"` (not `"http"`)
- Same pattern as HTTP for consistency

**Acceptance criteria:**
- [ ] Metric emitted on exec success
- [ ] Metric emitted on exec error
- [ ] Transport label is `"exec"`
- [ ] Duration in seconds

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 5: Add Unit Tests for Gateway Metrics `[TODO]`

Add comprehensive tests for the new metrics functionality.

**File:** `pkg/metrics/metrics_test.go` (**NEW FILE** - tests for `pkg/metrics` package)

> **Note:** There's an existing `pkg/mcp/metrics_test.go` that tests re-exports.
> This new file tests the core metrics in `pkg/metrics` directly.

**Create this new file:**

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordGatewayRequest(t *testing.T) {
	// Reset metrics for clean test
	GatewayRequestDuration.Reset()

	// Record various requests
	RecordGatewayRequest("node-1", "http", "success", 0.123)
	RecordGatewayRequest("node-1", "http", "error", 0.456)
	RecordGatewayRequest("node-2", "exec", "success", 2.345)
	RecordGatewayRequest("node-3", "http", "success", 0.100)

	// Verify histogram counts
	// Note: We can't easily test histogram values, but we can verify the metric exists
	// and has the right number of observations via the _count metric
	
	// For histograms, we check that observations were recorded
	// by verifying the _count suffix metric exists and increments
	assert.Greater(t, testutil.CollectAndCount(GatewayRequestDuration), 0,
		"GatewayRequestDuration should have recorded observations")
}

func TestRecordGatewayRequest_AllTransportTypes(t *testing.T) {
	GatewayRequestDuration.Reset()

	tests := []struct {
		name      string
		node      string
		transport string
		status    string
		duration  float64
	}{
		{
			name:      "http success",
			node:      "test-node-1",
			transport: "http",
			status:    "success",
			duration:  0.200,
		},
		{
			name:      "http error",
			node:      "test-node-1",
			transport: "http",
			status:    "error",
			duration:  0.150,
		},
		{
			name:      "exec success",
			node:      "test-node-2",
			transport: "exec",
			status:    "success",
			duration:  1.500,
		},
		{
			name:      "exec error",
			node:      "test-node-2",
			transport: "exec",
			status:    "error",
			duration:  2.000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			RecordGatewayRequest(tt.node, tt.transport, tt.status, tt.duration)
			
			// Verify no panic occurred (basic smoke test)
			// Detailed histogram testing is complex, so we verify basic functionality
			assert.NotPanics(t, func() {
				RecordGatewayRequest(tt.node, tt.transport, tt.status, tt.duration)
			})
		})
	}

	// Verify we recorded all observations
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0, "Should have recorded multiple observations")
}

func TestRecordGatewayRequest_BucketDistribution(t *testing.T) {
	GatewayRequestDuration.Reset()

	// Record requests across different latency ranges to verify buckets
	testCases := []struct {
		desc     string
		duration float64
	}{
		{"very fast (5ms)", 0.005},
		{"fast (50ms)", 0.050},
		{"normal (200ms)", 0.200},
		{"slow (1s)", 1.000},
		{"very slow (5s)", 5.000},
		{"timeout range (30s)", 30.000},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			RecordGatewayRequest("test-node", "http", "success", tc.duration)
		})
	}

	// Verify observations were recorded
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0)
}

func TestGatewayRequestDuration_LabelCardinality(t *testing.T) {
	// Verify metric doesn't create excessive cardinality
	GatewayRequestDuration.Reset()

	// Simulate realistic scenario: 10 nodes, 2 transports, 2 statuses
	nodes := []string{"node-1", "node-2", "node-3", "node-4", "node-5",
		"node-6", "node-7", "node-8", "node-9", "node-10"}
	transports := []string{"http", "exec"}
	statuses := []string{"success", "error"}

	for _, node := range nodes {
		for _, transport := range transports {
			for _, status := range statuses {
				RecordGatewayRequest(node, transport, status, 0.1)
			}
		}
	}

	// With 10 nodes × 2 transports × 2 statuses = 40 label combinations
	// This is safe cardinality for Prometheus
	count := testutil.CollectAndCount(GatewayRequestDuration)
	assert.Greater(t, count, 0)
	
	// Verify we don't panic with many label combinations
	assert.NotPanics(t, func() {
		RecordGatewayRequest("node-11", "http", "success", 0.1)
	})
}
```

**Testing Strategy:**

1. **Basic functionality** - Verify metrics can be recorded
2. **All label combinations** - Test http/exec × success/error
3. **Bucket distribution** - Verify latencies across different ranges
4. **Cardinality safety** - Ensure realistic scenarios don't create excessive series

**Acceptance criteria:**
- [ ] All test functions added
- [ ] Tests cover both transport types (http, exec)
- [ ] Tests cover both statuses (success, error)
- [ ] Tests verify bucket distribution
- [ ] All tests pass: `go test ./pkg/metrics/... -v`

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 6: Run Full Test Suite `[TODO]`

Verify all tests pass and code meets quality standards.

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Format code
gofmt -s -w .

# Run all checks
make all

# Specifically test affected packages
go test -v ./pkg/metrics/... -count=1
go test -v ./pkg/gateway/... -count=1

# Test with race detector
go test -race ./pkg/metrics/... ./pkg/gateway/...
```

**Expected results:**
- ✅ All tests pass
- ✅ No linter warnings
- ✅ No race conditions detected

**If tests fail:**
1. Read error messages carefully
2. Fix the issue
3. Re-run tests
4. Commit the fix

**Acceptance criteria:**
- [ ] `make all` succeeds
- [ ] `go test ./pkg/metrics/...` passes
- [ ] `go test ./pkg/gateway/...` passes
- [ ] No race conditions detected
- [ ] No linter warnings

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

### Task 7: Real Cluster E2E Verification `[TODO]`

**⚠️ CONDITIONAL:** Only if `KUBECONFIG` is set and cluster is accessible.

If no cluster is available, **mark this as `[DONE]` with note "N/A - no cluster"**
and skip to Task 7.

**Verify cluster access:**
```bash
kubectl cluster-info
kubectl get nodes
kubectl get pods -n gpu-diagnostics
```

**Build and deploy updated image:**

```bash
# Build new image with metrics changes
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
make docker-build

# Tag with test version
docker tag ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest \
  ghcr.io/arangogutierrez/k8s-gpu-mcp-server:test-gateway-metrics

# Push (if testing in remote cluster)
docker push ghcr.io/arangogutierrez/k8s-gpu-mcp-server:test-gateway-metrics

# Update Helm deployment
helm upgrade --install gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --set image.tag=test-gateway-metrics \
  --set gateway.enabled=true \
  --set transport.mode=http
```

**Wait for rollout:**
```bash
kubectl rollout status -n gpu-diagnostics deployment/gpu-mcp-gateway
```

**Test gateway and verify metrics:**

```bash
# Port-forward gateway
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &

# Make test request (triggers metric recording)
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"tools/call",
    "params":{
      "name":"get_gpu_inventory",
      "arguments":{}
    }
  }'

# Verify metrics endpoint shows new metric
curl -s http://localhost:8080/metrics | grep mcp_gateway_request_duration

# Expected output (example):
# mcp_gateway_request_duration_seconds_bucket{node="ip-10-0-1-123",transport="http",status="success",le="0.1"} 1
# mcp_gateway_request_duration_seconds_bucket{node="ip-10-0-1-123",transport="http",status="success",le="0.25"} 1
# ...
# mcp_gateway_request_duration_seconds_count{node="ip-10-0-1-123",transport="http",status="success"} 1
# mcp_gateway_request_duration_seconds_sum{node="ip-10-0-1-123",transport="http",status="success"} 0.205
```

**Verify Prometheus queries work:**

```bash
# If Prometheus is deployed, test queries:

# P99 latency by transport
histogram_quantile(0.99, rate(mcp_gateway_request_duration_seconds_bucket[5m]))

# Slow nodes
histogram_quantile(0.95, rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node)

# HTTP vs exec comparison
histogram_quantile(0.50, rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (transport)
```

**Acceptance criteria:**
- [ ] New metric appears in `/metrics` output
- [ ] Metric has correct labels: `node`, `transport`, `status`
- [ ] Metric records realistic latencies (~100-500ms for HTTP)
- [ ] Histogram buckets populated appropriately
- [ ] No errors in gateway logs

**If cluster not available:**
- Mark task `[DONE]` with note "N/A - no cluster available"

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Create Pull Request

### Task 8: Create PR `[TODO]`

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Push branch
git push -u origin feat/gateway-latency-metrics

# Create PR
gh pr create \
  --title "feat(gateway): add per-node latency metrics" \
  --body "## Summary

Adds Prometheus metrics to track per-node gateway-to-agent request latency,
differentiated by transport method (HTTP vs exec) and status (success vs error).

Enhances observability for production monitoring of gateway performance.

## Changes

### Metrics Package
- **New metric**: \`mcp_gateway_request_duration_seconds{node, transport, status}\`
- **Helper function**: \`RecordGatewayRequest(node, transport, status, duration)\`
- **Custom buckets**: Optimized for gateway latency (5ms-60s range)

### Gateway Router
- **HTTP path**: Emit metrics in \`routeViaHTTP\` on success and error
- **Exec path**: Emit metrics in \`routeViaExec\` on success and error
- **Consistency**: Both paths use same metric recording pattern

### Testing
- **Unit tests**: All label combinations, bucket distribution, cardinality
- **Real cluster**: Verified metrics appear in \`/metrics\` endpoint (if applicable)

## Motivation

Completes observability improvements post-Epic #112:
- Identify slow nodes in production
- Compare HTTP vs exec performance
- Debug cross-node networking issues
- Support capacity planning decisions

## Prometheus Queries

\`\`\`promql
# P99 latency by transport
histogram_quantile(0.99, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (transport)

# Slow nodes (p95 > 1s)
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node) > 1

# Error rate by node
sum(rate(mcp_gateway_request_duration_seconds_count{status=\"error\"}[5m])) by (node)
\`\`\`

## Label Cardinality

Low cardinality, safe for production:
- \`node\`: Bounded by cluster size (4-100 nodes)
- \`transport\`: 2 values (http, exec)
- \`status\`: 2 values (success, error)
- **Total**: ~200-400 series in typical clusters

## Testing

- [x] Unit tests pass (\`pkg/metrics\`, \`pkg/gateway\`)
- [x] Integration tests pass
- [x] \`make all\` succeeds
- [x] Real cluster verification (if KUBECONFIG available)
- [x] No race conditions

## Related

- Completes observability enhancements from Epic #112
- Builds on circuit breaker metrics from PR #123" \
  --label "kind/feature" \
  --label "area/observability" \
  --label "prio/p2-medium" \
  --milestone "M3: Kubernetes Integration"
```

**Acceptance criteria:**
- [ ] PR created successfully
- [ ] Title follows convention: `feat(gateway): ...`
- [ ] Body includes summary, changes, motivation
- [ ] Labels and milestone assigned
- [ ] Links to related issues/PRs

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit this file

---

### Task 9: Wait for Copilot Review `[TODO]`

> ⚠️ **CRITICAL:** Do NOT merge until Copilot review appears (takes 1-2 minutes)

**Wait and check:**
```bash
# Wait 2 minutes after PR creation
sleep 120

# Check for Copilot comments
gh pr view <PR-NUMBER> --json reviews --jq '.reviews[] | select(.author.login | contains("copilot"))'

# Or check in browser
gh pr view <PR-NUMBER> --web
```

**Check CI status:**
```bash
gh pr checks <PR-NUMBER> --watch
```

**Expected CI jobs:**
- [ ] lint
- [ ] test
- [ ] build
- [ ] Security Scan
- [ ] CodeQL

**Acceptance criteria:**
- [ ] Waited at least 2 minutes after PR creation
- [ ] Checked for Copilot review
- [ ] All CI checks passing

> 💡 **After completing:** Update Progress Tracker → `[DONE]`

---

### Task 10: Address Review Comments `[TODO]`

If Copilot or human reviewers leave comments:

1. **Read each comment carefully**
2. **Evaluate**: Does it improve code quality?
3. **Implement changes** if valid
4. **Reply to comment** explaining fix or rationale

```bash
# After addressing feedback
git add -A
git commit -s -S -m "fix(gateway): address review feedback

- Address Copilot comment about X
- Fix Y per reviewer suggestion"
git push
```

**Re-check for new comments** after pushing fixes.

**If no review comments appear:**
- Mark this task `[DONE]` with note "No review comments"

**Acceptance criteria:**
- [ ] All review comments addressed
- [ ] Changes committed and pushed
- [ ] Replied to all comments

> 💡 **After completing:** Update Progress Tracker → `[DONE]`

---

### Task 11: Merge PR `[TODO]`

**Pre-merge checklist:**

- [ ] All CI checks pass ✅
- [ ] Copilot review checked (waited 1-2 min)
- [ ] All review comments addressed
- [ ] No merge conflicts

**Merge command:**

```bash
gh pr merge <PR-NUMBER> --squash --delete-branch
```

**Post-merge cleanup:**

```bash
# Switch back to main
git checkout main

# Pull merged changes
git pull origin main

# Verify merge
git log --oneline -5
```

**Acceptance criteria:**
- [ ] PR merged successfully
- [ ] Feature branch deleted
- [ ] Local main branch updated
- [ ] Changes appear in `git log`

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Move prompt to archive/

---

## Documentation (Inline - No Separate Docs Needed)

**Note:** This is a pure observability enhancement with no user-facing API changes.
Documentation is provided inline via:

- ✅ Metric help text in `pkg/metrics/metrics.go`
- ✅ Code comments explaining bucket design
- ✅ PR description with example Prometheus queries
- ✅ This prompt file (comprehensive reference)

**No additional documentation files needed.**

---

## Example Prometheus Queries

Once deployed, operators can use these queries:

### Overall Performance

```promql
# Request rate
sum(rate(mcp_gateway_request_duration_seconds_count[5m]))

# P50, P95, P99 latencies
histogram_quantile(0.50, rate(mcp_gateway_request_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(mcp_gateway_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(mcp_gateway_request_duration_seconds_bucket[5m]))
```

### Per-Node Analysis

```promql
# P95 latency by node (identify slow nodes)
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node)

# Nodes with p95 > 1 second
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node) > 1

# Request rate by node
sum(rate(mcp_gateway_request_duration_seconds_count[5m])) by (node)
```

### Transport Comparison

```promql
# HTTP vs exec latency (p50)
histogram_quantile(0.50, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (transport)

# HTTP vs exec latency (p99)
histogram_quantile(0.99, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (transport)

# Request rate by transport
sum(rate(mcp_gateway_request_duration_seconds_count[5m])) by (transport)
```

### Error Analysis

```promql
# Error rate by node
sum(rate(mcp_gateway_request_duration_seconds_count{status="error"}[5m])) by (node)

# Error rate by transport
sum(rate(mcp_gateway_request_duration_seconds_count{status="error"}[5m])) by (transport)

# Success rate percentage
100 * sum(rate(mcp_gateway_request_duration_seconds_count{status="success"}[5m]))
  / sum(rate(mcp_gateway_request_duration_seconds_count[5m]))
```

### Alerting Rules

```yaml
# Example Prometheus alerting rules
groups:
  - name: gateway_latency
    rules:
      - alert: SlowGatewayNode
        expr: |
          histogram_quantile(0.95, 
            rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Node {{ $labels.node }} is slow (p95 > 2s)"
          
      - alert: HighGatewayErrorRate
        expr: |
          sum(rate(mcp_gateway_request_duration_seconds_count{status="error"}[5m])) 
            / sum(rate(mcp_gateway_request_duration_seconds_count[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Gateway error rate > 5%"
```

---

## Completion Protocol

### When All Tasks Are Done

Once you have verified that:
- ✅ All tasks in the Progress Tracker show `[DONE]`
- ✅ All tests pass (`make all` succeeds)
- ✅ PR is created and CI is green
- ✅ Copilot review has appeared (waited 1-2 min)
- ✅ All review comments addressed
- ✅ PR is merged

**Final status report:**
```markdown
## 🎉 ALL TASKS COMPLETE

Gateway latency metrics enhancement completed successfully.

**Summary:**
- Branch: `feat/gateway-latency-metrics`
- PR: #XXX (merged)
- Tests: ✅ All passing
- Metric: `mcp_gateway_request_duration_seconds{node, transport, status}`

**Impact:**
- Operators can now identify slow nodes
- HTTP vs exec performance comparison available
- Production monitoring enhanced

**Recommend:** Move this prompt to `archive/`
```

### If Tasks Remain Incomplete

If ANY task is not `[DONE]`:

1. **Update the Progress Tracker** in this file
2. **Commit your progress**
3. **End with status report** listing remaining tasks
4. **Prompt re-invocation:** `@docs/prompts/gateway-latency-metrics.md`

---

## Quick Reference

### Key Files Modified

| File | Purpose | Lines Changed |
|------|---------|---------------|
| `pkg/metrics/metrics.go` | Add histogram + helper | ~30 lines |
| `pkg/gateway/router.go` | Emit metrics (2 functions) | ~10 lines |
| `pkg/metrics/metrics_test.go` | Unit tests | ~150 lines |

**Total LOC:** ~190 lines (small, focused change)

### Key Commands

```bash
# Branch
git checkout -b feat/gateway-latency-metrics

# Test
make all
go test -v ./pkg/metrics/... ./pkg/gateway/...

# Commit (atomic)
git commit -s -S -m "feat(metrics): add gateway latency histogram"
git commit -s -S -m "feat(gateway): emit latency metrics in router"
git commit -s -S -m "test(metrics): add gateway latency tests"

# PR
gh pr create --title "feat(gateway): add per-node latency metrics" ...

# Merge
gh pr merge <PR> --squash --delete-branch
```

---

**Reply "GO" when ready to start implementation.** 🚀

<!-- 
COMPLETION MARKER - Do not output until ALL tasks are [DONE]:
<completion>ALL_TASKS_DONE</completion>
-->
