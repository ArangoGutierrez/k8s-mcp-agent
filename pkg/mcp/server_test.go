// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful creation with all required fields",
			config: Config{
				Mode:       "read-only",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nvml.NewMock(2),
			},
			expectError: false,
		},
		{
			name: "successful creation with operator mode",
			config: Config{
				Mode:       "operator",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nvml.NewMock(2),
			},
			expectError: false,
		},
		{
			name: "successful creation with default mode",
			config: Config{
				Mode:       "",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nvml.NewMock(2),
			},
			expectError: false,
		},
		{
			name: "fails without NVML client",
			config: Config{
				Mode:       "read-only",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nil,
			},
			expectError: true,
			errorMsg:    "NVMLClient is required",
		},
		{
			name: "successful creation with HTTP transport",
			config: Config{
				Mode:       "read-only",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nvml.NewMock(2),
				Transport:  TransportHTTP,
				HTTPAddr:   "0.0.0.0:8080",
			},
			expectError: false,
		},
		{
			name: "fails HTTP transport without HTTPAddr",
			config: Config{
				Mode:       "read-only",
				Version:    "1.0.0",
				GitCommit:  "abc123",
				NVMLClient: nvml.NewMock(2),
				Transport:  TransportHTTP,
				HTTPAddr:   "",
			},
			expectError: true,
			errorMsg:    "HTTPAddr is required for HTTP transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, server)
			} else {
				require.NoError(t, err)
				require.NotNil(t, server)
				assert.NotNil(t, server.mcpServer)
				assert.NotNil(t, server.nvmlClient)

				// Verify mode defaults
				if tt.config.Mode == "" {
					assert.Equal(t, "read-only", server.mode)
				} else {
					assert.Equal(t, tt.config.Mode, server.mode)
				}

				// Verify transport defaults
				if tt.config.Transport == "" {
					assert.Equal(t, TransportStdio, server.transport)
				} else {
					assert.Equal(t, tt.config.Transport, server.transport)
				}

				// Verify httpAddr and version stored correctly
				assert.Equal(t, tt.config.HTTPAddr, server.httpAddr)
				assert.Equal(t, tt.config.Version, server.version)
			}
		})
	}
}

func TestServer_Shutdown(t *testing.T) {
	mockClient := nvml.NewMock(2)
	server, err := New(Config{
		Mode:       "read-only",
		Version:    "1.0.0",
		GitCommit:  "test",
		NVMLClient: mockClient,
	})
	require.NoError(t, err)

	err = server.Shutdown()
	assert.NoError(t, err)
}

func TestLogToStderr(t *testing.T) {
	// This function logs to stderr, so we just test it doesn't panic
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	assert.NotPanics(t, func() {
		LogToStderr("info", "test message", fields)
	})

	assert.NotPanics(t, func() {
		LogToStderr("error", "error message", nil)
	})
}
