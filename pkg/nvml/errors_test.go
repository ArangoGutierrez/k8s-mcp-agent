// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestErrInvalidDevice(t *testing.T) {
	m := NewMock(2)

	_, err := m.GetDeviceByIndex(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for invalid device index")
	}
	if !errors.Is(err, ErrInvalidDevice) {
		t.Errorf("expected ErrInvalidDevice, got %v", err)
	}

	// Verify error message contains the device index
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("error should contain device index, got %v", err)
	}
}

func TestErrInvalidDevice_NegativeIndex(t *testing.T) {
	m := NewMock(2)

	_, err := m.GetDeviceByIndex(context.Background(), -1)
	if err == nil {
		t.Fatal("expected error for negative device index")
	}
	if !errors.Is(err, ErrInvalidDevice) {
		t.Errorf("expected ErrInvalidDevice, got %v", err)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Verify sentinel errors are not accidentally equal
	sentinels := []error{
		ErrNotInitialized,
		ErrNotSupported,
		ErrNotImplemented,
		ErrInvalidDevice,
		ErrCGORequired,
		ErrContextCancelled,
	}

	for i, err1 := range sentinels {
		for j, err2 := range sentinels {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("sentinel errors should be distinct: %v == %v", err1, err2)
			}
		}
	}
}

func TestSentinelErrors_HaveMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"ErrNotInitialized", ErrNotInitialized, "not initialized"},
		{"ErrNotSupported", ErrNotSupported, "not supported"},
		{"ErrNotImplemented", ErrNotImplemented, "not implemented"},
		{"ErrInvalidDevice", ErrInvalidDevice, "invalid device"},
		{"ErrCGORequired", ErrCGORequired, "CGO"},
		{"ErrContextCancelled", ErrContextCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if msg == "" {
				t.Error("error message should not be empty")
			}
			if !strings.Contains(msg, tt.contains) {
				t.Errorf("error message %q should contain %q", msg, tt.contains)
			}
		})
	}
}
