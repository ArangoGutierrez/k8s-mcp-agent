# HTTP/SSE Transport Implementation

## Issue Reference

- **Issue:** [#71 - feat: HTTP/SSE transport for remote MCP access](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/71)
- **Priority:** P0-Blocker
- **Labels:** kind/feature, area/mcp-protocol, prio/p0-blocker
- **Milestone:** M3: The Ephemeral Tunnel
- **Blocks:** #72 (Gateway mode)

## Background

Currently, `k8s-gpu-mcp-server` only supports **stdio transport**, requiring
`kubectl exec` to interact with the agent. This limits deployment options:

- No ingress-based access
- No load balancing
- No serverless deployments
- Gateway mode (#72) requires HTTP transport

### Reference Implementation

The `containers/kubernetes-mcp-server` supports `--port` flag for HTTP mode:

```bash
kubernetes-mcp-server --port 8080
```

The `mcp-go` library provides `server.NewStreamableHTTPServer()` for HTTP
transport alongside `server.ServeStdio()` for stdio.

---

## Objective

Add HTTP/SSE transport mode to enable remote MCP access via HTTP POST and
Server-Sent Events, while maintaining backward compatibility with stdio mode.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/http-transport
```

Verify:
```bash
git branch --show-current
# Should output: feat/http-transport
```

---

## Implementation Tasks

### Task 1: Add CLI Flags for HTTP Mode

Update `cmd/agent/main.go` to add HTTP-related flags.

**Files to modify:**
- `cmd/agent/main.go`

**Changes:**

```go
var (
    mode     = flag.String("mode", ModeReadOnly, "Operation mode: read-only or operator")
    nvmlMode = flag.String("nvml-mode", "mock", "NVML mode: mock or real")
    showVer  = flag.Bool("version", false, "Show version and exit")
    logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error")
    
    // NEW: HTTP transport flags
    port     = flag.Int("port", 0, "HTTP port (0 = stdio mode, >0 = HTTP mode)")
    addr     = flag.String("addr", "0.0.0.0", "HTTP listen address")
)
```

**Acceptance criteria:**
- [ ] `--port` flag added (default 0 = stdio mode)
- [ ] `--addr` flag added (default 0.0.0.0)
- [ ] Validation: port must be 0 or 1024-65535
- [ ] Help text explains the flags

---

### Task 2: Update Server Config and Interface

Update `pkg/mcp/server.go` to support both transport modes.

**Files to modify:**
- `pkg/mcp/server.go`

**Update Config struct:**

```go
// Config holds server configuration.
type Config struct {
    Mode       string          // "read-only" or "operator"
    Version    string
    GitCommit  string
    NVMLClient nvml.Interface
    
    // Transport configuration
    Transport  TransportType   // "stdio" or "http"
    HTTPAddr   string          // HTTP listen address (e.g., "0.0.0.0:8080")
}

// TransportType defines the transport mode.
type TransportType string

const (
    TransportStdio TransportType = "stdio"
    TransportHTTP  TransportType = "http"
)
```

**Acceptance criteria:**
- [ ] `TransportType` constants defined
- [ ] `Config` includes transport settings
- [ ] Default transport is stdio

---

### Task 3: Implement HTTP Transport Handler

Add HTTP transport support using `mcp-go`'s Streamable HTTP server.

**Files to create:**
- `pkg/mcp/http.go`

**Implementation:**

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/mark3labs/mcp-go/server"
)

// HTTPServer wraps the MCP server with HTTP transport.
type HTTPServer struct {
    mcpServer  *server.MCPServer
    httpServer *http.Server
    addr       string
}

// NewHTTPServer creates an HTTP transport server.
func NewHTTPServer(mcpServer *server.MCPServer, addr string) *HTTPServer {
    return &HTTPServer{
        mcpServer: mcpServer,
        addr:      addr,
    }
}

// ListenAndServe starts the HTTP server.
func (h *HTTPServer) ListenAndServe(ctx context.Context) error {
    mux := http.NewServeMux()
    
    // MCP endpoint - Streamable HTTP transport
    streamableServer := server.NewStreamableHTTPServer(h.mcpServer)
    mux.Handle("/mcp", streamableServer)
    
    // Health check endpoint
    mux.HandleFunc("/healthz", h.handleHealthz)
    mux.HandleFunc("/readyz", h.handleReadyz)
    
    // Version endpoint
    mux.HandleFunc("/version", h.handleVersion)
    
    h.httpServer = &http.Server{
        Addr:              h.addr,
        Handler:           mux,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      30 * time.Second,
        ReadHeaderTimeout: 10 * time.Second,
    }
    
    log.Printf(`{"level":"info","msg":"HTTP server starting","addr":"%s"}`, h.addr)
    
    // Start server in goroutine
    errCh := make(chan error, 1)
    go func() {
        if err := h.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            errCh <- err
        }
    }()
    
    // Wait for context cancellation or error
    select {
    case <-ctx.Done():
        return h.Shutdown()
    case err := <-errCh:
        return err
    }
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPServer) Shutdown() error {
    if h.httpServer == nil {
        return nil
    }
    
    log.Printf(`{"level":"info","msg":"HTTP server shutting down"}`)
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    return h.httpServer.Shutdown(ctx)
}

// handleHealthz handles liveness probe.
func (h *HTTPServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
}

// handleReadyz handles readiness probe.
func (h *HTTPServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
    // TODO: Check NVML initialization status
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ready",
    })
}

// handleVersion returns version information.
func (h *HTTPServer) handleVersion(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "server":  "k8s-gpu-mcp-server",
        "version": "0.1.0", // TODO: Use actual version
    })
}
```

**Acceptance criteria:**
- [ ] `HTTPServer` struct created
- [ ] `/mcp` endpoint uses Streamable HTTP
- [ ] `/healthz` returns 200 OK
- [ ] `/readyz` returns 200 OK
- [ ] `/version` returns version JSON
- [ ] Graceful shutdown with timeout

---

### Task 4: Update Server Run Method

Modify `Server.Run()` to use the appropriate transport.

**Files to modify:**
- `pkg/mcp/server.go`

**Update Run method:**

```go
// Run starts the MCP server with the configured transport.
func (s *Server) Run(ctx context.Context) error {
    switch s.transport {
    case TransportHTTP:
        return s.runHTTP(ctx)
    default:
        return s.runStdio(ctx)
    }
}

// runStdio runs the server with stdio transport (existing code).
func (s *Server) runStdio(ctx context.Context) error {
    log.Printf(`{"level":"info","msg":"MCP server starting",` +
        `"transport":"stdio","mode":"%s"}`, s.mode)
    
    errCh := make(chan error, 1)
    go func() {
        if err := server.ServeStdio(s.mcpServer); err != nil {
            errCh <- fmt.Errorf("MCP server error: %w", err)
        }
    }()
    
    select {
    case <-ctx.Done():
        log.Printf(`{"level":"info","msg":"MCP server stopping",` +
            `"reason":"context cancelled"}`)
        return s.Shutdown()
    case err := <-errCh:
        return err
    }
}

// runHTTP runs the server with HTTP transport.
func (s *Server) runHTTP(ctx context.Context) error {
    log.Printf(`{"level":"info","msg":"MCP server starting",` +
        `"transport":"http","addr":"%s","mode":"%s"}`, s.httpAddr, s.mode)
    
    httpServer := NewHTTPServer(s.mcpServer, s.httpAddr)
    return httpServer.ListenAndServe(ctx)
}
```

**Acceptance criteria:**
- [ ] `Run()` dispatches to correct transport
- [ ] `runStdio()` preserves existing behavior
- [ ] `runHTTP()` uses new HTTP server

---

### Task 5: Update main.go to Configure Transport

Update `cmd/agent/main.go` to configure the transport based on flags.

**Files to modify:**
- `cmd/agent/main.go`

**Changes:**

```go
// Determine transport mode
var transport mcp.TransportType
var httpAddr string

if *port > 0 {
    if *port < 1024 || *port > 65535 {
        log.Fatalf(`{"level":"fatal","msg":"invalid port","port":%d,` +
            `"valid":"1024-65535 or 0 for stdio"}`, *port)
    }
    transport = mcp.TransportHTTP
    httpAddr = fmt.Sprintf("%s:%d", *addr, *port)
    log.Printf(`{"level":"info","msg":"HTTP mode enabled","addr":"%s"}`, httpAddr)
} else {
    transport = mcp.TransportStdio
}

// Initialize MCP server
mcpServer, err := mcp.New(mcp.Config{
    Mode:       *mode,
    Version:    buildInfo.Version,
    GitCommit:  buildInfo.GitCommit,
    NVMLClient: nvmlClient,
    Transport:  transport,
    HTTPAddr:   httpAddr,
})
```

**Acceptance criteria:**
- [ ] Port 0 = stdio mode
- [ ] Port > 0 = HTTP mode
- [ ] Port validation (1024-65535)
- [ ] Log message indicates transport mode

---

### Task 6: Add Tests for HTTP Transport

Create tests for the HTTP server functionality.

**Files to create:**
- `pkg/mcp/http_test.go`

**Test cases:**

```go
func TestHTTPServer_Healthz(t *testing.T) {
    // Test /healthz returns 200
}

func TestHTTPServer_Readyz(t *testing.T) {
    // Test /readyz returns 200
}

func TestHTTPServer_Version(t *testing.T) {
    // Test /version returns JSON
}

func TestHTTPServer_MCP(t *testing.T) {
    // Test /mcp endpoint accepts JSON-RPC
}

func TestHTTPServer_Shutdown(t *testing.T) {
    // Test graceful shutdown
}
```

**Acceptance criteria:**
- [ ] Health endpoint tests pass
- [ ] Version endpoint tests pass
- [ ] MCP endpoint accepts requests
- [ ] Shutdown is graceful

---

### Task 7: Update Helm Chart for HTTP Mode

Update the Helm chart to support HTTP mode deployment.

**Files to modify:**
- `deployment/helm/k8s-gpu-mcp-server/values.yaml`
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml`

**Add to values.yaml:**

```yaml
transport:
  # mode: stdio or http
  mode: stdio
  
  # HTTP mode settings (only used if mode: http)
  http:
    port: 8080
    # Service for HTTP mode
    service:
      enabled: false
      type: ClusterIP
```

**Acceptance criteria:**
- [ ] values.yaml has transport config
- [ ] DaemonSet passes --port when http mode
- [ ] Service template for HTTP mode

---

### Task 8: Update Documentation

Update docs to cover HTTP transport.

**Files to modify:**
- `docs/mcp-usage.md`
- `docs/quickstart.md`
- `README.md`

**Example content for mcp-usage.md:**

```markdown
## Transport Modes

### Stdio Mode (Default)

```bash
./bin/agent --nvml-mode=mock
```

### HTTP Mode

```bash
./bin/agent --nvml-mode=mock --port 8080

# Test with curl
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Health check
curl http://localhost:8080/healthz
```
```

**Acceptance criteria:**
- [ ] HTTP mode documented
- [ ] Curl examples provided
- [ ] Health endpoints documented

---

## Testing Requirements

### Local Testing

```bash
# Run all checks
make all

# Test stdio mode (existing)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | ./bin/agent --nvml-mode=mock

# Test HTTP mode
./bin/agent --nvml-mode=mock --port 8080 &
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
kill %1
```

### Integration Testing

```bash
# In Kubernetes (HTTP mode)
kubectl port-forward svc/k8s-gpu-mcp-server 8080:8080
curl http://localhost:8080/mcp -d '...'
```

---

## Pre-Commit Checklist

- [ ] `make fmt` - Code formatted
- [ ] `make lint` - Linter passes
- [ ] `make test` - All tests pass
- [ ] New tests for HTTP transport
- [ ] Documentation updated

---

## Commit and Push

```bash
git add -A
git commit -s -S -m "feat(mcp): add HTTP/SSE transport for remote MCP access

- Add --port and --addr CLI flags
- Implement HTTP transport using mcp-go StreamableHTTP
- Add /healthz, /readyz, /version endpoints
- Update Helm chart for HTTP mode
- Maintain backward compatibility (--port 0 = stdio)

Fixes #71"

git push -u origin feat/http-transport
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(mcp): add HTTP/SSE transport for remote MCP access" \
  --body "Fixes #71

## Summary
Adds HTTP transport mode alongside stdio for remote MCP access.

## Changes
- \`--port\` and \`--addr\` CLI flags
- HTTP server with \`/mcp\` endpoint (Streamable HTTP)
- \`/healthz\` and \`/readyz\` health endpoints
- \`/version\` endpoint
- Helm chart updates for HTTP mode
- Documentation updates

## Testing
- [ ] Stdio mode still works
- [ ] HTTP mode starts on specified port
- [ ] Health endpoints return 200
- [ ] MCP requests work over HTTP
- [ ] Graceful shutdown

## Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| \`/mcp\` | POST | MCP JSON-RPC |
| \`/healthz\` | GET | Liveness probe |
| \`/readyz\` | GET | Readiness probe |
| \`/version\` | GET | Version info |

## Usage
\`\`\`bash
# Stdio mode (default)
./agent --nvml-mode=mock

# HTTP mode
./agent --nvml-mode=mock --port 8080
\`\`\`" \
  --label "kind/feature" \
  --label "area/mcp-protocol" \
  --label "prio/p0-blocker" \
  --milestone "M3: The Ephemeral Tunnel"
```

---

## File Structure After Implementation

```
pkg/mcp/
├── server.go        # Updated with transport support
├── server_test.go   # Existing tests
├── http.go          # NEW: HTTP transport
└── http_test.go     # NEW: HTTP tests

cmd/agent/
└── main.go          # Updated with --port, --addr flags

deployment/helm/k8s-gpu-mcp-server/
├── values.yaml      # Updated with transport config
└── templates/
    └── daemonset.yaml  # Updated for HTTP mode
```

---

## Acceptance Criteria

**Must Have:**
- [ ] `--port 8080` starts HTTP server
- [ ] `--port 0` or no port = stdio mode (backward compatible)
- [ ] `/mcp` endpoint accepts JSON-RPC over HTTP POST
- [ ] `/healthz` returns 200 OK
- [ ] `/readyz` returns 200 OK
- [ ] Graceful shutdown

**Should Have:**
- [ ] SSE streaming at `/mcp` for notifications
- [ ] `/version` endpoint
- [ ] Helm chart HTTP mode support

**Nice to Have:**
- [ ] Request logging middleware
- [ ] CORS support
- [ ] TLS support

---

## Related Issues

- **#72** - Gateway mode (depends on this)
- **#28** - K8s client (can run in parallel)
- **#88** - Healthz endpoint (partially addressed)

---

## Quick Reference

```bash
# Branch
git checkout -b feat/http-transport

# Build
make agent

# Test stdio
echo '...' | ./bin/agent --nvml-mode=mock

# Test HTTP
./bin/agent --nvml-mode=mock --port 8080 &
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/mcp -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Commit
git commit -s -S -m "feat(mcp): add HTTP/SSE transport"

# PR
gh pr create --title "..." --label "kind/feature" --milestone "M3: The Ephemeral Tunnel"
```

---

**Reply "GO" when ready to start implementation.** 🚀

