// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGPUInventoryHandler(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewGPUInventoryHandler(mockClient)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.nvmlClient)
}

func TestGPUInventoryHandler_Handle(t *testing.T) {
	tests := []struct {
		name         string
		deviceCount  int
		expectError  bool
		validateFunc func(*testing.T, *mcp.CallToolResult)
	}{
		{
			name:        "successful inventory with 2 devices",
			deviceCount: 2,
			expectError: false,
			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
				require.NotNil(t, result)
				assert.NotEmpty(t, result.Content)

				// Cast to TextContent and parse JSON response
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok, "content should be TextContent")

				var response map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Equal(t, float64(2), response["device_count"])

				devices, ok := response["devices"].([]interface{})
				require.True(t, ok)
				assert.Len(t, devices, 2)

				// Check first device has expected fields (snake_case JSON)
				device0 := devices[0].(map[string]interface{})
				assert.Contains(t, device0, "index")
				assert.Contains(t, device0, "name")
				assert.Contains(t, device0, "uuid")
				assert.Contains(t, device0, "bus_id")
				assert.Contains(t, device0, "memory")
				assert.Contains(t, device0, "temperature")
				assert.Contains(t, device0, "power")
				assert.Contains(t, device0, "clocks")
				assert.Contains(t, device0, "utilization")
				assert.Contains(t, device0, "ecc")
			},
		},
		{
			name:        "successful inventory with single device",
			deviceCount: 1,
			expectError: false,
			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var response map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(1), response["device_count"])
				devices := response["devices"].([]interface{})
				assert.Len(t, devices, 1)
			},
		},
		{
			name:        "successful inventory with 4 devices",
			deviceCount: 4,
			expectError: false,
			validateFunc: func(t *testing.T, result *mcp.CallToolResult) {
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var response map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &response)
				require.NoError(t, err)

				assert.Equal(t, float64(4), response["device_count"])
				devices := response["devices"].([]interface{})
				assert.Len(t, devices, 4)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := nvml.NewMock(tt.deviceCount)
			handler := NewGPUInventoryHandler(mockClient)

			ctx := context.Background()
			request := mcp.CallToolRequest{}

			result, err := handler.Handle(ctx, request)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validateFunc != nil {
					tt.validateFunc(t, result)
				}
			}
		})
	}
}

func TestGPUInventoryHandler_HandleContextCancellation(t *testing.T) {
	mockClient := nvml.NewMock(10) // Many devices to increase chance of cancellation
	handler := NewGPUInventoryHandler(mockClient)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	// Should handle cancellation gracefully
	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should indicate cancellation
	assert.True(t, result.IsError)
}

func TestGPUInventoryHandler_collectDeviceInfo(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewGPUInventoryHandler(mockClient)

	ctx := context.Background()
	device, err := mockClient.GetDeviceByIndex(ctx, 0)
	require.NoError(t, err)

	info, err := handler.collectDeviceInfo(ctx, 0, device)
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify basic fields
	assert.Equal(t, 0, info.Index)
	assert.NotEmpty(t, info.Name)
	assert.Contains(t, info.Name, "NVIDIA")
	assert.NotEmpty(t, info.UUID)
	assert.Contains(t, info.UUID, "GPU-")
	assert.NotEmpty(t, info.BusID)

	// Verify nested memory spec
	assert.Greater(t, info.Memory.TotalBytes, uint64(0))
	assert.GreaterOrEqual(t, info.Memory.UsedBytes, uint64(0))
	assert.Greater(t, info.Memory.FreeBytes, uint64(0))

	// Verify nested temperature spec with thresholds
	assert.Greater(t, info.Temperature.CurrentCelsius, uint32(0))
	assert.Greater(t, info.Temperature.SlowdownCelsius, uint32(0))
	assert.Greater(t, info.Temperature.ShutdownCelsius, uint32(0))

	// Verify nested power spec with limit
	assert.Greater(t, info.Power.CurrentMW, uint32(0))
	assert.Greater(t, info.Power.LimitMW, uint32(0))

	// Verify nested clocks spec
	assert.Greater(t, info.Clocks.SMMHZ, uint32(0))
	assert.Greater(t, info.Clocks.MemoryMHZ, uint32(0))

	// Verify nested utilization spec
	assert.LessOrEqual(t, info.Utilization.GPUPercent, uint32(100))
	assert.LessOrEqual(t, info.Utilization.MemoryPercent, uint32(100))

	// Verify ECC spec (mock has ECC enabled)
	require.NotNil(t, info.ECC)
	assert.True(t, info.ECC.Enabled)
}

func TestGPUInventoryHandler_NestedStructures(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewGPUInventoryHandler(mockClient)

	result, err := handler.Handle(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)

	// Verify JSON structure contains nested objects
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	responseText := textContent.Text

	// Should have nested objects with snake_case keys
	assert.Contains(t, responseText, `"memory":`)
	assert.Contains(t, responseText, `"total_bytes":`)
	assert.Contains(t, responseText, `"temperature":`)
	assert.Contains(t, responseText, `"current_celsius":`)
	assert.Contains(t, responseText, `"slowdown_celsius":`)
	assert.Contains(t, responseText, `"shutdown_celsius":`)
	assert.Contains(t, responseText, `"power":`)
	assert.Contains(t, responseText, `"current_mw":`)
	assert.Contains(t, responseText, `"limit_mw":`)
	assert.Contains(t, responseText, `"clocks":`)
	assert.Contains(t, responseText, `"sm_mhz":`)
	assert.Contains(t, responseText, `"memory_mhz":`)
	assert.Contains(t, responseText, `"utilization":`)
	assert.Contains(t, responseText, `"gpu_percent":`)
	assert.Contains(t, responseText, `"memory_percent":`)
	assert.Contains(t, responseText, `"ecc":`)
	assert.Contains(t, responseText, `"enabled":`)
}

func TestGetGPUInventoryTool(t *testing.T) {
	tool := GetGPUInventoryTool()

	assert.Equal(t, "get_gpu_inventory", tool.Name)
	assert.NotEmpty(t, tool.Description)
}
