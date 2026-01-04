# Enhance GPU Inventory with Extended NVML Data

## PROJECT CONTEXT

**Repository:** https://github.com/ArangoGutierrez/k8s-mcp-agent  
**Issue:** Enhancement of `get_gpu_inventory` tool  
**Milestone:** M2: Hardware Introspection (Due: Jan 17, 2026)  
**Workspace:** `/Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent`

### Current State (Jan 4, 2026)
- ✅ M1 Complete: MCP stdio server working
- ✅ M2 Partial: 4 tools working (`echo_test`, `get_gpu_inventory`, `analyze_xid_errors`, `get_gpu_health`)
- ✅ NVML Interface Extended: 6 new methods added and tested on Tesla T4
- ✅ Tests: All passing
- ⚠️ `get_gpu_inventory` only uses basic NVML methods
- ⚠️ Missing: power limits, ECC status, clocks, temperature thresholds

### Tech Stack
- **Go:** 1.25.5
- **NVML:** `github.com/NVIDIA/go-nvml v0.13.0-1`
- **Testing:** `github.com/stretchr/testify`

### Remote GPU Machine
- **SSH:** `ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com`
- **GPU:** Tesla T4 (16GB), Driver 575.57.08, CUDA 12.9
- **Go:** 1.25.5 at `/usr/local/go/bin/go`
- **Code Location:** `~/k8s-mcp-agent`

---

## FIRST TASK: Create Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-mcp-agent
git checkout main && git pull
git checkout -b feat/m2-inventory-enhancement
```

---

## OBJECTIVE

Enhance `get_gpu_inventory` to include the new NVML data fields added in the interface extension. This provides a complete hardware specification view for each GPU.

### Current Output (Basic)

```json
{
  "status": "success",
  "device_count": 1,
  "devices": [{
    "index": 0,
    "name": "Tesla T4",
    "uuid": "GPU-d129fc5b-...",
    "bus_id": "0000:00:1E.0",
    "memory_total": 16106127360,
    "memory_used": 469041152,
    "memory_free": 15637086208,
    "temperature": 28,
    "power_usage": 13837,
    "gpu_util": 0,
    "memory_util": 0
  }]
}
```

### Target Output (Enhanced)

```json
{
  "status": "success",
  "device_count": 1,
  "devices": [{
    "index": 0,
    "name": "Tesla T4",
    "uuid": "GPU-d129fc5b-...",
    "bus_id": "0000:00:1E.0",
    "memory": {
      "total_bytes": 16106127360,
      "used_bytes": 469041152,
      "free_bytes": 15637086208
    },
    "temperature": {
      "current_celsius": 28,
      "slowdown_celsius": 93,
      "shutdown_celsius": 96
    },
    "power": {
      "current_mw": 13837,
      "limit_mw": 70000
    },
    "clocks": {
      "sm_mhz": 300,
      "memory_mhz": 405
    },
    "utilization": {
      "gpu_percent": 0,
      "memory_percent": 0
    },
    "ecc": {
      "enabled": true,
      "correctable_errors": 0,
      "uncorrectable_errors": 0
    }
  }]
}
```

---

## IMPLEMENTATION TASKS

### Task 1: Extend GPUInfo Struct

Update `pkg/nvml/interface.go` to add new fields to `GPUInfo`:

```go
// GPUInfo is a consolidated view of GPU device information.
type GPUInfo struct {
    Index       int    `json:"index"`
    Name        string `json:"name"`
    UUID        string `json:"uuid"`
    BusID       string `json:"bus_id"`
    
    // Memory information
    Memory MemorySpec `json:"memory"`
    
    // Temperature with thresholds
    Temperature TempSpec `json:"temperature"`
    
    // Power with limits
    Power PowerSpec `json:"power"`
    
    // Clock frequencies
    Clocks ClockSpec `json:"clocks"`
    
    // Utilization rates
    Utilization UtilSpec `json:"utilization"`
    
    // ECC status (nil if not supported)
    ECC *ECCSpec `json:"ecc,omitempty"`
}

// MemorySpec contains memory capacity information.
type MemorySpec struct {
    TotalBytes uint64 `json:"total_bytes"`
    UsedBytes  uint64 `json:"used_bytes"`
    FreeBytes  uint64 `json:"free_bytes"`
}

// TempSpec contains temperature with thresholds.
type TempSpec struct {
    CurrentCelsius  uint32 `json:"current_celsius"`
    SlowdownCelsius uint32 `json:"slowdown_celsius"`
    ShutdownCelsius uint32 `json:"shutdown_celsius"`
}

// PowerSpec contains power usage and limits.
type PowerSpec struct {
    CurrentMW uint32 `json:"current_mw"`
    LimitMW   uint32 `json:"limit_mw"`
}

// ClockSpec contains clock frequencies.
type ClockSpec struct {
    SMMHZ     uint32 `json:"sm_mhz"`
    MemoryMHZ uint32 `json:"memory_mhz"`
}

// UtilSpec contains utilization rates.
type UtilSpec struct {
    GPUPercent    uint32 `json:"gpu_percent"`
    MemoryPercent uint32 `json:"memory_percent"`
}

// ECCSpec contains ECC memory status.
type ECCSpec struct {
    Enabled             bool   `json:"enabled"`
    CorrectableErrors   uint64 `json:"correctable_errors"`
    UncorrectableErrors uint64 `json:"uncorrectable_errors"`
}
```

**Note:** Keep the old flat fields for backward compatibility but mark as deprecated, or make a clean break since this is pre-1.0.

---

### Task 2: Update collectDeviceInfo

Update `pkg/tools/gpu_inventory.go` to use new NVML methods:

```go
// collectDeviceInfo gathers all information for a single device.
func (h *GPUInventoryHandler) collectDeviceInfo(
    ctx context.Context,
    index int,
    device nvml.Device,
) (*nvml.GPUInfo, error) {
    info := &nvml.GPUInfo{
        Index: index,
    }

    // Get name
    name, err := device.GetName(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get name: %w", err)
    }
    info.Name = name

    // Get UUID
    uuid, err := device.GetUUID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get UUID: %w", err)
    }
    info.UUID = uuid

    // Get PCI info
    pciInfo, err := device.GetPCIInfo(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get PCI info: %w", err)
    }
    info.BusID = pciInfo.BusID

    // Collect memory info
    if memInfo, err := device.GetMemoryInfo(ctx); err != nil {
        log.Printf(`{"level":"warn","msg":"failed to get memory info",`+
            `"index":%d,"error":"%s"}`, index, err)
    } else {
        info.Memory = nvml.MemorySpec{
            TotalBytes: memInfo.Total,
            UsedBytes:  memInfo.Used,
            FreeBytes:  memInfo.Free,
        }
    }

    // Collect temperature with thresholds
    if temp, err := device.GetTemperature(ctx); err != nil {
        log.Printf(`{"level":"warn","msg":"failed to get temperature",`+
            `"index":%d,"error":"%s"}`, index, err)
    } else {
        info.Temperature.CurrentCelsius = temp
    }
    
    if slowdown, err := device.GetTemperatureThreshold(ctx, nvml.TempThresholdSlowdown); err == nil {
        info.Temperature.SlowdownCelsius = slowdown
    }
    if shutdown, err := device.GetTemperatureThreshold(ctx, nvml.TempThresholdShutdown); err == nil {
        info.Temperature.ShutdownCelsius = shutdown
    }

    // Collect power with limit
    if power, err := device.GetPowerUsage(ctx); err != nil {
        log.Printf(`{"level":"warn","msg":"failed to get power usage",`+
            `"index":%d,"error":"%s"}`, index, err)
    } else {
        info.Power.CurrentMW = power
    }
    
    if limit, err := device.GetPowerManagementLimit(ctx); err == nil {
        info.Power.LimitMW = limit
    }

    // Collect clock frequencies
    if smClock, err := device.GetClockInfo(ctx, nvml.ClockGraphics); err == nil {
        info.Clocks.SMMHZ = smClock
    }
    if memClock, err := device.GetClockInfo(ctx, nvml.ClockMemory); err == nil {
        info.Clocks.MemoryMHZ = memClock
    }

    // Collect utilization
    if util, err := device.GetUtilizationRates(ctx); err != nil {
        log.Printf(`{"level":"warn","msg":"failed to get utilization",`+
            `"index":%d,"error":"%s"}`, index, err)
    } else {
        info.Utilization.GPUPercent = util.GPU
        info.Utilization.MemoryPercent = util.Memory
    }

    // Collect ECC status (optional - may not be supported)
    if enabled, _, err := device.GetEccMode(ctx); err == nil {
        eccSpec := &nvml.ECCSpec{Enabled: enabled}
        if enabled {
            if correctable, err := device.GetTotalEccErrors(ctx, nvml.EccErrorCorrectable); err == nil {
                eccSpec.CorrectableErrors = correctable
            }
            if uncorrectable, err := device.GetTotalEccErrors(ctx, nvml.EccErrorUncorrectable); err == nil {
                eccSpec.UncorrectableErrors = uncorrectable
            }
        }
        info.ECC = eccSpec
    }

    return info, nil
}
```

---

### Task 3: Update Mock Device

Ensure `pkg/nvml/mock.go` returns consistent data for inventory tests.

The mock already has all required fields from the NVML interface extension. Verify the `MockDevice` returns sensible values for inventory display.

---

### Task 4: Update Unit Tests

Update `pkg/tools/gpu_inventory_test.go`:

```go
func TestGPUInventoryHandler_Handle(t *testing.T) {
    mock := nvml.NewMock(2)
    handler := NewGPUInventoryHandler(mock)
    
    result, err := handler.Handle(context.Background(), mcp.CallToolRequest{})
    require.NoError(t, err)
    require.NotNil(t, result)
    
    // Parse response
    var response struct {
        Status      string          `json:"status"`
        DeviceCount int             `json:"device_count"`
        Devices     []nvml.GPUInfo  `json:"devices"`
    }
    err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &response)
    require.NoError(t, err)
    
    assert.Equal(t, "success", response.Status)
    assert.Equal(t, 2, response.DeviceCount)
    assert.Len(t, response.Devices, 2)
    
    // Verify enhanced fields
    gpu := response.Devices[0]
    assert.Greater(t, gpu.Temperature.SlowdownCelsius, uint32(0))
    assert.Greater(t, gpu.Temperature.ShutdownCelsius, uint32(0))
    assert.Greater(t, gpu.Power.LimitMW, uint32(0))
    assert.Greater(t, gpu.Clocks.SMMHZ, uint32(0))
    assert.NotNil(t, gpu.ECC)
    assert.True(t, gpu.ECC.Enabled)
}

func TestGPUInventoryHandler_NestedStructures(t *testing.T) {
    mock := nvml.NewMock(1)
    handler := NewGPUInventoryHandler(mock)
    
    result, err := handler.Handle(context.Background(), mcp.CallToolRequest{})
    require.NoError(t, err)
    
    // Verify JSON structure
    responseText := result.Content[0].(mcp.TextContent).Text
    
    // Should have nested objects
    assert.Contains(t, responseText, `"memory":`)
    assert.Contains(t, responseText, `"total_bytes":`)
    assert.Contains(t, responseText, `"temperature":`)
    assert.Contains(t, responseText, `"slowdown_celsius":`)
    assert.Contains(t, responseText, `"power":`)
    assert.Contains(t, responseText, `"limit_mw":`)
    assert.Contains(t, responseText, `"clocks":`)
    assert.Contains(t, responseText, `"sm_mhz":`)
    assert.Contains(t, responseText, `"ecc":`)
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
(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 0.5; echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}}}'; sleep 0.5) | ./bin/agent --nvml-mode=mock 2>/dev/null
```

### Remote Testing (Real Tesla T4)

```bash
ssh -i /Users/eduardoa/.ssh/cnt-ci.pem ubuntu@ec2-54-176-252-175.us-west-1.compute.amazonaws.com

cd ~/k8s-mcp-agent
git fetch origin
git checkout feat/m2-inventory-enhancement

export PATH=/usr/local/go/bin:$PATH
go build -o bin/agent ./cmd/agent

# Test with real NVML
(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 0.5; echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}}}'; sleep 0.5) | ./bin/agent --nvml-mode=real 2>/dev/null
```

Expected Tesla T4 output:
```json
{
  "temperature": {"current_celsius": 28, "slowdown_celsius": 93, "shutdown_celsius": 96},
  "power": {"current_mw": 13837, "limit_mw": 70000},
  "clocks": {"sm_mhz": 300, "memory_mhz": 405},
  "ecc": {"enabled": true, "correctable_errors": 0, "uncorrectable_errors": 0}
}
```

---

## ACCEPTANCE CRITERIA

**Must Have:**
- [ ] GPUInfo struct updated with nested structures
- [ ] New spec types added (MemorySpec, TempSpec, PowerSpec, ClockSpec, UtilSpec, ECCSpec)
- [ ] collectDeviceInfo uses all 6 new NVML methods
- [ ] Graceful fallback when methods return errors
- [ ] Unit tests updated and passing
- [ ] `make all` passes

**Should Have:**
- [ ] Integration test on Tesla T4
- [ ] JSON output uses snake_case consistently
- [ ] ECC field omitted when not supported (omitempty)

**Nice to Have:**
- [ ] Add `driver_version` field
- [ ] Add `cuda_version` field
- [ ] Add `compute_capability` field

---

## FILE PLAN

| Path | Purpose | Acceptance |
|------|---------|------------|
| `pkg/nvml/interface.go` | Add 6 new spec types + update GPUInfo | Compiles |
| `pkg/tools/gpu_inventory.go` | Use new NVML methods | Works with mock |
| `pkg/tools/gpu_inventory_test.go` | Test enhanced output | All pass |

---

## IMPLEMENTATION ORDER

1. **Create branch:** `feat/m2-inventory-enhancement`
2. **Update interface.go:** Add spec types, update GPUInfo struct
3. **Update gpu_inventory.go:** Use new NVML methods in collectDeviceInfo
4. **Update gpu_inventory_test.go:** Test new nested structures
5. **Run make all:** Verify everything compiles and tests pass
6. **Test locally:** Mock mode working with new output format
7. **Test on T4:** Real NVML working
8. **Create PR:** With proper labels

---

## BACKWARD COMPATIBILITY NOTE

This change modifies the JSON output structure of `get_gpu_inventory`. Since the project is pre-1.0, breaking changes are acceptable. However, consider:

1. **Option A (Clean Break):** Replace flat fields with nested structures
2. **Option B (Additive):** Keep old flat fields, add new nested structures alongside

Recommendation: **Option A** - Clean break for cleaner API design.

---

## QUICK REFERENCE

### Key Files
- **Interface:** `pkg/nvml/interface.go`
- **Inventory:** `pkg/tools/gpu_inventory.go`
- **Tests:** `pkg/tools/gpu_inventory_test.go`

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
cd ~/k8s-mcp-agent
export PATH=/usr/local/go/bin:$PATH
```

---

**Reply "GO" when ready to start implementation.** 🚀

