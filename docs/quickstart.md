# Quick Start Guide

Get started with `k8s-gpu-mcp-server` in under 5 minutes.

## Prerequisites

- **Go 1.25+** for building from source
- **Docker** or **Podman** for container builds
- **NVIDIA GPU** (optional, for real hardware testing)
- **Kubernetes cluster** (optional, for production deployment)

## Installation

### Option 1: Download Pre-built Binary

```bash
# Download latest release (coming in M4)
curl -LO https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/releases/latest/download/agent-linux-amd64
chmod +x agent-linux-amd64
mv agent-linux-amd64 /usr/local/bin/k8s-gpu-mcp-server
```

### Option 2: Build from Source

```bash
# Clone repository
git clone https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git
cd k8s-gpu-mcp-server

# Build agent
make agent

# Binary available at: bin/agent
./bin/agent --version
```

### Option 3: Pull Container Image

```bash
# Pull latest image (coming in M3)
docker pull ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

## Basic Usage

### Local Testing (Mock Mode)

The agent includes mock NVML for testing without GPU hardware:

```bash
# Start agent in mock mode (default)
./bin/agent --mode=read-only --nvml-mode=mock

# In another terminal, send JSON-RPC requests
cat examples/initialize.json | ./bin/agent --nvml-mode=mock
cat examples/gpu_inventory.json | ./bin/agent --nvml-mode=mock
```

**Example Output:**
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
      "MemoryTotal": 42949672960,
      "Temperature": 45,
      "PowerUsage": 150000,
      "GPUUtil": 30
    }
  ]
}
```

### Real GPU Testing

With NVIDIA GPU and driver installed:

```bash
# Start agent with real NVML
./bin/agent --mode=read-only --nvml-mode=real

# Test GPU inventory
cat examples/gpu_inventory.json | ./bin/agent --nvml-mode=real

# Or use single-line JSON-RPC
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}' | ./bin/agent --nvml-mode=real
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}' | ./bin/agent --nvml-mode=real
```

## Kubernetes Deployment

### Prerequisites

GPU access in Kubernetes requires one of:
- **NVIDIA GPU Operator** (configures RuntimeClass automatically) — *recommended*
- **nvidia-ctk** configured with containerd/cri-o
- **NVIDIA Device Plugin** (fallback mode)

### Install with Helm

```bash
# Add the chart (if published) or use local path
helm install k8s-gpu-mcp-server ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --create-namespace

# Verify pods are running on GPU nodes
kubectl get pods -n gpu-diagnostics -o wide

# Verify HTTP endpoints are working
kubectl port-forward -n gpu-diagnostics svc/k8s-gpu-mcp-server-gateway 8080:8080 &
curl -s http://localhost:8080/healthz  # Should return {"status":"healthy"}
curl -s http://localhost:8080/readyz   # Should return {"status":"ready"}
```

> **Note:** The default deployment uses HTTP transport mode. Agents run as
> persistent HTTP servers and the gateway routes requests via HTTP for
> low-latency, low-memory operation.

```bash
```

### Fallback Mode (No RuntimeClass)

For clusters without RuntimeClass configured (e.g., cri-dockerd):

```bash
helm install k8s-gpu-mcp-server ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --create-namespace \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceRequest.enabled=true
```

> ⚠️ **Warning:** Fallback mode requests `nvidia.com/gpu` resources, consuming
> GPU capacity from the scheduler.

### Start Diagnostic Session

```bash
# Find agent pod on target node
NODE_NAME=gpu-node-5
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  --field-selector spec.nodeName=$NODE_NAME \
  -o jsonpath='{.items[0].metadata.name}')

# Start read-only diagnostic session
kubectl exec -it -n gpu-diagnostics $POD -- /agent --mode=read-only

# With operator mode (enables kill/reset operations)
kubectl exec -it -n gpu-diagnostics $POD -- /agent --mode=operator
```

### GPU Access Modes

| Mode | Helm Values | Description |
|------|-------------|-------------|
| **RuntimeClass** (default) | `gpu.runtimeClass.enabled=true` | Uses nvidia RuntimeClass + `NVIDIA_VISIBLE_DEVICES=all` |
| **Resource Request** | `gpu.resourceRequest.enabled=true` | Requests `nvidia.com/gpu` from device plugin |

The default RuntimeClass mode does NOT request `nvidia.com/gpu` resources,
allowing the agent to monitor all GPUs without blocking the scheduler.

## Available Tools

Once the agent is running, you can call these MCP tools:

### 1. GPU Inventory
**Purpose:** Get hardware inventory and current telemetry

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_gpu_inventory",
    "arguments": {}
  },
  "id": 2
}
```

**Response includes:**
- GPU model name
- UUID (unique identifier)
- PCI Bus ID
- Memory (total, used, free)
- Temperature (°C)
- Power usage (milliwatts)
- Utilization (GPU %, Memory %)

## Common Workflows

### Debugging a Stuck Training Job

```bash
# 1. Connect to the problematic node
kubectl debug node/gpu-node-5 \
  --image=ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest \
  --profile=sysadmin \
  -- /bin/bash

# 2. Inside the debug pod, run agent
/agent --mode=read-only --nvml-mode=real

# 3. Query GPU inventory (from Claude Desktop or another terminal)
# Send JSON-RPC requests via stdin
```

### Checking GPU Health

```bash
# Get inventory to check temperatures and power
cat examples/gpu_inventory.json | ./bin/agent --nvml-mode=real | jq '.devices[].Temperature'

# Look for high temps (>80°C) or power throttling
```

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `read-only` | Operation mode: `read-only` or `operator` |
| `--nvml-mode` | `mock` | NVML mode: `mock` or `real` |
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--version` | - | Show version and exit |

## Troubleshooting

### "Failed to initialize NVML"

**Cause:** NVIDIA driver not found or GPU not available

**Solution:**
```bash
# Check NVIDIA driver
nvidia-smi

# If not found, install NVIDIA driver
# Ubuntu: sudo apt-get install nvidia-driver-XXX

# Or use mock mode for testing
./bin/agent --nvml-mode=mock
```

### "Real NVML requires CGO"

**Cause:** Binary built without CGO support

**Solution:**
```bash
# Rebuild with CGO enabled
CGO_ENABLED=1 go build -o bin/agent ./cmd/agent

# Or use pre-built binaries with CGO
```

### "Parse error" in JSON-RPC

**Cause:** MCP protocol requires initialization handshake first

**Solution:**
```bash
# Always send initialize first
echo '{"jsonrpc":"2.0","method":"initialize",...}' | ./bin/agent
# Then send tool calls
echo '{"jsonrpc":"2.0","method":"tools/call",...}' | ./bin/agent
```

## Next Steps

- Read the [Architecture Documentation](architecture.md) to understand the design
- See [MCP Usage Guide](mcp-usage.md) for detailed protocol examples
- Check [DEVELOPMENT.md](../DEVELOPMENT.md) for contributing
- View [Examples](../examples/) for more JSON-RPC requests

## Getting Help

- 📖 [Full Documentation](README.md)
- 🐛 [Report Issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
- 💬 [Discussions](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/discussions)
- 📧 Contact: [@ArangoGutierrez](https://github.com/ArangoGutierrez)

