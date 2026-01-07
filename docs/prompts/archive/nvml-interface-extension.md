# Extend NVML Interface for Full GPU Health Monitoring

## PROJECT CONTEXT

**Repository:** https://github.com/ArangoGutierrez/k8s-gpu-mcp-server  
**Issue:** #5 - [NVML] Implement Wrapper Interface  
**Milestone:** M2: Hardware Introspection (Due: Jan 17, 2026)  
**Workspace:** `/Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server`

### Current State (Jan 4, 2026)
- ✅ M1 Complete: MCP stdio server working
- ✅ M2 Partial: 4 tools working (`echo_test`, `get_gpu_inventory`, `analyze_xid_errors`, `get_gpu_health`)
- ✅ Tests: 74/74 passing
- ⚠️ `get_gpu_health` uses hardcoded defaults for ECC, throttling, power limits
- ⚠️ Need real NVML methods to replace stubs

### Tech Stack
- **Go:** 1.25.5
- **NVML:** `github.com/NVIDIA/go-nvml v0.13.0-1`
- **Testing:** `github.com/stretchr/testify`

### Remote GPU Machine
- **SSH:** `ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- **GPU:** Tesla T4 (16GB), Driver 575.57.08, CUDA 12.9
- **Go:** 1.25.5 at `/usr/local/go/bin/go`
- **Code Location:** `~/k8s-gpu-mcp-server`

---

## FIRST TASK: Create Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main && git pull
git checkout -b feat/m2-nvml-extension
```

---

## OBJECTIVE

Extend `pkg/nvml/interface.go` with additional NVML methods needed for complete GPU health monitoring. Currently `get_gpu_health` uses hardcoded defaults; this task adds real NVML bindings.

### Methods to Add

| Method | Purpose | Used By |
|--------|---------|---------|
| `GetPowerManagementLimit` | Get actual GPU TDP limit | `checkPower()` |
| `GetEnforcedPowerLimit` | Get current enforced power limit | `checkPower()` |
| `GetEccMode` | Check if ECC is enabled | `checkECCErrors()` |
| `GetTotalEccErrors` | Get ECC error counts | `checkECCErrors()` |
| `GetCurrentClocksThrottleReasons` | Get throttling bitmask | `checkThrottling()` |
| `GetClockInfo` | Get current clock frequencies | `checkPerformance()` |
| `GetTemperatureThreshold` | Get thermal thresholds | `checkTemperature()` |

---

## IMPLEMENTATION TASKS

### Task 1: Extend Device Interface

Update `pkg/nvml/interface.go`:

```go
// Device represents a single GPU device.
type Device interface {
    // Existing methods...
    GetName(ctx context.Context) (string, error)
    GetUUID(ctx context.Context) (string, error)
    GetPCIInfo(ctx context.Context) (*PCIInfo, error)
    GetMemoryInfo(ctx context.Context) (*MemoryInfo, error)
    GetTemperature(ctx context.Context) (uint32, error)
    GetPowerUsage(ctx context.Context) (uint32, error)
    GetUtilizationRates(ctx context.Context) (*Utilization, error)

    // NEW: Power management
    // GetPowerManagementLimit returns the power management limit in milliwatts.
    // This is the maximum power the GPU is allowed to draw.
    GetPowerManagementLimit(ctx context.Context) (uint32, error)

    // NEW: ECC status
    // GetEccMode returns whether ECC is currently enabled and pending mode.
    GetEccMode(ctx context.Context) (current, pending bool, err error)

    // NEW: ECC errors
    // GetTotalEccErrors returns the total count of ECC errors.
    // errorType: 0 = correctable (single-bit), 1 = uncorrectable (double-bit)
    GetTotalEccErrors(ctx context.Context, errorType int) (uint64, error)

    // NEW: Throttling
    // GetCurrentClocksThrottleReasons returns a bitmask of throttle reasons.
    // See ThrottleReason constants for bit definitions.
    GetCurrentClocksThrottleReasons(ctx context.Context) (uint64, error)

    // NEW: Clock frequencies
    // GetClockInfo returns the current clock frequency for the given clock type.
    // clockType: 0 = graphics (SM), 1 = memory
    GetClockInfo(ctx context.Context, clockType int) (uint32, error)

    // NEW: Temperature thresholds
    // GetTemperatureThreshold returns the temperature threshold for the given type.
    // thresholdType: 0 = shutdown, 1 = slowdown
    GetTemperatureThreshold(ctx context.Context, thresholdType int) (uint32, error)
}

// ThrottleReason constants for interpreting GetCurrentClocksThrottleReasons.
const (
    ThrottleReasonGpuIdle            uint64 = 0x0000000000000001
    ThrottleReasonApplicationsClocks uint64 = 0x0000000000000002
    ThrottleReasonSwPowerCap         uint64 = 0x0000000000000004
    ThrottleReasonHwSlowdown         uint64 = 0x0000000000000008
    ThrottleReasonSyncBoost          uint64 = 0x0000000000000010
    ThrottleReasonSwThermalSlowdown  uint64 = 0x0000000000000020
    ThrottleReasonHwThermalSlowdown  uint64 = 0x0000000000000040
    ThrottleReasonHwPowerBrake       uint64 = 0x0000000000000080
)

// ClockType constants for GetClockInfo.
const (
    ClockGraphics = 0 // SM clock
    ClockMemory   = 1 // Memory clock
)

// TemperatureThresholdType constants for GetTemperatureThreshold.
const (
    TempThresholdShutdown = 0
    TempThresholdSlowdown = 1
)

// EccErrorType constants for GetTotalEccErrors.
const (
    EccErrorCorrectable   = 0 // Single-bit errors
    EccErrorUncorrectable = 1 // Double-bit errors
)
```

---

### Task 2: Implement Real NVML Methods

Update `pkg/nvml/real.go` to implement the new methods:

```go
// GetPowerManagementLimit returns the power management limit in milliwatts.
func (d *RealDevice) GetPowerManagementLimit(
    ctx context.Context,
) (uint32, error) {
    if err := ctx.Err(); err != nil {
        return 0, fmt.Errorf("context cancelled: %w", err)
    }

    limit, ret := d.device.GetPowerManagementLimit()
    if ret != nvml.SUCCESS {
        return 0, fmt.Errorf("failed to get power limit: %s",
            nvml.ErrorString(ret))
    }
    return limit, nil
}

// GetEccMode returns whether ECC is currently enabled.
func (d *RealDevice) GetEccMode(
    ctx context.Context,
) (current, pending bool, err error) {
    if err := ctx.Err(); err != nil {
        return false, false, fmt.Errorf("context cancelled: %w", err)
    }

    curr, pend, ret := d.device.GetEccMode()
    if ret != nvml.SUCCESS {
        // ECC not supported is not an error, just return false
        if ret == nvml.ERROR_NOT_SUPPORTED {
            return false, false, nil
        }
        return false, false, fmt.Errorf("failed to get ECC mode: %s",
            nvml.ErrorString(ret))
    }
    return curr == nvml.FEATURE_ENABLED, pend == nvml.FEATURE_ENABLED, nil
}

// GetTotalEccErrors returns total ECC error count.
func (d *RealDevice) GetTotalEccErrors(
    ctx context.Context,
    errorType int,
) (uint64, error) {
    if err := ctx.Err(); err != nil {
        return 0, fmt.Errorf("context cancelled: %w", err)
    }

    var nvmlErrorType nvml.MemoryErrorType
    if errorType == EccErrorCorrectable {
        nvmlErrorType = nvml.MEMORY_ERROR_TYPE_CORRECTED
    } else {
        nvmlErrorType = nvml.MEMORY_ERROR_TYPE_UNCORRECTED
    }

    // Get aggregate errors across all memory locations
    count, ret := d.device.GetTotalEccErrors(
        nvmlErrorType,
        nvml.AGGREGATE_ECC,
    )
    if ret != nvml.SUCCESS {
        if ret == nvml.ERROR_NOT_SUPPORTED {
            return 0, nil
        }
        return 0, fmt.Errorf("failed to get ECC errors: %s",
            nvml.ErrorString(ret))
    }
    return count, nil
}

// GetCurrentClocksThrottleReasons returns the current throttle reason bitmask.
func (d *RealDevice) GetCurrentClocksThrottleReasons(
    ctx context.Context,
) (uint64, error) {
    if err := ctx.Err(); err != nil {
        return 0, fmt.Errorf("context cancelled: %w", err)
    }

    reasons, ret := d.device.GetCurrentClocksThrottleReasons()
    if ret != nvml.SUCCESS {
        if ret == nvml.ERROR_NOT_SUPPORTED {
            return 0, nil
        }
        return 0, fmt.Errorf("failed to get throttle reasons: %s",
            nvml.ErrorString(ret))
    }
    return reasons, nil
}

// GetClockInfo returns the current clock frequency in MHz.
func (d *RealDevice) GetClockInfo(
    ctx context.Context,
    clockType int,
) (uint32, error) {
    if err := ctx.Err(); err != nil {
        return 0, fmt.Errorf("context cancelled: %w", err)
    }

    var nvmlClockType nvml.ClockType
    if clockType == ClockGraphics {
        nvmlClockType = nvml.CLOCK_GRAPHICS
    } else {
        nvmlClockType = nvml.CLOCK_MEM
    }

    clock, ret := d.device.GetClockInfo(nvmlClockType)
    if ret != nvml.SUCCESS {
        return 0, fmt.Errorf("failed to get clock info: %s",
            nvml.ErrorString(ret))
    }
    return clock, nil
}

// GetTemperatureThreshold returns the temperature threshold in Celsius.
func (d *RealDevice) GetTemperatureThreshold(
    ctx context.Context,
    thresholdType int,
) (uint32, error) {
    if err := ctx.Err(); err != nil {
        return 0, fmt.Errorf("context cancelled: %w", err)
    }

    var nvmlThresholdType nvml.TemperatureThresholds
    if thresholdType == TempThresholdShutdown {
        nvmlThresholdType = nvml.TEMPERATURE_THRESHOLD_SHUTDOWN
    } else {
        nvmlThresholdType = nvml.TEMPERATURE_THRESHOLD_SLOWDOWN
    }

    temp, ret := d.device.GetTemperatureThreshold(nvmlThresholdType)
    if ret != nvml.SUCCESS {
        if ret == nvml.ERROR_NOT_SUPPORTED {
            return 0, nil
        }
        return 0, fmt.Errorf("failed to get temp threshold: %s",
            nvml.ErrorString(ret))
    }
    return temp, nil
}
```

---

### Task 3: Implement Mock Methods

Update `pkg/nvml/mock.go` to add the new methods:

```go
// Add to MockDevice struct
type MockDevice struct {
    // ... existing fields ...
    
    // New fields
    powerLimit       uint32
    eccEnabled       bool
    eccCorrectable   uint64
    eccUncorrectable uint64
    throttleReasons  uint64
    smClock          uint32
    memClock         uint32
    tempShutdown     uint32
    tempSlowdown     uint32
}

// Update NewMock to initialize new fields
func NewMock(deviceCount int) *Mock {
    // ... existing code ...
    
    for i := 0; i < deviceCount; i++ {
        m.devices[i] = &MockDevice{
            // ... existing fields ...
            
            // New fields with reasonable defaults
            powerLimit:       400000, // 400W for A100
            eccEnabled:       true,
            eccCorrectable:   0,
            eccUncorrectable: 0,
            throttleReasons:  0, // No throttling
            smClock:          1410,
            memClock:         1215,
            tempShutdown:     90,
            tempSlowdown:     82,
        }
    }
    return m
}

// Implement new methods
func (d *MockDevice) GetPowerManagementLimit(ctx context.Context) (uint32, error) {
    return d.powerLimit, nil
}

func (d *MockDevice) GetEccMode(ctx context.Context) (bool, bool, error) {
    return d.eccEnabled, d.eccEnabled, nil
}

func (d *MockDevice) GetTotalEccErrors(ctx context.Context, errorType int) (uint64, error) {
    if errorType == EccErrorCorrectable {
        return d.eccCorrectable, nil
    }
    return d.eccUncorrectable, nil
}

func (d *MockDevice) GetCurrentClocksThrottleReasons(ctx context.Context) (uint64, error) {
    return d.throttleReasons, nil
}

func (d *MockDevice) GetClockInfo(ctx context.Context, clockType int) (uint32, error) {
    if clockType == ClockGraphics {
        return d.smClock, nil
    }
    return d.memClock, nil
}

func (d *MockDevice) GetTemperatureThreshold(ctx context.Context, thresholdType int) (uint32, error) {
    if thresholdType == TempThresholdShutdown {
        return d.tempShutdown, nil
    }
    return d.tempSlowdown, nil
}
```

---

### Task 4: Update real_stub.go

Ensure `pkg/nvml/real_stub.go` has stub implementations for non-CGO builds:

```go
//go:build !cgo
// +build !cgo

package nvml

// Add stub implementations for all new methods
// These return errors indicating NVML is not available

func (d *RealDevice) GetPowerManagementLimit(ctx context.Context) (uint32, error) {
    return 0, fmt.Errorf("NVML not available: built without CGO")
}

// ... same pattern for all new methods ...
```

---

### Task 5: Write Unit Tests

Create/update `pkg/nvml/mock_test.go`:

```go
func TestMockDevice_GetPowerManagementLimit(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    limit, err := device.GetPowerManagementLimit(ctx)
    require.NoError(t, err)
    assert.Equal(t, uint32(400000), limit)
}

func TestMockDevice_GetEccMode(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    current, pending, err := device.GetEccMode(ctx)
    require.NoError(t, err)
    assert.True(t, current)
    assert.True(t, pending)
}

func TestMockDevice_GetTotalEccErrors(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    correctable, err := device.GetTotalEccErrors(ctx, EccErrorCorrectable)
    require.NoError(t, err)
    assert.Equal(t, uint64(0), correctable)
    
    uncorrectable, err := device.GetTotalEccErrors(ctx, EccErrorUncorrectable)
    require.NoError(t, err)
    assert.Equal(t, uint64(0), uncorrectable)
}

func TestMockDevice_GetCurrentClocksThrottleReasons(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    reasons, err := device.GetCurrentClocksThrottleReasons(ctx)
    require.NoError(t, err)
    assert.Equal(t, uint64(0), reasons) // No throttling
}

func TestMockDevice_GetClockInfo(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    smClock, err := device.GetClockInfo(ctx, ClockGraphics)
    require.NoError(t, err)
    assert.Greater(t, smClock, uint32(0))
    
    memClock, err := device.GetClockInfo(ctx, ClockMemory)
    require.NoError(t, err)
    assert.Greater(t, memClock, uint32(0))
}

func TestMockDevice_GetTemperatureThreshold(t *testing.T) {
    mock := NewMock(1)
    ctx := context.Background()
    device, _ := mock.GetDeviceByIndex(ctx, 0)
    
    shutdown, err := device.GetTemperatureThreshold(ctx, TempThresholdShutdown)
    require.NoError(t, err)
    assert.Equal(t, uint32(90), shutdown)
    
    slowdown, err := device.GetTemperatureThreshold(ctx, TempThresholdSlowdown)
    require.NoError(t, err)
    assert.Equal(t, uint32(82), slowdown)
}
```

---

### Task 6: Update gpu_health.go to Use New Methods

Once the NVML interface is extended, update `pkg/tools/gpu_health.go`:

#### checkTemperature - Use real thresholds:
```go
func (h *GPUHealthHandler) checkTemperature(ctx context.Context, device nvml.Device) TemperatureHealth {
    temp, err := device.GetTemperature(ctx)
    if err != nil {
        return TemperatureHealth{Status: "unknown"}
    }
    
    // Get real thresholds from device
    threshold, _ := device.GetTemperatureThreshold(ctx, nvml.TempThresholdSlowdown)
    maxTemp, _ := device.GetTemperatureThreshold(ctx, nvml.TempThresholdShutdown)
    
    // Fallback to defaults if not available
    if threshold == 0 { threshold = defaultTempThreshold }
    if maxTemp == 0 { maxTemp = defaultTempMax }
    
    // ... rest of logic using real thresholds ...
}
```

#### checkPower - Use real power limit:
```go
func (h *GPUHealthHandler) checkPower(ctx context.Context, device nvml.Device) PowerHealth {
    power, err := device.GetPowerUsage(ctx)
    if err != nil {
        return PowerHealth{Status: "unknown"}
    }
    
    // Get real power limit from device
    limit, err := device.GetPowerManagementLimit(ctx)
    if err != nil || limit == 0 {
        limit = defaultPowerLimit
    }
    
    // ... rest of logic using real limit ...
}
```

#### checkECCErrors - Use real ECC data:
```go
func (h *GPUHealthHandler) checkECCErrors(ctx context.Context, device nvml.Device) ECCHealth {
    enabled, _, err := device.GetEccMode(ctx)
    if err != nil {
        return ECCHealth{Enabled: false, Status: "unknown"}
    }
    
    if !enabled {
        return ECCHealth{Enabled: false, Status: "disabled"}
    }
    
    correctable, _ := device.GetTotalEccErrors(ctx, nvml.EccErrorCorrectable)
    uncorrectable, _ := device.GetTotalEccErrors(ctx, nvml.EccErrorUncorrectable)
    
    // ... rest of logic using real counts ...
}
```

#### checkThrottling - Use real throttle reasons:
```go
func (h *GPUHealthHandler) checkThrottling(ctx context.Context, device nvml.Device) ThrottlingStatus {
    reasons, err := device.GetCurrentClocksThrottleReasons(ctx)
    if err != nil {
        return ThrottlingStatus{Status: "unknown"}
    }
    
    // Ignore idle throttling (normal)
    activeReasons := reasons &^ nvml.ThrottleReasonGpuIdle
    
    if activeReasons == 0 {
        return ThrottlingStatus{Active: false, Status: "none"}
    }
    
    // Parse throttle reasons
    var reasonStrings []string
    if activeReasons&nvml.ThrottleReasonHwThermalSlowdown != 0 {
        reasonStrings = append(reasonStrings, "hw_thermal")
    }
    if activeReasons&nvml.ThrottleReasonSwThermalSlowdown != 0 {
        reasonStrings = append(reasonStrings, "sw_thermal")
    }
    if activeReasons&nvml.ThrottleReasonHwSlowdown != 0 {
        reasonStrings = append(reasonStrings, "hw_slowdown")
    }
    if activeReasons&nvml.ThrottleReasonSwPowerCap != 0 {
        reasonStrings = append(reasonStrings, "power_cap")
    }
    if activeReasons&nvml.ThrottleReasonHwPowerBrake != 0 {
        reasonStrings = append(reasonStrings, "power_brake")
    }
    
    // Determine severity
    var status string
    if len(reasonStrings) >= 2 {
        status = "severe"
    } else {
        status = "minor"
    }
    
    return ThrottlingStatus{
        Active:  true,
        Reasons: reasonStrings,
        Status:  status,
    }
}
```

#### checkPerformance - Add clock frequencies:
```go
func (h *GPUHealthHandler) checkPerformance(ctx context.Context, device nvml.Device) PerformanceHealth {
    util, err := device.GetUtilizationRates(ctx)
    if err != nil {
        return PerformanceHealth{Status: "unknown"}
    }
    
    smClock, _ := device.GetClockInfo(ctx, nvml.ClockGraphics)
    memClock, _ := device.GetClockInfo(ctx, nvml.ClockMemory)
    
    return PerformanceHealth{
        GPUUtil:     util.GPU,
        MemoryUtil:  util.Memory,
        SMClock:     smClock,
        MemoryClock: memClock,
        Status:      determinePerformanceStatus(util.GPU),
    }
}
```

---

## TESTING PROCEDURE

### Local Testing (Mock NVML)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

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

cd ~/k8s-gpu-mcp-server
git fetch origin
git checkout feat/m2-nvml-extension

export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent

# Test with real NVML
cat examples/gpu_health.json | ./bin/agent --nvml-mode=real

# Run integration tests
go test -tags=integration -v ./pkg/nvml/
```

---

## ACCEPTANCE CRITERIA

**Must Have:**
- [ ] 6 new methods added to `Device` interface
- [ ] All methods implemented in `real.go` with proper error handling
- [ ] All methods implemented in `mock.go` with configurable values
- [ ] Stub implementations in `real_stub.go` for non-CGO builds
- [ ] Unit tests for all new mock methods
- [ ] `make all` passes (0 lint/vet errors)
- [ ] All existing 74 tests still pass

**Should Have:**
- [ ] Integration test on Tesla T4
- [ ] Update `gpu_health.go` to use new methods
- [ ] Graceful fallback when methods return errors
- [ ] Documentation comments on all new methods

**Nice to Have:**
- [ ] Helper functions for parsing throttle reasons
- [ ] Constants exported for consumers

---

## FILE PLAN

| Path | Purpose | Acceptance |
|------|---------|------------|
| `pkg/nvml/interface.go` | Add 6 new methods + constants | Compiles |
| `pkg/nvml/real.go` | Implement for real NVML | Works on T4 |
| `pkg/nvml/real_stub.go` | Stub for non-CGO | Compiles |
| `pkg/nvml/mock.go` | Mock implementation | Tests pass |
| `pkg/nvml/mock_test.go` | Unit tests | All pass |
| `pkg/tools/gpu_health.go` | Use new methods | Real data in output |

---

## IMPLEMENTATION ORDER

1. **Create branch:** `feat/m2-nvml-extension`
2. **Update interface.go:** Add method signatures and constants
3. **Update mock.go:** Add mock implementation and fields
4. **Update mock_test.go:** Add tests for new methods
5. **Run tests:** Ensure mock tests pass
6. **Update real.go:** Add real NVML implementation
7. **Update real_stub.go:** Add stub implementations
8. **Run make all:** Verify everything compiles
9. **Test locally:** Mock mode working
10. **Test on T4:** Real NVML working
11. **Update gpu_health.go:** Use new methods
12. **Final testing:** All tools working
13. **Create PR:** With proper labels

---

## NVML REFERENCE

### go-nvml Methods Used

```go
// Power
device.GetPowerManagementLimit() (uint32, Return)

// ECC
device.GetEccMode() (EnableState, EnableState, Return)
device.GetTotalEccErrors(MemoryErrorType, EccCounterType) (uint64, Return)

// Throttling
device.GetCurrentClocksThrottleReasons() (uint64, Return)

// Clocks
device.GetClockInfo(ClockType) (uint32, Return)

// Temperature
device.GetTemperatureThreshold(TemperatureThresholds) (uint32, Return)
```

### Constants from go-nvml

```go
// Memory error types
nvml.MEMORY_ERROR_TYPE_CORRECTED
nvml.MEMORY_ERROR_TYPE_UNCORRECTED

// ECC counter types
nvml.AGGREGATE_ECC

// Clock types
nvml.CLOCK_GRAPHICS
nvml.CLOCK_MEM

// Temperature thresholds
nvml.TEMPERATURE_THRESHOLD_SHUTDOWN
nvml.TEMPERATURE_THRESHOLD_SLOWDOWN

// Throttle reasons (bitmask)
nvml.ClocksThrottleReasonGpuIdle
nvml.ClocksThrottleReasonApplicationsClocksSetting
nvml.ClocksThrottleReasonSwPowerCap
nvml.ClocksThrottleReasonHwSlowdown
nvml.ClocksThrottleReasonSyncBoost
nvml.ClocksThrottleReasonSwThermalSlowdown
nvml.ClocksThrottleReasonHwThermalSlowdown
nvml.ClocksThrottleReasonHwPowerBrakeSlowdown
```

---

## QUICK REFERENCE

### Key Files
- **Interface:** `pkg/nvml/interface.go`
- **Real NVML:** `pkg/nvml/real.go`
- **Mock:** `pkg/nvml/mock.go`
- **Stub:** `pkg/nvml/real_stub.go`
- **Tests:** `pkg/nvml/mock_test.go`, `pkg/nvml/real_test.go`
- **Consumer:** `pkg/tools/gpu_health.go`

### Commands
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
cd ~/k8s-gpu-mcp-server
export PATH=/usr/local/go/bin:$PATH
```

---

**Reply "GO" when ready to start implementation.** 🚀

