// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

// Package tools implements MCP tool handlers for GPU operations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
	"github.com/mark3labs/mcp-go/mcp"
)

// GPUInventoryHandler handles the get_gpu_inventory tool.
type GPUInventoryHandler struct {
	nvmlClient nvml.Interface
}

// NewGPUInventoryHandler creates a new GPU inventory handler.
func NewGPUInventoryHandler(nvmlClient nvml.Interface) *GPUInventoryHandler {
	return &GPUInventoryHandler{
		nvmlClient: nvmlClient,
	}
}

// Handle processes the get_gpu_inventory tool request.
func (h *GPUInventoryHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf(`{"level":"info","msg":"get_gpu_inventory invoked"}`)

	// Get device count
	count, err := h.nvmlClient.GetDeviceCount(ctx)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to get device count",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to get device count: %s", err)), nil
	}

	// Collect information for all devices
	gpus := make([]nvml.GPUInfo, 0, count)
	for i := 0; i < count; i++ {
		device, err := h.nvmlClient.GetDeviceByIndex(ctx, i)
		if err != nil {
			log.Printf(`{"level":"error","msg":"failed to get device",`+
				`"index":%d,"error":"%s"}`, i, err)
			continue
		}

		gpuInfo, err := h.collectDeviceInfo(ctx, i, device)
		if err != nil {
			log.Printf(`{"level":"error","msg":"failed to collect device info",`+
				`"index":%d,"error":"%s"}`, i, err)
			continue
		}

		gpus = append(gpus, *gpuInfo)
	}

	// Create response
	response := map[string]interface{}{
		"status":       "success",
		"device_count": count,
		"devices":      gpus,
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal response",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to marshal response: %s", err)), nil
	}

	log.Printf(`{"level":"info","msg":"get_gpu_inventory completed",`+
		`"device_count":%d}`, count)

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// collectDeviceInfo gathers all information for a single device.
func (h *GPUInventoryHandler) collectDeviceInfo(
	ctx context.Context,
	index int,
	device nvml.Device,
) (*nvml.GPUInfo, error) {
	info := &nvml.GPUInfo{
		Index: index,
	}

	// Get name
	name, err := device.GetName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get name: %w", err)
	}
	info.Name = name

	// Get UUID
	uuid, err := device.GetUUID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get UUID: %w", err)
	}
	info.UUID = uuid

	// Get PCI info
	pciInfo, err := device.GetPCIInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get PCI info: %w", err)
	}
	info.BusID = pciInfo.BusID

	// Get memory info
	memInfo, err := device.GetMemoryInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}
	info.MemoryTotal = memInfo.Total
	info.MemoryUsed = memInfo.Used
	info.MemoryFree = memInfo.Free

	// Get temperature
	temp, err := device.GetTemperature(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get temperature: %w", err)
	}
	info.Temperature = temp

	// Get power usage
	power, err := device.GetPowerUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get power usage: %w", err)
	}
	info.PowerUsage = power

	// Get utilization
	util, err := device.GetUtilizationRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get utilization: %w", err)
	}
	info.GPUUtil = util.GPU
	info.MemoryUtil = util.Memory

	return info, nil
}

// GetGPUInventoryTool returns the MCP tool definition for get_gpu_inventory.
func GetGPUInventoryTool() mcp.Tool {
	return mcp.NewTool("get_gpu_inventory",
		mcp.WithDescription(
			"Returns static hardware inventory for all GPU devices "+
				"including model, UUID, bus ID, and current telemetry",
		),
	)
}
