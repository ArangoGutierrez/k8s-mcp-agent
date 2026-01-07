// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package xid

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		wantExists bool
		wantName   string
	}{
		{
			name:       "known_xid_48_fatal",
			code:       48,
			wantExists: true,
			wantName:   "Double Bit ECC Error",
		},
		{
			name:       "known_xid_79_fatal",
			code:       79,
			wantExists: true,
			wantName:   "GPU Fallen Off Bus",
		},
		{
			name:       "known_xid_31_critical",
			code:       31,
			wantExists: true,
			wantName:   "GPU Exception",
		},
		{
			name:       "known_xid_13_critical",
			code:       13,
			wantExists: true,
			wantName:   "Graphics Exception",
		},
		{
			name:       "known_xid_45_warning",
			code:       45,
			wantExists: true,
			wantName:   "Preemption Error",
		},
		{
			name:       "unknown_xid_999",
			code:       999,
			wantExists: false,
			wantName:   "",
		},
		{
			name:       "unknown_xid_0",
			code:       0,
			wantExists: false,
			wantName:   "",
		},
		{
			name:       "unknown_xid_negative",
			code:       -1,
			wantExists: false,
			wantName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, exists := Lookup(tt.code)
			assert.Equal(t, tt.wantExists, exists, "exists mismatch")

			if tt.wantExists {
				assert.Equal(t, tt.code, info.Code, "code mismatch")
				assert.Equal(t, tt.wantName, info.Name, "name mismatch")
				assert.NotEmpty(t, info.Description, "description should not be empty")
				assert.NotEmpty(t, info.Severity, "severity should not be empty")
				assert.NotEmpty(t, info.Action, "action should not be empty")
				assert.NotEmpty(t, info.Category, "category should not be empty")
			} else {
				assert.Equal(t, 0, info.Code, "unknown code should return zero value")
				assert.Empty(t, info.Name, "unknown code should return empty name")
			}
		})
	}
}

func TestLookupOrUnknown(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		wantName     string
		wantSeverity string
		wantCategory string
	}{
		{
			name:         "known_xid_48",
			code:         48,
			wantName:     "Double Bit ECC Error",
			wantSeverity: "fatal",
			wantCategory: "memory",
		},
		{
			name:         "known_xid_79",
			code:         79,
			wantName:     "GPU Fallen Off Bus",
			wantSeverity: "fatal",
			wantCategory: "hardware",
		},
		{
			name:         "known_xid_74_nvlink",
			code:         74,
			wantName:     "NVLink Error",
			wantSeverity: "critical",
			wantCategory: "nvlink",
		},
		{
			name:         "unknown_xid_999",
			code:         999,
			wantName:     "Unknown XID 999",
			wantSeverity: "warning",
			wantCategory: "unknown",
		},
		{
			name:         "unknown_xid_0",
			code:         0,
			wantName:     "Unknown XID 0",
			wantSeverity: "warning",
			wantCategory: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := LookupOrUnknown(tt.code)

			assert.Equal(t, tt.code, info.Code, "code mismatch")
			assert.Equal(t, tt.wantName, info.Name, "name mismatch")
			assert.Equal(t, tt.wantSeverity, info.Severity, "severity mismatch")
			assert.Equal(t, tt.wantCategory, info.Category, "category mismatch")
			assert.NotEmpty(t, info.Description, "description should not be empty")
			assert.NotEmpty(t, info.Action, "action should not be empty")
		})
	}
}

func TestAllXIDsHaveMetadata(t *testing.T) {
	// Verify we have at least 15 XIDs as per spec
	require.GreaterOrEqual(t, len(ErrorCodes), 15,
		"should have at least 15 XID codes")

	// Verify all XIDs have complete metadata
	for code, info := range ErrorCodes {
		t.Run(fmt.Sprintf("xid_%d", code), func(t *testing.T) {
			assert.Equal(t, code, info.Code,
				"code field should match map key")
			assert.NotEmpty(t, info.Name,
				"name should not be empty")
			assert.NotEmpty(t, info.Description,
				"description should not be empty")
			assert.NotEmpty(t, info.Severity,
				"severity should not be empty")
			assert.NotEmpty(t, info.Action,
				"action should not be empty")
			assert.NotEmpty(t, info.Category,
				"category should not be empty")

			// Verify severity is one of the allowed values
			assert.Contains(t, []string{"info", "warning", "critical", "fatal"},
				info.Severity,
				"severity must be one of: info, warning, critical, fatal")

			// Verify category is one of the allowed values
			assert.Contains(t,
				[]string{"hardware", "memory", "thermal", "power", "nvlink"},
				info.Category,
				"category must be a known type")
		})
	}
}

func TestSeverityDistribution(t *testing.T) {
	// Count XIDs by severity
	severityCounts := make(map[string]int)
	for _, info := range ErrorCodes {
		severityCounts[info.Severity]++
	}

	// Verify we have XIDs in multiple severity levels
	assert.Greater(t, severityCounts["fatal"], 0,
		"should have at least one fatal XID")
	assert.Greater(t, severityCounts["critical"], 0,
		"should have at least one critical XID")
	assert.Greater(t, severityCounts["warning"], 0,
		"should have at least one warning XID")

	t.Logf("Severity distribution: fatal=%d, critical=%d, warning=%d, info=%d",
		severityCounts["fatal"],
		severityCounts["critical"],
		severityCounts["warning"],
		severityCounts["info"])
}

func TestCategoryDistribution(t *testing.T) {
	// Count XIDs by category
	categoryCounts := make(map[string]int)
	for _, info := range ErrorCodes {
		categoryCounts[info.Category]++
	}

	// Verify we have memory and hardware categories at minimum
	assert.Greater(t, categoryCounts["memory"], 0,
		"should have at least one memory XID")
	assert.Greater(t, categoryCounts["hardware"], 0,
		"should have at least one hardware XID")

	t.Logf("Category distribution: %+v", categoryCounts)
}

func TestCriticalXIDsPresent(t *testing.T) {
	// Verify the critical XIDs from the spec are present
	criticalXIDs := []int{13, 31, 43, 45, 48, 61, 62, 63, 64, 68, 69, 74, 79, 94, 95}

	for _, code := range criticalXIDs {
		t.Run(fmt.Sprintf("critical_xid_%d", code), func(t *testing.T) {
			info, exists := Lookup(code)
			assert.True(t, exists,
				"critical XID %d should be in error table", code)
			if exists {
				assert.NotEmpty(t, info.Name,
					"critical XID %d should have a name", code)
			}
		})
	}
}

func TestFatalXIDsHaveDrainAction(t *testing.T) {
	// Fatal XIDs should mention draining in their action
	for code, info := range ErrorCodes {
		if info.Severity == "fatal" {
			t.Run(fmt.Sprintf("fatal_xid_%d", code), func(t *testing.T) {
				assert.Contains(t, info.Action, "DRAIN",
					"fatal XID %d should recommend draining node", code)
			})
		}
	}
}

func TestXID48SpecificProperties(t *testing.T) {
	// XID 48 is the most critical - double-bit ECC error
	info, exists := Lookup(48)
	require.True(t, exists, "XID 48 must exist")

	assert.Equal(t, "Double Bit ECC Error", info.Name)
	assert.Equal(t, "fatal", info.Severity)
	assert.Equal(t, "memory", info.Category)
	assert.Contains(t, info.Action, "DRAIN NODE IMMEDIATELY")
	assert.Contains(t, info.Description, "Uncorrectable")
}

func TestXID79SpecificProperties(t *testing.T) {
	// XID 79 is fallen off bus - complete GPU failure
	info, exists := Lookup(79)
	require.True(t, exists, "XID 79 must exist")

	assert.Equal(t, "GPU Fallen Off Bus", info.Name)
	assert.Equal(t, "fatal", info.Severity)
	assert.Equal(t, "hardware", info.Category)
	assert.Contains(t, info.Action, "DRAIN NODE IMMEDIATELY")
}
