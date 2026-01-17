// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import "errors"

// Sentinel errors for NVML operations.
// Use errors.Is() to check for these error types.
var (
	// ErrNotInitialized indicates NVML library was not initialized.
	// Call Init() before other operations.
	ErrNotInitialized = errors.New("NVML not initialized")

	// ErrNotSupported indicates the operation is not supported on this GPU.
	// This is not a failure - the feature is simply unavailable.
	// Reserved for future use: tool handlers may use this for graceful
	// degradation when optional features are unavailable.
	ErrNotSupported = errors.New("operation not supported")

	// ErrNotImplemented indicates a method is not implemented.
	// Used by UnimplementedInterface and UnimplementedDevice for forward
	// compatibility when new methods are added to interfaces.
	ErrNotImplemented = errors.New("method not implemented")

	// ErrInvalidDevice indicates an invalid device index was provided.
	ErrInvalidDevice = errors.New("invalid device index")

	// ErrCGORequired indicates CGO is required but the binary was built
	// without CGO support. Rebuild with CGO_ENABLED=1.
	ErrCGORequired = errors.New("real NVML requires CGO")

	// ErrContextCancelled indicates the operation was cancelled via context.
	ErrContextCancelled = errors.New("context cancelled")
)
