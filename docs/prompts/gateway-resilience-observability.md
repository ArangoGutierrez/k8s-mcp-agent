# Gateway Resilience Patterns and Observability

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/gateway-resilience-observability.md`, the
> agent MUST continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/gateway-resilience-observability.md`

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[TODO]` | `feat/gateway-resilience` |
| 1 | Create CircuitBreaker | `[TODO]` | `pkg/gateway/circuit_breaker.go` |
| 2 | Add CircuitBreaker tests | `[TODO]` | `pkg/gateway/circuit_breaker_test.go` |
| 3 | Integrate CircuitBreaker with Router | `[TODO]` | `pkg/gateway/router.go` |
| 4 | Add correlation ID tracing | `[TODO]` | Context-based request tracing |
| 5 | Create Prometheus metrics | `[TODO]` | `pkg/mcp/metrics.go` |
| 6 | Add /metrics endpoint | `[TODO]` | `pkg/mcp/http.go` |
| 7 | Add metrics tests | `[TODO]` | `pkg/mcp/metrics_test.go` |
| 8 | Create NetworkPolicy template | `[TODO]` | `templates/networkpolicy.yaml` |
| 9 | Update Helm values | `[TODO]` | `values.yaml` |
| 10 | Run tests and verify | `[TODO]` | `make all` |
| 11 | Create pull request | `[TODO]` | |
| 12 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 13 | Address review comments | `[TODO]` | |
| 14 | Merge after reviews | `[TODO]` | |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** [#116 - feat(gateway): Add resilience patterns and observability](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/116)
- **Priority:** P2-Medium
- **Labels:** kind/feature, prio/p2-medium
- **Parent Epic:** #112 - HTTP transport refactor
- **Depends on:** #122 (merged) - HTTP routing to agents
- **Autonomous Mode:** ✅ Enabled

## Background

With the HTTP routing implementation (#122) complete, the gateway now communicates
directly with agent pods via HTTP. For production deployments, we need:

1. **Circuit breaker** - Prevent cascading failures from unhealthy nodes
2. **Partial success** - Return data from healthy nodes even if some fail
3. **Metrics** - Prometheus-compatible monitoring
4. **Tracing** - Request correlation for debugging
5. **NetworkPolicy** - Secure pod-to-pod communication

### Current Architecture

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
│  │  (GPU×2)  │  │  (GPU×4)  │  │  (GPU×8)  │  │  (GPU×2)  │       │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘       │
└─────────────────────────────────────────────────────────────────────┘
```

**Problem:** If Node B becomes unresponsive:
- Currently: All requests wait for timeout (60s)
- Goal: Circuit breaker trips after N failures, skips unhealthy node

---

## Objective

Add production-ready resilience patterns (circuit breaker, partial success) and
observability (Prometheus metrics, request tracing) to the gateway.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gateway-resilience
```

---

## Implementation Tasks

### Task 1: Create CircuitBreaker `[TODO]`

Create a per-node circuit breaker to prevent requests to failing nodes.

**File:** `pkg/gateway/circuit_breaker.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed allows requests through (normal operation).
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks requests (node is unhealthy).
	CircuitOpen
	// CircuitHalfOpen allows one test request through.
	CircuitHalfOpen
)

// CircuitBreaker tracks node health and prevents requests to failing nodes.
type CircuitBreaker struct {
	mu           sync.RWMutex
	failures     map[string]int
	lastFailure  map[string]time.Time
	state        map[string]CircuitState
	threshold    int
	resetTimeout time.Duration
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// Threshold is the number of failures before opening the circuit.
	Threshold int
	// ResetTimeout is how long to wait before trying a half-open request.
	ResetTimeout time.Duration
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Threshold:    3,
		ResetTimeout: 30 * time.Second,
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		failures:     make(map[string]int),
		lastFailure:  make(map[string]time.Time),
		state:        make(map[string]CircuitState),
		threshold:    cfg.Threshold,
		resetTimeout: cfg.ResetTimeout,
	}
}

// Allow checks if a request to the given node should be allowed.
// Returns true if the circuit is closed or half-open.
func (cb *CircuitBreaker) Allow(node string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.state[node]

	switch state {
	case CircuitOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailure[node]) > cb.resetTimeout {
			cb.state[node] = CircuitHalfOpen
			return true
		}
		return false

	case CircuitHalfOpen:
		// Allow one test request
		return true

	default: // CircuitClosed
		return true
	}
}

// RecordSuccess records a successful request to a node.
// Resets the failure count and closes the circuit.
func (cb *CircuitBreaker) RecordSuccess(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[node] = 0
	cb.state[node] = CircuitClosed
}

// RecordFailure records a failed request to a node.
// Opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[node]++
	cb.lastFailure[node] = time.Now()

	if cb.failures[node] >= cb.threshold {
		cb.state[node] = CircuitOpen
	}
}

// State returns the current state for a node.
func (cb *CircuitBreaker) State(node string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state[node]
}

// Failures returns the current failure count for a node.
func (cb *CircuitBreaker) Failures(node string) int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures[node]
}

// Reset resets the circuit breaker state for a node.
func (cb *CircuitBreaker) Reset(node string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failures, node)
	delete(cb.lastFailure, node)
	delete(cb.state, node)
}

// String returns a string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
```

**Acceptance criteria:**
- [ ] `CircuitBreaker` struct with per-node tracking
- [ ] `Allow()` returns false for open circuits
- [ ] `RecordSuccess()` closes circuit
- [ ] `RecordFailure()` opens circuit after threshold
- [ ] Half-open state after reset timeout

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(gateway): add CircuitBreaker for node health tracking"`

---

### Task 2: Add CircuitBreaker Tests `[TODO]`

**File:** `pkg/gateway/circuit_breaker_test.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_Allow_ClosedCircuit(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// New node should be allowed (circuit closed)
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
}

func TestCircuitBreaker_RecordFailure_OpensCircuit(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 3
	cb := NewCircuitBreaker(cfg)

	// Record failures below threshold
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))

	// Record failure at threshold - circuit opens
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitOpen, cb.State("node-1"))
}

func TestCircuitBreaker_RecordSuccess_ClosesCircuit(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// Success resets the circuit
	cb.RecordSuccess("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
	assert.Equal(t, 0, cb.Failures("node-1"))
}

func TestCircuitBreaker_HalfOpen_AfterTimeout(t *testing.T) {
	cfg := CircuitBreakerConfig{
		Threshold:    2,
		ResetTimeout: 50 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitOpen, cb.State("node-1"))

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow request
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitHalfOpen, cb.State("node-1"))
}

func TestCircuitBreaker_MultipleNodes(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open circuit for node-1
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// node-2 should still be allowed
	assert.True(t, cb.Allow("node-2"))

	// Success on node-2
	cb.RecordSuccess("node-2")
	assert.True(t, cb.Allow("node-2"))
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Threshold = 2
	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure("node-1")
	cb.RecordFailure("node-1")
	assert.False(t, cb.Allow("node-1"))

	// Reset should clear state
	cb.Reset("node-1")
	assert.True(t, cb.Allow("node-1"))
	assert.Equal(t, CircuitClosed, cb.State("node-1"))
	assert.Equal(t, 0, cb.Failures("node-1"))
}

func TestCircuitState_String(t *testing.T) {
	assert.Equal(t, "closed", CircuitClosed.String())
	assert.Equal(t, "open", CircuitOpen.String())
	assert.Equal(t, "half-open", CircuitHalfOpen.String())
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()

	assert.Equal(t, 3, cfg.Threshold)
	assert.Equal(t, 30*time.Second, cfg.ResetTimeout)
}
```

**Acceptance criteria:**
- [ ] Test closed circuit allows requests
- [ ] Test circuit opens after threshold failures
- [ ] Test success closes circuit
- [ ] Test half-open after reset timeout
- [ ] Test multiple independent nodes
- [ ] Test reset functionality

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "test(gateway): add CircuitBreaker unit tests"`

---

### Task 3: Integrate CircuitBreaker with Router `[TODO]`

Update the Router to use CircuitBreaker before routing requests.

**File:** `pkg/gateway/router.go`

**Changes:**

1. Add circuit breaker to Router struct:

```go
// Router forwards MCP requests to node agents.
type Router struct {
	k8sClient      *k8s.Client
	httpClient     *AgentHTTPClient
	routingMode    RoutingMode
	circuitBreaker *CircuitBreaker
}

// WithCircuitBreaker sets a custom circuit breaker.
func WithCircuitBreaker(cb *CircuitBreaker) RouterOption {
	return func(r *Router) {
		r.circuitBreaker = cb
	}
}

// NewRouter creates a new gateway router.
func NewRouter(k8sClient *k8s.Client, opts ...RouterOption) *Router {
	r := &Router{
		k8sClient:      k8sClient,
		httpClient:     NewAgentHTTPClient(),
		routingMode:    RoutingModeHTTP,
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerConfig()),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
```

2. Update `routeToGPUNode` to check circuit breaker:

```go
func (r *Router) routeToGPUNode(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
) ([]byte, error) {
	if !node.Ready {
		return nil, fmt.Errorf("agent on node %s is not ready", node.Name)
	}

	// Check circuit breaker
	if !r.circuitBreaker.Allow(node.Name) {
		log.Printf(`{"level":"warn","msg":"circuit open, skipping node",`+
			`"node":"%s","state":"%s"}`,
			node.Name, r.circuitBreaker.State(node.Name))
		return nil, fmt.Errorf("circuit open for node %s", node.Name)
	}

	startTime := time.Now()

	// ... existing routing logic ...

	// Record result with circuit breaker
	if err != nil {
		r.circuitBreaker.RecordFailure(node.Name)
		return nil, err
	}

	r.circuitBreaker.RecordSuccess(node.Name)
	return response, nil
}
```

3. Update `RouteToAllNodes` to handle partial success:

```go
func (r *Router) RouteToAllNodes(
	ctx context.Context,
	mcpRequest []byte,
) ([]NodeResult, error) {
	// ... existing node listing ...

	// Track circuit breaker skips
	skippedCount := 0
	
	for _, node := range nodes {
		if !node.Ready {
			// ... existing skip logic ...
			continue
		}

		// Check circuit breaker before spawning goroutine
		if !r.circuitBreaker.Allow(node.Name) {
			log.Printf(`{"level":"warn","msg":"circuit open, skipping node",`+
				`"node":"%s"}`, node.Name)
			skippedCount++
			
			mu.Lock()
			results = append(results, NodeResult{
				NodeName: node.Name,
				PodName:  node.PodName,
				Error:    fmt.Sprintf("circuit open (state: %s)",
					r.circuitBreaker.State(node.Name)),
			})
			mu.Unlock()
			continue
		}

		// ... existing goroutine routing logic ...
	}

	// ... wait and aggregate ...

	// Partial success: return results even if some failed
	// Only return error if ALL nodes failed
	if successCount == 0 && len(results) > 0 {
		return results, fmt.Errorf("all %d nodes failed", len(results))
	}

	log.Printf(`{"level":"info","msg":"routing complete",`+
		`"total_nodes":%d,"success":%d,"failed":%d,"skipped":%d}`,
		len(nodes), successCount, failCount, skippedCount)

	return results, nil
}
```

**Acceptance criteria:**
- [ ] Router has CircuitBreaker field
- [ ] `routeToGPUNode` checks circuit before routing
- [ ] Failures recorded with circuit breaker
- [ ] Successes recorded with circuit breaker
- [ ] `RouteToAllNodes` skips open circuits
- [ ] Partial success returns results from healthy nodes

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(gateway): integrate CircuitBreaker with Router"`

---

### Task 4: Add Correlation ID Tracing `[TODO]`

Add correlation IDs to requests for distributed tracing.

**File:** `pkg/gateway/tracing.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// CorrelationIDKey is the context key for correlation IDs.
type correlationIDKeyType struct{}

var correlationIDKey = correlationIDKeyType{}

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context.
// Returns empty string if not found.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}
```

**Update ProxyHandler to use correlation IDs:**

```go
func (p *ProxyHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// Generate correlation ID if not present
	correlationID := CorrelationIDFromContext(ctx)
	if correlationID == "" {
		correlationID = NewCorrelationID()
		ctx = WithCorrelationID(ctx, correlationID)
	}

	log.Printf(`{"level":"info","msg":"proxy_tool invoked","tool":"%s",`+
		`"routing_mode":"%s","correlation_id":"%s"}`,
		p.toolName, p.router.RoutingMode(), correlationID)

	// ... rest of handler ...
}
```

**Acceptance criteria:**
- [ ] `NewCorrelationID()` generates unique IDs
- [ ] Context carries correlation ID
- [ ] Logs include correlation ID
- [ ] ID propagates through request chain

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(gateway): add correlation ID tracing"`

---

### Task 5: Create Prometheus Metrics `[TODO]`

Add Prometheus metrics for monitoring.

**File:** `pkg/mcp/metrics.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total MCP requests by tool and status.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_requests_total",
			Help: "Total MCP requests processed",
		},
		[]string{"tool", "status"},
	)

	// RequestDuration tracks MCP request latency.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_request_duration_seconds",
			Help:    "MCP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool"},
	)

	// NodeHealth tracks per-node health status.
	NodeHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_node_health",
			Help: "Node health status (1=healthy, 0=unhealthy)",
		},
		[]string{"node"},
	)

	// CircuitBreakerState tracks circuit breaker state per node.
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"node"},
	)

	// ActiveRequests tracks in-flight requests.
	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_active_requests",
			Help: "Number of active MCP requests",
		},
	)
)

// RecordRequest records metrics for a completed request.
func RecordRequest(tool, status string, durationSeconds float64) {
	RequestsTotal.WithLabelValues(tool, status).Inc()
	RequestDuration.WithLabelValues(tool).Observe(durationSeconds)
}

// SetNodeHealth sets the health status for a node.
func SetNodeHealth(node string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	NodeHealth.WithLabelValues(node).Set(value)
}

// SetCircuitState sets the circuit breaker state for a node.
func SetCircuitState(node string, state int) {
	CircuitBreakerState.WithLabelValues(node).Set(float64(state))
}
```

**Acceptance criteria:**
- [ ] `mcp_requests_total` counter with tool/status labels
- [ ] `mcp_request_duration_seconds` histogram
- [ ] `mcp_node_health` gauge per node
- [ ] `mcp_circuit_breaker_state` gauge per node
- [ ] Helper functions for recording metrics

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(mcp): add Prometheus metrics"`

---

### Task 6: Add /metrics Endpoint `[TODO]`

Expose Prometheus metrics via HTTP endpoint.

**File:** `pkg/mcp/http.go`

**Add to HTTPServer:**

```go
import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHTTPServer creates a new HTTP server for MCP.
func NewHTTPServer(mcpServer *server.MCPServer, addr, version string) *HTTPServer {
	mux := http.NewServeMux()
	
	// ... existing routes ...

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// ... rest of setup ...
}
```

**Update go.mod to add prometheus dependency:**

```bash
go get github.com/prometheus/client_golang
```

**Acceptance criteria:**
- [ ] `/metrics` endpoint returns Prometheus format
- [ ] Metrics include all defined counters/gauges
- [ ] No authentication required (cluster-internal)

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(mcp): add /metrics endpoint for Prometheus"`

---

### Task 7: Add Metrics Tests `[TODO]`

**File:** `pkg/mcp/metrics_test.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordRequest(t *testing.T) {
	// Reset metrics for clean test
	RequestsTotal.Reset()
	RequestDuration.Reset()

	RecordRequest("get_gpu_inventory", "success", 0.123)
	RecordRequest("get_gpu_health", "error", 0.456)
	RecordRequest("get_gpu_inventory", "success", 0.100)

	// Check counter values
	assert.Equal(t, 2.0, testutil.ToFloat64(
		RequestsTotal.WithLabelValues("get_gpu_inventory", "success")))
	assert.Equal(t, 1.0, testutil.ToFloat64(
		RequestsTotal.WithLabelValues("get_gpu_health", "error")))
}

func TestSetNodeHealth(t *testing.T) {
	NodeHealth.Reset()

	SetNodeHealth("node-1", true)
	SetNodeHealth("node-2", false)

	assert.Equal(t, 1.0, testutil.ToFloat64(
		NodeHealth.WithLabelValues("node-1")))
	assert.Equal(t, 0.0, testutil.ToFloat64(
		NodeHealth.WithLabelValues("node-2")))
}

func TestSetCircuitState(t *testing.T) {
	CircuitBreakerState.Reset()

	SetCircuitState("node-1", 0) // closed
	SetCircuitState("node-2", 1) // open
	SetCircuitState("node-3", 2) // half-open

	assert.Equal(t, 0.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-1")))
	assert.Equal(t, 1.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-2")))
	assert.Equal(t, 2.0, testutil.ToFloat64(
		CircuitBreakerState.WithLabelValues("node-3")))
}
```

**Acceptance criteria:**
- [ ] Test request counter increments
- [ ] Test node health gauge
- [ ] Test circuit breaker state gauge

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "test(mcp): add Prometheus metrics tests"`

---

### Task 8: Create NetworkPolicy Template `[TODO]`

**File:** `deployment/helm/k8s-gpu-mcp-server/templates/networkpolicy.yaml`

```yaml
{{/*
Copyright 2026 k8s-gpu-mcp-server contributors
SPDX-License-Identifier: Apache-2.0
*/}}
{{- if .Values.networkPolicy.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-agent
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
  labels:
    {{- include "k8s-gpu-mcp-server.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 6 }}
      app.kubernetes.io/component: agent
  policyTypes:
  - Ingress
  ingress:
  # Allow traffic from gateway pods
  - from:
    - podSelector:
        matchLabels:
          {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 10 }}
          app.kubernetes.io/component: gateway
    ports:
    - protocol: TCP
      port: {{ .Values.transport.http.port }}
  {{- if .Values.networkPolicy.allowPrometheus }}
  # Allow Prometheus scraping
  - from:
    - namespaceSelector:
        matchLabels:
          {{- toYaml .Values.networkPolicy.prometheusNamespaceSelector | nindent 10 }}
    ports:
    - protocol: TCP
      port: {{ .Values.transport.http.port }}
  {{- end }}
---
{{- if .Values.gateway.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-gateway
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
  labels:
    {{- include "k8s-gpu-mcp-server.labels" . | nindent 4 }}
spec:
  podSelector:
    matchLabels:
      {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 6 }}
      app.kubernetes.io/component: gateway
  policyTypes:
  - Ingress
  - Egress
  ingress:
  # Allow external traffic to gateway
  - ports:
    - protocol: TCP
      port: {{ .Values.gateway.port }}
  egress:
  # Allow gateway to reach agent pods
  - to:
    - podSelector:
        matchLabels:
          {{- include "k8s-gpu-mcp-server.selectorLabels" . | nindent 10 }}
    ports:
    - protocol: TCP
      port: {{ .Values.transport.http.port }}
  # Allow DNS resolution
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
{{- end }}
{{- end }}
```

**Acceptance criteria:**
- [ ] Agent NetworkPolicy restricts ingress to gateway
- [ ] Gateway NetworkPolicy allows egress to agents
- [ ] Optional Prometheus scraping allowed
- [ ] DNS egress allowed for gateway

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(helm): add NetworkPolicy templates"`

---

### Task 9: Update Helm Values `[TODO]`

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Add networkPolicy section:**

```yaml
# NetworkPolicy configuration
networkPolicy:
  # -- Enable NetworkPolicy for pod-to-pod communication security
  enabled: false

  # -- Allow Prometheus to scrape agent metrics
  allowPrometheus: false

  # -- Namespace selector for Prometheus (if allowPrometheus: true)
  prometheusNamespaceSelector:
    kubernetes.io/metadata.name: monitoring
```

**Acceptance criteria:**
- [ ] `networkPolicy.enabled` defaults to false
- [ ] `networkPolicy.allowPrometheus` option
- [ ] Prometheus namespace selector configurable

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit:
> `git commit -s -S -m "feat(helm): add networkPolicy configuration"`

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Run gateway tests specifically
go test ./pkg/gateway/... -v -count=1

# Run metrics tests
go test ./pkg/mcp/... -v -count=1 -run Metrics

# Verify Helm template renders correctly
helm template gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --set gateway.enabled=true \
  --set networkPolicy.enabled=true | grep -A 20 "NetworkPolicy"
```

### Integration Testing (Real Cluster)

```bash
# Deploy with NetworkPolicy enabled
helm upgrade --install gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics --create-namespace \
  --set gateway.enabled=true \
  --set networkPolicy.enabled=true

# Verify metrics endpoint
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &
curl http://localhost:8080/metrics | grep mcp_

# Test circuit breaker (kill an agent, check it gets skipped)
kubectl delete pod -n gpu-diagnostics -l app.kubernetes.io/component=agent --field-selector spec.nodeName=<node>
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
| 1 | `feat(gateway): add CircuitBreaker for node health tracking` |
| 2 | `test(gateway): add CircuitBreaker unit tests` |
| 3 | `feat(gateway): integrate CircuitBreaker with Router` |
| 4 | `feat(gateway): add correlation ID tracing` |
| 5 | `feat(mcp): add Prometheus metrics` |
| 6 | `feat(mcp): add /metrics endpoint for Prometheus` |
| 7 | `test(mcp): add Prometheus metrics tests` |
| 8 | `feat(helm): add NetworkPolicy templates` |
| 9 | `feat(helm): add networkPolicy configuration` |

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(gateway): add resilience patterns and observability" \
  --body "Fixes #116

## Summary

Adds production-ready resilience patterns and observability to the gateway:

- **Circuit breaker** - Prevents requests to failing nodes
- **Partial success** - Returns data from healthy nodes even if some fail
- **Prometheus metrics** - Exposed at \`/metrics\` endpoint
- **Correlation IDs** - Request tracing across components
- **NetworkPolicy** - Secures pod-to-pod communication

## Changes

- Add \`CircuitBreaker\` with per-node health tracking
- Integrate circuit breaker with Router
- Add correlation ID tracing
- Add Prometheus metrics (requests, duration, node health)
- Add \`/metrics\` endpoint
- Add NetworkPolicy Helm templates

## Metrics Exposed

| Metric | Type | Description |
|--------|------|-------------|
| \`mcp_requests_total\` | Counter | Total requests by tool/status |
| \`mcp_request_duration_seconds\` | Histogram | Request latency |
| \`mcp_node_health\` | Gauge | Node health (1=healthy) |
| \`mcp_circuit_breaker_state\` | Gauge | Circuit state per node |

## Testing

- [ ] Unit tests pass
- [ ] Helm template renders correctly
- [ ] Metrics endpoint works

## Related

- Parent epic: #112
- Depends on: #122 (HTTP routing)" \
  --label "kind/feature" \
  --label "prio/p2-medium"
```

---

## Success Criteria

| Metric | Before | After |
|--------|--------|-------|
| Unhealthy node handling | Wait for timeout | Circuit trips, node skipped |
| Partial failures | All-or-nothing | Return healthy node data |
| Observability | Logs only | Prometheus metrics |
| Request tracing | None | Correlation IDs |
| Network security | Open | NetworkPolicy restricted |

---

## Related Files

- `pkg/gateway/circuit_breaker.go` - **New:** Circuit breaker
- `pkg/gateway/tracing.go` - **New:** Correlation ID
- `pkg/mcp/metrics.go` - **New:** Prometheus metrics
- `pkg/gateway/router.go` - Circuit breaker integration
- `pkg/gateway/proxy.go` - Correlation ID integration
- `pkg/mcp/http.go` - /metrics endpoint
- `deployment/helm/.../templates/networkpolicy.yaml` - **New:** NetworkPolicy
- `deployment/helm/.../values.yaml` - networkPolicy config

---

## Agent Self-Check (Before Ending Each Turn)

Before you finish ANY response, perform this self-check:

```
┌─────────────────────────────────────────────────────────────────┐
│ SELF-CHECK: Can I end this turn?                                │
├─────────────────────────────────────────────────────────────────┤
│ □ Have I made progress on at least one task?                    │
│ □ Did I update the Progress Tracker above?                      │
│ □ Did I commit my changes? (if code was modified)               │
│ □ Are there any [TODO] tasks I can continue working on?         │
├─────────────────────────────────────────────────────────────────┤
│ If tasks remain → Tell user to re-invoke this prompt            │
│ If ALL [DONE] → Congratulate and suggest archiving prompt       │
└─────────────────────────────────────────────────────────────────┘
```

---

## End-of-Turn Status Report

**Always end your turn with this format:**

```markdown
## 📊 Status Report

**Completed this turn:**
- [x] Task X - description
- [x] Task Y - description

**Remaining tasks:**
- [ ] Task Z (next priority)
- [ ] Task W

**Next invocation will:** [describe what happens next]

➡️ **Re-invoke to continue:** `@docs/prompts/gateway-resilience-observability.md`
```

---

## Completion Protocol

### When All Tasks Are Done

Once ALL tasks show `[DONE]`:
- ✅ All code implemented and tests pass
- ✅ PR created and CI is green
- ✅ Copilot review appeared (waited 1-2 min)
- ✅ All review comments addressed
- ✅ PR merged

**Final status:**
```markdown
## 🎉 ALL TASKS COMPLETE

All tasks in this prompt have been completed successfully.

**Summary:**
- Branch: `feat/gateway-resilience`
- PR: #XXX (merged)
- Tests: ✅ Passing

**Recommend:** Move this prompt to `archive/`
```

---

**Reply "GO" when ready to start implementation.** 🚀
