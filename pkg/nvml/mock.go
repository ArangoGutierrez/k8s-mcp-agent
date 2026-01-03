// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import (
	"context"
	"fmt"
)

// Mock is a mock implementation of the NVML Interface for testing.
// It returns fake but consistent GPU data without requiring real hardware.
type Mock struct {
	deviceCount int
	devices     []*MockDevice
}

// NewMock creates a new mock NVML implementation with the specified
// number of fake GPU devices.
func NewMock(deviceCount int) *Mock {
	if deviceCount <= 0 {
		deviceCount = 2 // Default to 2 fake GPUs
	}

	m := &Mock{
		deviceCount: deviceCount,
		devices:     make([]*MockDevice, deviceCount),
	}

	// Create fake devices
	for i := 0; i < deviceCount; i++ {
		m.devices[i] = &MockDevice{
			index:       i,
			name:        fmt.Sprintf("NVIDIA A100-SXM4-40GB (Mock %d)", i),
			uuid:        fmt.Sprintf("GPU-%08d-0000-0000-0000-%012d", i, i),
			busID:       fmt.Sprintf("0000:%02x:00.0", i+1),
			domain:      0,
			bus:         uint32(i + 1),
			device:      0,
			memoryTotal: 42949672960, // 40 GB
			memoryUsed:  8589934592,  // 8 GB
			temperature: 45 + uint32(i*5),
			powerUsage:  150000 + uint32(i*10000), // milliwatts
			gpuUtil:     30 + uint32(i*10),
			memoryUtil:  20 + uint32(i*5),
		}
	}

	return m
}

// Init initializes the mock NVML library (no-op).
func (m *Mock) Init(ctx context.Context) error {
	return nil
}

// Shutdown shuts down the mock NVML library (no-op).
func (m *Mock) Shutdown(ctx context.Context) error {
	return nil
}

// GetDeviceCount returns the number of mock GPU devices.
func (m *Mock) GetDeviceCount(ctx context.Context) (int, error) {
	return m.deviceCount, nil
}

// GetDeviceByIndex returns a mock Device handle for the given index.
func (m *Mock) GetDeviceByIndex(ctx context.Context, idx int) (Device, error) {
	if idx < 0 || idx >= m.deviceCount {
		return nil, fmt.Errorf("invalid device index %d (count: %d)",
			idx, m.deviceCount)
	}
	return m.devices[idx], nil
}

// MockDevice is a mock implementation of the Device interface.
type MockDevice struct {
	index       int
	name        string
	uuid        string
	busID       string
	domain      uint32
	bus         uint32
	device      uint32
	memoryTotal uint64
	memoryUsed  uint64
	temperature uint32
	powerUsage  uint32
	gpuUtil     uint32
	memoryUtil  uint32
}

// GetName returns the mock device name.
func (d *MockDevice) GetName(ctx context.Context) (string, error) {
	return d.name, nil
}

// GetUUID returns the mock device UUID.
func (d *MockDevice) GetUUID(ctx context.Context) (string, error) {
	return d.uuid, nil
}

// GetPCIInfo returns mock PCI information.
func (d *MockDevice) GetPCIInfo(ctx context.Context) (*PCIInfo, error) {
	return &PCIInfo{
		BusID:  d.busID,
		Domain: d.domain,
		Bus:    d.bus,
		Device: d.device,
	}, nil
}

// GetMemoryInfo returns mock memory usage information.
func (d *MockDevice) GetMemoryInfo(ctx context.Context) (*MemoryInfo, error) {
	return &MemoryInfo{
		Total: d.memoryTotal,
		Used:  d.memoryUsed,
		Free:  d.memoryTotal - d.memoryUsed,
	}, nil
}

// GetTemperature returns the mock temperature.
func (d *MockDevice) GetTemperature(ctx context.Context) (uint32, error) {
	return d.temperature, nil
}

// GetPowerUsage returns the mock power usage.
func (d *MockDevice) GetPowerUsage(ctx context.Context) (uint32, error) {
	return d.powerUsage, nil
}

// GetUtilizationRates returns mock utilization rates.
func (d *MockDevice) GetUtilizationRates(
	ctx context.Context,
) (*Utilization, error) {
	return &Utilization{
		GPU:    d.gpuUtil,
		Memory: d.memoryUtil,
	}, nil
}
