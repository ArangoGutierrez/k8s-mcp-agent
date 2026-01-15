# Documentation

Complete documentation for `k8s-gpu-mcp-server` - Just-in-Time SRE Diagnostic
Agent for NVIDIA GPU Clusters on Kubernetes.

## 📚 Documentation Index

### Getting Started

- **[Quick Start Guide](quickstart.md)** - Get running in 5 minutes
  - Installation options
  - Basic usage examples
  - Testing with mock/real GPUs
  - Kubernetes deployment

### Understanding the Project

- **[Architecture](architecture.md)** - System design and technical details
  - Design principles
  - Component architecture
  - Data flow diagrams
  - Technology stack
  - Design decisions

### Using the Agent

- **[MCP Usage Guide](mcp-usage.md)** - How to consume the MCP server
  - Protocol basics
  - Claude Desktop integration
  - Cursor IDE integration
  - Manual JSON-RPC examples
  - Client implementation guides

### Development

- **[Development Guide](../DEVELOPMENT.md)** - Contributing to the project
  - Setup instructions
  - Code standards
  - Testing guidelines
  - Git workflow

### Project Information

- **[Main README](../README.md)** - Project overview
- **[License](../LICENSE)** - Apache 2.0
- **[Milestone Reports](reports/)** - Completion reports
  - [M1 Completion](reports/m1-completion.md)
  - [M2 Completion](reports/m2-completion.md)
  - [Project 360 Review (Jan 15, 2026)](reports/project-360-review-2026-01-15.md)

### Troubleshooting

- **[Cross-Node Networking](troubleshooting/cross-node-networking.md)** - CNI issues with Calico/AWS

### Internal Resources

- **[Implementation Prompts](prompts/)** - AI-assisted development prompts
  - [NPM kubectl Bridge](prompts/npm-kubectl-bridge.md) - Remote cluster access
  - [M3 Kubernetes Tools](prompts/m3-kubernetes-tools.md) - K8s integration

## 🎯 Quick Links

### For Users
- [Installation](quickstart.md#installation)
- [Basic Usage](quickstart.md#basic-usage)
- [Available Tools](mcp-usage.md#available-tools)
- [Troubleshooting](quickstart.md#troubleshooting)

### For Developers
- [Project Structure](architecture.md#file-structure)
- [Adding New Tools](architecture.md#adding-new-tools)
- [Testing](../DEVELOPMENT.md#testing)
- [Contributing](../DEVELOPMENT.md#development-workflow)

### For SREs
- [Kubernetes Deployment](quickstart.md#kubernetes-deployment)
- [Security Model](architecture.md#security-model)
- [Performance](architecture.md#performance-considerations)

## 🔗 External Resources

- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP Specification
- [NVIDIA NVML](https://docs.nvidia.com/deploy/nvml-api/) - GPU Management Library
- [kubectl debug](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/) - Kubernetes Debugging

## 📝 Examples

All code examples are available in the [`examples/`](../examples/) directory:

- `initialize.json` - MCP session initialization
- `gpu_inventory.json` - GPU inventory request
- `gpu_health.json` - GPU health check request
- `analyze_xid.json` - XID error analysis request
- `test_mcp.sh` - Automated testing script

## 🤝 Getting Help

- **Issues**: [GitHub Issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/discussions)
- **Maintainer**: [@ArangoGutierrez](https://github.com/ArangoGutierrez)

## 📄 License

Apache License 2.0 - See [LICENSE](../LICENSE) for details.

