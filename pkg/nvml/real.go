// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo
// +build cgo

package nvml

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// Real is a real implementation of the NVML Interface using go-nvml.
// This requires the NVIDIA driver and libnvidia-ml.so to be available.
type Real struct {
	initialized bool
}

// NewReal creates a new real NVML implementation.
func NewReal() *Real {
	return &Real{
		initialized: false,
	}
}

// Init initializes the NVML library.
func (r *Real) Init(ctx context.Context) error {
	if r.initialized {
		return nil
	}

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %s", nvml.ErrorString(ret))
	}

	r.initialized = true
	return nil
}

// Shutdown shuts down the NVML library.
func (r *Real) Shutdown(ctx context.Context) error {
	if !r.initialized {
		return nil
	}

	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to shutdown NVML: %s", nvml.ErrorString(ret))
	}

	r.initialized = false
	return nil
}

// GetDeviceCount returns the number of GPU devices.
func (r *Real) GetDeviceCount(ctx context.Context) (int, error) {
	if !r.initialized {
		return 0, fmt.Errorf("NVML not initialized")
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get device count: %s",
			nvml.ErrorString(ret))
	}

	return count, nil
}

// GetDeviceByIndex returns a Device handle for the given index.
func (r *Real) GetDeviceByIndex(ctx context.Context, idx int) (Device, error) {
	if !r.initialized {
		return nil, fmt.Errorf("NVML not initialized")
	}

	device, ret := nvml.DeviceGetHandleByIndex(idx)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device %d: %s", idx,
			nvml.ErrorString(ret))
	}

	return &RealDevice{device: device}, nil
}

// RealDevice is a real implementation of the Device interface.
type RealDevice struct {
	device nvml.Device
}

// GetName returns the product name of the device.
func (d *RealDevice) GetName(ctx context.Context) (string, error) {
	name, ret := d.device.GetName()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to get device name: %s",
			nvml.ErrorString(ret))
	}
	return name, nil
}

// GetUUID returns the globally unique identifier of the device.
func (d *RealDevice) GetUUID(ctx context.Context) (string, error) {
	uuid, ret := d.device.GetUUID()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to get device UUID: %s",
			nvml.ErrorString(ret))
	}
	return uuid, nil
}

// GetPCIInfo returns PCI bus information for the device.
func (d *RealDevice) GetPCIInfo(ctx context.Context) (*PCIInfo, error) {
	pciInfo, ret := d.device.GetPciInfo()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get PCI info: %s",
			nvml.ErrorString(ret))
	}

	// Convert BusIdLegacy byte array to string
	busID := string(pciInfo.BusIdLegacy[:])
	// Trim null bytes
	for i, b := range pciInfo.BusIdLegacy {
		if b == 0 {
			busID = string(pciInfo.BusIdLegacy[:i])
			break
		}
	}

	return &PCIInfo{
		BusID:  busID,
		Domain: pciInfo.Domain,
		Bus:    pciInfo.Bus,
		Device: pciInfo.Device,
	}, nil
}

// GetMemoryInfo returns memory usage information.
func (d *RealDevice) GetMemoryInfo(ctx context.Context) (*MemoryInfo, error) {
	memInfo, ret := d.device.GetMemoryInfo()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get memory info: %s",
			nvml.ErrorString(ret))
	}

	return &MemoryInfo{
		Total: memInfo.Total,
		Used:  memInfo.Used,
		Free:  memInfo.Free,
	}, nil
}

// GetTemperature returns the current temperature in Celsius.
func (d *RealDevice) GetTemperature(ctx context.Context) (uint32, error) {
	temp, ret := d.device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get temperature: %s",
			nvml.ErrorString(ret))
	}
	return temp, nil
}

// GetPowerUsage returns the current power usage in milliwatts.
func (d *RealDevice) GetPowerUsage(ctx context.Context) (uint32, error) {
	power, ret := d.device.GetPowerUsage()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get power usage: %s",
			nvml.ErrorString(ret))
	}
	return power, nil
}

// GetUtilizationRates returns GPU and memory utilization rates.
func (d *RealDevice) GetUtilizationRates(
	ctx context.Context,
) (*Utilization, error) {
	util, ret := d.device.GetUtilizationRates()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get utilization rates: %s",
			nvml.ErrorString(ret))
	}

	return &Utilization{
		GPU:    util.Gpu,
		Memory: util.Memory,
	}, nil
}
