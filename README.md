# k8s-mcp-agent

**Just-in-Time SRE Diagnostic Agent for NVIDIA GPU Clusters on Kubernetes**

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub Issues](https://img.shields.io/github/issues/ArangoGutierrez/k8s-mcp-agent)](https://github.com/ArangoGutierrez/k8s-mcp-agent/issues)

## Overview

`k8s-mcp-agent` is an **ephemeral diagnostic agent** that provides surgical, 
real-time NVIDIA GPU hardware introspection for Kubernetes clusters via the 
[Model Context Protocol (MCP)](https://modelcontextprotocol.io/). Unlike 
traditional monitoring systems, this agent is designed for **AI-assisted 
troubleshooting** by SREs debugging complex hardware failures that standard 
Kubernetes APIs cannot detect.

### Key Features

- **🎯 Ephemeral Injection**: No DaemonSets, no standing infrastructure
- **🔌 Stdio Transport**: JSON-RPC 2.0 over `kubectl debug` SPDY tunneling
- **🔍 Hardware Introspection**: XID errors, NVLink topology, ECC counters
- **🤖 AI-Native**: Built for Claude Desktop, Cursor, and MCP-compatible hosts
- **🔒 Read-Only Default**: Safe operations with explicit operator mode

## Architecture

```
┌─────────────┐         kubectl debug          ┌──────────────┐
│   Claude    │ ──────────────────────────────> │  K8s Node    │
│  Desktop    │    (SPDY Stdio Tunnel)          │              │
└─────────────┘                                 │  ┌────────┐  │
                                                │  │ Agent  │  │
       ▲                                        │  │ (Pod)  │  │
       │                                        │  └───┬────┘  │
       │         JSON-RPC 2.0                   │      │       │
       │         MCP Protocol                   │      ▼       │
       └────────────────────────────────────────│  ┌────────┐ │
                                                │  │ NVML   │ │
                                                │  │  API   │ │
                                                │  └───┬────┘ │
                                                │      │       │
                                                │      ▼       │
                                                │  GPU 0...N   │
                                                └──────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes cluster with NVIDIA GPUs
- `kubectl` CLI configured
- NVIDIA GPU Operator installed (for NVML library access)
- [Claude Desktop](https://claude.ai/download) or MCP-compatible client

### Launch Agent

```bash
# Read-only mode (default)
kubectl debug node/gpu-node-5 \
  --image=ghcr.io/arangogutierrez/k8s-mcp-agent:latest \
  --profile=sysadmin \
  -- /agent --mode=read-only

# Operator mode (enables kill/reset operations)
kubectl debug node/gpu-node-5 \
  --image=ghcr.io/arangogutierrez/k8s-mcp-agent:latest \
  --profile=sysadmin \
  -- /agent --mode=operator
```

### Available Tools

| Tool | Description | Mode |
|------|-------------|------|
| `get_gpu_inventory` | Static hardware map (Model, UUID, Bus ID) | Read-Only |
| `get_gpu_telemetry` | Real-time state (Temp, Power, Memory) | Read-Only |
| `inspect_topology` | NVLink/PCIe P2P capabilities | Read-Only |
| `analyze_xid_errors` | Parse and interpret XID errors from dmesg | Read-Only |
| `snapshot_ecc` | ECC error counters (SBE/DBE) | Read-Only |
| `kill_gpu_process` | Terminate GPU process by PID | Operator |
| `reset_gpu` | Secondary bus reset | Operator |

## Project Status

### Current Milestone: [M1: Foundation & API](https://github.com/ArangoGutierrez/k8s-mcp-agent/milestone/1)
**Due:** Jan 10, 2026

**Progress:** ✅ 3/3 core issues complete (Scaffolding, MCP Server, CI)

**Upcoming:**
- [M2: Hardware Introspection](https://github.com/ArangoGutierrez/k8s-mcp-agent/milestone/2) - Due Jan 17
- [M3: The Ephemeral Tunnel](https://github.com/ArangoGutierrez/k8s-mcp-agent/milestone/3) - Due Jan 24
- [M4: Safety & Release](https://github.com/ArangoGutierrez/k8s-mcp-agent/milestone/4) - Due Jan 31

See [Milestones](https://github.com/ArangoGutierrez/k8s-mcp-agent/milestones) 
and [Issues](https://github.com/ArangoGutierrez/k8s-mcp-agent/issues) for details.

## Development

### Prerequisites

- Go 1.23+
- Docker or Podman
- `golangci-lint` (optional, for linting)
- Access to NVIDIA GPU for integration tests

### Project Structure

```
k8s-mcp-agent/
├── cmd/agent/          # Main application entry point
├── pkg/                # Public libraries
│   ├── mcp/           # MCP server implementation
│   ├── nvml/          # NVML wrapper interface
│   └── tools/         # Tool implementations
├── internal/           # Private implementation
├── hack/              # Scripts and utilities
├── deploy/            # Container and deployment manifests
└── .cursor/rules/     # Cursor IDE development rules
```

### Build

```bash
# Using Makefile (recommended)
make agent                  # Build agent binary
make all                    # Run checks, tests, and build
make image                  # Build container image

# Direct Go build
go build -o bin/agent ./cmd/agent

# Release build (stripped, optimized)
go build -ldflags="-s -w" -o bin/agent ./cmd/agent

# View all available targets
make help
```

### Testing

```bash
# Using Makefile (recommended)
make test                   # Run tests with race detector
make test-short             # Run tests without race detector
make test-integration       # Run integration tests (requires GPU)
make coverage               # Generate coverage report
make coverage-html          # Generate HTML coverage report

# Direct Go commands
go test ./... -count=1
go test ./... -race
go test ./... -tags=integration
```

### Testing MCP Protocol Locally

The agent includes a mock NVML implementation for testing without GPU hardware:

```bash
# Build the agent
make agent

# Test echo tool (validates JSON-RPC round-trip)
cat examples/echo_test.json | ./bin/agent

# Test GPU inventory tool (returns mock GPU data)
cat examples/gpu_inventory.json | ./bin/agent

# Initialize MCP session
cat examples/initialize.json | ./bin/agent
```

Expected output for `echo_test`:
```json
{
  "echo": "Hello from k8s-mcp-agent!",
  "timestamp": "2026-01-03T12:00:00Z",
  "mode": "read-only"
}
```

Expected output for `get_gpu_inventory`:
```json
{
  "status": "success",
  "device_count": 2,
  "devices": [
    {
      "Index": 0,
      "Name": "NVIDIA A100-SXM4-40GB (Mock 0)",
      "UUID": "GPU-00000000-0000-0000-0000-000000000000",
      "BusID": "0000:01:00.0",
      ...
    }
  ]
}
```

## Contributing

This project follows the [standard Git protocol](init.md#git-protocol):

1. Check [open issues](https://github.com/ArangoGutierrez/k8s-mcp-agent/issues)
2. Create feature branch: `git checkout -b feat/description`
3. Commit with DCO: `git commit -s -S -m "feat(scope): description"`
4. Open PR linked to issue with labels and milestone
5. Pass CI checks before merge

See [init.md](init.md) for full workflow standards.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## Acknowledgments

- [NVIDIA NVML](https://developer.nvidia.com/nvidia-management-library-nvml)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [mcp-go](https://github.com/mark3labs/mcp-go)
- [Anthropic Claude](https://www.anthropic.com/claude)

---

**Status:** 🚧 Active Development - M1: Foundation & API  
**Maintainer:** [@ArangoGutierrez](https://github.com/ArangoGutierrez)  
**Version:** 0.1.0 (Prototype)
