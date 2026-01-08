# k8s-gpu-mcp-server

MCP server for NVIDIA GPU diagnostics in Kubernetes clusters.

## Prerequisites

- Node.js 18+
- `kubectl` installed and configured with cluster access
- k8s-gpu-mcp-server gateway deployed in cluster

## Installation

```bash
npm install -g k8s-gpu-mcp-server
```

## Usage with Cursor

Add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server"]
    }
  }
}
```

Restart Cursor and the GPU diagnostics tools will be available.

## Usage with Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server"]
    }
  }
}
```

## Configuration

Configure via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_GPU_MCP_NAMESPACE` | Gateway namespace | `gpu-diagnostics` |
| `K8S_GPU_MCP_SERVICE` | Gateway service name | `gpu-mcp-gateway` |
| `K8S_GPU_MCP_CONTEXT` | Kubernetes context | Current context |
| `K8S_GPU_MCP_SERVICE_PORT` | Gateway service port | `8080` |
| `K8S_GPU_MCP_LOCAL_PORT` | Local port for port-forward | Auto-select |
| `KUBECONFIG` | Path to kubeconfig | `~/.kube/config` |

## How It Works

1. Discovers gateway service by label or uses configured values
2. Spawns `kubectl port-forward` to the gateway service
3. Bridges stdin/stdout to HTTP requests
4. Cleans up port-forward on exit

## Available Tools

- `get_gpu_inventory` - Hardware inventory and telemetry
- `get_gpu_health` - GPU health monitoring with scoring
- `analyze_xid_errors` - Parse GPU XID errors from kernel logs

## Manual Testing

```bash
# Test the bridge
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | npx k8s-gpu-mcp-server
```

## Troubleshooting

### "kubectl not found"

Ensure kubectl is installed and in your PATH:

```bash
which kubectl
kubectl version --client
```

### "No gateway service found"

Deploy the gateway using Helm:

```bash
helm install gpu-mcp oci://ghcr.io/arangogutierrez/charts/k8s-gpu-mcp-server \
  --set gateway.enabled=true
```

### Connection timeout

Check cluster connectivity and gateway service:

```bash
kubectl get svc -n gpu-diagnostics
kubectl get pods -n gpu-diagnostics
```

### Permission denied

Ensure you have access to the gateway namespace:

```bash
kubectl auth can-i get services -n gpu-diagnostics
```

## Documentation

- [Full Documentation](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server)
- [Architecture](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/architecture.md)
- [MCP Usage Guide](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/mcp-usage.md)

## License

Apache-2.0
