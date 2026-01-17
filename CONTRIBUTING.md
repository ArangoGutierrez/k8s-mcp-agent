# Contributing to k8s-gpu-mcp-server

Thank you for your interest in contributing to k8s-gpu-mcp-server! This document
provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Access to a Kubernetes cluster (for integration testing)
- NVIDIA GPU (optional, for real hardware testing)
- Docker or Podman (for container builds)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/k8s-gpu-mcp-server.git
cd k8s-gpu-mcp-server
```

3. Add the upstream remote:

```bash
git remote add upstream https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git
```

## Development Setup

### Build the Project

```bash
# Build the binary
make build

# Run tests
make test

# Run linter
make lint

# Build container image
make container-build
```

### Project Structure

```
k8s-gpu-mcp-server/
├── cmd/agent/          # Main application entry point
├── pkg/
│   ├── gateway/        # HTTP gateway and routing
│   ├── k8s/            # Kubernetes client
│   ├── mcp/            # MCP protocol implementation
│   ├── metrics/        # Prometheus metrics
│   ├── nvml/           # NVIDIA Management Library interface
│   ├── tools/          # MCP tool implementations
│   └── xid/            # XID error parsing and analysis
├── deployment/
│   ├── helm/           # Helm chart
│   └── rbac/           # RBAC configurations
├── docs/               # Documentation
└── examples/           # Example MCP requests
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring

### Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding or updating tests
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `chore`: Changes to build process or auxiliary tools

Examples:

```
feat(tools): add describe_gpu_node tool

Implements comprehensive GPU node diagnostics including
hardware info, driver versions, and device health status.

Closes #40
```

```
fix(nvml): handle device not found error gracefully

Previously returned a panic when device was not found.
Now returns a proper error message.

Fixes #123
```

## Pull Request Process

### Before Submitting

1. **Update your branch** with the latest upstream changes:

```bash
git fetch upstream
git rebase upstream/main
```

2. **Run all checks**:

```bash
make lint
make test
go vet ./...
```

3. **Ensure tests pass** with the race detector:

```bash
go test -race ./...
```

### PR Requirements

- [ ] Tests pass locally
- [ ] New code has appropriate test coverage
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] No unrelated changes included

### Review Process

1. Submit your PR with a clear description
2. Address reviewer feedback
3. Maintainer will merge once approved

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Keep functions focused and small
- Use meaningful variable names

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to get GPU info: %w", err)
}

// Bad: Bare error returns
if err != nil {
    return err
}
```

### Comments

- Export comments for all public types and functions
- Keep comments up to date with code changes
- Use complete sentences

```go
// GPUInfo contains hardware information for a single GPU device.
// It includes identification, memory stats, and health status.
type GPUInfo struct {
    // ...
}
```

### MCP Tool Implementation

When adding new MCP tools:

1. Create tool file in `pkg/tools/`
2. Define input schema with JSON tags
3. Implement the tool handler
4. Register in `pkg/mcp/server.go`
5. Add comprehensive tests
6. Add example request in `examples/`

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./pkg/tools/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Requires a Kubernetes cluster
go test -tags=integration ./...
```

### Mock Mode

For development without real GPU hardware:

```bash
./bin/agent --mock
```

## Documentation

### When to Update Docs

- Adding new features or tools
- Changing configuration options
- Modifying deployment procedures
- Fixing incorrect documentation

### Documentation Locations

- `README.md` - Project overview and quick start
- `docs/` - Detailed documentation
- `docs/prompts/` - Development task prompts
- Code comments - API documentation

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Join discussions in GitHub Discussions

## Recognition

Contributors are recognized in release notes. Thank you for helping improve
k8s-gpu-mcp-server!
