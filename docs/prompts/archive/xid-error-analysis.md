# Implement XID Error Analysis Tool

## PROJECT CONTEXT

**Repository:** https://github.com/ArangoGutierrez/k8s-gpu-mcp-server  
**Current Branch:** `main`  
**Workspace:** `/Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server`

### Current State (Jan 3, 2026)
- ✅ M1 Complete: MCP stdio server working, Mock NVML implemented
- ✅ M2 Phase 1 Complete: Real NVML integration tested on Tesla T4
- ✅ Documentation: 1,235 lines in docs/ folder
- ✅ Tests: 44/44 passing (39 unit + 5 integration on real GPU)
- ✅ Working Tools: `echo_test`, `get_gpu_inventory`

### Tech Stack
- **Go:** 1.25.5
- **MCP Protocol:** 2025-06-18 via `github.com/mark3labs/mcp-go v0.43.2`
- **NVML:** `github.com/NVIDIA/go-nvml v0.13.0-1`
- **Testing:** `github.com/stretchr/testify`

### Remote GPU Machine
- **SSH:** `ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- **GPU:** Tesla T4 (15GB), Driver 575.57.08, CUDA 12.9
- **Go:** 1.25.5 at `/usr/local/go/bin/go`
- **Code Location:** `~/k8s-gpu-mcp-server`
- **Kubernetes:** v1.33.3 single node

---

## OBJECTIVE: Implement XID Error Analysis

### What are XID Errors?

XID (eXception ID) errors are **GPU hardware failures** logged by NVIDIA driver to the kernel ring buffer. They indicate critical issues like memory corruption, bus failures, or thermal problems.

**Example dmesg line:**
```
[1234.567890] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3, Ch 00000002
```

**Critical XIDs to implement:**
- **XID 13** - Graphics exception
- **XID 31** - GPU exception  
- **XID 43** - GPU stopped responding → Reset or drain
- **XID 45** - Preemption error
- **XID 48** - Double-bit ECC error → **DRAIN NODE IMMEDIATELY**
- **XID 61, 62, 63** - Internal memory errors
- **XID 64** - ECC page retirement pending
- **XID 68, 69** - FBPA/FBP exceptions
- **XID 74** - NVLink error
- **XID 79** - Fallen off bus → **DRAIN NODE IMMEDIATELY**
- **XID 94, 95** - Contained/uncontained errors

**Reference:** https://docs.nvidia.com/deploy/xid-errors/

---

## IMPLEMENTATION TASKS

### Task 1: Create XID Lookup Package

Create `pkg/xid/codes.go`:

```go
package xid

type ErrorInfo struct {
    Code        int    `json:"code"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Severity    string `json:"severity"` // "info", "warning", "critical", "fatal"
    Action      string `json:"sre_action"`
    Category    string `json:"category"` // "hardware", "memory", "thermal", "power", "nvlink"
}

var ErrorCodes = map[int]ErrorInfo{
    // Populate with 15-20 common XIDs from NVIDIA docs
}

func Lookup(code int) (ErrorInfo, bool) {
    info, exists := ErrorCodes[code]
    return info, exists
}

func LookupOrUnknown(code int) ErrorInfo {
    if info, exists := ErrorCodes[code]; exists {
        return info
    }
    return ErrorInfo{
        Code:        code,
        Name:        fmt.Sprintf("Unknown XID %d", code),
        Description: "XID not in known error table",
        Severity:    "warning",
        Action:      "Check NVIDIA documentation for XID details",
        Category:    "unknown",
    }
}
```

**Test file:** `pkg/xid/codes_test.go`
- Test Lookup() with valid XIDs
- Test unknown XIDs
- Test all severities present

---

### Task 2: Implement Dmesg Parser

Create `pkg/xid/parser.go`:

```go
package xid

import (
    "context"
    "os/exec"
    "regexp"
    "time"
)

type Parser struct{}

type XIDEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    XIDCode     int       `json:"xid_code"`
    PCIBusID    string    `json:"pci_bus_id"`
    GPUIndex    int       `json:"gpu_index"`
    PID         int       `json:"pid,omitempty"`
    ProcessName string    `json:"process_name,omitempty"`
    RawMessage  string    `json:"raw_message"`
}

func NewParser() *Parser {
    return &Parser{}
}

func (p *Parser) ParseDmesg(ctx context.Context) ([]XIDEvent, error) {
    // Check context
    if err := ctx.Err(); err != nil {
        return nil, fmt.Errorf("context cancelled: %w", err)
    }
    
    // Run dmesg
    cmd := exec.CommandContext(ctx, "dmesg", "--raw", "--level=err,warn")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("failed to read dmesg: %w", err)
    }
    
    return parseDmesgOutput(string(output)), nil
}

func parseDmesgOutput(output string) []XIDEvent {
    var events []XIDEvent
    
    // Regex to match: Xid (PCI:0000:00:1E.0): 48
    xidRegex := regexp.MustCompile(`Xid \(PCI:([0-9a-fA-F:\.]+)\):\s*(\d+)`)
    
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        if !strings.Contains(line, "NVRM") || !strings.Contains(line, "Xid") {
            continue
        }
        
        matches := xidRegex.FindStringSubmatch(line)
        if len(matches) >= 3 {
            event := XIDEvent{
                PCIBusID:   matches[1],
                XIDCode:    parseInt(matches[2]),
                RawMessage: line,
                GPUIndex:   -1, // Will be filled by tool handler
            }
            
            // Extract timestamp, PID if present
            // ... parse additional fields ...
            
            events = append(events, event)
        }
    }
    
    return events
}
```

**Test file:** `pkg/xid/parser_test.go`
- Test with mock dmesg output (no real kernel access)
- Test various XID formats
- Test empty output
- Test malformed lines

---

### Task 3: Create Tool Handler

Create `pkg/tools/analyze_xid.go`:

```go
package tools

import (
    "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
    "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/xid"
    "github.com/mark3labs/mcp-go/mcp"
)

type AnalyzeXIDHandler struct {
    nvmlClient nvml.Interface
    parser     *xid.Parser
}

func NewAnalyzeXIDHandler(nvmlClient nvml.Interface) *AnalyzeXIDHandler {
    return &AnalyzeXIDHandler{
        nvmlClient: nvmlClient,
        parser:     xid.NewParser(),
    }
}

type EnrichedXIDError struct {
    XIDCode     int       `json:"xid"`
    Name        string    `json:"name"`
    Severity    string    `json:"severity"`
    Description string    `json:"description"`
    SREAction   string    `json:"sre_action"`
    Category    string    `json:"category"`
    GPUIndex    int       `json:"gpu_index"`
    GPUName     string    `json:"gpu_name"`
    GPUUUID     string    `json:"gpu_uuid"`
    PCIBusID    string    `json:"pci_bus_id"`
    Timestamp   time.Time `json:"timestamp"`
}

func (h *AnalyzeXIDHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 1. Parse dmesg for XID events
    events, err := h.parser.ParseDmesg(ctx)
    
    // 2. Enrich each event with XID info and GPU details
    var enrichedErrors []EnrichedXIDError
    for _, event := range events {
        // Lookup XID info
        info := xid.LookupOrUnknown(event.XIDCode)
        
        // Map PCI bus ID to GPU index via NVML
        gpuIndex, gpuInfo := h.findGPUByPCI(ctx, event.PCIBusID)
        
        enriched := EnrichedXIDError{
            XIDCode:     event.XIDCode,
            Name:        info.Name,
            Severity:    info.Severity,
            Description: info.Description,
            SREAction:   info.Action,
            Category:    info.Category,
            GPUIndex:    gpuIndex,
            GPUName:     gpuInfo.Name,
            GPUUUID:     gpuInfo.UUID,
            PCIBusID:    event.PCIBusID,
            Timestamp:   event.Timestamp,
        }
        enrichedErrors = append(enrichedErrors, enriched)
    }
    
    // 3. Create summary
    summary := createSummary(enrichedErrors)
    
    // 4. Generate recommendation
    recommendation := generateRecommendation(enrichedErrors)
    
    // 5. Format response
    response := map[string]interface{}{
        "status":         determineStatus(enrichedErrors),
        "error_count":    len(enrichedErrors),
        "errors":         enrichedErrors,
        "summary":        summary,
        "recommendation": recommendation,
    }
    
    return mcp.NewToolResultText(marshalJSON(response)), nil
}
```

**Test file:** `pkg/tools/analyze_xid_test.go`
- Test with mock parser
- Test empty results
- Test with fatal XIDs
- Test JSON formatting

---

### Task 4: Register Tool

In `pkg/mcp/server.go`, add to imports:
```go
import "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/xid"
```

In `New()` function after gpu_inventory registration:
```go
// Register XID analysis tool
xidHandler := tools.NewAnalyzeXIDHandler(cfg.NVMLClient)
mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidHandler.Handle)
```

Create tool definition:
```go
func GetAnalyzeXIDTool() mcp.Tool {
    return mcp.NewTool("analyze_xid_errors",
        mcp.WithDescription(
            "Analyze NVIDIA GPU XID errors from kernel logs. "+
            "Returns structured error data with severity and SRE recommendations.",
        ),
    )
}
```

---

### Task 5: Add Example

Create `examples/analyze_xid.json`:
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

## TESTING PROCEDURE

### Local Testing (Mock dmesg)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run unit tests
make test

# Build
make agent

# Test with mock mode
cat examples/analyze_xid.json | ./bin/agent --nvml-mode=mock
```

### Remote Testing (Real Tesla T4)

```bash
# SSH to GPU machine
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com

# Navigate to code
cd ~/k8s-gpu-mcp-server

# Pull latest changes
git fetch origin
git checkout feat/m2-xid-error-analysis

# Build
export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent

# Check for XIDs in kernel log
sudo dmesg | grep -i "xid"
sudo dmesg | grep -i "nvrm"

# Test tool
cat examples/analyze_xid.json | ./bin/agent --nvml-mode=real

# Run integration tests
go test -tags=integration -v ./pkg/xid/
```

---

## EXAMPLE OUTPUT (Expected)

```json
{
  "status": "ok",
  "error_count": 0,
  "errors": [],
  "summary": {
    "fatal": 0,
    "critical": 0,
    "warning": 0,
    "info": 0
  },
  "recommendation": "No XID errors detected. GPU health is good."
}
```

Or if errors exist:

```json
{
  "status": "critical",
  "error_count": 2,
  "errors": [
    {
      "xid": 48,
      "name": "Double Bit ECC Error",
      "severity": "fatal",
      "description": "Uncorrectable memory error detected",
      "sre_action": "Drain node immediately. Memory corruption detected.",
      "category": "memory",
      "gpu_index": 0,
      "gpu_name": "Tesla T4",
      "gpu_uuid": "GPU-d129fc5b-2d51-cec7-d985-49168c12716f",
      "pci_bus_id": "0000:00:1E.0",
      "timestamp": "2026-01-03T15:30:45Z"
    }
  ],
  "summary": {
    "fatal": 1,
    "critical": 0,
    "warning": 1
  },
  "recommendation": "URGENT: 1 fatal error detected. Drain node immediately."
}
```

---

## IMPLEMENTATION STEPS

### Step 1: Create Branch and Structure
```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main && git pull
git checkout -b feat/m2-xid-error-analysis

mkdir -p pkg/xid
touch pkg/xid/codes.go
touch pkg/xid/codes_test.go
touch pkg/xid/parser.go
touch pkg/xid/parser_test.go

touch pkg/tools/analyze_xid.go
touch pkg/tools/analyze_xid_test.go

touch examples/analyze_xid.json
```

### Step 2: Implement XID Lookup Table

Research and populate `pkg/xid/codes.go` with at least 15 XID codes from:
https://docs.nvidia.com/deploy/xid-errors/

**Priority XIDs:** 13, 31, 43, 45, 48, 61, 62, 63, 64, 68, 69, 74, 79, 94, 95

For each, provide:
- Name (brief)
- Description (detailed)
- Severity (info/warning/critical/fatal)
- SRE Action (what to do)
- Category (hardware/memory/thermal/power/nvlink)

### Step 3: Implement Dmesg Parser

In `pkg/xid/parser.go`:
- Use `exec.CommandContext(ctx, "dmesg", "--raw", "--level=err,warn")`
- Parse NVRM Xid lines with regex
- Extract: XID code, PCI bus ID, timestamp, PID
- Handle permission errors gracefully
- Return empty slice if no XIDs found

### Step 4: Implement Tool Handler

In `pkg/tools/analyze_xid.go`:
- Follow pattern from `gpu_inventory.go`
- Check context cancellation
- Parse dmesg via xid.Parser
- For each XID:
  - Lookup error info
  - Map PCI bus → GPU index using NVML
  - Get GPU name/UUID from NVML
- Group by severity
- Generate overall recommendation
- Return structured JSON

### Step 5: Register Tool

Update `pkg/mcp/server.go`:
- Import `pkg/xid` (if needed for types)
- Create handler: `xidHandler := tools.NewAnalyzeXIDHandler(cfg.NVMLClient)`
- Register: `mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidHandler.Handle)`

### Step 6: Write Tests

**Unit Tests:**
- `pkg/xid/codes_test.go` - Lookup valid/invalid XIDs
- `pkg/xid/parser_test.go` - Parse mock dmesg output
- `pkg/tools/analyze_xid_test.go` - Handler with mock data

**Integration Test:**
- `pkg/xid/parser_integration_test.go` (tag: integration)
- Test on real dmesg from Tesla T4

### Step 7: Verify Locally

```bash
# Format and test
make fmt
make all

# Verify tests pass
make test

# Check binary size
make agent
ls -lh bin/agent
```

### Step 8: Test on Tesla T4

```bash
# Build on remote
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com \
  "cd ~/k8s-gpu-mcp-server && git pull && git checkout feat/m2-xid-error-analysis && \
   export PATH=/usr/local/go/bin:\$PATH && go build -o bin/agent ./cmd/agent"

# Check for XIDs
ssh ... "sudo dmesg | grep -i xid"

# Test tool
ssh ... "cd ~/k8s-gpu-mcp-server && cat examples/analyze_xid.json | ./bin/agent --nvml-mode=real"

# Run integration tests
ssh ... "cd ~/k8s-gpu-mcp-server && go test -tags=integration -v ./pkg/xid/"
```

### Step 9: Documentation

Optional but recommended - create `docs/xid-errors.md`:
- Explain what XIDs are
- List common XIDs with descriptions
- Troubleshooting guide
- When to drain nodes vs reset GPUs

Update `docs/mcp-usage.md`:
- Add `analyze_xid_errors` tool example

### Step 10: Create PR

```bash
# Commit with DCO and GPG
git add -A
git commit -s -S -m "feat(xid): implement XID error analysis tool

M2 Phase 2: XID Error Analysis

Deliverables:
- pkg/xid/codes.go: Lookup table with 15+ common XIDs
- pkg/xid/parser.go: Dmesg parser with regex extraction
- pkg/tools/analyze_xid.go: MCP tool handler
- Comprehensive unit tests for all components
- Integration test on Tesla T4
- Example JSON-RPC request

Features:
- Parse kernel ring buffer for NVIDIA XID errors
- Classify by severity (info/warning/critical/fatal)
- Provide SRE action recommendations
- Map PCI bus to GPU index via NVML
- Context cancellation support

Testing:
- XX/XX unit tests passing
- Integration test on Tesla T4 verified
- All existing tests still pass

Closes #X"

# Push
git push -u origin feat/m2-xid-error-analysis

# Create PR
gh pr create \
  --title "feat(xid): Implement XID error analysis tool" \
  --body "See commit message for details" \
  --label "kind/feature" \
  --label "area/nvml-binding" \
  --milestone "M2: Hardware Introspection"

# Watch CI
gh pr checks <PR#> --watch

# After CI passes, merge
gh pr merge <PR#> --merge --delete-branch
```

---

## CODE GUIDELINES (from .cursor/rules/)

### Error Handling
```go
// ✅ Good
if err != nil {
    return fmt.Errorf("failed to parse dmesg: %w", err)
}

// ❌ Bad
if err != nil {
    return err // No context
}
```

### Context Checks
```go
// ✅ Good - check before expensive operations
if err := ctx.Err(); err != nil {
    return fmt.Errorf("context cancelled: %w", err)
}
```

### Testing
```go
// ✅ Good - table-driven
func TestXIDLookup(t *testing.T) {
    tests := []struct {
        name string
        xid  int
        want string
    }{
        {"known xid", 48, "Double Bit ECC Error"},
        {"unknown xid", 999, "Unknown XID 999"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            info := xid.LookupOrUnknown(tt.xid)
            assert.Contains(t, info.Name, tt.want)
        })
    }
}
```

---

## ACCEPTANCE CRITERIA

**Must Have:**
- [ ] XID lookup table with 15+ codes
- [ ] Dmesg parser extracts XID code, GPU PCI, timestamp
- [ ] Tool returns structured JSON with severity + recommendations
- [ ] Maps PCI bus ID → GPU index correctly
- [ ] Unit tests: >80% coverage
- [ ] All existing 44 tests still pass
- [ ] Handles "no XIDs found" gracefully
- [ ] Handles permission errors with helpful message
- [ ] `make all` passes (0 lint issues)

**Should Have:**
- [ ] Integration test on Tesla T4
- [ ] 20+ XID codes in lookup table
- [ ] Extracts PID/process name from dmesg
- [ ] Groups by severity in response
- [ ] Overall recommendation field

**Nice to Have:**
- [ ] docs/xid-errors.md documentation
- [ ] Filter by GPU index argument
- [ ] Time range filtering
- [ ] Historical XID tracking

---

## POTENTIAL ISSUES & SOLUTIONS

### Issue 1: dmesg Requires Root
**Solution:** Document in tool description. Return helpful error if permission denied.

### Issue 2: No XIDs in Tesla T4 Logs
**Solution:** 
- Test with mock dmesg output in unit tests
- Document how to generate test XIDs (if safe)
- Tool returns "No errors found" successfully

### Issue 3: PCI Bus ID Format Variations
**Solution:**
- Normalize format (e.g., "0000:00:1E.0" vs "00:1E.0")
- Test with actual Tesla T4 PCI ID: "0000:00:1E.0"

### Issue 4: Multiple XID Formats
**Solution:**
- Test with various driver versions
- Use flexible regex
- Log unparsed lines at debug level

---

## EXAMPLE TEST CASES

```go
// Test data - mock dmesg output
const mockDmesgWithXID = `
[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3, Ch 00000002
[  200.654321] NVRM: Xid (PCI:0000:00:1E.0): 79, pid='<unknown>', name=<unknown>
`

func TestParseDmesgOutput(t *testing.T) {
    events := parseDmesgOutput(mockDmesgWithXID)
    
    assert.Len(t, events, 2)
    assert.Equal(t, 48, events[0].XIDCode)
    assert.Equal(t, "0000:00:1E.0", events[0].PCIBusID)
    assert.Equal(t, 79, events[1].XIDCode)
}
```

---

## QUICK REFERENCE

### Key Files to Reference
- **Pattern:** `pkg/tools/gpu_inventory.go` (tool handler pattern)
- **Tests:** `pkg/tools/gpu_inventory_test.go` (testing pattern)
- **NVML:** `pkg/nvml/interface.go` (interface definition)
- **Server:** `pkg/mcp/server.go` (tool registration)

### Key Commands
```bash
make fmt              # Format code
make test             # Run unit tests
make all              # Full check suite
git commit -s -S      # Commit with DCO + GPG
gh pr create          # Create PR
gh pr checks --watch  # Monitor CI
```

### Remote Machine
```bash
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com
cd ~/k8s-gpu-mcp-server
export PATH=/usr/local/go/bin:$PATH
```

---

## START HERE

1. Create branch: `git checkout -b feat/m2-xid-error-analysis`
2. Create file structure (mkdir pkg/xid, etc.)
3. **Start with `pkg/xid/codes.go`** - Research and populate XID lookup table
4. Then implement parser, handler, tests in order
5. Test locally, then on Tesla T4
6. Create PR when all checks pass

**Research:** https://docs.nvidia.com/deploy/xid-errors/ for XID definitions.

---

**Reply "GO" when ready to start implementation.** 🚀

