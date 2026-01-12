# Gateway HTTP Routing to Agent Pods

## Issue Reference

- **Issue:** [#115 - feat(gateway): Route to agents via HTTP instead of kubectl exec](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/115)
- **Priority:** P1-High
- **Labels:** kind/feature, prio/p1-high
- **Parent Epic:** #112 - HTTP transport refactor
- **Depends on:** #121 (merged) - HTTP transport as default

## Background

The gateway currently communicates with agent pods via `kubectl exec` (SPDY protocol),
which requires complex framing and has significant overhead:

### Current Architecture (5 protocol layers)

```
┌─────────────────────────────────────────────────────────────────────┐
│                           GATEWAY                                   │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │ ProxyHandler│───►│   Router    │───►│ k8sClient.ExecInPod()   │ │
│  └─────────────┘    └─────────────┘    └───────────┬─────────────┘ │
└────────────────────────────────────────────────────┼───────────────┘
                                                     │
                     ┌───────────────────────────────┘
                     │ SPDY exec (kubectl exec equivalent)
                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         AGENT POD                                   │
│  ┌──────────────┐                                                   │
│  │ sleep infinity│ ◄── Pod running idle                            │
│  └──────────────┘                                                   │
│           │                                                         │
│           │ exec spawns agent process                               │
│           ▼                                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  /agent --oneshot=2  (init + tool)                          │   │
│  │      │                                                      │   │
│  │      ├──► NVML Init() ──► Query ──► Exit                    │   │
│  │      │                                                      │   │
│  │  Process spawned per request, NVML init every time          │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

**Problems:**
- SPDY exec overhead per request (~50-100ms)
- NVML initialization per request (~50-100ms)
- Process spawn overhead (~10-20ms)
- Complex oneshot framing (init + tool messages)
- Resource competition when parallel requests

### Target Architecture (3 protocol layers)

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
│                         AGENT POD                                   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  /agent --port=8080  (persistent HTTP server)               │   │
│  │      │                                                      │   │
│  │      ├──► NVML Init() (once at startup)                     │   │
│  │      │                                                      │   │
│  │      └──► HTTP :8080/mcp ──► Query ──► Response             │   │
│  │                                                             │   │
│  │  Single process, NVML warm, handles all requests            │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Direct HTTP: No SPDY, no exec overhead
- NVML warm: Initialized once at pod startup
- Connection pooling: Reuse HTTP connections
- Simple protocol: Standard HTTP POST, no framing

---

## Objective

Refactor the gateway router to send MCP requests to agent pods via HTTP
(pod-to-pod networking) instead of `kubectl exec` (SPDY), while maintaining
exec as a fallback for legacy deployments.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/gateway-http-routing
```

---

## Implementation Tasks

### Task 1: Create AgentHTTPClient

Create a new HTTP client with connection pooling and retry logic for
communicating with agent pods.

**File:** `pkg/gateway/http_client.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// DefaultAgentHTTPPort is the default port agents listen on in HTTP mode.
const DefaultAgentHTTPPort = 8080

// AgentHTTPClient handles HTTP communication with agent pods.
type AgentHTTPClient struct {
	client      *http.Client
	retryPolicy RetryPolicy
}

// RetryPolicy defines retry behavior for failed requests.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy returns sensible retry defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   2 * time.Second,
	}
}

// NewAgentHTTPClient creates an HTTP client optimized for agent communication.
func NewAgentHTTPClient() *AgentHTTPClient {
	return &AgentHTTPClient{
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		retryPolicy: DefaultRetryPolicy(),
	}
}

// CallMCP sends an MCP request to an agent pod and returns the response.
// The endpoint should be the full URL (e.g., "http://10.0.0.5:8080").
func (c *AgentHTTPClient) CallMCP(
	ctx context.Context,
	endpoint string,
	request []byte,
) ([]byte, error) {
	url := endpoint + "/mcp"
	
	var lastErr error
	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			log.Printf(`{"level":"debug","msg":"retrying request",`+
				`"attempt":%d,"delay":"%s","endpoint":"%s"}`,
				attempt, delay, endpoint)
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := c.doRequest(ctx, url, request)
		if err == nil {
			return response, nil
		}
		lastErr = err
		
		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w",
		c.retryPolicy.MaxRetries+1, lastErr)
}

// doRequest performs a single HTTP request.
func (c *AgentHTTPClient) doRequest(
	ctx context.Context,
	url string,
	body []byte,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s",
			resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// calculateBackoff returns the delay for a retry attempt using exponential
// backoff with jitter.
func (c *AgentHTTPClient) calculateBackoff(attempt int) time.Duration {
	delay := c.retryPolicy.BaseDelay * time.Duration(1<<uint(attempt-1))
	if delay > c.retryPolicy.MaxDelay {
		delay = c.retryPolicy.MaxDelay
	}
	return delay
}
```

**Acceptance criteria:**
- [ ] `AgentHTTPClient` struct with connection pooling
- [ ] `CallMCP()` method with retry logic
- [ ] Exponential backoff between retries
- [ ] Context cancellation respected

> 💡 **Commit:** `feat(gateway): add AgentHTTPClient with connection pooling`

---

### Task 2: Add Unit Tests for AgentHTTPClient

**File:** `pkg/gateway/http_client_test.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentHTTPClient_CallMCP_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/mcp", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
	}))
	defer server.Close()

	client := NewAgentHTTPClient()
	resp, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	require.NoError(t, err)
	assert.Contains(t, string(resp), "jsonrpc")
}

func TestAgentHTTPClient_CallMCP_RetryOnFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewAgentHTTPClient()
	client.retryPolicy.BaseDelay = 10 * time.Millisecond // Speed up test
	
	resp, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
	assert.Contains(t, string(resp), "success")
}

func TestAgentHTTPClient_CallMCP_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAgentHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.CallMCP(ctx, server.URL, []byte(`{}`))

	assert.Error(t, err)
}

func TestAgentHTTPClient_CallMCP_AllRetriesFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewAgentHTTPClient()
	client.retryPolicy.MaxRetries = 2
	client.retryPolicy.BaseDelay = 1 * time.Millisecond

	_, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	
	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, policy.BaseDelay)
	assert.Equal(t, 2*time.Second, policy.MaxDelay)
}

func TestAgentHTTPClient_calculateBackoff(t *testing.T) {
	client := NewAgentHTTPClient()
	client.retryPolicy.BaseDelay = 100 * time.Millisecond
	client.retryPolicy.MaxDelay = 1 * time.Second

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},  // 100ms * 2^0
		{2, 200 * time.Millisecond},  // 100ms * 2^1
		{3, 400 * time.Millisecond},  // 100ms * 2^2
		{4, 800 * time.Millisecond},  // 100ms * 2^3
		{5, 1 * time.Second},         // capped at MaxDelay
	}

	for _, tt := range tests {
		delay := client.calculateBackoff(tt.attempt)
		assert.Equal(t, tt.expected, delay, "attempt %d", tt.attempt)
	}
}
```

**Acceptance criteria:**
- [ ] Test successful request
- [ ] Test retry on transient failure
- [ ] Test context cancellation
- [ ] Test all retries exhausted
- [ ] Test backoff calculation

> 💡 **Commit:** `test(gateway): add AgentHTTPClient unit tests`

---

### Task 3: Update Router with HTTP Routing Mode

Modify the Router to support both HTTP and exec routing modes.

**File:** `pkg/gateway/router.go`

**Changes:**

1. Add routing mode and HTTP client to Router struct:

```go
// RoutingMode specifies how the gateway communicates with agents.
type RoutingMode string

const (
	// RoutingModeHTTP routes requests via HTTP to agent pods (recommended).
	RoutingModeHTTP RoutingMode = "http"
	// RoutingModeExec routes requests via kubectl exec (legacy).
	RoutingModeExec RoutingMode = "exec"
)

// Router forwards MCP requests to node agents.
type Router struct {
	k8sClient   *k8s.Client
	httpClient  *AgentHTTPClient
	routingMode RoutingMode
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithRoutingMode sets the routing mode.
func WithRoutingMode(mode RoutingMode) RouterOption {
	return func(r *Router) {
		r.routingMode = mode
	}
}

// NewRouter creates a new gateway router.
func NewRouter(k8sClient *k8s.Client, opts ...RouterOption) *Router {
	r := &Router{
		k8sClient:   k8sClient,
		httpClient:  NewAgentHTTPClient(),
		routingMode: RoutingModeHTTP, // Default to HTTP
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
```

2. Update `routeToGPUNode` to use HTTP when available:

```go
// routeToGPUNode sends an MCP request to a known GPU node's agent.
func (r *Router) routeToGPUNode(
	ctx context.Context,
	node k8s.GPUNode,
	mcpRequest []byte,
) ([]byte, error) {
	if !node.Ready {
		return nil, fmt.Errorf("agent on node %s is not ready", node.Name)
	}

	startTime := time.Now()

	// Try HTTP routing if enabled and pod has IP
	if r.routingMode == RoutingModeHTTP {
		endpoint := node.GetAgentHTTPEndpoint()
		if endpoint != "" {
			return r.routeViaHTTP(ctx, node, endpoint, mcpRequest, startTime)
		}
		log.Printf(`{"level":"warn","msg":"pod has no IP, falling back to exec",`+
			`"node":"%s","pod":"%s"}`, node.Name, node.PodName)
	}

	// Fall back to exec routing
	return r.routeViaExec(ctx, node, mcpRequest, startTime)
}

// routeViaHTTP sends request via HTTP to agent pod.
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

	// For HTTP mode, we send just the tool call - no init framing needed
	// The agent HTTP server handles the full MCP session
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

// routeViaExec sends request via kubectl exec to agent pod (legacy mode).
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

**Acceptance criteria:**
- [ ] `RoutingMode` type with HTTP and Exec constants
- [ ] `WithRoutingMode()` option function
- [ ] Default routing mode is HTTP
- [ ] HTTP routing uses `GetAgentHTTPEndpoint()`
- [ ] Falls back to exec if pod has no IP
- [ ] Structured logging for both modes

> 💡 **Commit:** `feat(gateway): add HTTP routing mode to Router`

---

### Task 4: Update ProxyHandler for HTTP Mode

When using HTTP mode, the proxy doesn't need to build init+tool framing.
The agent HTTP server handles the full MCP session.

**File:** `pkg/gateway/proxy.go`

**Update `Handle` method:**

```go
// Handle proxies the tool call to all node agents and aggregates results.
func (p *ProxyHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf(`{"level":"info","msg":"proxy_tool invoked","tool":"%s",`+
		`"routing_mode":"%s"}`, p.toolName, p.router.routingMode)

	var mcpRequest []byte
	var err error

	if p.router.routingMode == RoutingModeHTTP {
		// HTTP mode: Build single tool call request (no init needed)
		mcpRequest, err = BuildHTTPToolRequest(p.toolName, request.GetArguments())
	} else {
		// Exec mode: Build init + tool framing for oneshot agents
		mcpRequest, err = BuildMCPRequest(p.toolName, request.GetArguments())
	}

	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to build MCP request",`+
			`"tool":"%s","error":"%v"}`, p.toolName, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to build request: %v", err)), nil
	}

	// Route to all nodes
	results, err := p.router.RouteToAllNodes(ctx, mcpRequest)
	if err != nil {
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to route to nodes: %v", err)), nil
	}

	// Aggregate results - parsing differs by mode
	aggregated := p.aggregateResults(results)
	// ... rest unchanged
}
```

---

### Task 5: Add HTTP Request Builder

Add a builder for HTTP mode requests (simpler than stdio framing).

**File:** `pkg/gateway/framing.go`

**Add function:**

```go
// BuildHTTPToolRequest creates a JSON-RPC request for HTTP mode agents.
// Unlike BuildMCPRequest, this does not include init framing since HTTP
// agents maintain persistent sessions.
func BuildHTTPToolRequest(toolName string, arguments interface{}) ([]byte, error) {
	if toolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: MCPToolCallParams{
			Name:      toolName,
			Arguments: arguments,
		},
		ID: 1,
	}

	return json.Marshal(req)
}

// ParseHTTPResponse extracts the tool result from an HTTP mode response.
// HTTP responses contain a single JSON-RPC response (no multi-line parsing).
func ParseHTTPResponse(response []byte) (interface{}, error) {
	if len(response) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(response, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s",
			mcpResp.Error.Code, mcpResp.Error.Message)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(mcpResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return nil, fmt.Errorf("tool error: %s", toolResult.Content[0].Text)
		}
		return nil, fmt.Errorf("tool error: unknown")
	}

	if len(toolResult.Content) == 0 {
		return nil, nil
	}

	text := toolResult.Content[0].Text
	var data interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return text, nil
	}

	return data, nil
}
```

**Acceptance criteria:**
- [ ] `BuildHTTPToolRequest()` creates single JSON-RPC message
- [ ] `ParseHTTPResponse()` handles single-object response
- [ ] No newline framing needed for HTTP mode

> 💡 **Commit:** `feat(gateway): add HTTP request/response helpers`

---

### Task 6: Add Helm Configuration for Routing Mode

**File:** `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**Update gateway section:**

```yaml
# Gateway configuration
gateway:
  # -- Enable gateway deployment
  enabled: false

  # -- Number of gateway replicas
  replicas: 1

  # -- Gateway HTTP port
  port: 8080

  # -- Routing mode for agent communication
  # http: Direct HTTP to agent pods (recommended, requires transport.mode=http)
  # exec: kubectl exec to agent pods (legacy, works with any transport.mode)
  routingMode: http

  # -- Timeout for kubectl exec operations (only used in exec mode)
  execTimeout: "60s"

  # ... rest unchanged
```

**Acceptance criteria:**
- [ ] `gateway.routingMode` defaults to `http`
- [ ] Comment explains relationship to `transport.mode`

> 💡 **Commit:** `feat(helm): add gateway.routingMode configuration`

---

### Task 7: Update Gateway Deployment Template

**File:** `deployment/helm/k8s-gpu-mcp-server/templates/gateway-deployment.yaml`

**Update args:**

```yaml
args:
- "--gateway"
- "--port={{ .Values.gateway.port }}"
- "--namespace={{ include "k8s-gpu-mcp-server.namespace" . }}"
- "--mode={{ .Values.agent.mode }}"
- "--routing-mode={{ .Values.gateway.routingMode }}"
```

**Acceptance criteria:**
- [ ] `--routing-mode` flag passed to gateway

> 💡 **Commit:** `feat(helm): pass routing-mode to gateway deployment`

---

### Task 8: Update Agent Main to Accept Routing Mode Flag

**File:** `cmd/agent/main.go`

**Add flag and pass to router:**

```go
var routingMode = flag.String("routing-mode", "http",
	"Gateway routing mode: http or exec")

// In gateway setup:
router := gateway.NewRouter(k8sClient,
	gateway.WithRoutingMode(gateway.RoutingMode(*routingMode)))
```

**Acceptance criteria:**
- [ ] `--routing-mode` flag added
- [ ] Passed to Router via option

> 💡 **Commit:** `feat(agent): add --routing-mode flag for gateway`

---

### Task 9: Update Router Unit Tests

**File:** `pkg/gateway/router_test.go`

**Add tests for routing mode:**

```go
func TestRouter_RoutingModeHTTP(t *testing.T) {
	// Test that HTTP routing is used when pod has IP
}

func TestRouter_FallbackToExec(t *testing.T) {
	// Test fallback to exec when pod has no IP
}

func TestRouter_ExecModeOnly(t *testing.T) {
	// Test exec mode when explicitly configured
}
```

**Acceptance criteria:**
- [ ] Test HTTP routing mode
- [ ] Test exec fallback
- [ ] Test explicit exec mode

> 💡 **Commit:** `test(gateway): add router routing mode tests`

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Run gateway tests specifically
go test ./pkg/gateway/... -v -count=1

# Verify Helm template renders correctly
helm template gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  --set gateway.enabled=true \
  --set gateway.routingMode=http | grep -A 5 "routing-mode"
```

### Integration Testing (Real Cluster)

```bash
# Deploy with gateway enabled (HTTP mode)
helm upgrade --install gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics --create-namespace \
  --set gateway.enabled=true \
  --set gateway.routingMode=http

# Verify gateway pod is running
kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway

# Check gateway logs for HTTP routing
kubectl logs -n gpu-diagnostics -l app.kubernetes.io/component=gateway \
  | grep -i "routing"

# Test via gateway service
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-gateway 8080:8080 &
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
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
| 1 | `feat(gateway): add AgentHTTPClient with connection pooling` |
| 2 | `test(gateway): add AgentHTTPClient unit tests` |
| 3 | `feat(gateway): add HTTP routing mode to Router` |
| 4 | `feat(gateway): add HTTP request/response helpers` |
| 5 | `feat(helm): add gateway.routingMode configuration` |
| 6 | `feat(helm): pass routing-mode to gateway deployment` |
| 7 | `feat(agent): add --routing-mode flag for gateway` |
| 8 | `test(gateway): add router routing mode tests` |

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(gateway): route to agents via HTTP instead of kubectl exec" \
  --body "Fixes #115

## Summary

Refactors the gateway router to send MCP requests to agent pods via HTTP
(pod-to-pod networking) instead of \`kubectl exec\` (SPDY).

## Changes

- Add \`AgentHTTPClient\` with connection pooling and retry logic
- Add \`RoutingMode\` (http/exec) to Router with HTTP as default
- Add \`BuildHTTPToolRequest()\` for simpler HTTP mode framing
- Add \`gateway.routingMode\` Helm configuration
- Add \`--routing-mode\` flag to agent binary
- Exec fallback when pod has no IP

## Architecture

**Before (5 layers):**
\`\`\`
Gateway → client-go → SPDY exec → Agent process → NVML
\`\`\`

**After (3 layers):**
\`\`\`
Gateway → HTTP client → Agent HTTP server → NVML
\`\`\`

## Performance Impact

| Metric | Exec mode | HTTP mode | Improvement |
|--------|-----------|-----------|-------------|
| Protocol layers | 5 | 3 | 40% fewer |
| Connection reuse | No | Yes (pooled) | Reduced latency |
| Request framing | init+tool | tool only | Simpler |

## Testing

- [ ] Unit tests pass
- [ ] Integration tested in real cluster
- [ ] Exec fallback verified
- [ ] Helm template renders correctly

## Backward Compatibility

Exec mode remains available via \`gateway.routingMode: exec\` for clusters
using stdio transport mode.

## Related

- Parent epic: #112
- Depends on: #121 (HTTP transport default)" \
  --label "kind/feature" \
  --label "prio/p1-high"
```

---

## Success Criteria

| Metric | Before | After |
|--------|--------|-------|
| Protocol layers | 5 (Gateway→client-go→SPDY→Agent→NVML) | 3 (Gateway→HTTP→Agent) |
| Connection reuse | No (new exec per request) | Yes (connection pooling) |
| Request framing | init+tool (2 JSON objects) | tool only (1 JSON object) |
| Routing mode | exec only | http (default) / exec (fallback) |

---

## Related Files

- `pkg/gateway/http_client.go` - **New:** HTTP client
- `pkg/gateway/router.go` - Router with routing mode
- `pkg/gateway/proxy.go` - ProxyHandler
- `pkg/gateway/framing.go` - Request/response framing
- `pkg/k8s/client.go` - GPUNode with GetAgentHTTPEndpoint()
- `deployment/helm/k8s-gpu-mcp-server/values.yaml` - Helm config
- `cmd/agent/main.go` - CLI flags

---

## Notes

- This change is **backward compatible** - exec mode remains available
- HTTP mode requires agents running in HTTP transport mode (#121)
- Connection pooling significantly reduces latency for repeated requests
- Retry with exponential backoff handles transient network issues

---

**Reply "GO" when ready to start implementation.** 🚀
