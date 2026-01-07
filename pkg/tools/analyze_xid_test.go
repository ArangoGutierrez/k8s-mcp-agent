// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/xid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockXIDParser is a test double for xid.Parser
type mockXIDParser struct {
	events []xid.XIDEvent
	err    error
}

func (m *mockXIDParser) ParseDmesg(ctx context.Context) ([]xid.XIDEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func TestNewAnalyzeXIDHandler(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewAnalyzeXIDHandler(mockClient)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.nvmlClient)
	assert.NotNil(t, handler.parser)
}

func TestAnalyzeXIDHandler_Handle_NoErrors(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	// Replace parser with mock that returns no events
	handler.parser = &mockXIDParser{
		events: []xid.XIDEvent{},
		err:    nil,
	}

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "content should be TextContent")

	var response AnalyzeXIDResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, 0, response.ErrorCount)
	assert.Empty(t, response.Errors)
	assert.Equal(t, 0, response.Summary.Fatal)
	assert.Equal(t, 0, response.Summary.Critical)
	assert.Equal(t, 0, response.Summary.Warning)
	assert.Contains(t, response.Recommendation, "No XID errors detected")
}

func TestAnalyzeXIDHandler_Handle_WithFatalError(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	// Replace parser with mock that returns XID 48 (fatal)
	handler.parser = &mockXIDParser{
		events: []xid.XIDEvent{
			{
				XIDCode:     48,
				PCIBusID:    "0000:01:00.0", // Mock GPU 0
				PID:         1234,
				ProcessName: "python3",
				RawMessage:  "[100.0] NVRM: Xid (PCI:0000:01:00.0): 48",
				GPUIndex:    -1,
			},
		},
		err: nil,
	}

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "content should be TextContent")

	var response AnalyzeXIDResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, "critical", response.Status)
	assert.Equal(t, 1, response.ErrorCount)
	assert.Len(t, response.Errors, 1)

	// Check error details
	errDetail := response.Errors[0]
	assert.Equal(t, 48, errDetail.XIDCode)
	assert.Equal(t, "fatal", errDetail.Severity)
	assert.Equal(t, "Double Bit ECC Error", errDetail.Name)
	assert.Equal(t, 0, errDetail.GPUIndex)          // Should be mapped to GPU 0
	assert.Contains(t, errDetail.GPUName, "NVIDIA") // Mock returns A100
	assert.NotEmpty(t, errDetail.GPUUUID)
	assert.Equal(t, 1234, errDetail.PID)
	assert.Equal(t, "python3", errDetail.ProcessName)

	// Check summary
	assert.Equal(t, 1, response.Summary.Fatal)
	assert.Equal(t, 0, response.Summary.Critical)
	assert.Equal(t, 0, response.Summary.Warning)

	// Check recommendation
	assert.Contains(t, response.Recommendation, "URGENT")
	assert.Contains(t, response.Recommendation, "Drain")
}

func TestAnalyzeXIDHandler_Handle_WithMultipleErrors(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	// Replace parser with mock that returns multiple XIDs
	handler.parser = &mockXIDParser{
		events: []xid.XIDEvent{
			{
				XIDCode:    48,             // Fatal
				PCIBusID:   "0000:01:00.0", // Mock GPU 0
				RawMessage: "[100.0] NVRM: Xid (PCI:0000:01:00.0): 48",
			},
			{
				XIDCode:    31,             // Critical
				PCIBusID:   "0000:01:00.0", // Mock GPU 0
				RawMessage: "[200.0] NVRM: Xid (PCI:0000:01:00.0): 31",
			},
			{
				XIDCode:    45,             // Warning
				PCIBusID:   "0000:02:00.0", // Mock GPU 1
				RawMessage: "[300.0] NVRM: Xid (PCI:0000:02:00.0): 45",
			},
		},
		err: nil,
	}

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "content should be TextContent")

	var response AnalyzeXIDResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, "critical", response.Status)
	assert.Equal(t, 3, response.ErrorCount)
	assert.Len(t, response.Errors, 3)

	// Check summary
	assert.Equal(t, 1, response.Summary.Fatal)
	assert.Equal(t, 1, response.Summary.Critical)
	assert.Equal(t, 1, response.Summary.Warning)

	// Check recommendation mentions all severities
	assert.Contains(t, response.Recommendation, "fatal")
	assert.Contains(t, response.Recommendation, "critical")
}

func TestAnalyzeXIDHandler_Handle_ContextCancellation(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return error result
	assert.True(t, result.IsError)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "cancelled")
}

func TestAnalyzeXIDHandler_Handle_ParserError(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	// Replace parser with mock that returns error
	handler.parser = &mockXIDParser{
		events: nil,
		err:    assert.AnError,
	}

	request := mcp.CallToolRequest{}
	result, err := handler.Handle(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return error result
	assert.True(t, result.IsError)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "failed to parse")
}

func TestAnalyzeXIDHandler_findGPUByPCI(t *testing.T) {
	mockClient := nvml.NewMock(2)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	tests := []struct {
		name         string
		pciBusID     string
		wantIndex    int
		wantFoundGPU bool
	}{
		{
			name:         "found_gpu_0",
			pciBusID:     "0000:01:00.0", // Mock GPU 0
			wantIndex:    0,
			wantFoundGPU: true,
		},
		{
			name:         "found_gpu_1",
			pciBusID:     "0000:02:00.0", // Mock GPU 1
			wantIndex:    1,
			wantFoundGPU: true,
		},
		{
			name:         "not_found",
			pciBusID:     "0000:FF:FF.0",
			wantIndex:    -1,
			wantFoundGPU: false,
		},
		{
			name:         "case_insensitive",
			pciBusID:     "0000:01:00.0", // Same as GPU 0 but case will vary in test
			wantIndex:    0,
			wantFoundGPU: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, info := handler.findGPUByPCI(ctx, tt.pciBusID)

			assert.Equal(t, tt.wantIndex, index)

			if tt.wantFoundGPU {
				assert.NotEmpty(t, info.Name)
				assert.NotEmpty(t, info.UUID)
				assert.NotEqual(t, "Unknown GPU", info.Name)
			} else {
				assert.Equal(t, "Unknown GPU", info.Name)
				assert.Equal(t, "unknown", info.UUID)
			}
		})
	}
}

func TestAnalyzeXIDHandler_createSummary(t *testing.T) {
	handler := &AnalyzeXIDHandler{}

	tests := []struct {
		name   string
		errors []EnrichedXIDError
		want   SeveritySummary
	}{
		{
			name:   "empty",
			errors: []EnrichedXIDError{},
			want:   SeveritySummary{},
		},
		{
			name: "one_of_each",
			errors: []EnrichedXIDError{
				{Severity: "fatal"},
				{Severity: "critical"},
				{Severity: "warning"},
				{Severity: "info"},
			},
			want: SeveritySummary{
				Fatal:    1,
				Critical: 1,
				Warning:  1,
				Info:     1,
			},
		},
		{
			name: "multiple_fatal",
			errors: []EnrichedXIDError{
				{Severity: "fatal"},
				{Severity: "fatal"},
				{Severity: "fatal"},
			},
			want: SeveritySummary{
				Fatal: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.createSummary(tt.errors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAnalyzeXIDHandler_determineStatus(t *testing.T) {
	handler := &AnalyzeXIDHandler{}

	tests := []struct {
		name    string
		summary SeveritySummary
		want    string
	}{
		{
			name:    "ok_no_errors",
			summary: SeveritySummary{},
			want:    "ok",
		},
		{
			name: "warning_only",
			summary: SeveritySummary{
				Warning: 1,
			},
			want: "warning",
		},
		{
			name: "critical_only",
			summary: SeveritySummary{
				Critical: 1,
			},
			want: "degraded",
		},
		{
			name: "fatal_only",
			summary: SeveritySummary{
				Fatal: 1,
			},
			want: "critical",
		},
		{
			name: "fatal_takes_precedence",
			summary: SeveritySummary{
				Fatal:    1,
				Critical: 5,
				Warning:  10,
			},
			want: "critical",
		},
		{
			name: "critical_over_warning",
			summary: SeveritySummary{
				Critical: 1,
				Warning:  10,
			},
			want: "degraded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.determineStatus(tt.summary)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAnalyzeXIDHandler_generateRecommendation(t *testing.T) {
	handler := &AnalyzeXIDHandler{}

	tests := []struct {
		name    string
		errors  []EnrichedXIDError
		summary SeveritySummary
		want    []string // Substrings that should be in recommendation
	}{
		{
			name:    "no_errors",
			errors:  []EnrichedXIDError{},
			summary: SeveritySummary{},
			want:    []string{"No XID errors detected", "good"},
		},
		{
			name: "fatal_errors",
			errors: []EnrichedXIDError{
				{
					XIDCode:  48,
					Name:     "Double Bit ECC Error",
					Severity: "fatal",
					GPUIndex: 0,
				},
			},
			summary: SeveritySummary{Fatal: 1},
			want:    []string{"URGENT", "Drain", "fatal"},
		},
		{
			name: "critical_errors",
			errors: []EnrichedXIDError{
				{
					XIDCode:  31,
					Severity: "critical",
				},
			},
			summary: SeveritySummary{Critical: 1},
			want:    []string{"critical", "Investigate"},
		},
		{
			name: "warning_errors",
			errors: []EnrichedXIDError{
				{
					XIDCode:  45,
					Severity: "warning",
				},
			},
			summary: SeveritySummary{Warning: 1},
			want:    []string{"warning", "Monitor"},
		},
		{
			name: "mixed_errors",
			errors: []EnrichedXIDError{
				{Severity: "fatal"},
				{Severity: "critical"},
				{Severity: "warning"},
			},
			summary: SeveritySummary{
				Fatal:    1,
				Critical: 1,
				Warning:  1,
			},
			want: []string{"URGENT", "Drain", "critical", "warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.generateRecommendation(tt.errors, tt.summary)

			for _, substr := range tt.want {
				assert.Contains(t, got, substr,
					"recommendation should contain: %s", substr)
			}
		})
	}
}

func TestGetAnalyzeXIDTool(t *testing.T) {
	tool := GetAnalyzeXIDTool()

	assert.Equal(t, "analyze_xid_errors", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.Description, "XID")
	assert.Contains(t, tool.Description, "kernel logs")
	assert.Contains(t, tool.Description, "severity")
}

func TestAnalyzeXIDHandler_enrichEvents(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	events := []xid.XIDEvent{
		{
			XIDCode:     48,
			PCIBusID:    "0000:01:00.0", // Mock GPU 0
			PID:         1234,
			ProcessName: "python3",
			RawMessage:  "[100.0] NVRM: Xid (PCI:0000:01:00.0): 48",
		},
	}

	enriched, err := handler.enrichEvents(ctx, events)

	require.NoError(t, err)
	require.Len(t, enriched, 1)

	e := enriched[0]
	assert.Equal(t, 48, e.XIDCode)
	assert.Equal(t, "Double Bit ECC Error", e.Name)
	assert.Equal(t, "fatal", e.Severity)
	assert.Equal(t, "memory", e.Category)
	assert.NotEmpty(t, e.Description)
	assert.NotEmpty(t, e.SREAction)
	assert.Equal(t, 0, e.GPUIndex)
	assert.Contains(t, e.GPUName, "NVIDIA") // Mock name varies
	assert.NotEmpty(t, e.GPUUUID)
	assert.Equal(t, 1234, e.PID)
	assert.Equal(t, "python3", e.ProcessName)
}

func TestAnalyzeXIDHandler_enrichEvents_ContextCancellation(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := []xid.XIDEvent{
		{XIDCode: 48, PCIBusID: "0000:01:00.0"}, // Mock GPU 0
	}

	enriched, err := handler.enrichEvents(ctx, events)

	assert.Error(t, err)
	assert.Nil(t, enriched)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestAnalyzeXIDHandler_enrichEvents_UnknownXID(t *testing.T) {
	mockClient := nvml.NewMock(1)
	handler := NewAnalyzeXIDHandler(mockClient)
	ctx := context.Background()

	events := []xid.XIDEvent{
		{
			XIDCode:    999,            // Unknown XID
			PCIBusID:   "0000:01:00.0", // Mock GPU 0
			RawMessage: "[100.0] NVRM: Xid (PCI:0000:01:00.0): 999",
		},
	}

	enriched, err := handler.enrichEvents(ctx, events)

	require.NoError(t, err)
	require.Len(t, enriched, 1)

	e := enriched[0]
	assert.Equal(t, 999, e.XIDCode)
	assert.Contains(t, e.Name, "Unknown XID 999")
	assert.Equal(t, "warning", e.Severity)
	assert.Equal(t, "unknown", e.Category)
}

func TestEnrichedXIDError_JSONSerialization(t *testing.T) {
	err := EnrichedXIDError{
		XIDCode:     48,
		Name:        "Test Error",
		Severity:    "fatal",
		Description: "Test description",
		SREAction:   "Test action",
		Category:    "memory",
		GPUIndex:    0,
		GPUName:     "Tesla T4",
		GPUUUID:     "GPU-12345",
		PCIBusID:    "0000:00:1E.0",
		PID:         1234,
		ProcessName: "python3",
		RawMessage:  "test message",
	}

	jsonBytes, jsonErr := json.Marshal(err)
	require.NoError(t, jsonErr)

	var decoded EnrichedXIDError
	jsonErr = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, jsonErr)

	assert.Equal(t, err, decoded)
}

func TestAnalyzeXIDResponse_JSONSerialization(t *testing.T) {
	response := AnalyzeXIDResponse{
		Status:     "critical",
		ErrorCount: 1,
		Errors: []EnrichedXIDError{
			{
				XIDCode:  48,
				Severity: "fatal",
				GPUIndex: 0,
			},
		},
		Summary: SeveritySummary{
			Fatal: 1,
		},
		Recommendation: "Test recommendation",
	}

	jsonBytes, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded AnalyzeXIDResponse
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.Status, decoded.Status)
	assert.Equal(t, response.ErrorCount, decoded.ErrorCount)
	assert.Equal(t, response.Summary, decoded.Summary)
}
