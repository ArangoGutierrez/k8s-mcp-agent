// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

// Package nvml provides an abstraction layer over NVIDIA NVML library.
// This allows for testing without real hardware and decouples the
// application from the CGO-based NVML implementation.
package nvml

import (
	"context"
)

// Interface defines the contract for NVML operations.
// This interface can be implemented by both real NVML bindings
// and mock implementations for testing.
type Interface interface {
	// Init initializes the NVML library.
	// Must be called before any other NVML operations.
	Init(ctx context.Context) error

	// Shutdown shuts down the NVML library.
	// Should be called when done using NVML.
	Shutdown(ctx context.Context) error

	// GetDeviceCount returns the number of GPU devices.
	GetDeviceCount(ctx context.Context) (int, error)

	// GetDeviceByIndex returns a Device handle for the given index.
	GetDeviceByIndex(ctx context.Context, idx int) (Device, error)
}

// Device represents a single GPU device.
type Device interface {
	// GetName returns the product name of the device.
	GetName(ctx context.Context) (string, error)

	// GetUUID returns the globally unique identifier of the device.
	GetUUID(ctx context.Context) (string, error)

	// GetPCIInfo returns PCI bus information for the device.
	GetPCIInfo(ctx context.Context) (*PCIInfo, error)

	// GetMemoryInfo returns memory usage information.
	GetMemoryInfo(ctx context.Context) (*MemoryInfo, error)

	// GetTemperature returns the current temperature in Celsius.
	GetTemperature(ctx context.Context) (uint32, error)

	// GetPowerUsage returns the current power usage in milliwatts.
	GetPowerUsage(ctx context.Context) (uint32, error)

	// GetUtilizationRates returns GPU and memory utilization rates.
	GetUtilizationRates(ctx context.Context) (*Utilization, error)
}

// PCIInfo contains PCI bus information for a device.
type PCIInfo struct {
	// BusID is the PCI bus ID (e.g., "0000:01:00.0")
	BusID string
	// Domain is the PCI domain
	Domain uint32
	// Bus is the PCI bus number
	Bus uint32
	// Device is the PCI device number
	Device uint32
}

// MemoryInfo contains memory usage information.
type MemoryInfo struct {
	// Total memory in bytes
	Total uint64
	// Used memory in bytes
	Used uint64
	// Free memory in bytes
	Free uint64
}

// Utilization contains GPU and memory utilization rates.
type Utilization struct {
	// GPU utilization rate (0-100%)
	GPU uint32
	// Memory utilization rate (0-100%)
	Memory uint32
}

// GPUInfo is a consolidated view of GPU device information.
type GPUInfo struct {
	Index       int
	Name        string
	UUID        string
	BusID       string
	MemoryTotal uint64
	MemoryUsed  uint64
	MemoryFree  uint64
	Temperature uint32
	PowerUsage  uint32
	GPUUtil     uint32
	MemoryUtil  uint32
}
