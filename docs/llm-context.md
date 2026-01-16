# LLM Context: k8s-gpu-mcp-server

> **For AI Assistants**: This document provides structured context for understanding
> and working with the k8s-gpu-mcp-server codebase. It is designed to be consumed
> by LLMs (Claude, GPT, etc.) to quickly understand the project architecture,
> available tools, and common patterns.

## Project Identity

| Property | Value |
|----------|-------|
| **Name** | k8s-gpu-mcp-server |
| **Purpose** | NVIDIA GPU diagnostics for Kubernetes via MCP |
| **Language** | Go 1.25.0+ |
| **Protocol** | MCP (Model Context Protocol) / JSON-RPC 2.0 |
| **Transport** | HTTP (default), Stdio (legacy) |
| **License** | Apache 2.0 |

## One-Line Summary

Ephemeral diagnostic agent providing real-time NVIDIA GPU hardware introspection
for Kubernetes clusters via MCP, designed for AI-assisted SRE troubleshooting.

## Architecture Overview

```
MCP Client (Claude/Cursor)
    │
    │ HTTP/stdio
    ▼
Gateway Pod (:8080) ─────► Routes to all nodes
    │
    │ HTTP (pod-to-pod)
    ▼
Agent Pods (DaemonSet)
    │
    │ CGO/NVML
    ▼
GPU Hardware
```

**Key Design Decisions:**
- HTTP-first transport (150× faster than exec routing)
- Interface abstraction for NVML (testable without GPU)
- Gateway with circuit breaker for multi-node clusters
- ~15-20MB memory footprint when idle

## Available MCP Tools

### 1. `get_gpu_inventory`
**Category:** NVML  
**Arguments:** None  
**Returns:** Hardware inventory + telemetry (name, UUID, memory, temp, power, utilization)

### 2. `get_gpu_health`
**Category:** NVML  
**Arguments:** None  
**Returns:** Health scoring (0-100), status checks, recommendations

### 3. `analyze_xid_errors`
**Category:** NVML  
**Arguments:** None  
**Returns:** XID errors from /dev/kmsg with severity and recommendations

### 4. `describe_gpu_node`
**Category:** K8s + NVML  
**Arguments:** `node_name` (required)  
**Returns:** Node metadata + GPU hardware + running pods

### 5. `get_pod_gpu_allocation`
**Category:** K8s  
**Arguments:** `node_name` (required), `namespace` (optional)  
**Returns:** GPU-to-Pod correlation via resource requests

## File Structure (Key Paths)

```
cmd/agent/main.go          # Entry point, CLI flags
pkg/mcp/server.go          # MCP server, tool registration
pkg/mcp/http.go            # HTTP transport
pkg/gateway/router.go      # Gateway routing
pkg/gateway/circuit_breaker.go
pkg/tools/                  # Tool handlers (5 files)
pkg/nvml/interface.go      # NVML abstraction
pkg/nvml/mock.go           # Mock for testing
pkg/nvml/real.go           # Real NVML (CGO)
pkg/k8s/client.go          # Kubernetes client
deployment/helm/           # Helm chart
```

## Common Patterns

### Tool Handler Pattern
```go
type XYZHandler struct {
    nvmlClient nvml.Interface
}

func (h *XYZHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 1. Validate arguments
    // 2. Query NVML/K8s
    // 3. Return JSON result
}
```

### Error Handling
```go
// Always wrap errors with context
return fmt.Errorf("failed to get device %d: %w", idx, err)

// Tool errors return JSON, not Go errors
return mcp.NewToolResultError("node_name is required"), nil
```

### Context Propagation
```go
// All I/O operations accept context
func GetDeviceInfo(ctx context.Context, idx int) (*DeviceInfo, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... operation
}
```

## CLI Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `read-only` | `read-only` or `operator` |
| `--nvml-mode` | `mock` | `mock` or `real` |
| `--port` | `0` | HTTP port (0=stdio) |
| `--gateway` | `false` | Enable gateway mode |
| `--routing-mode` | `http` | `http` or `exec` |
| `--namespace` | `gpu-diagnostics` | Agent namespace |

## Testing

```bash
make test        # 538 tests, race detector enabled
make coverage    # Coverage report
```

**Mock mode** allows testing without GPU hardware.

## Common Troubleshooting Patterns

### XID Errors
- XID 48: Double-bit ECC error → Hardware replacement
- XID 79: GPU fell off bus → Check PCIe, reseat card
- XID 13: Graphics engine exception → Check workload/driver

### Gateway Issues
- Cross-node HTTP timeout → Check CNI (Calico VXLAN mode)
- Circuit breaker open → Node unhealthy, check agent pod

### NVML Failures
- "Failed to initialize NVML" → Driver not loaded
- "Real NVML requires CGO" → Binary built without CGO

## Kubernetes Resources

| Resource | Name | Purpose |
|----------|------|---------|
| DaemonSet | k8s-gpu-mcp-server | Agent on GPU nodes |
| Deployment | k8s-gpu-mcp-server-gateway | Request router |
| Service | k8s-gpu-mcp-server | Agent headless |
| ServiceAccount | k8s-gpu-mcp-server-agent | Agent RBAC |
| ClusterRole | k8s-gpu-mcp-server-agent | Node/Pod read |

## JSON-RPC Examples

### Initialize Session
```json
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"client","version":"1.0"}},"id":0}
```

### Call Tool
```json
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}
```

### List Tools
```json
{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}
```

## Development Guidelines

1. **No stdout in logs** - Breaks MCP stdio protocol
2. **Context everywhere** - All I/O functions take `ctx`
3. **Interface abstraction** - Mock NVML for testing
4. **Table-driven tests** - Use testify/assert
5. **DCO + GPG signing** - Required for all commits

## Related Documentation

- [Architecture](architecture.md) - Full system design
- [MCP Usage](mcp-usage.md) - Protocol details
- [Quick Start](quickstart.md) - Getting started
- [Security](security.md) - RBAC and permissions
- [Development Guide](../DEVELOPMENT.md) - Contributing

## Cursor IDE Rules

The project includes AI-development rules in `.cursor/rules/`:

| Rule | Purpose |
|------|---------|
| `00-general-go.mdc` | Go style, testing, error handling |
| `01-mcp-server.mdc` | MCP protocol, transport modes |
| `02-nvml-hardware.mdc` | NVML/CGO safety |
| `03-k8s-constraints.mdc` | Kubernetes deployment |
| `04-workflow-git.mdc` | Git/DCO workflow |

These rules are automatically loaded by Cursor IDE to provide context-aware
assistance when working on this codebase.
