# GitHub Copilot Instructions for k8s-gpu-mcp-server

This file provides context and guidelines for GitHub Copilot when generating code, 
reviews, and suggestions for the k8s-gpu-mcp-server project.

## Project Context

**k8s-gpu-mcp-server** is an ephemeral diagnostic agent that provides surgical, real-time 
NVIDIA GPU hardware introspection for Kubernetes clusters via the Model Context 
Protocol (MCP). It is designed for AI-assisted troubleshooting by SREs debugging 
complex hardware failures.

### Key Architectural Principles

1. **Ephemeral Injection Pattern**: No DaemonSets, no standing infrastructure
2. **Stdio Transport Only**: JSON-RPC 2.0 over `kubectl debug` SPDY tunneling
3. **Read-Only Default**: Safe operations with explicit operator mode for destructive actions
4. **Minimal Dependencies**: Prefer standard library over third-party packages
5. **Hardware Safety**: Isolate CGO/NVML calls, never panic on missing hardware

## Code Generation Guidelines

### Go Style and Conventions

- Follow **Effective Go** principles strictly
- Use `gofmt -s` (simplify) formatting
- Documentation comments limited to 80 characters; wrap to multiple lines if needed
- Always use `context.Context` as first parameter for I/O operations
- Error wrapping with `fmt.Errorf("context: %w", err)` - preserve error chains
- **NO PANIC** in production code (except `main()` initialization failures)

### Error Handling Pattern

```go
// Good: Contextual error with wrapping
if err := nvml.Init(); err != nil {
    return fmt.Errorf("failed to initialize NVML: %w", err)
}

// Bad: Silent failure or generic error
if err := nvml.Init(); err != nil {
    return err
}
```

### Concurrency and Context

```go
// Always pass context as first parameter
func GetGPUHealth(ctx context.Context, deviceIndex int) (*Health, error) {
    // Respect context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Use timeouts for hardware calls
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    // ... implementation
}
```

### MCP Server Specifics

- **stdout**: MCP protocol ONLY (JSON-RPC messages)
- **stderr**: Structured JSON logging exclusively
- **NEVER** write logs or debug output to stdout - breaks protocol
- Tool responses must be `application/json` strings for complex data

```go
// Good: Structured logging to stderr
log.Printf(`{"level":"info","component":"mcp-server","msg":"tool invoked","tool":"get_gpu_health"}`)

// Bad: Anything to stdout except MCP protocol
fmt.Println("Debug: calling NVML...") // BREAKS STDIO TRANSPORT
```

### NVML Hardware Interaction

- Always use interfaces for NVML wrapper (enables mocking)
- Isolate CGO calls to dedicated package (`pkg/nvml`)
- Never crash on missing GPU hardware - return clean errors
- Read-only operations by default; flag destructive ops clearly

```go
// Good: Interface for testing
type NVMLClient interface {
    DeviceGetCount() (int, error)
    DeviceGetHandleByIndex(int) (Device, error)
}

// Good: Graceful degradation
func Init() error {
    if err := nvml.Init(); err != nil {
        return fmt.Errorf("NVML unavailable (no GPU hardware?): %w", err)
    }
    return nil
}
```

### Testing Pattern

```go
// Table-driven tests with testify/assert
func TestAnalyzeXIDError(t *testing.T) {
    tests := []struct {
        name     string
        xidCode  int
        want     Severity
        wantErr  bool
    }{
        {"xid_79_fatal", 79, SeverityFatal, false},
        {"xid_48_critical", 48, SeverityCritical, false},
        {"invalid_code", -1, "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := AnalyzeXID(tt.xidCode)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got.Severity)
        })
    }
}
```

## Pull Request Descriptions

When generating PR descriptions or reviews, use this structure:

### PR Description Template

**What changed:**
- Clear, bullet-point summary of modifications
- Reference affected components (MCP server, NVML binding, tools, etc.)

**Why:**
- Link to issue(s) being addressed
- Explain the problem or use case
- Justify the chosen approach

**How:**
- High-level implementation overview
- Key design decisions or trade-offs
- Any new dependencies or breaking changes

**Testing:**
- Test coverage added/modified
- Manual testing steps performed
- Environment details (K8s version, GPU model, etc.)

### Code Review Focus Areas

1. **Safety**: No panics, graceful error handling, context cancellation
2. **I/O Separation**: stdout for MCP only, stderr for logs
3. **Error Context**: All errors wrapped with meaningful context
4. **Testing**: Table-driven tests, mocked dependencies
5. **Documentation**: 80-char limit, clear godoc for public APIs
6. **Security**: Input validation, no shell injection, sanitized logs
7. **Git Protocol**: DCO signed, GPG signed, proper commit format

### Review Comment Template

```markdown
**Observation:** [Describe the issue]

**Concern:** [Why this matters - safety/performance/maintainability]

**Suggestion:**
\`\`\`go
// Proposed fix
\`\`\`

**Reference:** [Link to style guide or documentation if applicable]
```

## Common Patterns to Suggest

### 1. Structured Tool Response

```go
type ToolResponse struct {
    Status  string      `json:"status"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
    Message string      `json:"message,omitempty"`
}

func (r ToolResponse) JSON() string {
    b, _ := json.Marshal(r)
    return string(b)
}
```

### 2. NVML Wrapper with Context

```go
func (c *Client) GetDeviceCount(ctx context.Context) (int, error) {
    // Check context before expensive operation
    if err := ctx.Err(); err != nil {
        return 0, err
    }
    
    count, ret := nvml.DeviceGetCount()
    if ret != nvml.SUCCESS {
        return 0, fmt.Errorf("nvml.DeviceGetCount failed: %v", 
            nvml.ErrorString(ret))
    }
    return count, nil
}
```

### 3. Tool Input Validation

```go
func ValidateGPUIndex(index, maxGPUs int) error {
    if index < 0 || index >= maxGPUs {
        return fmt.Errorf("invalid GPU index %d: must be 0-%d", 
            index, maxGPUs-1)
    }
    return nil
}
```

## Security Considerations

- **Input Validation**: Sanitize all tool inputs (PIDs, indices, counts)
- **No Shell Execution**: Never use `os/exec` with user-provided strings
- **Sensitive Data**: Sanitize logs (no API keys, tokens, or passwords)
- **Privilege Awareness**: Document when operations require elevated privileges
- **Operator Mode**: Flag destructive operations clearly with mode checks

## Component-Specific Notes

### cmd/agent
- Keep `main()` clean: parse flags, setup, run, shutdown
- Fatal errors only in initialization (before server starts)

### pkg/mcp
- Stdio transport exclusively
- Structured JSON logging to stderr
- Tool schemas with explicit validation

### pkg/nvml
- Interface-based design for mocking
- Graceful degradation on missing hardware
- Context-aware operations with timeouts

### pkg/tools
- Each tool returns structured JSON
- Validate inputs before NVML calls
- Include SRE-friendly action recommendations

### internal/
- Private implementation details
- No public API surface guarantees
- Can be more aggressive with refactoring

## References

- [Project Init Documentation](../init.md)
- [MCP Protocol Spec](https://modelcontextprotocol.io/)
- [Effective Go](https://go.dev/doc/effective_go)
- [NVIDIA NVML Documentation](https://docs.nvidia.com/deploy/nvml-api/)

---

**Note to Copilot:** This project prioritizes safety, simplicity, and operational 
clarity. When in doubt, prefer explicit error handling over clever code, and always 
consider the SRE debugging experience.

