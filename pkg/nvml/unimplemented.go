// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import (
	"context"
)

// Compile-time interface satisfaction checks.
var (
	_ Interface = UnimplementedInterface{}
	_ Device    = UnimplementedDevice{}
)

// UnimplementedInterface provides default implementations that return
// ErrNotImplemented for all Interface methods. Embed this in your
// implementation for forward compatibility when new methods are added.
//
// Example:
//
//	type MyNVML struct {
//	    nvml.UnimplementedInterface
//	    // your fields
//	}
type UnimplementedInterface struct{}

// Init returns ErrNotImplemented.
func (UnimplementedInterface) Init(ctx context.Context) error {
	return ErrNotImplemented
}

// Shutdown returns ErrNotImplemented.
func (UnimplementedInterface) Shutdown(ctx context.Context) error {
	return ErrNotImplemented
}

// GetDeviceCount returns ErrNotImplemented.
func (UnimplementedInterface) GetDeviceCount(ctx context.Context) (int, error) {
	return 0, ErrNotImplemented
}

// GetDeviceByIndex returns ErrNotImplemented.
func (UnimplementedInterface) GetDeviceByIndex(
	ctx context.Context,
	idx int,
) (Device, error) {
	return nil, ErrNotImplemented
}

// GetDriverVersion returns ErrNotImplemented.
func (UnimplementedInterface) GetDriverVersion(
	ctx context.Context,
) (string, error) {
	return "", ErrNotImplemented
}

// GetCudaDriverVersion returns ErrNotImplemented.
func (UnimplementedInterface) GetCudaDriverVersion(
	ctx context.Context,
) (string, error) {
	return "", ErrNotImplemented
}

// UnimplementedDevice provides default implementations that return
// ErrNotImplemented for all Device methods. Embed this in your
// implementation for forward compatibility when new methods are added.
//
// Example:
//
//	type MyDevice struct {
//	    nvml.UnimplementedDevice
//	    // your fields
//	}
type UnimplementedDevice struct{}

// GetName returns ErrNotImplemented.
func (UnimplementedDevice) GetName(ctx context.Context) (string, error) {
	return "", ErrNotImplemented
}

// GetUUID returns ErrNotImplemented.
func (UnimplementedDevice) GetUUID(ctx context.Context) (string, error) {
	return "", ErrNotImplemented
}

// GetPCIInfo returns ErrNotImplemented.
func (UnimplementedDevice) GetPCIInfo(ctx context.Context) (*PCIInfo, error) {
	return nil, ErrNotImplemented
}

// GetMemoryInfo returns ErrNotImplemented.
func (UnimplementedDevice) GetMemoryInfo(
	ctx context.Context,
) (*MemoryInfo, error) {
	return nil, ErrNotImplemented
}

// GetTemperature returns ErrNotImplemented.
func (UnimplementedDevice) GetTemperature(ctx context.Context) (uint32, error) {
	return 0, ErrNotImplemented
}

// GetPowerUsage returns ErrNotImplemented.
func (UnimplementedDevice) GetPowerUsage(ctx context.Context) (uint32, error) {
	return 0, ErrNotImplemented
}

// GetUtilizationRates returns ErrNotImplemented.
func (UnimplementedDevice) GetUtilizationRates(
	ctx context.Context,
) (*Utilization, error) {
	return nil, ErrNotImplemented
}

// GetPowerManagementLimit returns ErrNotImplemented.
func (UnimplementedDevice) GetPowerManagementLimit(
	ctx context.Context,
) (uint32, error) {
	return 0, ErrNotImplemented
}

// GetEccMode returns ErrNotImplemented.
func (UnimplementedDevice) GetEccMode(
	ctx context.Context,
) (current, pending bool, err error) {
	return false, false, ErrNotImplemented
}

// GetTotalEccErrors returns ErrNotImplemented.
func (UnimplementedDevice) GetTotalEccErrors(
	ctx context.Context,
	errorType int,
) (uint64, error) {
	return 0, ErrNotImplemented
}

// GetCurrentClocksThrottleReasons returns ErrNotImplemented.
func (UnimplementedDevice) GetCurrentClocksThrottleReasons(
	ctx context.Context,
) (uint64, error) {
	return 0, ErrNotImplemented
}

// GetClockInfo returns ErrNotImplemented.
func (UnimplementedDevice) GetClockInfo(
	ctx context.Context,
	clockType int,
) (uint32, error) {
	return 0, ErrNotImplemented
}

// GetTemperatureThreshold returns ErrNotImplemented.
func (UnimplementedDevice) GetTemperatureThreshold(
	ctx context.Context,
	thresholdType int,
) (uint32, error) {
	return 0, ErrNotImplemented
}

// GetCudaComputeCapability returns ErrNotImplemented.
func (UnimplementedDevice) GetCudaComputeCapability(
	ctx context.Context,
) (string, error) {
	return "", ErrNotImplemented
}
