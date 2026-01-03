// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

package nvml

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMock(t *testing.T) {
	tests := []struct {
		name        string
		deviceCount int
		expected    int
	}{
		{
			name:        "default device count",
			deviceCount: 0,
			expected:    2,
		},
		{
			name:        "negative device count",
			deviceCount: -1,
			expected:    2,
		},
		{
			name:        "custom device count",
			deviceCount: 4,
			expected:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMock(tt.deviceCount)
			require.NotNil(t, mock)
			assert.Equal(t, tt.expected, mock.deviceCount)
			assert.Len(t, mock.devices, tt.expected)
		})
	}
}

func TestMockInit(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	err := mock.Init(ctx)
	assert.NoError(t, err)
}

func TestMockShutdown(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	err := mock.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMockGetDeviceCount(t *testing.T) {
	mock := NewMock(3)
	ctx := context.Background()

	count, err := mock.GetDeviceCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestMockGetDeviceByIndex(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	tests := []struct {
		name      string
		index     int
		expectErr bool
	}{
		{
			name:      "valid index 0",
			index:     0,
			expectErr: false,
		},
		{
			name:      "valid index 1",
			index:     1,
			expectErr: false,
		},
		{
			name:      "invalid negative index",
			index:     -1,
			expectErr: true,
		},
		{
			name:      "invalid out of bounds index",
			index:     2,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, err := mock.GetDeviceByIndex(ctx, tt.index)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, device)
			} else {
				require.NoError(t, err)
				require.NotNil(t, device)
			}
		})
	}
}

func TestMockDeviceGetName(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	name, err := device.GetName(ctx)
	require.NoError(t, err)
	assert.Contains(t, name, "NVIDIA A100")
	assert.Contains(t, name, "Mock")
}

func TestMockDeviceGetUUID(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	uuid, err := device.GetUUID(ctx)
	require.NoError(t, err)
	assert.Contains(t, uuid, "GPU-")
}

func TestMockDeviceGetPCIInfo(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	pciInfo, err := device.GetPCIInfo(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, pciInfo.BusID)
	assert.Contains(t, pciInfo.BusID, "0000:")
}

func TestMockDeviceGetMemoryInfo(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	memInfo, err := device.GetMemoryInfo(ctx)
	require.NoError(t, err)
	assert.Greater(t, memInfo.Total, uint64(0))
	assert.Greater(t, memInfo.Used, uint64(0))
	assert.Equal(t, memInfo.Total-memInfo.Used, memInfo.Free)
}

func TestMockDeviceGetTemperature(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	temp, err := device.GetTemperature(ctx)
	require.NoError(t, err)
	assert.Greater(t, temp, uint32(0))
	assert.Less(t, temp, uint32(100))
}

func TestMockDeviceGetPowerUsage(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	power, err := device.GetPowerUsage(ctx)
	require.NoError(t, err)
	assert.Greater(t, power, uint32(0))
}

func TestMockDeviceGetUtilizationRates(t *testing.T) {
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	util, err := device.GetUtilizationRates(ctx)
	require.NoError(t, err)
	assert.LessOrEqual(t, util.GPU, uint32(100))
	assert.LessOrEqual(t, util.Memory, uint32(100))
}

func TestMockDeviceConsistency(t *testing.T) {
	// Test that mock devices return consistent data across calls
	mock := NewMock(2)
	ctx := context.Background()

	device, err := mock.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	// Get UUID twice
	uuid1, err := device.GetUUID(ctx)
	require.NoError(t, err)

	uuid2, err := device.GetUUID(ctx)
	require.NoError(t, err)

	// Should be the same
	assert.Equal(t, uuid1, uuid2)
}
