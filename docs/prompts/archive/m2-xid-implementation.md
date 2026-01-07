# M2 Phase 2: XID Error Analysis - Implementation Prompt

**Project:** `k8s-gpu-mcp-server` - Just-in-Time SRE Diagnostic Agent for NVIDIA GPU Clusters  
**Repository:** https://github.com/ArangoGutierrez/k8s-gpu-mcp-server  
**Branch:** Create new: `feat/m2-xid-error-analysis`  
**Milestone:** M2: Hardware Introspection (Due: Jan 17, 2026)

---

## 📋 CONTEXT

### Project Overview
We are building an **ephemeral diagnostic agent** that provides real-time NVIDIA GPU introspection via the Model Context Protocol (MCP). The agent runs as a just-in-time pod injected via `kubectl debug` and communicates over stdio using JSON-RPC 2.0.

### Current State (as of Jan 3, 2026)

**Completed:**
- ✅ M1: Foundation & API (Go module, MCP server, Mock NVML)
- ✅ M2 Phase 1: Real NVML integration (tested on Tesla T4)
- ✅ Comprehensive documentation (1,235 lines)
- ✅ 39/39 unit tests + 5/5 integration tests passing
- ✅ Binary: 7.9MB, Go 1.25, MCP Protocol 2025-06-18

**Available Tools:**
1. `echo_test` - Protocol validation
2. `get_gpu_inventory` - Hardware inventory + telemetry

**Architecture:**
```
cmd/agent/main.go           # Entry point, flag parsing
pkg/mcp/server.go           # MCP stdio server
pkg/nvml/                   # NVML abstraction
  ├── interface.go          # Interface definition
  ├── mock.go               # Fake GPUs for testing
  └── real.go               # Real NVML (CGO, tested on Tesla T4)
pkg/tools/                  # MCP tool handlers
  └── gpu_inventory.go      # Current tool
```

### Remote GPU Machine Available
- **SSH:** `ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- **GPU:** Tesla T4 (15GB) with NVIDIA Driver 575.57.08
- **Go:** 1.25.5 installed at `/usr/local/go/bin/go`
- **Kubernetes:** v1.33.3 single node
- **Code:** `~/k8s-gpu-mcp-server` (clone available)

---

## 🎯 OBJECTIVE: Implement XID Error Analysis Tool

### What are XID Errors?

XID (eXception ID) errors are **critical GPU hardware errors** reported by the NVIDIA driver to the kernel. They indicate serious issues like:

- **XID 43**: GPU stopped responding (fallen off bus)
- **XID 48**: Double-bit ECC error (memory corruption)
- **XID 63**: ECC page retirement limit reached
- **XID 79**: GPU has fallen off the bus (catastrophic)

These errors appear in:
- Kernel ring buffer (`dmesg`)
- Kernel log (`/var/log/kern.log`)
- System journal (`journalctl -k`)

**Example dmesg output:**
```
[1234.567890] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='<unknown>', name=<unknown>, Ch 00000002, timestamp 12:34:56.789
```

### Why This Matters

When a Kubernetes GPU job hangs:
- `kubectl` shows pod as "Running"
- Logs show no errors
- **But GPU has failed at hardware level**

The SRE needs to:
1. **Detect** the XID error
2. **Understand** what it means
3. **Take action** (drain node, reset GPU, file RMA)

Our tool provides this surgical diagnostic capability to AI assistants (Claude).

---

## 📦 DELIVERABLES

### 1. XID Lookup Table (`pkg/xid/codes.go`)

Create a static map of XID codes → descriptions + actions:

```go
package xid

type ErrorInfo struct {
    Code        int
    Name        string
    Description string
    Severity    string // "info", "warning", "critical", "fatal"
    Action      string // SRE action recommendation
    Category    string // "hardware", "driver", "thermal", "power", "memory"
}

var ErrorCodes = map[int]ErrorInfo{
    31: {
        Code:        31,
        Name:        "GPU Exception",
        Description: "GPU encountered a hardware exception",
        Severity:    "critical",
        Action:      "Check dmesg for details. May require GPU reset or node drain.",
        Category:    "hardware",
    },
    43: {
        Code:        43,
        Name:        "GPU Stopped Responding",
        Description: "GPU fallen off the bus",
        Severity:    "fatal",
        Action:      "Drain node immediately. GPU needs replacement or bus reset.",
        Category:    "hardware",
    },
    48: {
        Code:        48,
        Name:        "Double Bit ECC Error",
        Description: "Uncorrectable memory error detected",
        Severity:    "fatal",
        Action:      "Drain node immediately. Memory corruption detected.",
        Category:    "memory",
    },
    // Add top 20 XIDs from NVIDIA documentation
}
```

**Reference:** Populate from NVIDIA XID documentation:
- https://docs.nvidia.com/deploy/xid-errors/
- Common XIDs: 13, 31, 43, 45, 48, 61, 62, 63, 64, 68, 69, 74, 79, 94, 95

### 2. Kernel Buffer Parser (`pkg/xid/parser.go`)

```go
package xid

import "context"

type Parser struct{}

type XIDEvent struct {
    Timestamp   time.Time
    XIDCode     int
    GPUIndex    int
    GPUUUID     string
    PCIBusID    string
    PID         int
    ProcessName string
    RawMessage  string
}

// ParseDmesg reads kernel ring buffer and extracts XID errors
func (p *Parser) ParseDmesg(ctx context.Context) ([]XIDEvent, error) {
    // 1. Read dmesg output
    // 2. Filter for NVRM lines with "Xid"
    // 3. Parse each line to extract:
    //    - XID code
    //    - PCI bus ID
    //    - Timestamp
    //    - PID/process name
    // 4. Map PCI bus ID → GPU index (using NVML)
    // 5. Return structured events
}

// Example line to parse:
// [1234.567890] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='12345', name=python3, Ch 00000002
```

**Implementation Notes:**
- Use `os/exec` to run `dmesg --raw --level=err,warn` 
- Use regex to parse NVRM Xid lines
- Handle variations in format (older/newer drivers)
- Map PCI bus ID to GPU index via NVML
- Return events sorted by timestamp (newest first)

### 3. MCP Tool Handler (`pkg/tools/analyze_xid.go`)

```go
package tools

type AnalyzeXIDHandler struct {
    nvmlClient nvml.Interface
    parser     *xid.Parser
}

func (h *AnalyzeXIDHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // 1. Parse dmesg for XID events
    // 2. For each event:
    //    a. Lookup XID code in error table
    //    b. Add severity + action
    //    c. Add GPU name/UUID from NVML
    // 3. Group by severity
    // 4. Return structured JSON with recommendations
}
```

**Response Format:**
```json
{
  "status": "warning|critical|ok",
  "error_count": 3,
  "errors": [
    {
      "xid": 48,
      "name": "Double Bit ECC Error",
      "severity": "fatal",
      "gpu_index": 0,
      "gpu_name": "Tesla T4",
      "gpu_uuid": "GPU-d129fc5b-2d51-cec7-d985-49168c12716f",
      "pci_bus_id": "0000:00:1E.0",
      "timestamp": "2026-01-03T15:30:45Z",
      "description": "Uncorrectable memory error detected",
      "sre_action": "Drain node immediately. Memory corruption detected.",
      "category": "memory"
    }
  ],
  "summary": {
    "fatal": 1,
    "critical": 0,
    "warning": 0
  },
  "recommendation": "URGENT: 1 fatal error detected. Drain node immediately."
}
```

### 4. Unit Tests

**Required test files:**

**pkg/xid/codes_test.go:**
- Test XID lookup
- Test unknown XIDs
- Test severity classification

**pkg/xid/parser_test.go:**
- Test parsing various dmesg formats
- Test with no XIDs
- Test with multiple XIDs
- Test PCI bus ID extraction
- Use mock dmesg output (no actual kernel access)

**pkg/tools/analyze_xid_test.go:**
- Test handler with mock parser
- Test JSON response format
- Test error handling
- Test context cancellation

### 5. Integration Test on Tesla T4

On the remote GPU machine, test with real dmesg:

```bash
# Generate test XID (if possible, or use existing dmesg)
sudo dmesg | grep -i xid

# Test tool
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze_xid_errors","arguments":{}},"id":3}' | \
  ./bin/agent --nvml-mode=real
```

### 6. Documentation

**docs/xid-errors.md** (new file):
- Explanation of XID errors
- List of top 20 XIDs with meanings
- Troubleshooting guide
- References to NVIDIA docs

**Update docs/mcp-usage.md:**
- Add `analyze_xid_errors` tool example

---

## 🔧 IMPLEMENTATION STEPS

### Step 1: Create XID Package Structure
```bash
mkdir -p pkg/xid
touch pkg/xid/codes.go
touch pkg/xid/codes_test.go
touch pkg/xid/parser.go
touch pkg/xid/parser_test.go
```

### Step 2: Implement XID Lookup Table

Research and add top 20 XIDs from NVIDIA documentation:
- https://docs.nvidia.com/deploy/xid-errors/

Priority XIDs (must include):
- 13 (Graphics exception)
- 31 (GPU Exception)
- 43 (GPU stopped responding)
- 45 (Preemption error)
- 48 (Double Bit ECC)
- 61/62/63 (Internal error/page retirement)
- 64 (ECC page retirement pending)
- 74 (NVLink error)
- 79 (Fallen off bus - catastrophic)
- 94/95 (Contained/uncontained error)

### Step 3: Implement Dmesg Parser

**Parse this format:**
```
[timestamp] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='12345', name=python3
```

**Regex pattern (example):**
```go
xidRegex := regexp.MustCompile(`Xid \(PCI:([0-9a-f:\.]+)\): (\d+)`)
```

**Handle:**
- Different timestamp formats
- Missing PID/name fields
- Multiple XID formats (older drivers)

### Step 4: Implement Tool Handler

Follow the pattern from `gpu_inventory.go`:
- Check context cancellation
- Parse arguments (optional: gpu_index filter)
- Call parser
- Enrich with NVML data
- Format as JSON
- Return MCP result

### Step 5: Register Tool in MCP Server

**pkg/mcp/server.go:**
```go
// Add import
import "github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/xid"

// In New() function, register tool:
xidHandler := tools.NewAnalyzeXIDHandler(cfg.NVMLClient)
mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidHandler.Handle)
```

### Step 6: Add Example Request

**examples/analyze_xid.json:**
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

### Step 7: Test on Tesla T4

```bash
# On remote machine
cd ~/k8s-gpu-mcp-server
git pull
git checkout feat/m2-xid-error-analysis
export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent

# Check for existing XIDs
sudo dmesg | grep -i "xid"

# Test tool
cat examples/analyze_xid.json | ./bin/agent --nvml-mode=real
```

### Step 8: Write Tests

**Minimum coverage:**
- XID lookup (valid, invalid, edge cases)
- Parser with mock dmesg output
- Tool handler with mock data
- Integration test with real dmesg

### Step 9: Documentation

Add usage examples and XID reference guide.

### Step 10: Create PR

Follow git protocol:
- Commit with DCO: `git commit -s -S`
- Format: `feat(m2): implement XID error analysis tool`
- Link to milestone
- Run `make all` before pushing

---

## 📚 REFERENCES

### NVIDIA XID Documentation
- **Official Docs:** https://docs.nvidia.com/deploy/xid-errors/
- **Driver Release Notes:** Check for version-specific XID codes
- **NVML API:** Error logging functions in go-nvml

### Kernel Log Format
- **dmesg:** `man dmesg` for output format
- **NVRM Prefix:** All NVIDIA driver messages start with "NVRM:"
- **Log Levels:** err, warn, crit

### Similar Projects
- Look at how DCGM (Data Center GPU Manager) handles XIDs
- Check nvidia-smi source for XID parsing

### Project Guidelines
- **Error Handling:** Wrap with `%w`, return structured errors
- **Context:** Pass context.Context, check cancellation
- **Testing:** Table-driven tests with testify
- **Logging:** Structured JSON to stderr
- **Documentation:** 80 char line limit

---

## 🧪 TESTING STRATEGY

### Unit Tests (No GPU Required)
```go
func TestXIDLookup(t *testing.T) {
    tests := []struct {
        xid      int
        expected string
    }{
        {48, "Double Bit ECC Error"},
        {79, "Fallen off bus"},
        {999, ""}, // Unknown XID
    }
    // Test lookup
}

func TestParseDmesg(t *testing.T) {
    mockDmesg := `[100.123] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234'`
    events := ParseDmesgOutput(mockDmesg)
    assert.Len(t, events, 1)
    assert.Equal(t, 48, events[0].XIDCode)
}
```

### Integration Tests (Requires GPU)
```bash
go test -tags=integration -v ./pkg/xid/
```

Test on real dmesg from Tesla T4.

---

## 🚨 ACCEPTANCE CRITERIA

- [ ] XID lookup table with at least 15 common XIDs
- [ ] Parser extracts XID code, GPU, PCI bus, timestamp
- [ ] Tool returns structured JSON with recommendations
- [ ] Severity classification (info, warning, critical, fatal)
- [ ] SRE action for each XID
- [ ] Maps PCI bus ID to GPU index via NVML
- [ ] Unit tests: 100% coverage for lookup, >80% for parser
- [ ] Integration test passes on Tesla T4
- [ ] All existing tests still pass (39/39)
- [ ] Documentation updated
- [ ] Example JSON-RPC request added
- [ ] Tested with `make all` (0 lint issues)

---

## 🐛 POTENTIAL CHALLENGES

### Challenge 1: dmesg Permissions
**Issue:** dmesg may require root access  
**Solution:** 
- Document that agent needs privileged mode
- Handle permission errors gracefully
- Return helpful error message

### Challenge 2: XID Format Variations
**Issue:** Different driver versions have different formats  
**Solution:**
- Test with multiple format examples
- Use flexible regex
- Log unparsed lines for debugging

### Challenge 3: PCI Bus ID → GPU Index Mapping
**Issue:** NVML and dmesg may report different PCI formats  
**Solution:**
- Normalize PCI format (remove leading zeros, etc.)
- Test mapping with real Tesla T4
- Handle missing/unknown GPUs

### Challenge 4: No XIDs in Test Environment
**Issue:** Tesla T4 may not have errors  
**Solution:**
- Use mock dmesg output for tests
- Test parser independently
- Document how to generate test XIDs

---

## 📝 EXAMPLE IMPLEMENTATION OUTLINE

```go
// pkg/xid/codes.go
package xid

var ErrorCodes = map[int]ErrorInfo{
    // Populate from NVIDIA docs
}

func Lookup(code int) (ErrorInfo, bool) {
    info, exists := ErrorCodes[code]
    return info, exists
}

// pkg/xid/parser.go
package xid

func ParseDmesg(ctx context.Context) ([]XIDEvent, error) {
    // Run: dmesg --raw --level=err,warn
    cmd := exec.CommandContext(ctx, "dmesg", "--raw", "--level=err,warn")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("failed to read dmesg: %w", err)
    }
    
    return parseDmesgOutput(string(output)), nil
}

func parseDmesgOutput(output string) []XIDEvent {
    var events []XIDEvent
    xidRegex := regexp.MustCompile(`Xid \(PCI:([0-9a-fA-F:\.]+)\): (\d+)`)
    
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        if !strings.Contains(line, "NVRM") || !strings.Contains(line, "Xid") {
            continue
        }
        
        matches := xidRegex.FindStringSubmatch(line)
        if len(matches) >= 3 {
            event := XIDEvent{
                PCIBusID: matches[1],
                XIDCode:  parseInt(matches[2]),
                RawMessage: line,
            }
            events = append(events, event)
        }
    }
    
    return events
}

// pkg/tools/analyze_xid.go
package tools

func (h *AnalyzeXIDHandler) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Parse dmesg
    events, err := h.parser.ParseDmesg(ctx)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("failed to parse dmesg: %s", err)), nil
    }
    
    // 2. Enrich each event
    var enrichedErrors []EnrichedXIDError
    for _, event := range events {
        // Lookup XID info
        info, exists := xid.Lookup(event.XIDCode)
        if !exists {
            info = xid.UnknownXID(event.XIDCode)
        }
        
        // Get GPU info from NVML
        gpuInfo := h.getGPUInfoByPCI(ctx, event.PCIBusID)
        
        enriched := EnrichedXIDError{
            XID:         event.XIDCode,
            Name:        info.Name,
            Severity:    info.Severity,
            Description: info.Description,
            SREAction:   info.Action,
            GPUIndex:    gpuInfo.Index,
            GPUName:     gpuInfo.Name,
            GPUUUID:     gpuInfo.UUID,
            Timestamp:   event.Timestamp,
        }
        enrichedErrors = append(enrichedErrors, enriched)
    }
    
    // 3. Create summary
    summary := createSummary(enrichedErrors)
    
    // 4. Format response
    response := map[string]interface{}{
        "status":         determineOverallStatus(enrichedErrors),
        "error_count":    len(enrichedErrors),
        "errors":         enrichedErrors,
        "summary":        summary,
        "recommendation": generateRecommendation(enrichedErrors),
    }
    
    return mcp.NewToolResultText(marshalJSON(response)), nil
}
```

---

## 🔍 TESTING CHECKLIST

### Before Committing
- [ ] Run `make fmt` (format code)
- [ ] Run `make lint` (0 issues)
- [ ] Run `make test` (all unit tests pass)
- [ ] Run `make test-integration` (on Tesla T4)
- [ ] Test tool manually with examples/analyze_xid.json
- [ ] Verify mock mode still works
- [ ] Check binary size (should be <10MB)

### On Remote GPU Machine
- [ ] Build succeeds: `go build -o bin/agent ./cmd/agent`
- [ ] Check dmesg: `sudo dmesg | grep -i xid`
- [ ] Test tool: `cat examples/analyze_xid.json | ./bin/agent --nvml-mode=real`
- [ ] Verify GPU mapping correct
- [ ] Check response format

---

## 📖 SUCCESS CRITERIA

**Minimum Viable:**
- Parse dmesg successfully
- Return at least 10 XID codes in lookup table
- Structured JSON response
- Unit tests pass

**Ideal:**
- 20+ XID codes with descriptions
- Severity classification
- SRE action recommendations
- Integration test on Tesla T4
- Handles edge cases (no XIDs, permission errors)
- Documentation complete

---

## 🚀 GETTING STARTED

### Commands to Run

```bash
# On your local machine
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull
git checkout -b feat/m2-xid-error-analysis

# Create package structure
mkdir -p pkg/xid
touch pkg/xid/codes.go
touch pkg/xid/codes_test.go
touch pkg/xid/parser.go
touch pkg/xid/parser_test.go

touch pkg/tools/analyze_xid.go
touch pkg/tools/analyze_xid_test.go

touch examples/analyze_xid.json

# Start implementing...
```

### First Implementation Task

Start with **pkg/xid/codes.go** - create the XID lookup table. This is
foundational and has no dependencies. Research NVIDIA XID docs and populate
the map with at least 15 common XIDs.

---

## ⚠️ IMPORTANT NOTES

1. **Permissions:** `dmesg` may need root. Document this clearly.

2. **Driver Versions:** XID formats may vary. Test with NVIDIA 575.57.08 (Tesla T4).

3. **Context Awareness:** All functions must respect `ctx.Err()` per project guidelines.

4. **Error Handling:** Return structured errors, never panic.

5. **Testing:** Mock external dependencies (dmesg) for unit tests.

6. **Git Protocol:** 
   - Sign commits: `git commit -s -S`
   - Format: `feat(xid): description`
   - Run `make all` before committing

---

## 📞 RESOURCES

- **Project Docs:** `docs/` folder
- **Existing Code:** `pkg/tools/gpu_inventory.go` (pattern to follow)
- **Test Examples:** `pkg/tools/gpu_inventory_test.go`
- **Remote GPU:** SSH command provided above
- **NVIDIA Docs:** https://docs.nvidia.com/deploy/xid-errors/

---

## ✅ DELIVERABLE CHECKLIST

When implementation is complete, you should have:

- [ ] pkg/xid/ package with codes and parser
- [ ] pkg/tools/analyze_xid.go tool handler
- [ ] Unit tests (>80% coverage)
- [ ] Integration test (tested on Tesla T4)
- [ ] examples/analyze_xid.json
- [ ] docs/xid-errors.md (optional but recommended)
- [ ] All 39+ tests passing
- [ ] PR created with proper labels and milestone
- [ ] CI checks passing

---

**READY TO START!** Follow the steps above and implement XID error analysis.

Good luck! 🚀

