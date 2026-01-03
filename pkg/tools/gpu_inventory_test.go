// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
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

				// Check first device has expected fields
				device0 := devices[0].(map[string]interface{})
				assert.Contains(t, device0, "Index")
				assert.Contains(t, device0, "Name")
				assert.Contains(t, device0, "UUID")
				assert.Contains(t, device0, "BusID")
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

	// Verify all fields are populated
	assert.Equal(t, 0, info.Index)
	assert.NotEmpty(t, info.Name)
	assert.Contains(t, info.Name, "NVIDIA")
	assert.NotEmpty(t, info.UUID)
	assert.Contains(t, info.UUID, "GPU-")
	assert.NotEmpty(t, info.BusID)
	assert.Greater(t, info.MemoryTotal, uint64(0))
	assert.Greater(t, info.Temperature, uint32(0))
	assert.Greater(t, info.PowerUsage, uint32(0))
	assert.LessOrEqual(t, info.GPUUtil, uint32(100))
	assert.LessOrEqual(t, info.MemoryUtil, uint32(100))
}

func TestGetGPUInventoryTool(t *testing.T) {
	tool := GetGPUInventoryTool()

	assert.Equal(t, "get_gpu_inventory", tool.Name)
	assert.NotEmpty(t, tool.Description)
}
