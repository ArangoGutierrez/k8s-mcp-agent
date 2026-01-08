# k8s-gpu-mcp-server

MCP server for NVIDIA GPU diagnostics in Kubernetes clusters.

## Installation

```bash
# Run directly with npx (recommended)
npx k8s-gpu-mcp-server@latest

# Or install globally
npm install -g k8s-gpu-mcp-server
```

## Usage with Cursor

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

## Usage with Claude Desktop

Add to your Claude Desktop configuration
(`~/Library/Application Support/Claude/claude_desktop_config.json`):

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

## Available Tools

- `get_gpu_inventory` - Hardware inventory and telemetry
- `get_gpu_health` - GPU health monitoring with scoring
- `analyze_xid_errors` - Parse GPU XID errors from kernel logs

## Documentation

- [Full Documentation](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server)
- [Architecture](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/architecture.md)
- [MCP Usage Guide](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/mcp-usage.md)

## License

Apache-2.0

