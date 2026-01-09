// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// createTestServer creates a minimal MCP server for testing.
func createTestServer() *server.MCPServer {
	s := server.NewMCPServer("test-server", "1.0.0")

	// Add a simple echo tool for testing
	echoTool := mcp.NewTool("echo",
		mcp.WithDescription("Echo the input"),
		mcp.WithString("message", mcp.Description("Message to echo")),
	)
	s.AddTool(echoTool, func(ctx context.Context,
		req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

		args := req.GetArguments()
		msg, _ := args["message"].(string)
		return mcp.NewToolResultText("echo: " + msg), nil
	})

	return s
}

func TestNewOneshotTransport_ValidConfig(t *testing.T) {
	cfg := OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(""),
		Writer:      &bytes.Buffer{},
		MaxRequests: 2,
	}

	transport, err := NewOneshotTransport(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestNewOneshotTransport_MissingMCPServer(t *testing.T) {
	cfg := OneshotConfig{
		MCPServer:   nil,
		Reader:      strings.NewReader(""),
		Writer:      &bytes.Buffer{},
		MaxRequests: 2,
	}

	_, err := NewOneshotTransport(cfg)
	if err == nil {
		t.Fatal("expected error for missing MCPServer")
	}
	if !strings.Contains(err.Error(), "MCPServer") {
		t.Errorf("error should mention MCPServer: %v", err)
	}
}

func TestNewOneshotTransport_MissingReader(t *testing.T) {
	cfg := OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      nil,
		Writer:      &bytes.Buffer{},
		MaxRequests: 2,
	}

	_, err := NewOneshotTransport(cfg)
	if err == nil {
		t.Fatal("expected error for missing Reader")
	}
	if !strings.Contains(err.Error(), "Reader") {
		t.Errorf("error should mention Reader: %v", err)
	}
}

func TestNewOneshotTransport_MissingWriter(t *testing.T) {
	cfg := OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(""),
		Writer:      nil,
		MaxRequests: 2,
	}

	_, err := NewOneshotTransport(cfg)
	if err == nil {
		t.Fatal("expected error for missing Writer")
	}
	if !strings.Contains(err.Error(), "Writer") {
		t.Errorf("error should mention Writer: %v", err)
	}
}

func TestNewOneshotTransport_InvalidMaxRequests(t *testing.T) {
	tests := []struct {
		name        string
		maxRequests int
	}{
		{"zero", 0},
		{"negative", -1},
		{"very negative", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OneshotConfig{
				MCPServer:   createTestServer(),
				Reader:      strings.NewReader(""),
				Writer:      &bytes.Buffer{},
				MaxRequests: tt.maxRequests,
			}

			_, err := NewOneshotTransport(cfg)
			if err == nil {
				t.Fatal("expected error for invalid MaxRequests")
			}
			if !strings.Contains(err.Error(), "MaxRequests") {
				t.Errorf("error should mention MaxRequests: %v", err)
			}
		})
	}
}

func TestOneshotTransport_ExitsAfterNRequests(t *testing.T) {
	// Create input with 3 requests, but maxRequests=2
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}`,
		`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}`,
		`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":3}`,
	}, "\n") + "\n"

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(input),
		Writer:      output,
		MaxRequests: 2,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	result, err := transport.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have processed exactly 2 requests
	if result.Processed != 2 {
		t.Errorf("expected 2 processed, got %d", result.Processed)
	}

	// Output should contain exactly 2 responses (2 lines)
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 output lines, got %d: %v", len(lines), lines)
	}

	// Each line should be valid JSON-RPC
	for i, line := range lines {
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
		if resp["jsonrpc"] != "2.0" {
			t.Errorf("line %d missing jsonrpc field", i)
		}
	}
}

func TestOneshotTransport_HandlesEmptyLines(t *testing.T) {
	// Input with empty lines interspersed (3 empty lines total)
	// Note: strings.Join doesn't add trailing newline, and we exit after
	// processing 2 requests so the trailing empty line isn't read
	input := strings.Join([]string{
		"",
		`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}`,
		"",
		"",
		`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}`,
	}, "\n") + "\n"

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(input),
		Writer:      output,
		MaxRequests: 2,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	result, err := transport.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Empty lines should be skipped, not counted
	// We have: "", request1, "", "", request2 = 3 empty lines before hitting
	// maxRequests
	if result.Processed != 2 {
		t.Errorf("expected 2 processed, got %d", result.Processed)
	}
	if result.Skipped != 3 {
		t.Errorf("expected 3 skipped, got %d", result.Skipped)
	}
}

func TestOneshotTransport_HandlesEOFBeforeMaxRequests(t *testing.T) {
	// Only 1 request but maxRequests=5
	input := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}` + "\n"

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(input),
		Writer:      output,
		MaxRequests: 5,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	result, err := transport.Run(context.Background())
	if err != nil {
		t.Fatalf("Run should not error on EOF: %v", err)
	}

	// Should have processed only 1 request (hit EOF)
	if result.Processed != 1 {
		t.Errorf("expected 1 processed, got %d", result.Processed)
	}
}

func TestOneshotTransport_HandlesInvalidJSON(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}`,
		`{not valid json`,
		`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":3}`,
	}, "\n") + "\n"

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(input),
		Writer:      output,
		MaxRequests: 3,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	result, err := transport.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 2 successful, 1 error
	if result.Processed != 2 {
		t.Errorf("expected 2 processed, got %d", result.Processed)
	}
	if result.Errors != 1 {
		t.Errorf("expected 1 error, got %d", result.Errors)
	}

	// Output should contain 3 responses (2 success + 1 error)
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 output lines, got %d", len(lines))
	}

	// Check that the error response is valid JSON-RPC error
	var errResp struct {
		JSONRPC string `json:"jsonrpc"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &errResp); err != nil {
		t.Fatalf("error response is not valid JSON: %v", err)
	}
	if errResp.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", errResp.Error.Code)
	}
}

func TestOneshotTransport_WritesNewlineTerminatedOutput(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}` + "\n"

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      strings.NewReader(input),
		Writer:      output,
		MaxRequests: 1,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	_, err = transport.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Output should end with newline
	outputStr := output.String()
	if !strings.HasSuffix(outputStr, "\n") {
		t.Errorf("output should end with newline: %q", outputStr)
	}

	// And should not have double newlines at end
	if strings.HasSuffix(outputStr, "\n\n") {
		t.Errorf("output should not have double newlines: %q", outputStr)
	}
}

func TestOneshotTransport_RespectsContextCancellation(t *testing.T) {
	// Use a pipe so we can control when data is available
	pr, pw := io.Pipe()

	output := &bytes.Buffer{}

	transport, err := NewOneshotTransport(OneshotConfig{
		MCPServer:   createTestServer(),
		Reader:      pr,
		Writer:      output,
		MaxRequests: 5,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	// Write one request, then close the pipe (simulating EOF)
	go func() {
		pw.Write([]byte(`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":1}` + "\n"))
		// Close after a small delay to simulate EOF before maxRequests
		time.Sleep(50 * time.Millisecond)
		pw.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := transport.Run(ctx)

	// Should have processed 1 request before EOF
	if result.Processed != 1 {
		t.Errorf("expected 1 processed, got %d", result.Processed)
	}

	// Should return nil (clean exit on EOF, not context error)
	if err != nil {
		t.Errorf("expected no error on EOF, got: %v", err)
	}
}

func TestOneshotTransport_PreservesRequestID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // Expected ID in response
	}{
		{
			name:     "numeric_id",
			input:    `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":42}`,
			expected: "42",
		},
		{
			name:     "string_id",
			input:    `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test"}},"id":"abc-123"}`,
			expected: `"abc-123"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}

			transport, err := NewOneshotTransport(OneshotConfig{
				MCPServer:   createTestServer(),
				Reader:      strings.NewReader(tt.input + "\n"),
				Writer:      output,
				MaxRequests: 1,
			})
			if err != nil {
				t.Fatalf("failed to create transport: %v", err)
			}

			_, err = transport.Run(context.Background())
			if err != nil {
				t.Fatalf("Run failed: %v", err)
			}

			// Parse response and check ID
			var resp struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if string(resp.ID) != tt.expected {
				t.Errorf("expected ID %s, got %s", tt.expected, string(resp.ID))
			}
		})
	}
}
