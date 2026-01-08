# Remove echo_test Tool from Production Builds

## Issue Reference

- **Issue:** [#100 - Remove echo_test tool from production builds](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/100)
- **Priority:** P2-Medium
- **Labels:** enhancement, good first issue
- **Milestone:** M3: Kubernetes Integration

## Background

The `echo_test` tool was useful during development but has no value in
production. Every MCP tool is injected into user prompts, consuming tokens
unnecessarily:

- Tool definition: ~50-100 tokens per conversation
- Multiplied across all users = significant waste
- Creates noise in tool listings

The health endpoints (`/healthz`, `/readyz`) and actual GPU tools provide
sufficient validation for production use.

---

## Objective

Remove the `echo_test` tool from the default MCP server registration to reduce
token overhead in production.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b chore/remove-echo-test
```

---

## Implementation Tasks

### Task 1: Remove echo_test Registration

Remove the `echo_test` tool registration from the MCP server initialization.

**Files to modify:**
- `pkg/mcp/server.go` - Remove echo_test tool registration

**Current code to remove:**

```go
// Remove this line from NewServer()
mcpServer.AddTool(echoTool, s.handleEchoTest)
```

**Acceptance criteria:**
- [ ] `echo_test` tool is not registered in the MCP server
- [ ] Server starts without errors

> 💡 **Commit after completing this task**

---

### Task 2: Remove echo_test Handler and Tool Definition

Remove the handler function and tool definition for echo_test.

**Files to modify:**
- `pkg/mcp/server.go` - Remove `handleEchoTest` function and `echoTool` var

**Code to remove:**

```go
// Remove the echoTool variable definition
var echoTool = mcp.NewTool(
    "echo_test",
    // ...
)

// Remove the handler function
func (s *Server) handleEchoTest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // ...
}
```

**Acceptance criteria:**
- [ ] `echoTool` variable removed
- [ ] `handleEchoTest` function removed
- [ ] No dead code remaining

> 💡 **Commit after completing this task**

---

### Task 3: Remove echo_test from Tests

Update or remove any tests that reference echo_test.

**Files to check:**
- `pkg/mcp/server_test.go` - Remove echo_test test cases
- `examples/echo_test.json` - Delete example file

**Acceptance criteria:**
- [ ] No test references to echo_test
- [ ] `examples/echo_test.json` deleted
- [ ] All tests pass

> 💡 **Commit after completing this task**

---

### Task 4: Update Documentation

Remove references to echo_test from documentation.

**Files to check:**
- `README.md` - Remove any echo_test mentions
- `docs/mcp-usage.md` - Remove echo_test examples

**Acceptance criteria:**
- [ ] No documentation references to echo_test
- [ ] Examples updated to use real GPU tools

> 💡 **Commit after completing this task**

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Verify echo_test is not in tool list
./bin/agent --nvml-mode=mock < examples/initialize.json 2>/dev/null | \
  grep -c "echo_test" || echo "✅ echo_test not found"

# Verify other tools still work
./bin/agent --nvml-mode=mock < examples/gpu_inventory.json
```

---

## Pre-Commit Checklist

```bash
make fmt
make lint
make test
make all
```

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass

---

## Commit and Push

### Commits

```bash
git commit -s -S -m "chore(mcp): remove echo_test tool registration"
git commit -s -S -m "chore(mcp): remove echo_test handler and definition"
git commit -s -S -m "test(mcp): remove echo_test test cases"
git commit -s -S -m "docs: remove echo_test references"
```

### Push

```bash
git push -u origin chore/remove-echo-test
```

---

## Create Pull Request

```bash
gh pr create \
  --title "chore(mcp): remove echo_test tool from production" \
  --body "Fixes #100

## Summary
Removes the echo_test tool to reduce token overhead in production.

## Changes
- Remove echo_test tool registration
- Remove handler function and tool definition
- Remove test cases and example file
- Update documentation

## Testing
- [x] make all passes
- [x] Tool list no longer includes echo_test
- [x] GPU tools still work correctly" \
  --label "enhancement"
```

---

## Quick Reference

**Estimated Time:** 30 minutes

**Complexity:** Easy - straightforward removal

**Files Changed:**
- `pkg/mcp/server.go`
- `pkg/mcp/server_test.go`
- `examples/echo_test.json` (delete)
- Documentation files

---

**Reply "GO" when ready to start implementation.** 🚀
