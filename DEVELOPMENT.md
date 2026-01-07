# Development Guide

This guide provides detailed information for developers working on
`k8s-gpu-mcp-server`.

> 📖 **Also see:** [Architecture Documentation](docs/architecture.md) for
> system design details and [MCP Usage Guide](docs/mcp-usage.md) for protocol
> examples.

## Prerequisites

- **Go 1.23+**: Required for building the agent
- **golangci-lint v2.7+**: For code linting
- **Docker/Podman**: For container builds
- **make**: Build automation
- **Git**: Version control with DCO signing

## Project Structure

```
k8s-gpu-mcp-server/
├── cmd/
│   └── agent/              # Main application entry point
│       └── main.go         # CLI setup, server initialization
├── pkg/                    # Public, reusable packages
│   ├── mcp/                # MCP server implementation
│   │   └── server.go       # Stdio transport, tool registration
│   ├── nvml/               # NVML abstraction layer
│   │   ├── interface.go    # NVML interface definition
│   │   ├── mock.go         # Mock implementation (M1)
│   │   └── mock_test.go    # Unit tests
│   └── tools/              # MCP tool handlers
│       └── gpu_inventory.go # GPU inventory tool
├── internal/               # Private implementation details
│   └── info/               # Build-time version info
├── examples/               # Example JSON-RPC requests
│   ├── echo_test.json
│   ├── gpu_inventory.json
│   └── initialize.json
├── .github/
│   └── workflows/
│       └── ci.yml          # CI/CD pipeline
└── .cursor/rules/          # Cursor IDE development standards
```

## Development Workflow

### 1. Setup

```bash
# Clone repository
git clone https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git
cd k8s-gpu-mcp-server

# Download dependencies
go mod download

# Verify setup
make info
```

### 2. Make Changes

Follow the [Git Protocol](init.md#git-protocol):

```bash
# Create feature branch
git checkout -b feat/my-feature

# Make changes
vim pkg/tools/my_tool.go

# Format code
make fmt

# Run checks
make all
```

### 3. Testing

```bash
# Run unit tests
make test

# Run with coverage
make coverage

# View coverage in browser
make coverage-html
open coverage.html
```

### 4. Commit

All commits must be signed with DCO (`-s`) and GPG (`-S`):

```bash
git add .
git commit -s -S -m "feat(tools): add new GPU tool"
```

Commit message format: `type(scope): description`

**Types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`,
`security`, `perf`

### 5. Push and PR

```bash
git push origin feat/my-feature
gh pr create --title "feat(tools): add new GPU tool" \
  --body "Fixes #123" \
  --label "kind/feature" \
  --milestone "M2"
```

## Code Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt -s` for formatting
- Run `go vet` before commits
- Documentation comments limited to 80 characters

### Error Handling

```go
// ✅ Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to get device: %w", err)
}

// ❌ Bad: Silent failures
if err != nil {
    return nil
}

// ❌ Bad: No context
if err != nil {
    return err
}
```

### Concurrency

```go
// ✅ Good: Pass context as first parameter
func GetGPUInfo(ctx context.Context, index int) (*GPUInfo, error) {
    // Respect context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... implementation
}
```

### Testing

Use table-driven tests with `testify/assert`:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name      string
        input     int
        expected  string
        expectErr bool
    }{
        {
            name:      "valid input",
            input:     42,
            expected:  "42",
            expectErr: false,
        },
        {
            name:      "invalid input",
            input:     -1,
            expectErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)
            if tt.expectErr {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

## MCP Protocol Development

### Adding a New Tool

1. **Define the tool in `pkg/tools/`**:

```go
// pkg/tools/my_tool.go
package tools

import (
    "context"
    "github.com/mark3labs/mcp-go/mcp"
)

type MyToolHandler struct {
    nvmlClient nvml.Interface
}

func NewMyToolHandler(nvmlClient nvml.Interface) *MyToolHandler {
    return &MyToolHandler{nvmlClient: nvmlClient}
}

func (h *MyToolHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // Implementation
    return mcp.NewToolResultText("result"), nil
}

func GetMyTool() mcp.Tool {
    return mcp.NewTool("my_tool",
        mcp.WithDescription("My tool description"),
        mcp.WithString("param1",
            mcp.Required(),
            mcp.Description("Parameter description"),
        ),
    )
}
```

2. **Register in `pkg/mcp/server.go`**:

```go
myToolHandler := tools.NewMyToolHandler(cfg.NVMLClient)
mcpServer.AddTool(tools.GetMyTool(), myToolHandler.Handle)
```

3. **Add example request in `examples/my_tool.json`**:

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "my_tool",
    "arguments": {
      "param1": "value"
    }
  },
  "id": 3
}
```

4. **Test manually**:

```bash
make agent
cat examples/my_tool.json | ./bin/agent
```

### Logging Standards

**CRITICAL:** Logs must go to `stderr` only. `stdout` is reserved for MCP
protocol.

```go
// ✅ Good: Structured JSON to stderr
log.Printf(`{"level":"info","msg":"tool invoked","tool":"my_tool"}`)

// ❌ Bad: Unstructured logs
fmt.Println("Tool invoked")

// ❌ Bad: Logs to stdout (breaks MCP protocol)
fmt.Printf("Processing...")
```

## Build System

### Makefile Targets

```bash
make help              # Show all targets
make build             # Build all packages
make agent             # Build agent binary
make all               # Run checks, tests, build
make test              # Run tests with race detector
make lint              # Run golangci-lint
make fmt               # Format code
make clean             # Clean build artifacts
make image             # Build container image
make dist              # Build release binaries
```

### Build Flags

The agent is built with:

- **CGO_ENABLED=1**: Required for NVML bindings
- **-ldflags="-s -w"**: Strip debug info for smaller binaries
- **Version injection**: Git commit and version embedded at build time

## Debugging

### Local Development

```bash
# Run agent with verbose logging
LOG_LEVEL=debug ./bin/agent --mode=read-only

# Test with specific JSON-RPC request
cat examples/echo_test.json | ./bin/agent 2>agent.log

# Check logs (stderr)
cat agent.log | jq .
```

### Testing MCP Protocol

Use the MCP Inspector (if available) or manual JSON-RPC:

```bash
# Initialize session
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}' | ./bin/agent

# Call tool
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"echo_test","arguments":{"message":"test"}},"id":1}' | ./bin/agent
```

## CI/CD

The project uses GitHub Actions for CI:

- **Lint**: `gofmt`, `go vet`, `golangci-lint`
- **Test**: Unit tests with race detector
- **Build**: Multi-arch builds (linux/amd64, linux/arm64)
- **Security**: Trivy vulnerability scanning
- **Git Protocol**: DCO and commit message validation

### Running CI Locally

```bash
# Run all CI checks
make all

# Check binary size
make agent
ls -lh bin/agent

# Verify commit messages
git log --oneline -10
```

## Release Process

(To be defined in M4)

## Troubleshooting

### Build Fails with CGO Errors

Ensure you have a C compiler installed:

```bash
# macOS
xcode-select --install

# Ubuntu/Debian
sudo apt-get install build-essential

# Verify
gcc --version
```

### Tests Fail with Race Detector

This usually indicates a real concurrency issue. Fix the code, don't disable
the race detector.

### golangci-lint Fails

```bash
# Update golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run with verbose output
golangci-lint run --verbose
```

## Resources

- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [NVML Documentation](https://docs.nvidia.com/deploy/nvml-api/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Project Issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
- [Milestones](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/milestones)

## Getting Help

- Open an [issue](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/new)
- Check [existing issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues)
- Review [init.md](init.md) for workflow standards

