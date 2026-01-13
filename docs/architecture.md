# Architecture

This document describes the architecture, design decisions, and technical
implementation of `k8s-gpu-mcp-server`.

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [System Architecture](#system-architecture)
- [Component Design](#component-design)
- [Data Flow](#data-flow)
- [Security Model](#security-model)
- [Performance Considerations](#performance-considerations)

## Overview

`k8s-gpu-mcp-server` is an **ephemeral diagnostic agent** that provides real-time
NVIDIA GPU hardware introspection for Kubernetes clusters via the Model
Context Protocol (MCP).

### Key Characteristics

- **On-Demand**: Agent runs only during diagnostic sessions
- **HTTP Transport**: JSON-RPC 2.0 over HTTP/SSE (production default)
- **Stdio Transport**: Legacy mode for direct kubectl exec
- **AI-Native**: Designed for AI assistant consumption (Claude, Cursor)
- **Hardware-Focused**: Direct NVML access for deep GPU diagnostics
- **Multi-Platform**: Kubernetes (DaemonSet), Docker, Slurm, workstations

## Design Principles

### 1. On-Demand Diagnostics

Unlike traditional monitoring (always-running exporters with network endpoints),
the agent only runs during active diagnostic sessions:

```
Traditional Monitoring:              k8s-gpu-mcp-server:
┌─────────────┐                     ┌─────────────────────────┐
│ DaemonSet   │                     │ DaemonSet               │
│ (Always On) │                     │ └─ sleep infinity       │
└──────┬──────┘                     └───────────┬─────────────┘
       │                                        │
       ▼                                        │ kubectl exec
┌─────────────┐                                 ▼
│   Metrics   │                     ┌─────────────────────────┐
│   Server    │                     │ /agent runs             │
│  :9090/tcp  │                     │ └─ MCP session (stdio)  │
└─────────────┘                     └───────────┬─────────────┘
                                                │
                                                ▼ session ends
                                    ┌─────────────────────────┐
                                    │ Back to sleep infinity  │
                                    │ (near-zero resource)    │
                                    └─────────────────────────┘
```

**Benefits:**
- **Minimal resource usage** when idle (sleeping container ≈ 0 CPU)
- **No port exposure** or network listeners
- **On-demand** diagnostics via `kubectl exec`
- **No GPU allocation** — doesn't block scheduler
- **Works with AI agents** and human SREs alike

### 2. Transport Options

**Transport Modes:**

1. **Works through kubectl exec** SPDY tunneling
2. **Works with Docker** direct stdin/stdout
3. **No network configuration** required
4. **Firewall-friendly** (no listening ports)
5. **Simpler security model**
6. **Standard I/O redirection** compatible

**Trade-offs:**
- Cannot be used as standalone HTTP server (by design)
- Requires MCP-compatible client (Claude Desktop, Cursor, AI agents)

### 3. Interface Abstraction

We abstract NVML behind a Go interface:

```go
type Interface interface {
    Init(ctx context.Context) error
    GetDeviceCount(ctx context.Context) (int, error)
    GetDeviceByIndex(ctx context.Context, idx int) (Device, error)
    // ...
}
```

**Benefits:**
- **Testable**: Mock implementation for CI/development
- **Flexible**: Can add other GPU vendors (AMD, Intel)
- **Safe**: Isolates CGO complexity
- **Portable**: Tests run on any platform

## System Architecture

### Deployment Modes

k8s-gpu-mcp-server supports multiple deployment modes:

| Mode | Use Case | Command |
|------|----------|---------|
| **Kubernetes** | Production GPU clusters | `kubectl exec -it <pod> -- /agent` |
| **Docker** | Slurm, workstations, NVIDIA Spark | `docker run --gpus all ... /agent` |
| **Direct** | Development, testing | `./agent --nvml-mode=mock` |

### High-Level Architecture (Kubernetes)

```
┌──────────────────────────────────────────────────────────────┐
│                      AI Host / SRE Workstation                │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  MCP Client (Claude Desktop, Cursor, or AI Agent)      │  │
│  │  - Sends JSON-RPC 2.0 requests                         │  │
│  │  - Receives structured responses                       │  │
│  └────────────┬───────────────────────────────────────────┘  │
└───────────────┼──────────────────────────────────────────────┘
                │
                │ kubectl exec -it <daemonset-pod> -- /agent
                │ (SPDY Stdio Tunnel)
                ▼
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes Node (GPU-enabled)              │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  DaemonSet Pod (k8s-gpu-mcp-server)                         │  │
│  │  - runtimeClassName: nvidia (CDI injection)            │  │
│  │  - No nvidia.com/gpu request (doesn't block scheduler) │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  /agent (runs on exec, exits on disconnect)      │  │  │
│  │  │                                                   │  │  │
│  │  │  ┌─────────────┐      ┌──────────────┐          │  │  │
│  │  │  │ MCP Server  │◄────►│ Tool Handlers│          │  │  │
│  │  │  │ (stdio)     │      │              │          │  │  │
│  │  │  └─────────────┘      └──────┬───────┘          │  │  │
│  │  │                              │                   │  │  │
│  │  │                              ▼                   │  │  │
│  │  │                       ┌──────────────┐          │  │  │
│  │  │                       │ NVML Client  │          │  │  │
│  │  │                       │ (Real)       │          │  │  │
│  │  │                       └──────┬───────┘          │  │  │
│  │  └──────────────────────────────┼──────────────────┘  │  │
│  └─────────────────────────────────┼─────────────────────┘  │
│                                    │                         │
│                                    ▼                         │
│                            ┌──────────────┐                  │
│                            │ libnvidia-ml │                  │
│                            │  (via CDI)   │                  │
│                            └──────┬───────┘                  │
│                                   │                          │
│                                   ▼                          │
│                          ┌──────────────────┐                │
│                          │  GPU Hardware    │                │
│                          │  (Tesla T4, A100)│                │
│                          └──────────────────┘                │
└──────────────────────────────────────────────────────────────┘

### HTTP Transport Architecture

The gateway routes requests to agents via HTTP for low-overhead communication:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Gateway → Agent HTTP Routing                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Cursor/Claude                                                       │
│       │                                                              │
│       │ MCP over stdio                                               │
│       ▼                                                              │
│  ┌─────────────────┐                                                 │
│  │  NPM Bridge     │ (kubectl port-forward)                          │
│  │  (local)        │                                                 │
│  └────────┬────────┘                                                 │
│           │ HTTP                                                     │
│           ▼                                                          │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Kubernetes Cluster                        │    │
│  │  ┌─────────────────┐         ┌─────────────────────────┐    │    │
│  │  │  Gateway Pod    │  HTTP   │  Agent DaemonSet        │    │    │
│  │  │  :8080          │────────▶│  :8080 (per GPU node)   │    │    │
│  │  │  - Router       │         │  - NVML Client          │    │    │
│  │  │  - CircuitBreaker│        │  - GPU Tools            │    │    │
│  │  │  - Metrics      │         │  - Health Endpoints     │    │    │
│  │  └─────────────────┘         └─────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

**Key Benefits of HTTP Routing:**
- **Low latency**: Direct HTTP calls (~12-57ms per request)
- **Low memory**: Persistent process (~15-20MB vs 200MB spikes with exec)
- **Circuit breaker**: Automatic failover for unhealthy nodes
- **Metrics**: Prometheus-compatible observability

```

### GPU Access in Kubernetes

GPU access requires CDI (Container Device Interface) injection, which is provided
by the nvidia-container-toolkit. The agent supports clusters with:

- **NVIDIA Device Plugin** — Standard GPU scheduling
- **NVIDIA GPU Operator** — Full-stack GPU management
- **NVIDIA DRA Driver** — Dynamic Resource Allocation (K8s 1.32+)

> **Note:** The agent does NOT request `nvidia.com/gpu` resources. It monitors
> all GPUs on a node without consuming scheduler-visible resources.

For detailed deployment information, see
[Architecture Decision Report](reports/k8s-deploy-architecture-decision.md).

**Helm Chart:** [`deployment/helm/k8s-gpu-mcp-server/`](../deployment/helm/k8s-gpu-mcp-server/)

The chart supports three GPU access modes:

| Mode | Default | Description |
|------|---------|-------------|
| **RuntimeClass** | ✓ | Uses `runtimeClassName: nvidia` + `NVIDIA_VISIBLE_DEVICES=all` |
| **Resource Request** | | Requests `nvidia.com/gpu` from device plugin (fallback) |
| **DRA** | | Uses Kubernetes Dynamic Resource Allocation (K8s 1.26+) |

Install with:
```bash
# RuntimeClass mode (recommended)
helm install k8s-gpu-mcp-server ./deployment/helm/k8s-gpu-mcp-server

# Fallback mode (no RuntimeClass)
helm install k8s-gpu-mcp-server ./deployment/helm/k8s-gpu-mcp-server \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceRequest.enabled=true
```

### Component Layers

```
┌─────────────────────────────────────────┐
│  CLI Layer (cmd/agent/main.go)          │
│  - Flag parsing                          │
│  - Lifecycle management                  │
│  - Signal handling                       │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│  MCP Server Layer (pkg/mcp/)            │
│  - JSON-RPC 2.0 protocol                │
│  - Stdio transport                       │
│  - Tool registration                     │
│  - Request routing                       │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│  Tool Layer (pkg/tools/)                │
│  - get_gpu_inventory                     │
│  - get_gpu_health                        │
│  - analyze_xid_errors                    │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│  NVML Abstraction (pkg/nvml/)           │
│  ┌─────────────┐    ┌─────────────┐    │
│  │    Mock     │    │    Real     │    │
│  │ (Testing)   │    │ (go-nvml)   │    │
│  └─────────────┘    └─────────────┘    │
└───────────────────────────────────────┘
```

## Component Design

### MCP Server (`pkg/mcp/`)

**Responsibilities:**
- Initialize MCP server with stdio transport
- Register tool handlers
- Parse JSON-RPC 2.0 requests
- Route to appropriate handlers
- Marshal responses
- Manage server lifecycle

**Key Implementation:**
```go
type Server struct {
    mcpServer  *server.MCPServer
    mode       string
    nvmlClient nvml.Interface
}

func (s *Server) Run(ctx context.Context) error {
    // Blocks on stdio, serves JSON-RPC requests
    return server.ServeStdio(s.mcpServer)
}
```

### NVML Abstraction (`pkg/nvml/`)

**Interface Design:**
```go
type Interface interface {
    Init(ctx context.Context) error
    Shutdown(ctx context.Context) error
    GetDeviceCount(ctx context.Context) (int, error)
    GetDeviceByIndex(ctx context.Context, idx int) (Device, error)
}

type Device interface {
    GetName(ctx context.Context) (string, error)
    GetUUID(ctx context.Context) (string, error)
    // ... more methods
}
```

**Implementations:**

1. **Mock** (`mock.go`)
   - Fake NVIDIA A100 GPUs
   - No CGO dependency
   - Fast, deterministic tests
   - Used in CI/CD

2. **Real** (`real.go`) - **Requires CGO**
   - Wraps `github.com/NVIDIA/go-nvml`
   - Requires NVIDIA driver
   - Production use

3. **Stub** (`real_stub.go`) - **Non-CGO Fallback**
   - Compiles without CGO
   - Returns helpful error messages
   - Used in non-GPU CI builds

### Tool Handlers (`pkg/tools/`)

Each tool follows a consistent pattern:

```go
type XYZHandler struct {
    nvmlClient nvml.Interface
}

func (h *XYZHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 1. Check context cancellation
    // 2. Extract arguments
    // 3. Call NVML
    // 4. Format response as JSON
    // 5. Return MCP result
}
```

## Data Flow

### 1. Agent Startup

```
main() 
  └─► Parse flags (--mode, --nvml-mode)
      └─► Initialize NVML (mock or real)
          └─► Create MCP Server
              └─► Register tools
                  └─► Start stdio server
                      └─► Block on stdin
```

### 2. Request Processing

```
stdin (JSON-RPC)
  │
  ▼
MCP Server
  │
  ├─► Parse JSON-RPC 2.0
  ├─► Validate protocol version
  ├─► Route to handler
  │
  ▼
Tool Handler
  │
  ├─► Check context
  ├─► Validate arguments
  ├─► Call NVML client
  │
  ▼
NVML Client (Mock or Real)
  │
  ├─► Query GPU hardware
  ├─► Format data
  │
  ▼
Tool Handler
  │
  ├─► Marshal to JSON
  │
  ▼
MCP Server
  │
  ├─► Wrap in JSON-RPC response
  │
  ▼
stdout (JSON-RPC)
```

### 3. Logging Flow

```
Application Logs ──► stderr (Structured JSON)
MCP Protocol    ──► stdout (JSON-RPC messages)
```

**Critical**: Logs NEVER go to stdout (breaks MCP protocol)

## Security Model

### Read-Only Mode (Default)

```
Allowed:
✓ Query GPU properties (name, UUID, temp, memory)
✓ Read telemetry (power, utilization)
✓ Inspect topology (NVLink, PCIe)
✓ Read ECC counters
✓ Parse XID errors

Denied:
✗ Kill GPU processes
✗ Reset GPUs
✗ Modify settings
```

### Operator Mode (Explicit Flag)

```
--mode=operator enables:
✓ All read-only operations
✓ Kill GPU processes by PID
✓ Trigger GPU reset
```

**Activation:**
```bash
# Kubernetes
kubectl exec -it <daemonset-pod> -- /agent --mode=operator

# Docker
docker run --rm -it --gpus all <image> --mode=operator
```

### Kubernetes Security Context

When deployed as a DaemonSet with RuntimeClass:

```yaml
securityContext:
  runAsUser: 0                    # Root may be required for NVML
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
    # add: ["SYS_ADMIN"]          # Only if profiling metrics needed
```

**Key Security Properties:**
- **NOT Privileged**: Uses RuntimeClass for GPU access, not privileged mode
- **No GPU Resource Request**: Doesn't consume `nvidia.com/gpu` resources
- **Host Network**: No
- **Host PID**: No (not needed for basic monitoring)
- **Zero Trust**: Follows principle of least privilege

For detailed security analysis, see
[Architecture Decision Report](reports/k8s-deploy-architecture-decision.md).

### Input Sanitization

All tool inputs are validated:

```go
// Example: GPU index validation
if idx < 0 || idx >= deviceCount {
    return error
}

// Example: PID validation
if pid <= 0 || pid > maxPID {
    return error
}
```

## Performance Considerations

### Binary Size

| Component | Size | Strategy |
|-----------|------|----------|
| Go runtime | ~2MB | Stripped binaries (`-ldflags="-s -w"`) |
| MCP library | ~1MB | Minimal dependencies |
| NVML bindings | ~4MB | Dynamic linking to `libnvidia-ml.so` |
| **Total** | **~7MB** | 86% under 50MB target |

### Response Time

| Operation | Latency | Notes |
|-----------|---------|-------|
| Initialization | <100ms | One-time cost |
| GPU Inventory | <50ms | Cached device handles |
| Telemetry Query | <10ms | Direct NVML calls |
| XID Analysis | <200ms | Parses dmesg buffer |

### Memory Footprint

- **Base**: ~15MB (Go runtime + libraries)
- **Per GPU**: ~100KB (device handles + state)
- **Typical**: <30MB for 8-GPU node

### Concurrency Model

```go
// Main goroutine
main() {
    ctx, cancel := context.WithCancel(context.Background())
    
    // Server goroutine
    go mcpServer.Run(ctx)
    
    // Signal handler goroutine
    go handleSignals(cancel)
    
    <-done // Wait for completion
}
```

**Thread Safety:**
- NVML is **not thread-safe**: We serialize all calls
- Context propagation ensures coordinated cancellation
- No shared state between requests

## Technology Stack

### Core Dependencies

```go
require (
    github.com/mark3labs/mcp-go v0.43.2        // MCP protocol
    github.com/NVIDIA/go-nvml v0.13.0-1        // NVML bindings
    github.com/stretchr/testify v1.10.0        // Testing
)
```

### Build Configuration

- **Language**: Go 1.25+
- **CGO**: Enabled for real NVML, disabled for CI
- **Static Binary**: Yes (with dynamic NVML linking)
- **Base Image**: `gcr.io/distroless/base-debian12`

### NVML Library

**Dynamic Linking:**
```
agent (binary)
  │
  ├─► dlopen("libnvidia-ml.so.1")
  │     │
  │     └─► NVIDIA Driver (/usr/lib/x86_64-linux-gnu/)
  │
  └─► If not found: Error or fallback to mock
```

**Why Dynamic?**
- Different driver versions have different libraries
- Avoids binary bloat
- Works across NVIDIA driver versions

## Component Interactions

### Startup Sequence

```mermaid
sequenceDiagram
    participant M as main()
    participant N as NVML Client
    participant S as MCP Server
    participant H as Tool Handlers

    M->>M: Parse flags
    M->>N: Init(ctx)
    N-->>M: Success
    M->>S: New(config)
    S->>H: Register tools
    H-->>S: Tools registered
    M->>S: Run(ctx)
    S->>S: ServeStdio()
    Note over S: Blocks on stdin
```

### Request/Response Cycle

```mermaid
sequenceDiagram
    participant C as Client (stdin)
    participant S as MCP Server
    participant H as Tool Handler
    participant N as NVML Client
    participant G as GPU Hardware

    C->>S: JSON-RPC Request
    S->>S: Parse & Validate
    S->>H: Route to handler
    H->>H: Check context
    H->>H: Validate args
    H->>N: Query GPU
    N->>G: NVML API call
    G-->>N: Hardware response
    N-->>H: Typed data
    H->>H: Format as JSON
    H-->>S: MCP Result
    S->>S: Wrap in JSON-RPC
    S-->>C: Response (stdout)
```

### Error Handling Flow

```
User Error (bad args)
  └─► Tool Handler
      └─► Returns MCP error result
          └─► Client sees error, can retry

System Error (no GPU)
  └─► NVML Client
      └─► Returns Go error
          └─► Tool Handler
              └─► Returns MCP error result

Protocol Error (bad JSON)
  └─► MCP Server
      └─► Returns JSON-RPC error
          └─► Client sees protocol error
```

## Design Decisions

### Decision 1: Interface vs Direct NVML

**Chosen**: Interface abstraction

**Alternatives Considered:**
1. Direct `go-nvml` usage everywhere
2. Dependency injection
3. **Selected: Interface abstraction**

**Rationale:**
- Enables testing without GPU
- Isolates CGO complexity
- Future-proof for multi-vendor

### Decision 2: HTTP vs Stdio Transport

**Chosen**: HTTP primary, Stdio secondary

**Alternatives Considered:**
1. HTTP + Stdio
2. WebSocket
3. gRPC
4. **Selected: Stdio only**

**Rationale:**
- Aligns with on-demand design
- Works through kubectl exec
- Works with Docker direct execution
- Zero network exposure
- Simpler security

### Decision 5: DaemonSet vs kubectl debug

**Chosen**: DaemonSet with `kubectl exec`

**Alternatives Considered:**
1. `kubectl debug` ephemeral containers
2. On-demand Pod creation
3. **Selected: DaemonSet with sleeping container**

**Rationale:**
- `kubectl debug` cannot access GPUs (bypasses device plugin)
- On-demand Pod has ~5-10s startup overhead
- DaemonSet provides instant access via exec
- Sleeping container has near-zero resource usage
- Agent runs only during active sessions

See [Architecture Decision Report](reports/k8s-deploy-architecture-decision.md)
for detailed analysis.

### Decision 3: Runtime vs Compile-Time Mode Selection

**Chosen**: Runtime flag (`--nvml-mode`)

**Alternatives Considered:**
1. Compile two binaries (mock/real)
2. Auto-detect GPU
3. **Selected: Runtime flag**

**Rationale:**
- Single binary for all environments
- Explicit control
- Easy testing
- Clear intent

### Decision 4: Go 1.25 (Latest)

**Chosen**: Go 1.25.5

**Alternatives Considered:**
1. Go 1.23 (conservative)
2. Go 1.24 (stable)
3. **Selected: Go 1.25 (latest)**

**Rationale:**
- Go 1.23 EOL in August 2025
- Latest features and security patches
- Better performance
- Community moving forward

## File Structure

```
k8s-gpu-mcp-server/
├── cmd/agent/              # Entry point
│   └── main.go             # CLI, lifecycle, wiring
│
├── pkg/                    # Public packages
│   ├── mcp/                # MCP protocol layer
│   │   ├── server.go       # Stdio server wrapper
│   │   └── server_test.go  # Unit tests
│   │
│   ├── nvml/               # NVML abstraction
│   │   ├── interface.go    # Contract
│   │   ├── mock.go         # Fake implementation
│   │   ├── real.go         # Real NVML (CGO)
│   │   ├── real_stub.go    # Non-CGO stub
│   │   ├── mock_test.go    # Mock tests
│   │   └── real_test.go    # Integration tests
│   │
│   └── tools/              # MCP tool handlers
│       ├── gpu_inventory.go
│       └── gpu_inventory_test.go
│
├── internal/               # Private implementation
│   └── info/               # Version injection
│
├── examples/               # Sample requests
│   ├── initialize.json
│   ├── gpu_inventory.json
│   ├── gpu_health.json
│   └── analyze_xid.json
│
└── docs/                   # Documentation
    ├── quickstart.md
    ├── architecture.md     # This file
    └── mcp-usage.md
```

## Extension Points

### Adding New Tools

1. **Define tool in `pkg/tools/`**
2. **Create handler struct**
3. **Implement `Handle()` method**
4. **Register in `pkg/mcp/server.go`**
5. **Add example in `examples/`**
6. **Write tests**

Example:
```go
// pkg/tools/my_tool.go
func (h *MyToolHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // Implementation
}
```

### Supporting New GPU Vendors

To add AMD or Intel GPU support:

1. **Implement `nvml.Interface`** for vendor SDK
2. **Add runtime flag**: `--nvml-mode=amd|intel`
3. **Update main.go** selection logic
4. **Add tests**

No changes needed to MCP layer or tool handlers!

### Future Enhancements

- **eBPF Integration**: Real-time XID error streaming via kernel tracing
- **kubectl Plugin**: `kubectl mcp diagnose <node>` for simplified UX
- **Streaming Telemetry**: Server-Sent Events for real-time data
- **Batch Operations**: Query multiple GPUs in parallel
- **Caching**: Cache device handles to reduce latency
- **Multi-Vendor**: AMD ROCm, Intel oneAPI support

## References

- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [NVIDIA NVML Documentation](https://docs.nvidia.com/deploy/nvml-api/)
- [go-nvml Library](https://github.com/NVIDIA/go-nvml)
- [NVIDIA Device Plugin](https://github.com/NVIDIA/k8s-device-plugin)
- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator)
- [Architecture Decision Report](reports/k8s-deploy-architecture-decision.md)

