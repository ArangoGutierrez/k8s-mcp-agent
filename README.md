# k8s-gpu-mcp-server

**Just-in-Time SRE Diagnostic Agent for NVIDIA GPU Clusters on Kubernetes**

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![CI Status](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/workflows/CI/badge.svg)](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/actions)
[![GitHub Issues](https://img.shields.io/github/issues/ArangoGutierrez/k8s-gpu-mcp-server)](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
[![GitHub Stars](https://img.shields.io/github/stars/ArangoGutierrez/k8s-gpu-mcp-server?style=social)](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server)
[![MCP](https://img.shields.io/badge/MCP-2025--06--18-purple)](https://modelcontextprotocol.io/)

---

## Overview

`k8s-gpu-mcp-server` is an **ephemeral diagnostic agent** that provides surgical, 
real-time NVIDIA GPU hardware introspection for Kubernetes clusters via the 
[Model Context Protocol (MCP)](https://modelcontextprotocol.io/). 

Unlike traditional monitoring systems, this agent is designed for **AI-assisted 
troubleshooting** by SREs debugging complex hardware failures that standard 
Kubernetes APIs cannot detect.

### ✨ Key Features

- 🎯 **On-Demand Diagnostics** - Agent runs only during `kubectl exec` sessions
- 🔌 **Stdio Transport** - JSON-RPC 2.0 over `kubectl debug` SPDY tunneling
- 🔍 **Deep Hardware Access** - Direct NVML integration for GPU diagnostics
- 🤖 **AI-Native** - Built for Claude Desktop, Cursor, and MCP-compatible hosts
- 🔒 **Secure by Default** - Read-only operations with explicit operator mode
- ⚡ **Production Ready** - Real Tesla T4 testing, 74/74 tests passing

---

## 🚀 Quick Start

### One-Line Installation

```bash
# Using npx (recommended)
npx k8s-gpu-mcp-server@latest

# Or install globally
npm install -g k8s-gpu-mcp-server
```

### Use with Cursor

Add to your Cursor MCP configuration:

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

### Use with Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

### Install from Source

```bash
# Clone and build
git clone https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git
cd k8s-gpu-mcp-server
make agent

# Test with mock GPUs (no hardware required)
cat examples/gpu_inventory.json | ./bin/agent --nvml-mode=mock

# Test with real GPU (requires NVIDIA driver)
cat examples/gpu_inventory.json | ./bin/agent --nvml-mode=real
```

### Deploy to Kubernetes

```bash
# Deploy with Helm (RuntimeClass mode - recommended)
helm install k8s-gpu-mcp-server ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics --create-namespace

# Find agent pod on target node
NODE_NAME=<node-name>
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  --field-selector spec.nodeName=$NODE_NAME \
  -o jsonpath='{.items[0].metadata.name}')

# Start diagnostic session
kubectl exec -it -n gpu-diagnostics $POD -- /agent --mode=read-only
```

> **Note:** GPU access requires `runtimeClassName: nvidia` configured by
> GPU Operator or nvidia-ctk. For clusters without RuntimeClass, use fallback:
> `--set gpu.runtimeClass.enabled=false --set gpu.resourceRequest.enabled=true`

### Use with Claude Desktop

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "k8s-gpu-agent": {
      "command": "kubectl",
      "args": ["debug", "node/gpu-node-5", "--image=...", "--", "/agent"]
    }
  }
}
```

Then ask Claude: *"What's the temperature of the GPUs on node gpu-node-5?"*

📖 **[Full Quick Start Guide →](docs/quickstart.md)**

---

## 📊 Architecture

```
┌──────────────┐     kubectl debug      ┌────────────────┐
│   Claude     │ ──────────────────────> │  K8s Node      │
│   Desktop    │   SPDY Stdio Tunnel     │  ┌──────────┐  │
└──────────────┘                         │  │  Agent   │  │
       ▲                                 │  │  (stdio) │  │
       │         JSON-RPC 2.0             │  └────┬─────┘  │
       │         MCP Protocol             │       │        │
       └──────────────────────────────────│   ┌───▼────┐  │
                                          │   │  NVML  │  │
                                          │   │  API   │  │
                                          │   └───┬────┘  │
                                          │       │       │
                                          │   GPU 0...N   │
                                          └───────────────┘
```

**Design Principles:**
- **"Syringe Pattern"**: Ephemeral injection, zero idle footprint
- **Stdio-Only**: No network listeners, firewall-friendly
- **Interface Abstraction**: Testable, flexible, portable

📖 **[Architecture Documentation →](docs/architecture.md)**

---

## 🛠️ Available Tools

| Tool | Description | Status |
|------|-------------|--------|
| `get_gpu_inventory` | Hardware inventory + telemetry | ✅ Available |
| `analyze_xid_errors` | Parse GPU XID error codes from kernel logs | ✅ Available |
| `get_gpu_health` | GPU health monitoring with scoring | ✅ Available |
| `get_gpu_telemetry` | Real-time metrics | 🚧 M2 Phase 3 |
| `inspect_topology` | NVLink/PCIe topology | 🚧 M2 Phase 4 |
| `kill_gpu_process` | Terminate GPU process | 🚧 M3 (Operator) |
| `reset_gpu` | GPU reset | 🚧 M3 (Operator) |

📖 **[MCP Usage Guide →](docs/mcp-usage.md)**

---

## 📈 Project Status

### Current Milestone: [M2: Hardware Introspection](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/milestone/2)
**Due:** January 17, 2026  
**Progress:** Phase 1 Complete (Real NVML ✅)

### Completed Milestones
- ✅ [M1: Foundation & API](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/milestone/1) - Completed Jan 3, 2026
  - Go module scaffolding
  - MCP stdio server
  - Mock NVML implementation
  - Comprehensive CI/CD

### Recent Updates
- **Jan 4, 2026**: GPU health monitoring tool (`get_gpu_health`) merged
- **Jan 3, 2026**: XID error analysis tool (`analyze_xid_errors`) merged
- **Jan 3, 2026**: Real NVML integration complete, tested on Tesla T4
- **Jan 3, 2026**: 74/74 tests passing, 5/5 integration tests on real GPU

📊 **[View All Milestones →](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/milestones)**

---

## 🧪 Testing

### Unit Tests (No GPU Required)

```bash
make test                   # Run all unit tests (74/74 passing)
make coverage               # Generate coverage report
make coverage-html          # View coverage in browser
```

### Integration Tests (Requires GPU)

```bash
make test-integration       # Run on GPU hardware
# Or manually:
go test -tags=integration -v ./pkg/nvml/
```

**Latest Test Results on Tesla T4:**
```
✓ TestRealNVML_Integration
  - GPU: Tesla T4 (15GB)
  - Temperature: 29°C
  - Power: 13.9W
  - Utilization: 0% (idle)

✓ 5/5 integration tests passing
✓ 74/74 total tests passing
```

---

## 🏗️ Build

```bash
# Build for local platform
make agent

# Build for Linux (with real NVML)
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 make agent

# Build container image
make image

# Multi-arch release builds
make dist
```

**Binary Sizes:**
- Mock mode: **4.3MB** (CGO disabled)
- Real mode: **7.9MB** (CGO enabled)

---

## 📦 Installation

### Using npm (Recommended)

```bash
# Run directly with npx
npx k8s-gpu-mcp-server@latest

# Or install globally
npm install -g k8s-gpu-mcp-server
```

### From Source

```bash
git clone https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git
cd k8s-gpu-mcp-server
make agent
sudo mv bin/agent /usr/local/bin/k8s-gpu-mcp-server
```

### Using Go

```bash
go install github.com/ArangoGutierrez/k8s-gpu-mcp-server/cmd/agent@latest
```

### Container Image (Coming in M3)

```bash
docker pull ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

---

## 🤝 Contributing

We welcome contributions! Please see our [Development Guide](DEVELOPMENT.md)
for details.

### Quick Contribution Guide

1. Check [open issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
2. Fork and create feature branch: `git checkout -b feat/my-feature`
3. Make changes, add tests
4. Run checks: `make all`
5. Commit with DCO: `git commit -s -S -m "feat(scope): description"`
6. Open PR with labels and milestone

📖 **[Full Development Guide →](DEVELOPMENT.md)**

---

## 📚 Documentation

- **[Quick Start Guide](docs/quickstart.md)** - Get running in 5 minutes
- **[Architecture](docs/architecture.md)** - System design and components
- **[MCP Usage](docs/mcp-usage.md)** - How to consume the MCP server
- **[Development Guide](DEVELOPMENT.md)** - Contributing guidelines
- **[Examples](examples/)** - Sample JSON-RPC requests

---

## 🔧 Technology Stack

- **Language**: Go 1.25+ (latest stable)
- **MCP Protocol**: [mcp-go v0.43.2](https://github.com/mark3labs/mcp-go)
- **GPU Library**: [go-nvml v0.13.0-1](https://github.com/NVIDIA/go-nvml)
- **Testing**: [testify v1.10.0](https://github.com/stretchr/testify)
- **Container**: Distroless Debian 12 (coming in M3)

---

## 🎯 Use Cases

### 1. Debugging Stuck Training Jobs

```
SRE: "Why is the training job on node-5 stuck?"
Claude → k8s-gpu-mcp-server → Detects XID 48 (ECC Error)
Claude: "Node-5 has uncorrectable memory errors. Drain immediately."
```

### 2. Thermal Management

```
SRE: "Are any GPUs thermal throttling?"
Claude → k8s-gpu-mcp-server → Checks temps and throttle status
Claude: "GPU 3 is at 86°C and thermal throttling. Check cooling."
```

### 3. Topology Validation

```
SRE: "Is NVLink properly configured for multi-GPU training?"
Claude → k8s-gpu-mcp-server → Inspects NVLink topology
Claude: "All 8 GPUs connected via NVLink, 600GB/s bandwidth."
```

### 4. Zombie Process Hunting

```
SRE: "GPU memory is full but no pods are running"
Claude → k8s-gpu-mcp-server → Lists GPU processes
Claude: "Found zombie process PID 12345 using 8GB. Kill it?"
```

---

## 🏆 Achievements

- ✅ **Go 1.25** - Latest Go version
- ✅ **Real NVML** - Tested on Tesla T4
- ✅ **74/74 Tests** - 100% passing with race detector
- ✅ **Zero Lint Issues** - Clean codebase
- ✅ **7.9MB Binary** - 84% under 50MB target
- ✅ **MCP 2025-06-18** - Latest protocol version
- ✅ **Production Ready** - Used on real hardware

---

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

- [NVIDIA NVML](https://developer.nvidia.com/nvidia-management-library-nvml) - GPU Management Library
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP Specification
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP Go Implementation
- [Anthropic Claude](https://www.anthropic.com/claude) - AI Assistant
- [Cursor](https://cursor.sh/) - AI-Powered IDE

---

## 📞 Contact

**Maintainer:** [@ArangoGutierrez](https://github.com/ArangoGutierrez)  
**Issues:** [GitHub Issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)  
**Discussions:** [GitHub Discussions](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/discussions)

---

<div align="center">

**⭐ Star us on GitHub — it helps!**

[Report Bug](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/new?template=bug_report.yml) · 
[Request Feature](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/new?template=feature_request.yml) · 
[View Roadmap](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/milestones)

</div>
