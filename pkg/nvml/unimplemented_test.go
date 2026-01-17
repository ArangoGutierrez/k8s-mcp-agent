// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import (
	"context"
	"errors"
	"testing"
)

func TestUnimplementedInterface_ReturnsErrNotImplemented(t *testing.T) {
	var iface Interface = UnimplementedInterface{}
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Init", func() error { return iface.Init(ctx) }},
		{"Shutdown", func() error { return iface.Shutdown(ctx) }},
		{"GetDeviceCount", func() error { _, err := iface.GetDeviceCount(ctx); return err }},
		{"GetDeviceByIndex", func() error { _, err := iface.GetDeviceByIndex(ctx, 0); return err }},
		{"GetDriverVersion", func() error { _, err := iface.GetDriverVersion(ctx); return err }},
		{"GetCudaDriverVersion", func() error { _, err := iface.GetCudaDriverVersion(ctx); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, ErrNotImplemented) {
				t.Errorf("%s: expected ErrNotImplemented, got %v", tt.name, err)
			}
		})
	}
}

func TestUnimplementedDevice_ReturnsErrNotImplemented(t *testing.T) {
	var dev Device = UnimplementedDevice{}
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"GetName", func() error { _, err := dev.GetName(ctx); return err }},
		{"GetUUID", func() error { _, err := dev.GetUUID(ctx); return err }},
		{"GetPCIInfo", func() error { _, err := dev.GetPCIInfo(ctx); return err }},
		{"GetMemoryInfo", func() error { _, err := dev.GetMemoryInfo(ctx); return err }},
		{"GetTemperature", func() error { _, err := dev.GetTemperature(ctx); return err }},
		{"GetPowerUsage", func() error { _, err := dev.GetPowerUsage(ctx); return err }},
		{"GetUtilizationRates", func() error { _, err := dev.GetUtilizationRates(ctx); return err }},
		{"GetPowerManagementLimit", func() error { _, err := dev.GetPowerManagementLimit(ctx); return err }},
		{"GetEccMode", func() error { _, _, err := dev.GetEccMode(ctx); return err }},
		{"GetTotalEccErrors", func() error { _, err := dev.GetTotalEccErrors(ctx, 0); return err }},
		{"GetCurrentClocksThrottleReasons", func() error { _, err := dev.GetCurrentClocksThrottleReasons(ctx); return err }},
		{"GetClockInfo", func() error { _, err := dev.GetClockInfo(ctx, 0); return err }},
		{"GetTemperatureThreshold", func() error { _, err := dev.GetTemperatureThreshold(ctx, 0); return err }},
		{"GetCudaComputeCapability", func() error { _, err := dev.GetCudaComputeCapability(ctx); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, ErrNotImplemented) {
				t.Errorf("%s: expected ErrNotImplemented, got %v", tt.name, err)
			}
		})
	}
}

// TestForwardCompatibility verifies that embedding UnimplementedInterface
// allows new methods to be added without breaking existing implementations.
func TestForwardCompatibility(t *testing.T) {
	// This test demonstrates the forward compatibility pattern.
	// If a new method were added to Interface, implementations that embed
	// UnimplementedInterface would still compile and return ErrNotImplemented
	// for the new method.

	// Verify Mock embeds UnimplementedInterface (compiles = passes)
	var _ Interface = &Mock{}

	// Verify MockDevice embeds UnimplementedDevice (compiles = passes)
	var _ Device = &MockDevice{}

	// The embedded types don't interfere with existing implementations
	m := NewMock(1)
	ctx := context.Background()

	// Existing methods work as expected
	if err := m.Init(ctx); err != nil {
		t.Errorf("Mock.Init failed: %v", err)
	}

	count, err := m.GetDeviceCount(ctx)
	if err != nil || count != 1 {
		t.Errorf("Mock.GetDeviceCount failed: count=%d, err=%v", count, err)
	}

	dev, err := m.GetDeviceByIndex(ctx, 0)
	if err != nil {
		t.Errorf("Mock.GetDeviceByIndex failed: %v", err)
	}

	name, err := dev.GetName(ctx)
	if err != nil || name == "" {
		t.Errorf("MockDevice.GetName failed: name=%q, err=%v", name, err)
	}
}
