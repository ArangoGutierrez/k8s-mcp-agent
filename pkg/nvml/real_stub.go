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
