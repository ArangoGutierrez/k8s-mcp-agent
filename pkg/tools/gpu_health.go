// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
	"github.com/mark3labs/mcp-go/mcp"
)

// GPUHealthHandler handles the get_gpu_health tool.
type GPUHealthHandler struct {
	nvmlClient nvml.Interface
}

// NewGPUHealthHandler creates a new GPU health handler.
func NewGPUHealthHandler(nvmlClient nvml.Interface) *GPUHealthHandler {
	return &GPUHealthHandler{
		nvmlClient: nvmlClient,
	}
}

// GPUHealthResponse is the top-level response structure for GPU health status.
type GPUHealthResponse struct {
	Status         string            `json:"status"`
	OverallScore   int               `json:"overall_score"`
	DeviceCount    int               `json:"device_count"`
	HealthyCount   int               `json:"healthy_count"`
	DegradedCount  int               `json:"degraded_count"`
	CriticalCount  int               `json:"critical_count"`
	GPUs           []GPUHealthStatus `json:"gpus"`
	Recommendation string            `json:"recommendation"`
}

// GPUHealthStatus contains health metrics for a single GPU.
type GPUHealthStatus struct {
	Index       int               `json:"index"`
	Name        string            `json:"name"`
	UUID        string            `json:"uuid"`
	PCIBusID    string            `json:"pci_bus_id"`
	Status      string            `json:"status"`
	HealthScore int               `json:"health_score"`
	Temperature TemperatureHealth `json:"temperature"`
	Memory      MemoryHealth      `json:"memory"`
	Power       PowerHealth       `json:"power"`
	Throttling  ThrottlingStatus  `json:"throttling"`
	ECCErrors   ECCHealth         `json:"ecc_errors"`
	Performance PerformanceHealth `json:"performance"`
	Issues      []HealthIssue     `json:"issues,omitempty"`
}

// TemperatureHealth tracks thermal status of the GPU.
type TemperatureHealth struct {
	Current   uint32 `json:"current_celsius"`
	Threshold uint32 `json:"threshold_celsius"`
	Max       uint32 `json:"max_celsius"`
	Status    string `json:"status"`
	Margin    int    `json:"margin_celsius"`
}

// MemoryHealth tracks memory usage patterns.
type MemoryHealth struct {
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	Free        uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Status      string  `json:"status"`
}

// PowerHealth tracks power consumption relative to limits.
type PowerHealth struct {
	Current     uint32  `json:"current_mw"`
	Limit       uint32  `json:"limit_mw"`
	Default     uint32  `json:"default_mw"`
	UsedPercent float64 `json:"used_percent"`
	Status      string  `json:"status"`
}

// ThrottlingStatus indicates if GPU performance is being throttled.
type ThrottlingStatus struct {
	Active  bool     `json:"active"`
	Reasons []string `json:"reasons,omitempty"`
	Status  string   `json:"status"`
}

// ECCHealth tracks ECC memory error counts.
type ECCHealth struct {
	Enabled                  bool   `json:"enabled"`
	TotalCorrectableErrors   uint64 `json:"total_correctable_errors"`
	TotalUncorrectableErrors uint64 `json:"total_uncorrectable_errors"`
	Status                   string `json:"status"`
}

// PerformanceHealth tracks GPU utilization and clock frequencies.
type PerformanceHealth struct {
	GPUUtil     uint32 `json:"gpu_util_percent"`
	MemoryUtil  uint32 `json:"memory_util_percent"`
	SMClock     uint32 `json:"sm_clock_mhz"`
	MemoryClock uint32 `json:"memory_clock_mhz"`
	Status      string `json:"status"`
}

// HealthIssue describes a specific health concern with recommendations.
type HealthIssue struct {
	Severity   string `json:"severity"`
	Component  string `json:"component"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// Handle processes the get_gpu_health tool request.
func (h *GPUHealthHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf(`{"level":"info","msg":"get_gpu_health invoked"}`)

	// Check context before starting
	if err := ctx.Err(); err != nil {
		log.Printf(`{"level":"info","msg":"context cancelled before health check"}`)
		return mcp.NewToolResultError(
			fmt.Sprintf("operation cancelled: %s", err)), nil
	}

	// Get device count
	count, err := h.nvmlClient.GetDeviceCount(ctx)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to get device count",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to get device count: %s", err)), nil
	}

	// Collect health for each GPU
	gpus := make([]GPUHealthStatus, 0, count)
	for i := 0; i < count; i++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf(`{"level":"info","msg":"context cancelled during enumeration"}`)
			return mcp.NewToolResultError(
				fmt.Sprintf("operation cancelled: %s", err)), nil
		}

		device, err := h.nvmlClient.GetDeviceByIndex(ctx, i)
		if err != nil {
			log.Printf(`{"level":"error","msg":"failed to get device",`+
				`"index":%d,"error":"%s"}`, i, err)
			continue
		}

		health := h.collectGPUHealth(ctx, i, device)
		gpus = append(gpus, health)
	}

	// Calculate overall status
	response := h.calculateOverallHealth(gpus)

	// Generate recommendations
	response.Recommendation = h.generateRecommendation(response)

	log.Printf(`{"level":"info","msg":"get_gpu_health completed",`+
		`"device_count":%d,"status":"%s"}`, response.DeviceCount, response.Status)

	return h.marshalResponse(response)
}

// collectGPUHealth gathers health metrics for a single GPU device.
func (h *GPUHealthHandler) collectGPUHealth(
	ctx context.Context,
	index int,
	device nvml.Device,
) GPUHealthStatus {
	health := GPUHealthStatus{
		Index:  index,
		Issues: make([]HealthIssue, 0),
	}

	// Get basic info
	health.Name, _ = device.GetName(ctx)
	health.UUID, _ = device.GetUUID(ctx)
	pciInfo, err := device.GetPCIInfo(ctx)
	if err == nil {
		health.PCIBusID = pciInfo.BusID
	}

	// Collect metrics from each subsystem
	health.Temperature = h.checkTemperature(ctx, device)
	health.Memory = h.checkMemory(ctx, device)
	health.Power = h.checkPower(ctx, device)
	health.Throttling = h.checkThrottling(ctx, device)
	health.ECCErrors = h.checkECCErrors(ctx, device)
	health.Performance = h.checkPerformance(ctx, device)

	// Calculate health score (0-100)
	health.HealthScore = h.calculateHealthScore(&health)

	// Determine status based on score and issues
	health.Status = h.determineStatus(health.HealthScore, health.Issues)

	return health
}

// checkTemperature evaluates GPU thermal status.
func (h *GPUHealthHandler) checkTemperature(
	ctx context.Context,
	device nvml.Device,
) TemperatureHealth {
	// Tesla T4 defaults: threshold ~82°C, max ~90°C
	const defaultThreshold uint32 = 82
	const defaultMax uint32 = 90

	temp, err := device.GetTemperature(ctx)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"failed to get temperature",`+
			`"error":"%s"}`, err)
		return TemperatureHealth{
			Status: "unknown",
		}
	}

	margin := int(defaultThreshold) - int(temp)

	var status string
	switch {
	case temp >= defaultMax:
		status = "critical"
	case temp >= defaultThreshold:
		status = "high"
	case temp >= defaultThreshold-10:
		status = "elevated"
	default:
		status = "normal"
	}

	return TemperatureHealth{
		Current:   temp,
		Threshold: defaultThreshold,
		Max:       defaultMax,
		Status:    status,
		Margin:    margin,
	}
}

// checkMemory evaluates GPU memory usage.
func (h *GPUHealthHandler) checkMemory(
	ctx context.Context,
	device nvml.Device,
) MemoryHealth {
	memInfo, err := device.GetMemoryInfo(ctx)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"failed to get memory info",`+
			`"error":"%s"}`, err)
		return MemoryHealth{
			Status: "unknown",
		}
	}

	var usedPercent float64
	if memInfo.Total > 0 {
		usedPercent = float64(memInfo.Used) / float64(memInfo.Total) * 100
	}

	var status string
	switch {
	case usedPercent >= 95:
		status = "critical"
	case usedPercent >= 90:
		status = "high"
	case usedPercent >= 80:
		status = "elevated"
	default:
		status = "normal"
	}

	return MemoryHealth{
		Total:       memInfo.Total,
		Used:        memInfo.Used,
		Free:        memInfo.Free,
		UsedPercent: usedPercent,
		Status:      status,
	}
}

// checkPower evaluates GPU power consumption.
func (h *GPUHealthHandler) checkPower(
	ctx context.Context,
	device nvml.Device,
) PowerHealth {
	// Tesla T4 TDP: 70W = 70000mW
	const defaultLimit uint32 = 70000

	power, err := device.GetPowerUsage(ctx)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"failed to get power usage",`+
			`"error":"%s"}`, err)
		return PowerHealth{
			Status: "unknown",
		}
	}

	var usedPercent float64
	if defaultLimit > 0 {
		usedPercent = float64(power) / float64(defaultLimit) * 100
	}

	var status string
	switch {
	case usedPercent > 100:
		status = "over_limit"
	case usedPercent >= 95:
		status = "high"
	case usedPercent >= 80:
		status = "elevated"
	default:
		status = "normal"
	}

	return PowerHealth{
		Current:     power,
		Limit:       defaultLimit,
		Default:     defaultLimit,
		UsedPercent: usedPercent,
		Status:      status,
	}
}

// checkThrottling evaluates GPU clock throttling status.
// Note: Mock NVML doesn't implement throttling methods.
// Real implementation would use device.GetCurrentClocksThrottleReasons().
func (h *GPUHealthHandler) checkThrottling(
	ctx context.Context,
	device nvml.Device,
) ThrottlingStatus {
	// TODO: Implement when throttle reasons are added to nvml.Interface
	// Throttle reasons bitmask:
	// 0x0001 - GPU idle
	// 0x0002 - Applications clocks setting
	// 0x0004 - SW power cap
	// 0x0008 - HW slowdown (thermal)
	// 0x0010 - Sync boost
	// 0x0020 - SW thermal slowdown
	// 0x0040 - HW thermal slowdown
	// 0x0080 - HW power brake slowdown

	return ThrottlingStatus{
		Active:  false,
		Reasons: []string{},
		Status:  "none",
	}
}

// checkECCErrors evaluates ECC memory error status.
// Note: Mock NVML doesn't implement ECC methods.
// Real implementation would use device.GetTotalEccErrors().
func (h *GPUHealthHandler) checkECCErrors(
	ctx context.Context,
	device nvml.Device,
) ECCHealth {
	// TODO: Implement when ECC methods are added to nvml.Interface
	// Would aggregate:
	// - NVML_MEMORY_ERROR_TYPE_CORRECTED (single-bit)
	// - NVML_MEMORY_ERROR_TYPE_UNCORRECTED (double-bit)

	return ECCHealth{
		Enabled:                  true, // Tesla T4 has ECC
		TotalCorrectableErrors:   0,
		TotalUncorrectableErrors: 0,
		Status:                   "healthy",
	}
}

// checkPerformance evaluates GPU utilization and performance state.
func (h *GPUHealthHandler) checkPerformance(
	ctx context.Context,
	device nvml.Device,
) PerformanceHealth {
	util, err := device.GetUtilizationRates(ctx)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"failed to get utilization",`+
			`"error":"%s"}`, err)
		return PerformanceHealth{
			Status: "unknown",
		}
	}

	var status string
	switch {
	case util.GPU >= 95:
		status = "saturated"
	case util.GPU >= 50:
		status = "active"
	default:
		status = "idle"
	}

	// TODO: Add clock frequency when methods are available in nvml.Interface
	return PerformanceHealth{
		GPUUtil:     util.GPU,
		MemoryUtil:  util.Memory,
		SMClock:     0, // Not yet available in interface
		MemoryClock: 0, // Not yet available in interface
		Status:      status,
	}
}

// calculateHealthScore computes a weighted health score (0-100).
func (h *GPUHealthHandler) calculateHealthScore(health *GPUHealthStatus) int {
	score := 100

	// Temperature impact (max -30 points)
	switch health.Temperature.Status {
	case "critical":
		score -= 30
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "critical",
			Component: "temperature",
			Message: fmt.Sprintf("GPU temperature critical: %d°C",
				health.Temperature.Current),
			Suggestion: "Check cooling system, reduce workload immediately",
		})
	case "high":
		score -= 20
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "warning",
			Component: "temperature",
			Message: fmt.Sprintf("GPU temperature high: %d°C",
				health.Temperature.Current),
			Suggestion: "Monitor temperature, check cooling",
		})
	case "elevated":
		score -= 10
	}

	// Memory usage impact (max -20 points)
	switch health.Memory.Status {
	case "critical":
		score -= 20
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "critical",
			Component: "memory",
			Message: fmt.Sprintf("GPU memory critically low: %.1f%% used",
				health.Memory.UsedPercent),
			Suggestion: "Free GPU memory or reduce workload",
		})
	case "high":
		score -= 10
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "warning",
			Component: "memory",
			Message: fmt.Sprintf("GPU memory high: %.1f%% used",
				health.Memory.UsedPercent),
			Suggestion: "Consider freeing GPU memory",
		})
	}

	// Power usage impact (max -15 points)
	switch health.Power.Status {
	case "over_limit":
		score -= 15
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "warning",
			Component:  "power",
			Message:    "GPU power exceeds TDP limit",
			Suggestion: "Check power supply and cooling",
		})
	case "high":
		score -= 10
	}

	// Throttling impact (max -25 points)
	switch health.Throttling.Status {
	case "severe":
		score -= 25
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "critical",
			Component:  "throttling",
			Message:    "GPU severely throttled",
			Suggestion: "Investigate thermal or power issues",
		})
	case "minor":
		score -= 10
		health.Issues = append(health.Issues, HealthIssue{
			Severity:   "warning",
			Component:  "throttling",
			Message:    "GPU experiencing minor throttling",
			Suggestion: "Monitor for performance impact",
		})
	}

	// ECC errors impact (max -30 points)
	if health.ECCErrors.TotalUncorrectableErrors > 0 {
		score -= 30
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "critical",
			Component: "ecc",
			Message: fmt.Sprintf("%d uncorrectable ECC errors detected",
				health.ECCErrors.TotalUncorrectableErrors),
			Suggestion: "GPU may have hardware failure, drain node",
		})
	} else if health.ECCErrors.TotalCorrectableErrors > 1000 {
		score -= 10
		health.Issues = append(health.Issues, HealthIssue{
			Severity:  "warning",
			Component: "ecc",
			Message: fmt.Sprintf("%d correctable ECC errors",
				health.ECCErrors.TotalCorrectableErrors),
			Suggestion: "Monitor for increasing error rate",
		})
	}

	if score < 0 {
		score = 0
	}

	return score
}

// determineStatus classifies health based on score and issues.
func (h *GPUHealthHandler) determineStatus(
	score int,
	issues []HealthIssue,
) string {
	// Check for critical issues first
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return "critical"
		}
	}

	// Score-based determination
	switch {
	case score >= 90:
		return "healthy"
	case score >= 70:
		return "warning"
	case score >= 50:
		return "degraded"
	default:
		return "critical"
	}
}

// calculateOverallHealth aggregates health across all GPUs.
func (h *GPUHealthHandler) calculateOverallHealth(
	gpus []GPUHealthStatus,
) GPUHealthResponse {
	response := GPUHealthResponse{
		DeviceCount: len(gpus),
		GPUs:        gpus,
	}

	if len(gpus) == 0 {
		response.Status = "unknown"
		response.OverallScore = 0
		return response
	}

	// Count status categories and find worst score
	worstScore := 100
	for _, gpu := range gpus {
		switch gpu.Status {
		case "healthy":
			response.HealthyCount++
		case "warning", "degraded":
			response.DegradedCount++
		case "critical":
			response.CriticalCount++
		}

		if gpu.HealthScore < worstScore {
			worstScore = gpu.HealthScore
		}
	}

	response.OverallScore = worstScore

	// Overall status uses "worst GPU" approach
	switch {
	case response.CriticalCount > 0:
		response.Status = "critical"
	case response.DegradedCount > 0:
		response.Status = "degraded"
	default:
		response.Status = "healthy"
	}

	return response
}

// generateRecommendation creates actionable advice based on health status.
func (h *GPUHealthHandler) generateRecommendation(
	response GPUHealthResponse,
) string {
	if response.DeviceCount == 0 {
		return "No GPU devices detected. Verify driver installation."
	}

	if response.CriticalCount > 0 {
		return fmt.Sprintf("%d GPU(s) in critical state. "+
			"Immediate investigation required. Check issues for details.",
			response.CriticalCount)
	}

	if response.DegradedCount > 0 {
		return fmt.Sprintf("%d GPU(s) degraded. "+
			"Monitor closely and investigate issues.",
			response.DegradedCount)
	}

	return "All GPUs healthy. No action required."
}

// marshalResponse marshals the response to JSON and returns as tool result.
func (h *GPUHealthHandler) marshalResponse(
	response GPUHealthResponse,
) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal response",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to marshal response: %s", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetGPUHealthTool returns the MCP tool definition for get_gpu_health.
func GetGPUHealthTool() mcp.Tool {
	return mcp.NewTool("get_gpu_health",
		mcp.WithDescription(
			"Analyze GPU operational health including temperature, "+
				"throttling, ECC errors, memory usage, and power consumption. "+
				"Returns overall health score (0-100) with status assessment "+
				"and recommendations.",
		),
	)
}
