// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

//go:build !cgo
// +build !cgo

package nvml

import (
	"context"
	"fmt"
)

// Real is a stub that returns an error when CGO is disabled.
// This allows the code to compile without NVML library.
type Real struct{}

// NewReal creates a stub that will error on init.
func NewReal() *Real {
	return &Real{}
}

// Init returns an error indicating CGO is required.
func (r *Real) Init(ctx context.Context) error {
	return fmt.Errorf("real NVML requires CGO (build with CGO_ENABLED=1)")
}

// Shutdown is a no-op stub.
func (r *Real) Shutdown(ctx context.Context) error {
	return nil
}

// GetDeviceCount returns an error.
func (r *Real) GetDeviceCount(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetDeviceByIndex returns an error.
func (r *Real) GetDeviceByIndex(ctx context.Context, idx int) (Device, error) {
	return nil, fmt.Errorf("real NVML requires CGO")
}

// RealDevice is a stub for non-CGO builds.
type RealDevice struct{}

// GetName returns an error indicating CGO is required.
func (d *RealDevice) GetName(ctx context.Context) (string, error) {
	return "", fmt.Errorf("real NVML requires CGO")
}

// GetUUID returns an error indicating CGO is required.
func (d *RealDevice) GetUUID(ctx context.Context) (string, error) {
	return "", fmt.Errorf("real NVML requires CGO")
}

// GetPCIInfo returns an error indicating CGO is required.
func (d *RealDevice) GetPCIInfo(ctx context.Context) (*PCIInfo, error) {
	return nil, fmt.Errorf("real NVML requires CGO")
}

// GetMemoryInfo returns an error indicating CGO is required.
func (d *RealDevice) GetMemoryInfo(ctx context.Context) (*MemoryInfo, error) {
	return nil, fmt.Errorf("real NVML requires CGO")
}

// GetTemperature returns an error indicating CGO is required.
func (d *RealDevice) GetTemperature(ctx context.Context) (uint32, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetPowerUsage returns an error indicating CGO is required.
func (d *RealDevice) GetPowerUsage(ctx context.Context) (uint32, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetUtilizationRates returns an error indicating CGO is required.
func (d *RealDevice) GetUtilizationRates(
	ctx context.Context,
) (*Utilization, error) {
	return nil, fmt.Errorf("real NVML requires CGO")
}

// GetPowerManagementLimit returns an error indicating CGO is required.
func (d *RealDevice) GetPowerManagementLimit(
	ctx context.Context,
) (uint32, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetEccMode returns an error indicating CGO is required.
func (d *RealDevice) GetEccMode(
	ctx context.Context,
) (current, pending bool, err error) {
	return false, false, fmt.Errorf("real NVML requires CGO")
}

// GetTotalEccErrors returns an error indicating CGO is required.
func (d *RealDevice) GetTotalEccErrors(
	ctx context.Context,
	errorType int,
) (uint64, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetCurrentClocksThrottleReasons returns an error indicating CGO is required.
func (d *RealDevice) GetCurrentClocksThrottleReasons(
	ctx context.Context,
) (uint64, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetClockInfo returns an error indicating CGO is required.
func (d *RealDevice) GetClockInfo(
	ctx context.Context,
	clockType int,
) (uint32, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}

// GetTemperatureThreshold returns an error indicating CGO is required.
func (d *RealDevice) GetTemperatureThreshold(
	ctx context.Context,
	thresholdType int,
) (uint32, error) {
	return 0, fmt.Errorf("real NVML requires CGO")
}
