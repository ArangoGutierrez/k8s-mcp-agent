# Task: Implement XID Error Analysis for k8s-mcp-agent

**Copy this entire message to start a new chat window** ↓

---

## CONTEXT

I'm working on `k8s-mcp-agent` - an MCP server that provides NVIDIA GPU diagnostics for Kubernetes.

**Repository:** https://github.com/ArangoGutierrez/k8s-mcp-agent  
**Current Branch:** `main`  
**Workspace:** `/Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent`

**Current State:**
- ✅ M1 Complete: MCP stdio server, Mock NVML
- ✅ M2 Phase 1 Complete: Real NVML integration (tested on Tesla T4)
- ✅ Documentation complete (docs/ folder with 1,235 lines)
- ✅ 44 tests passing (39 unit + 5 integration on real GPU)

**Available Tools:**
1. `echo_test` - Protocol validation
2. `get_gpu_inventory` - GPU hardware info + telemetry

**Tech Stack:**
- Go 1.25.5
- MCP Protocol 2025-06-18 via `github.com/mark3labs/mcp-go`
- NVML via `github.com/NVIDIA/go-nvml`
- Tests with `github.com/stretchr/testify`

**GPU Hardware:**
- Remote machine: `ssh -i ~/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- Tesla T4 (15GB), NVIDIA Driver 575.57.08
- Go 1.25.5 at `/usr/local/go/bin/go`
- Code at `~/k8s-mcp-agent`

---

## OBJECTIVE

Implement **M2 Phase 2: XID Error Analysis Tool**

### What are XIDs?

XID (eXception ID) errors are GPU hardware failures logged by NVIDIA driver to kernel.

**Example:** `[1234.56] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234'`

**Critical XIDs:**
- **48** = Double-bit ECC error (memory corruption) → DRAIN NODE
- **79** = Fallen off bus (catastrophic) → DRAIN NODE
- **43** = GPU stopped responding → RESET or DRAIN
- **31** = GPU exception → INVESTIGATE

---

## TASK BREAKDOWN

### 1. Create XID Lookup Table (`pkg/xid/codes.go`)

```go
package xid

type ErrorInfo struct {
    Code        int
    Name        string
    Description string
    Severity    string // "info", "warning", "critical", "fatal"
    Action      string // SRE recommendation
    Category    string // "hardware", "memory", "thermal", etc.
}

var ErrorCodes = map[int]ErrorInfo{
    48: {
        Code:        48,
        Name:        "Double Bit ECC Error",
        Description: "Uncorrectable memory error",
        Severity:    "fatal",
        Action:      "Drain node immediately - memory corruption",
        Category:    "memory",
    },
    // Add 15-20 common XIDs from: https://docs.nvidia.com/deploy/xid-errors/
}
```

### 2. Implement Dmesg Parser (`pkg/xid/parser.go`)

```go
package xid

type XIDEvent struct {
    Timestamp   time.Time
    XIDCode     int
    PCIBusID    string
    GPUIndex    int
    GPUUUID     string
    RawMessage  string
}

func ParseDmesg(ctx context.Context) ([]XIDEvent, error) {
    // Run: dmesg --raw --level=err,warn
    // Parse NVRM Xid lines with regex
    // Return structured events
}
```

### 3. Create Tool Handler (`pkg/tools/analyze_xid.go`)

Follow pattern from `pkg/tools/gpu_inventory.go`:
- Use NVML client to map PCI → GPU
- Parse dmesg for XIDs
- Enrich with ErrorInfo from lookup
- Return JSON with severity + recommendations

**Response format:**
```json
{
  "status": "critical",
  "error_count": 1,
  "errors": [{
    "xid": 48,
    "severity": "fatal",
    "gpu_index": 0,
    "gpu_name": "Tesla T4",
    "description": "Uncorrectable memory error",
    "sre_action": "Drain node immediately"
  }],
  "recommendation": "URGENT: Fatal errors detected"
}
```

### 4. Register Tool in MCP Server

In `pkg/mcp/server.go`, add:
```go
xidHandler := tools.NewAnalyzeXIDHandler(cfg.NVMLClient)
mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidHandler.Handle)
```

### 5. Add Tests

- `pkg/xid/codes_test.go` - Lookup tests
- `pkg/xid/parser_test.go` - Parser with mock dmesg
- `pkg/tools/analyze_xid_test.go` - Handler tests

### 6. Add Example

`examples/analyze_xid.json`:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "analyze_xid_errors",
    "arguments": {}
  },
  "id": 3
}
```

---

## COMMANDS TO RUN

```bash
# Start
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent
git checkout main && git pull
git checkout -b feat/m2-xid-error-analysis

# Create structure
mkdir -p pkg/xid
touch pkg/xid/{codes,parser,codes_test,parser_test}.go
touch pkg/tools/{analyze_xid,analyze_xid_test}.go
touch examples/analyze_xid.json

# After implementing, test
make all
go test -tags=integration ./pkg/xid/

# Test on Tesla T4
ssh -i ~/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com
cd ~/k8s-mcp-agent && git pull && git checkout feat/m2-xid-error-analysis
export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent
sudo dmesg | grep -i xid  # Check for existing XIDs
cat examples/analyze_xid.json | ./bin/agent --nvml-mode=real

# Create PR
git add -A
git commit -s -S -m "feat(xid): implement XID error analysis tool"
git push -u origin feat/m2-xid-error-analysis
gh pr create --title "feat(xid): XID error analysis tool" --milestone "M2: Hardware Introspection"
```

---

## SUCCESS CRITERIA

- [ ] 15+ XIDs in lookup table with severity + actions
- [ ] Parser extracts XID code, GPU, timestamp from dmesg
- [ ] Tool returns structured JSON with recommendations
- [ ] Unit tests: >80% coverage
- [ ] Integration test on Tesla T4
- [ ] All existing 44 tests still pass
- [ ] `make all` passes (0 lint issues)
- [ ] PR created and CI passing

---

## IMPORTANT GUIDELINES

1. **Context Checks:** Add `ctx.Err()` before operations
2. **Error Handling:** Wrap errors with `%w`
3. **Testing:** Table-driven tests with testify
4. **Logging:** JSON to stderr only (never stdout)
5. **Git:** Sign commits with `git commit -s -S`
6. **Documentation:** 80 char line limit

---

## REFERENCE

- **XID Docs:** https://docs.nvidia.com/deploy/xid-errors/
- **Pattern:** Follow `pkg/tools/gpu_inventory.go`
- **Project Docs:** `docs/architecture.md`, `docs/mcp-usage.md`
- **Rules:** `.cursor/rules/` folder

---

**START HERE:** Create `pkg/xid/codes.go` with XID lookup table first.

Research NVIDIA XID documentation and populate map with top 20 XIDs (13, 31, 43, 45, 48, 61, 62, 63, 64, 68, 69, 74, 79, 94, 95, etc.).

---

**Ready? Reply "GO" to start implementation.** 🚀

