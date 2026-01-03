# Implement GPU Health Monitoring Tool

## PROJECT CONTEXT

**Repository:** https://github.com/ArangoGutierrez/k8s-mcp-agent  
**Current Branch:** `main`  
**Workspace:** `/Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent`

### Current State (Jan 3, 2026)
- ✅ M1 Complete: MCP stdio server working, Mock NVML implemented
- ✅ M2 Phase 1: Real NVML integration tested on Tesla T4
- ✅ M2 Phase 2: XID error analysis tool merged
- ✅ Documentation: Comprehensive docs in docs/ folder
- ✅ Tests: 59/59 passing (44 unit + 15 XID tests)
- ✅ Working Tools: `echo_test`, `get_gpu_inventory`, `analyze_xid_errors`

### Tech Stack
- **Go:** 1.25.5
- **MCP Protocol:** 2025-06-18 via `github.com/mark3labs/mcp-go v0.43.2`
- **NVML:** `github.com/NVIDIA/go-nvml v0.13.0-1`
- **Testing:** `github.com/stretchr/testify`

### Remote GPU Machine
- **SSH:** `ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- **GPU:** Tesla T4 (16GB), Driver 575.57.08, CUDA 12.9
- **Go:** 1.25.5 at `/usr/local/go/bin/go`
- **Code Location:** `~/k8s-mcp-agent`
- **Kubernetes:** v1.33.3 single node

---

## OBJECTIVE: Implement GPU Health Monitoring

### Issue Reference
**Issue #7:** [Logic] Implement 'get_gpu_health' Tool  
**Milestone:** M2: Hardware Introspection  
**Due:** Jan 17, 2026

### What is GPU Health Monitoring?

GPU health monitoring provides real-time assessment of GPU operational status by analyzing:
- **Temperature:** Current temp vs. thermal limits
- **Throttling:** Active throttling reasons (thermal, power, HW slowdown)
- **ECC Errors:** Correctable and uncorrectable memory errors
- **Memory:** Usage patterns and available capacity
- **Power:** Current draw vs. TDP limits
- **Performance:** Utilization rates and clock speeds

The tool must **interpret** these metrics to provide an overall health score and actionable recommendations.

---

## IMPLEMENTATION TASKS

### Task 1: Define Health Data Structures

Create `pkg/tools/gpu_health.go` with comprehensive health structures:

```go
package tools

import (
	"context"
	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
	"github.com/mark3labs/mcp-go/mcp"
)

// GPUHealthHandler handles the get_gpu_health tool.
type GPUHealthHandler struct {
	nvmlClient nvml.Interface
}

// GPUHealthResponse is the top-level response structure.
type GPUHealthResponse struct {
	Status         string            `json:"status"` // "healthy", "warning", "degraded", "critical"
	OverallScore   int               `json:"overall_score"` // 0-100
	DeviceCount    int               `json:"device_count"`
	HealthyCount   int               `json:"healthy_count"`
	DegradedCount  int               `json:"degraded_count"`
	CriticalCount  int               `json:"critical_count"`
	GPUs           []GPUHealthStatus `json:"gpus"`
	Recommendation string            `json:"recommendation"`
}

// GPUHealthStatus contains health metrics for a single GPU.
type GPUHealthStatus struct {
	Index          int               `json:"index"`
	Name           string            `json:"name"`
	UUID           string            `json:"uuid"`
	PCIBusID       string            `json:"pci_bus_id"`
	Status         string            `json:"status"` // "healthy", "warning", "degraded", "critical"
	HealthScore    int               `json:"health_score"` // 0-100
	Temperature    TemperatureHealth `json:"temperature"`
	Memory         MemoryHealth      `json:"memory"`
	Power          PowerHealth       `json:"power"`
	Throttling     ThrottlingStatus  `json:"throttling"`
	ECCErrors      ECCHealth         `json:"ecc_errors"`
	Performance    PerformanceHealth `json:"performance"`
	Issues         []HealthIssue     `json:"issues,omitempty"`
}

// TemperatureHealth tracks thermal status.
type TemperatureHealth struct {
	Current     uint32 `json:"current_celsius"`
	Threshold   uint32 `json:"threshold_celsius"`
	Max         uint32 `json:"max_celsius"`
	Status      string `json:"status"` // "normal", "elevated", "high", "critical"
	Margin      int    `json:"margin_celsius"` // Distance from threshold
}

// MemoryHealth tracks memory usage and errors.
type MemoryHealth struct {
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	Free        uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Status      string  `json:"status"` // "normal", "high", "critical"
}

// PowerHealth tracks power consumption.
type PowerHealth struct {
	Current       uint32  `json:"current_mw"`
	Limit         uint32  `json:"limit_mw"`
	Default       uint32  `json:"default_mw"`
	UsedPercent   float64 `json:"used_percent"`
	Status        string  `json:"status"` // "normal", "high", "over_limit"
}

// ThrottlingStatus indicates if GPU is throttled.
type ThrottlingStatus struct {
	Active        bool     `json:"active"`
	Reasons       []string `json:"reasons,omitempty"`
	Status        string   `json:"status"` // "none", "minor", "severe"
}

// ECCHealth tracks ECC memory errors.
type ECCHealth struct {
	Enabled             bool   `json:"enabled"`
	TotalCorrectableErrors   uint64 `json:"total_correctable_errors"`
	TotalUncorrectableErrors uint64 `json:"total_uncorrectable_errors"`
	Status              string `json:"status"` // "healthy", "concerning", "critical"
}

// PerformanceHealth tracks utilization and clocks.
type PerformanceHealth struct {
	GPUUtil       uint32 `json:"gpu_util_percent"`
	MemoryUtil    uint32 `json:"memory_util_percent"`
	SMClock       uint32 `json:"sm_clock_mhz"`
	MemoryClock   uint32 `json:"memory_clock_mhz"`
	Status        string `json:"status"` // "idle", "active", "saturated"
}

// HealthIssue describes a specific health concern.
type HealthIssue struct {
	Severity    string `json:"severity"` // "info", "warning", "critical"
	Component   string `json:"component"` // "temperature", "memory", "power", etc.
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion"`
}
```

**Test file:** `pkg/tools/gpu_health_test.go`
- Test with mock NVML client
- Test various health scenarios
- Test health score calculation
- Test recommendation generation

---

### Task 2: Implement Health Collection

Core collection logic in `gpu_health.go`:

```go
func NewGPUHealthHandler(nvmlClient nvml.Interface) *GPUHealthHandler {
	return &GPUHealthHandler{
		nvmlClient: nvmlClient,
	}
}

func (h *GPUHealthHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// 1. Get device count
	count, err := h.nvmlClient.GetDeviceCount(ctx)
	
	// 2. Collect health for each GPU
	gpus := make([]GPUHealthStatus, 0, count)
	for i := 0; i < count; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return mcp.NewToolResultError("operation cancelled"), nil
		default:
		}
		
		device, err := h.nvmlClient.GetDeviceByIndex(ctx, i)
		if err != nil {
			continue
		}
		
		health := h.collectGPUHealth(ctx, i, device)
		gpus = append(gpus, health)
	}
	
	// 3. Calculate overall status
	response := h.calculateOverallHealth(gpus)
	
	// 4. Generate recommendations
	response.Recommendation = h.generateRecommendation(response)
	
	// 5. Marshal and return
	return h.marshalResponse(response)
}

func (h *GPUHealthHandler) collectGPUHealth(
	ctx context.Context,
	index int,
	device nvml.Device,
) GPUHealthStatus {
	health := GPUHealthStatus{
		Index:  index,
		Issues: make([]HealthIssue, 0),
	}
	
	// Get basic info
	health.Name, _ = device.GetName(ctx)
	health.UUID, _ = device.GetUUID(ctx)
	pciInfo, _ := device.GetPCIInfo(ctx)
	health.PCIBusID = pciInfo.BusID
	
	// Collect metrics
	health.Temperature = h.checkTemperature(ctx, device)
	health.Memory = h.checkMemory(ctx, device)
	health.Power = h.checkPower(ctx, device)
	health.Throttling = h.checkThrottling(ctx, device)
	health.ECCErrors = h.checkECCErrors(ctx, device)
	health.Performance = h.checkPerformance(ctx, device)
	
	// Calculate health score (0-100)
	health.HealthScore = h.calculateHealthScore(health)
	
	// Determine status
	health.Status = h.determineStatus(health.HealthScore, health.Issues)
	
	return health
}
```

---

### Task 3: Implement Health Checks

Each subsystem needs evaluation logic:

#### Temperature Check
```go
func (h *GPUHealthHandler) checkTemperature(
	ctx context.Context,
	device nvml.Device,
) TemperatureHealth {
	temp, _ := device.GetTemperature(ctx)
	
	// NVML constants for Tesla T4:
	// Threshold: ~80-85°C (slowdown begins)
	// Max: 90°C (critical shutdown)
	threshold := uint32(82)
	maxTemp := uint32(90)
	
	margin := int(threshold) - int(temp)
	
	var status string
	switch {
	case temp >= maxTemp:
		status = "critical"
	case temp >= threshold:
		status = "high"
	case temp >= threshold-10:
		status = "elevated"
	default:
		status = "normal"
	}
	
	return TemperatureHealth{
		Current:   temp,
		Threshold: threshold,
		Max:       maxTemp,
		Status:    status,
		Margin:    margin,
	}
}
```

#### ECC Errors Check
```go
func (h *GPUHealthHandler) checkECCErrors(
	ctx context.Context,
	device nvml.Device,
) ECCHealth {
	// Note: Mock NVML doesn't implement ECC methods
	// Real implementation would use:
	// - device.GetTotalEccErrors() for aggregate counts
	// - device.GetMemoryErrorCounter() for specific types
	
	// For now, return safe defaults
	// In production, would aggregate:
	// - NVML_MEMORY_ERROR_TYPE_CORRECTED (single-bit)
	// - NVML_MEMORY_ERROR_TYPE_UNCORRECTED (double-bit)
	
	return ECCHealth{
		Enabled:                  true, // Tesla T4 has ECC
		TotalCorrectableErrors:   0,
		TotalUncorrectableErrors: 0,
		Status:                   "healthy",
	}
}
```

#### Throttling Check
```go
func (h *GPUHealthHandler) checkThrottling(
	ctx context.Context,
	device nvml.Device,
) ThrottlingStatus {
	// Note: Mock NVML doesn't implement throttling methods
	// Real implementation would use:
	// - device.GetCurrentClocksThrottleReasons()
	
	// Throttle reasons bitmask:
	// 0x0001 - GPU idle
	// 0x0002 - Applications clocks setting
	// 0x0004 - SW power cap
	// 0x0008 - HW slowdown (thermal)
	// 0x0010 - Sync boost
	// 0x0020 - SW thermal slowdown
	// 0x0040 - HW thermal slowdown
	// 0x0080 - HW power brake slowdown
	
	return ThrottlingStatus{
		Active:  false,
		Reasons: []string{},
		Status:  "none",
	}
}
```

---

### Task 4: Health Scoring Algorithm

Implement weighted scoring system:

```go
func (h *GPUHealthHandler) calculateHealthScore(health GPUHealthStatus) int {
	score := 100
	
	// Temperature impact (max -30 points)
	switch health.Temperature.Status {
	case "critical":
		score -= 30
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "critical",
			Component:  "temperature",
			Message:    fmt.Sprintf("GPU temperature critical: %d°C", health.Temperature.Current),
			Suggestion: "Check cooling system, reduce workload immediately",
		})
	case "high":
		score -= 20
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "warning",
			Component:  "temperature",
			Message:    fmt.Sprintf("GPU temperature high: %d°C", health.Temperature.Current),
			Suggestion: "Monitor temperature, check cooling",
		})
	case "elevated":
		score -= 10
	}
	
	// Memory usage impact (max -20 points)
	if health.Memory.UsedPercent > 95 {
		score -= 20
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "critical",
			Component:  "memory",
			Message:    fmt.Sprintf("GPU memory critically low: %.1f%% used", health.Memory.UsedPercent),
			Suggestion: "Free GPU memory or reduce workload",
		})
	} else if health.Memory.UsedPercent > 90 {
		score -= 10
	}
	
	// Power usage impact (max -15 points)
	if health.Power.UsedPercent > 100 {
		score -= 15
	} else if health.Power.UsedPercent > 95 {
		score -= 10
	}
	
	// Throttling impact (max -25 points)
	switch health.Throttling.Status {
	case "severe":
		score -= 25
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "critical",
			Component:  "throttling",
			Message:    "GPU severely throttled",
			Suggestion: "Investigate thermal or power issues",
		})
	case "minor":
		score -= 10
	}
	
	// ECC errors impact (max -30 points)
	if health.ECCErrors.TotalUncorrectableErrors > 0 {
		score -= 30
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "critical",
			Component:  "ecc",
			Message:    fmt.Sprintf("%d uncorrectable ECC errors", health.ECCErrors.TotalUncorrectableErrors),
			Suggestion: "GPU may have hardware failure, drain node",
		})
	} else if health.ECCErrors.TotalCorrectableErrors > 1000 {
		score -= 10
	}
	
	if score < 0 {
		score = 0
	}
	
	return score
}

func (h *GPUHealthHandler) determineStatus(score int, issues []HealthIssue) string {
	// Check for critical issues first
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return "critical"
		}
	}
	
	// Score-based determination
	switch {
	case score >= 90:
		return "healthy"
	case score >= 70:
		return "warning"
	case score >= 50:
		return "degraded"
	default:
		return "critical"
	}
}
```

---

### Task 5: Register Tool

In `pkg/mcp/server.go`, add after XID tool:

```go
// Register GPU health tool
healthHandler := tools.NewGPUHealthHandler(cfg.NVMLClient)
mcpServer.AddTool(tools.GetGPUHealthTool(), healthHandler.Handle)
```

Create tool definition:
```go
func GetGPUHealthTool() mcp.Tool {
	return mcp.NewTool("get_gpu_health",
		mcp.WithDescription(
			"Analyze GPU operational health including temperature, throttling, "+
			"ECC errors, memory usage, and power consumption. Returns overall "+
			"health score (0-100) with status assessment and recommendations.",
		),
	)
}
```

---

### Task 6: Add Example

Create `examples/gpu_health.json`:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_gpu_health",
    "arguments": {}
  },
  "id": 4
}
```

---

## TESTING PROCEDURE

### Local Testing (Mock NVML)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent

# Run unit tests
make test

# Build
make agent

# Test with mock mode
cat examples/gpu_health.json | ./bin/agent --nvml-mode=mock
```

### Remote Testing (Real Tesla T4)

```bash
# SSH to GPU machine
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com

# Navigate to code
cd ~/k8s-mcp-agent

# Pull latest changes
git fetch origin
git checkout feat/m2-gpu-health

# Build
export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent

# Test tool
cat examples/gpu_health.json | ./bin/agent --nvml-mode=real

# Run integration tests
go test -tags=integration -v ./pkg/tools/
```

---

## EXAMPLE OUTPUT (Expected)

### Healthy GPU
```json
{
  "status": "healthy",
  "overall_score": 95,
  "device_count": 1,
  "healthy_count": 1,
  "degraded_count": 0,
  "critical_count": 0,
  "gpus": [{
    "index": 0,
    "name": "Tesla T4",
    "uuid": "GPU-d129fc5b-2d51-cec7-d985-49168c12716f",
    "pci_bus_id": "0000:00:1E.0",
    "status": "healthy",
    "health_score": 95,
    "temperature": {
      "current_celsius": 29,
      "threshold_celsius": 82,
      "max_celsius": 90,
      "status": "normal",
      "margin_celsius": 53
    },
    "memory": {
      "total_bytes": 16106127360,
      "used_bytes": 469041152,
      "free_bytes": 15637086208,
      "used_percent": 2.9,
      "status": "normal"
    },
    "power": {
      "current_mw": 13935,
      "limit_mw": 70000,
      "default_mw": 70000,
      "used_percent": 19.9,
      "status": "normal"
    },
    "throttling": {
      "active": false,
      "reasons": [],
      "status": "none"
    },
    "ecc_errors": {
      "enabled": true,
      "total_correctable_errors": 0,
      "total_uncorrectable_errors": 0,
      "status": "healthy"
    },
    "performance": {
      "gpu_util_percent": 0,
      "memory_util_percent": 0,
      "sm_clock_mhz": 300,
      "memory_clock_mhz": 405,
      "status": "idle"
    },
    "issues": []
  }],
  "recommendation": "All GPUs healthy. No action required."
}
```

### Degraded GPU
```json
{
  "status": "degraded",
  "overall_score": 65,
  "device_count": 1,
  "healthy_count": 0,
  "degraded_count": 1,
  "critical_count": 0,
  "gpus": [{
    "index": 0,
    "name": "Tesla T4",
    "status": "degraded",
    "health_score": 65,
    "temperature": {
      "current_celsius": 85,
      "status": "high",
      "margin_celsius": -3
    },
    "memory": {
      "used_percent": 92.5,
      "status": "high"
    },
    "issues": [
      {
        "severity": "warning",
        "component": "temperature",
        "message": "GPU temperature high: 85°C",
        "suggestion": "Monitor temperature, check cooling"
      },
      {
        "severity": "warning",
        "component": "memory",
        "message": "GPU memory high: 92.5% used",
        "suggestion": "Consider freeing GPU memory"
      }
    ]
  }],
  "recommendation": "1 GPU(s) degraded. Monitor closely and investigate issues."
}
```

---

## IMPLEMENTATION STEPS

### Step 1: Create Branch and Structure
```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent
git checkout main && git pull
git checkout -b feat/m2-gpu-health

touch pkg/tools/gpu_health.go
touch pkg/tools/gpu_health_test.go
touch examples/gpu_health.json
```

### Step 2: Implement Data Structures

Create all health structures in `pkg/tools/gpu_health.go`:
- GPUHealthResponse
- GPUHealthStatus
- TemperatureHealth, MemoryHealth, PowerHealth, etc.
- HealthIssue

### Step 3: Implement Health Checks

Implement individual check functions:
- `checkTemperature()`
- `checkMemory()`
- `checkPower()`
- `checkThrottling()`
- `checkECCErrors()`
- `checkPerformance()`

### Step 4: Implement Scoring Algorithm

Create health scoring logic:
- `calculateHealthScore()` - weighted scoring
- `determineStatus()` - status classification
- `collectGPUHealth()` - orchestrate checks

### Step 5: Implement Handler

Create MCP tool handler:
- `NewGPUHealthHandler()`
- `Handle()` - main entry point
- `calculateOverallHealth()` - aggregate status
- `generateRecommendation()` - create advice

### Step 6: Register Tool

Update `pkg/mcp/server.go`:
- Add health handler creation
- Register tool with server
- Create `GetGPUHealthTool()` definition

### Step 7: Write Tests

Create comprehensive tests:
- Mock NVML with various scenarios
- Test healthy GPU
- Test degraded GPU
- Test critical issues
- Test scoring algorithm
- Test recommendations

### Step 8: Verify Locally

```bash
make fmt
make all
./bin/agent --nvml-mode=mock < examples/gpu_health.json
```

### Step 9: Test on Tesla T4

Push to remote and test on real GPU hardware.

### Step 10: Create PR

```bash
git add -A
git commit -s -S -m "feat(health): implement GPU health monitoring tool"
git push -u origin feat/m2-gpu-health
gh pr create --title "feat(health): Implement GPU health monitoring tool" \
  --label "kind/feature" --label "area/nvml-binding" --milestone "M2: Hardware Introspection"
```

---

## CODE GUIDELINES

### Error Handling
```go
// ✅ Good - provide context
if err != nil {
    return fmt.Errorf("failed to check temperature: %w", err)
}

// ❌ Bad - no context
if err != nil {
    return err
}
```

### Context Checks
```go
// ✅ Good - check in loops
for i := 0; i < count; i++ {
    if err := ctx.Err(); err != nil {
        return fmt.Errorf("context cancelled: %w", err)
    }
    // ... do work
}
```

### Testing
```go
// ✅ Good - table-driven tests
func TestCalculateHealthScore(t *testing.T) {
    tests := []struct {
        name  string
        health GPUHealthStatus
        want  int
    }{
        {"healthy_gpu", healthyGPU, 95},
        {"hot_gpu", hotGPU, 75},
        {"critical_gpu", criticalGPU, 30},
    }
    // ...
}
```

---

## ACCEPTANCE CRITERIA

**Must Have:**
- [ ] Health check for temperature, memory, power, throttling, ECC, performance
- [ ] Health score calculation (0-100) with weighted factors
- [ ] Status classification (healthy/warning/degraded/critical)
- [ ] Issue detection with severity levels
- [ ] Overall system status and recommendations
- [ ] Unit tests with mock NVML (>80% coverage)
- [ ] All existing 59 tests still pass
- [ ] `make all` passes (0 lint/vet errors)

**Should Have:**
- [ ] Integration test on Tesla T4
- [ ] Graceful handling of missing NVML data
- [ ] Context cancellation in all loops
- [ ] Detailed issue messages with suggestions

**Nice to Have:**
- [ ] Historical trend analysis
- [ ] Per-GPU health recommendations
- [ ] Temperature threshold configuration
- [ ] Custom health weights

---

## POTENTIAL ISSUES & SOLUTIONS

### Issue 1: ECC/Throttling Methods Not in Mock
**Solution:** Return safe defaults in mock, implement properly for real NVML. Add TODO comments for future enhancement.

### Issue 2: Threshold Values Vary by GPU Model
**Solution:** Start with Tesla T4 values (82°C threshold, 90°C max). Add model detection in future iterations.

### Issue 3: Health Score Subjectivity
**Solution:** Use conservative weights prioritizing hardware safety. Document scoring algorithm clearly.

### Issue 4: Different GPUs, Different Health Levels
**Solution:** Overall status uses "worst GPU" approach - one critical GPU = critical system.

---

## QUICK REFERENCE

### Key Files to Reference
- **Pattern:** `pkg/tools/analyze_xid.go` (similar tool structure)
- **Tests:** `pkg/tools/analyze_xid_test.go` (testing pattern)
- **NVML:** `pkg/nvml/interface.go` (available methods)
- **Server:** `pkg/mcp/server.go` (tool registration)

### Key Commands
```bash
make fmt              # Format code
make test             # Run unit tests
make all              # Full check suite
git commit -s -S      # Commit with DCO + GPG
gh pr create          # Create PR
```

### Remote Machine
```bash
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com
cd ~/k8s-mcp-agent
export PATH=/usr/local/go/bin:$PATH
```

---

## NVML METHODS AVAILABLE

From `pkg/nvml/interface.go`, currently implemented:

```go
// Device interface
GetName(ctx) (string, error)
GetUUID(ctx) (string, error)
GetPCIInfo(ctx) (*PCIInfo, error)
GetMemoryInfo(ctx) (*MemoryInfo, error)
GetTemperature(ctx) (uint32, error)
GetPowerUsage(ctx) (uint32, error)  // Returns milliwatts
GetUtilizationRates(ctx) (*UtilizationInfo, error)
```

**Note:** ECC and throttling methods not yet in interface. Will need to add or use safe defaults.

---

## START HERE

1. Create branch: `git checkout -b feat/m2-gpu-health`
2. Create file structure
3. **Start with data structures** in `pkg/tools/gpu_health.go`
4. Implement health checks one at a time
5. Add scoring algorithm
6. Write tests with each component
7. Register tool in MCP server
8. Test locally with mock
9. Test on Tesla T4
10. Create PR

**Reference:** Tesla T4 typical values:
- Idle temp: ~25-35°C
- Load temp: ~60-75°C
- Threshold: ~82°C
- Max: ~90°C
- TDP: 70W
- Memory: 16GB

---

**Reply "GO" when ready to start implementation.** 🚀

