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

	// ErrNotSupported indicates the operation is not supported by the
	// hardware or driver. The GPU physically cannot perform this operation.
	// Example: Querying ECC status on a consumer GPU that lacks ECC memory.
	//
	// Callers should handle this gracefully - the feature is unavailable
	// but the GPU is otherwise functional. This error typically comes from
	// the NVML library itself (nvml.ERROR_NOT_SUPPORTED).
	ErrNotSupported = errors.New("operation not supported")

	// ErrNotImplemented indicates a method exists in the interface but has
	// no implementation in the current code. This is distinct from
	// ErrNotSupported:
	//
	//   - ErrNotSupported: Hardware/driver limitation (GPU can't do this)
	//   - ErrNotImplemented: Code limitation (we haven't written this yet)
	//
	// Used by UnimplementedInterface and UnimplementedDevice to enable
	// forward compatibility. When new methods are added to Interface or
	// Device, existing implementations that embed Unimplemented* types
	// continue to compile and return ErrNotImplemented for new methods.
	//
	// Callers can use errors.Is(err, ErrNotImplemented) to detect when a
	// feature requires a newer version of the library.
	ErrNotImplemented = errors.New("method not implemented")

	// ErrInvalidDevice indicates an invalid device index was provided.
	ErrInvalidDevice = errors.New("invalid device index")

	// ErrCGORequired indicates CGO is required but the binary was built
	// without CGO support. Rebuild with CGO_ENABLED=1.
	ErrCGORequired = errors.New("real NVML requires CGO")

	// ErrContextCancelled indicates the operation was cancelled via context.
	ErrContextCancelled = errors.New("context cancelled")
)
