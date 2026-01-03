// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

package xid

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock dmesg outputs for testing
const (
	mockDmesgWithXID48 = `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3, Ch 00000002`

	mockDmesgWithXID79 = `[  200.654321] NVRM: Xid (PCI:0000:00:1E.0): 79, pid='<unknown>', name=<unknown>`

	mockDmesgMultipleXIDs = `[    0.000000] Linux version 5.15.0-1234-generic
[   10.123456] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3
[   15.234567] Some other kernel message
[   20.345678] NVRM: Xid (PCI:0000:00:1E.0): 79, pid='5678', name=nvidia-smi
[   25.456789] NVRM: GPU at PCI:0000:00:1E.0 GPU-d129fc5b-2d51-cec7-d985-49168c12716f
[   30.567890] NVRM: Xid (PCI:0000:01:00.0): 13, Ch 00000001`

	mockDmesgWithShortPCI = `[  100.123456] NVRM: Xid (PCI:00:1E.0): 48, pid=1234`

	mockDmesgNoPID = `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 31`

	mockDmesgNoTimestamp = `NVRM: Xid (PCI:0000:00:1E.0): 43, pid=9999, name=compute`

	mockDmesgEmpty = ``

	mockDmesgNoXIDs = `[    0.000000] Linux version 5.15.0-1234-generic
[    1.234567] ACPI: PCI Interrupt Link [LNKA] enabled
[    2.345678] Some other message
[    3.456789] NVRM: loading NVIDIA UNIX x86_64 Kernel Module`

	mockDmesgMalformed = `[  100.123456] NVRM: Xid (PCI:INVALID): NOT_A_NUMBER
[  200.123456] NVRM: Xid (PCI:0000:00:1E.0):
[  300.123456] NVRM: Xid (): 48
[  400.123456] Xid (PCI:0000:00:1E.0): 31`
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	require.NotNil(t, parser)
	assert.NotNil(t, parser.xidRegex)
	assert.NotNil(t, parser.pidRegex)
	assert.NotNil(t, parser.processNameRegex)
	assert.NotNil(t, parser.timestampRegex)
}

func TestParser_parseDmesgOutput(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantXIDs  []int
	}{
		{
			name:      "single_xid_48",
			input:     mockDmesgWithXID48,
			wantCount: 1,
			wantXIDs:  []int{48},
		},
		{
			name:      "single_xid_79",
			input:     mockDmesgWithXID79,
			wantCount: 1,
			wantXIDs:  []int{79},
		},
		{
			name:      "multiple_xids",
			input:     mockDmesgMultipleXIDs,
			wantCount: 3,
			wantXIDs:  []int{48, 79, 13},
		},
		{
			name:      "short_pci_format",
			input:     mockDmesgWithShortPCI,
			wantCount: 1,
			wantXIDs:  []int{48},
		},
		{
			name:      "no_pid",
			input:     mockDmesgNoPID,
			wantCount: 1,
			wantXIDs:  []int{31},
		},
		{
			name:      "no_timestamp",
			input:     mockDmesgNoTimestamp,
			wantCount: 1,
			wantXIDs:  []int{43},
		},
		{
			name:      "empty_output",
			input:     mockDmesgEmpty,
			wantCount: 0,
			wantXIDs:  []int{},
		},
		{
			name:      "no_xids_present",
			input:     mockDmesgNoXIDs,
			wantCount: 0,
			wantXIDs:  []int{},
		},
		{
			name:      "malformed_lines",
			input:     mockDmesgMalformed,
			wantCount: 0, // All lines are malformed and should be skipped
			wantXIDs:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := parser.parseDmesgOutput(tt.input)

			assert.Equal(t, tt.wantCount, len(events),
				"unexpected number of events")

			for i, wantXID := range tt.wantXIDs {
				if i < len(events) {
					assert.Equal(t, wantXID, events[i].XIDCode,
						"event %d: XID code mismatch", i)
				}
			}
		})
	}
}

func TestParser_parseXIDLine(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name            string
		line            string
		wantNil         bool
		wantXID         int
		wantPCIBusID    string
		wantPID         int
		wantProcessName string
	}{
		{
			name:            "full_xid_with_all_fields",
			line:            `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3, Ch 00000002`,
			wantNil:         false,
			wantXID:         48,
			wantPCIBusID:    "0000:00:1E.0",
			wantPID:         1234,
			wantProcessName: "python3",
		},
		{
			name:         "xid_with_quoted_pid",
			line:         `[  200.654321] NVRM: Xid (PCI:0000:00:1E.0): 79, pid='5678'`,
			wantNil:      false,
			wantXID:      79,
			wantPCIBusID: "0000:00:1E.0",
			wantPID:      5678,
		},
		{
			name:         "xid_with_unquoted_pid",
			line:         `[  300.123456] NVRM: Xid (PCI:0000:01:00.0): 31, pid=9999`,
			wantNil:      false,
			wantXID:      31,
			wantPCIBusID: "0000:01:00.0",
			wantPID:      9999,
		},
		{
			name:         "xid_minimal",
			line:         `NVRM: Xid (PCI:0000:00:1E.0): 13`,
			wantNil:      false,
			wantXID:      13,
			wantPCIBusID: "0000:00:1E.0",
			wantPID:      0,
		},
		{
			name:         "xid_with_short_pci",
			line:         `[  100.123456] NVRM: Xid (PCI:00:1E.0): 48`,
			wantNil:      false,
			wantXID:      48,
			wantPCIBusID: "0000:00:1E.0",
		},
		{
			name:         "xid_with_lowercase_pci",
			line:         `[  100.123456] NVRM: Xid (PCI:0000:00:1e.0): 48`,
			wantNil:      false,
			wantXID:      48,
			wantPCIBusID: "0000:00:1E.0",
		},
		{
			name:         "no_nvrm_prefix",
			line:         `[  100.123456] Xid (PCI:0000:00:1E.0): 48`,
			wantNil:      false,
			wantXID:      48,
			wantPCIBusID: "0000:00:1E.0",
		},
		{
			name:    "invalid_xid_code",
			line:    `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): NOT_A_NUMBER`,
			wantNil: true,
		},
		{
			name:    "missing_xid_code",
			line:    `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0):`,
			wantNil: true,
		},
		{
			name:    "missing_pci_bus_id",
			line:    `[  100.123456] NVRM: Xid (): 48`,
			wantNil: true,
		},
		{
			name:    "invalid_pci_format",
			line:    `[  100.123456] NVRM: Xid (PCI:INVALID): 48`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := parser.parseXIDLine(tt.line)

			if tt.wantNil {
				assert.Nil(t, event, "expected nil event")
				return
			}

			require.NotNil(t, event, "expected non-nil event")
			assert.Equal(t, tt.wantXID, event.XIDCode, "XID code mismatch")
			assert.Equal(t, tt.wantPCIBusID, event.PCIBusID, "PCI bus ID mismatch")
			assert.Equal(t, tt.wantPID, event.PID, "PID mismatch")

			if tt.wantProcessName != "" {
				assert.Equal(t, tt.wantProcessName, event.ProcessName,
					"process name mismatch")
			}

			assert.Equal(t, -1, event.GPUIndex,
				"GPU index should be -1 (unset)")
			assert.NotEmpty(t, event.RawMessage,
				"raw message should be preserved")
		})
	}
}

func TestParser_ParseDmesg_ContextCancellation(t *testing.T) {
	parser := NewParser()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	events, err := parser.ParseDmesg(ctx)

	assert.Error(t, err, "expected error with cancelled context")
	assert.Nil(t, events, "expected nil events with cancelled context")
	assert.Contains(t, err.Error(), "context cancelled",
		"error should mention context cancellation")
}

func TestParser_ParseDmesg_ContextTimeout(t *testing.T) {
	parser := NewParser()

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give context time to expire
	time.Sleep(1 * time.Millisecond)

	events, err := parser.ParseDmesg(ctx)

	// This will fail because context is cancelled before dmesg runs
	assert.Error(t, err, "expected error with expired context")
	assert.Nil(t, events, "expected nil events with expired context")
}

func TestNormalizePCIBusID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already_canonical",
			input: "0000:00:1E.0",
			want:  "0000:00:1E.0",
		},
		{
			name:  "missing_domain",
			input: "00:1E.0",
			want:  "0000:00:1E.0",
		},
		{
			name:  "lowercase_hex",
			input: "0000:00:1e.0",
			want:  "0000:00:1E.0",
		},
		{
			name:  "missing_domain_lowercase",
			input: "00:1e.0",
			want:  "0000:00:1E.0",
		},
		{
			name:  "different_bus",
			input: "01:00.0",
			want:  "0000:01:00.0",
		},
		{
			name:  "multi_gpu_bus",
			input: "0000:81:00.0",
			want:  "0000:81:00.0",
		},
		{
			name:  "invalid_single_colon",
			input: "invalid:format",
			want:  "0000:INVALID:FORMAT",
		},
		{
			name:  "no_colons",
			input: "invalid",
			want:  "INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePCIBusID(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "valid_positive",
			input: "1234",
			want:  1234,
		},
		{
			name:  "valid_zero",
			input: "0",
			want:  0,
		},
		{
			name:  "invalid_string",
			input: "not_a_number",
			want:  0,
		},
		{
			name:  "empty_string",
			input: "",
			want:  0,
		},
		{
			name:  "negative_number",
			input: "-1",
			want:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestXIDEvent_Structure(t *testing.T) {
	// Verify XIDEvent has all expected fields
	event := XIDEvent{
		Timestamp:   time.Now(),
		XIDCode:     48,
		PCIBusID:    "0000:00:1E.0",
		GPUIndex:    0,
		PID:         1234,
		ProcessName: "python3",
		RawMessage:  "test message",
	}

	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, 48, event.XIDCode)
	assert.Equal(t, "0000:00:1E.0", event.PCIBusID)
	assert.Equal(t, 0, event.GPUIndex)
	assert.Equal(t, 1234, event.PID)
	assert.Equal(t, "python3", event.ProcessName)
	assert.Equal(t, "test message", event.RawMessage)
}

func TestParser_parseXIDLine_TimestampExtraction(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		line         string
		wantZeroTime bool
	}{
		{
			name:         "with_timestamp",
			line:         `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 48`,
			wantZeroTime: false,
		},
		{
			name:         "without_timestamp",
			line:         `NVRM: Xid (PCI:0000:00:1E.0): 48`,
			wantZeroTime: true,
		},
		{
			name:         "malformed_timestamp",
			line:         `[ invalid ] NVRM: Xid (PCI:0000:00:1E.0): 48`,
			wantZeroTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := parser.parseXIDLine(tt.line)
			require.NotNil(t, event)

			if tt.wantZeroTime {
				assert.True(t, event.Timestamp.IsZero(),
					"expected zero timestamp")
			} else {
				assert.False(t, event.Timestamp.IsZero(),
					"expected non-zero timestamp")
			}
		})
	}
}

func TestParser_parseDmesgOutput_PreservesRawMessage(t *testing.T) {
	parser := NewParser()

	input := `[  100.123456] NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3`
	events := parser.parseDmesgOutput(input)

	require.Len(t, events, 1)
	assert.Equal(t, input, events[0].RawMessage,
		"raw message should be preserved exactly")
}

func TestParser_parseDmesgOutput_OnlyNVRMMessages(t *testing.T) {
	parser := NewParser()

	// Mix NVRM and non-NVRM messages
	input := `[  100.123456] Some other message
[  200.123456] NVRM: Xid (PCI:0000:00:1E.0): 48
[  300.123456] Another non-NVRM message
[  400.123456] NVRM: loading driver
[  500.123456] NVRM: Xid (PCI:0000:00:1E.0): 79`

	events := parser.parseDmesgOutput(input)

	// Should only find the 2 XID events, not other NVRM messages
	assert.Len(t, events, 2)
	assert.Equal(t, 48, events[0].XIDCode)
	assert.Equal(t, 79, events[1].XIDCode)
}
