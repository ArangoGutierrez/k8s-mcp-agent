// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

// Package xid provides NVIDIA XID error code lookup and classification.
// XID errors are GPU hardware failures logged by the NVIDIA driver to the
// kernel ring buffer. They indicate issues like memory corruption, bus
// failures, or thermal problems.
//
// Reference: https://docs.nvidia.com/deploy/xid-errors/
package xid

import "fmt"

// ErrorInfo contains metadata about an XID error code.
type ErrorInfo struct {
	Code        int    `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "info", "warning", "critical", "fatal"
	Action      string `json:"sre_action"`
	Category    string `json:"category"` // "hardware", "memory", "thermal", "power", "nvlink"
}

// ErrorCodes maps XID error codes to their metadata.
// This table includes the most common and critical XIDs observed in
// production GPU environments.
var ErrorCodes = map[int]ErrorInfo{
	8: {
		Code:        8,
		Name:        "Page Retirement Failure",
		Description: "GPU failed to retire a page of memory with uncorrectable errors",
		Severity:    "critical",
		Action:      "Monitor ECC error counts. If persistent, schedule GPU replacement.",
		Category:    "memory",
	},
	13: {
		Code:        13,
		Name:        "Graphics Exception",
		Description: "Graphics engine exception occurred during rendering or compute",
		Severity:    "critical",
		Action:      "Check application logs. If frequent, drain node and investigate workload.",
		Category:    "hardware",
	},
	31: {
		Code:        31,
		Name:        "GPU Exception",
		Description: "General GPU exception - internal error in GPU execution",
		Severity:    "critical",
		Action:      "Check for driver/firmware mismatch. Consider GPU reset if persistent.",
		Category:    "hardware",
	},
	32: {
		Code:        32,
		Name:        "Invalid Memory Access",
		Description: "GPU attempted to access invalid memory address",
		Severity:    "warning",
		Action:      "Review application memory usage. May indicate programming error.",
		Category:    "memory",
	},
	43: {
		Code:        43,
		Name:        "GPU Stopped Responding",
		Description: "GPU failed to respond to driver commands within timeout period",
		Severity:    "critical",
		Action:      "Reset GPU or drain node. Check for thermal throttling or power issues.",
		Category:    "hardware",
	},
	45: {
		Code:        45,
		Name:        "Preemption Error",
		Description: "GPU context preemption failed",
		Severity:    "warning",
		Action:      "Monitor for frequency. If rare, can be ignored. If frequent, investigate workload scheduling.",
		Category:    "hardware",
	},
	48: {
		Code:        48,
		Name:        "Double Bit ECC Error",
		Description: "Uncorrectable ECC error detected in GPU memory - data corruption has occurred",
		Severity:    "fatal",
		Action:      "DRAIN NODE IMMEDIATELY. Memory corruption detected. GPU must be replaced.",
		Category:    "memory",
	},
	61: {
		Code:        61,
		Name:        "Internal Micro-controller Error",
		Description: "GPU internal micro-controller detected an error condition",
		Severity:    "critical",
		Action:      "Reset GPU. If persistent, drain node and schedule replacement.",
		Category:    "hardware",
	},
	62: {
		Code:        62,
		Name:        "Internal Micro-controller Breakpoint",
		Description: "GPU micro-controller hit unexpected breakpoint",
		Severity:    "critical",
		Action:      "GPU firmware issue. Update driver/firmware or replace GPU.",
		Category:    "hardware",
	},
	63: {
		Code:        63,
		Name:        "Internal Micro-controller Halt",
		Description: "GPU micro-controller halted unexpectedly",
		Severity:    "critical",
		Action:      "GPU firmware failure. Reset required. If persistent, replace GPU.",
		Category:    "hardware",
	},
	64: {
		Code:        64,
		Name:        "ECC Page Retirement Pending",
		Description: "GPU has pages pending retirement due to excessive errors",
		Severity:    "warning",
		Action:      "Monitor ECC error rate. Schedule GPU replacement during next maintenance window.",
		Category:    "memory",
	},
	68: {
		Code:        68,
		Name:        "FBPA Exception",
		Description: "Frame Buffer Partition A exception - memory controller error",
		Severity:    "critical",
		Action:      "Memory subsystem failure. Drain node and replace GPU.",
		Category:    "memory",
	},
	69: {
		Code:        69,
		Name:        "FBP Exception",
		Description: "Frame Buffer Partition exception - memory controller error",
		Severity:    "critical",
		Action:      "Memory subsystem failure. Drain node and replace GPU.",
		Category:    "memory",
	},
	74: {
		Code:        74,
		Name:        "NVLink Error",
		Description: "NVLink interconnect detected error or link degradation",
		Severity:    "critical",
		Action:      "Check NVLink topology and cable connections. May require node drain if multi-GPU workload.",
		Category:    "nvlink",
	},
	79: {
		Code:        79,
		Name:        "GPU Fallen Off Bus",
		Description: "GPU is no longer accessible on PCIe bus - complete hardware failure",
		Severity:    "fatal",
		Action:      "DRAIN NODE IMMEDIATELY. GPU hardware failure. Check PCIe connection and replace GPU.",
		Category:    "hardware",
	},
	92: {
		Code:        92,
		Name:        "High Single Bit ECC Error Rate",
		Description: "Elevated rate of correctable ECC errors detected",
		Severity:    "warning",
		Action:      "Monitor trend. May indicate early memory degradation. Schedule replacement if rate increases.",
		Category:    "memory",
	},
	94: {
		Code:        94,
		Name:        "Contained Error",
		Description: "GPU detected and contained an error - no data corruption",
		Severity:    "warning",
		Action:      "Monitor frequency. Isolated occurrences acceptable. Investigate if frequent.",
		Category:    "hardware",
	},
	95: {
		Code:        95,
		Name:        "Uncontained Error",
		Description: "GPU error could not be contained - potential data corruption",
		Severity:    "fatal",
		Action:      "DRAIN NODE IMMEDIATELY. Potential data corruption. GPU must be replaced.",
		Category:    "hardware",
	},
}

// Lookup returns the ErrorInfo for a given XID code.
// Returns the info and true if the code exists, or a zero value and false
// if the code is unknown.
func Lookup(code int) (ErrorInfo, bool) {
	info, exists := ErrorCodes[code]
	return info, exists
}

// LookupOrUnknown returns the ErrorInfo for a given XID code.
// If the code is not in the known error table, it returns a generic
// ErrorInfo with "unknown" classification.
func LookupOrUnknown(code int) ErrorInfo {
	if info, exists := ErrorCodes[code]; exists {
		return info
	}
	return ErrorInfo{
		Code:        code,
		Name:        fmt.Sprintf("Unknown XID %d", code),
		Description: "XID not in known error table - check NVIDIA documentation for details",
		Severity:    "warning",
		Action:      "Check NVIDIA XID documentation at https://docs.nvidia.com/deploy/xid-errors/",
		Category:    "unknown",
	}
}
