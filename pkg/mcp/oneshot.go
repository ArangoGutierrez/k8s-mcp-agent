// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mark3labs/mcp-go/server"
	"k8s.io/klog/v2"
)

// OneshotTransport provides a stdio transport that exits after processing
// exactly N requests. This is designed for exec-based invocations where
// the process must terminate after handling requests to avoid blocking.
//
// Unlike server.ServeStdio which runs indefinitely, OneshotTransport:
//   - Exits cleanly after processing maxRequests
//   - Exits cleanly on stdin EOF (even if < maxRequests processed)
//   - Skips empty lines (don't count toward maxRequests)
//   - Writes newline-terminated JSON-RPC responses to stdout
type OneshotTransport struct {
	mcpServer   *server.MCPServer
	reader      io.Reader
	writer      io.Writer
	maxRequests int
}

// OneshotConfig holds configuration for the oneshot transport.
type OneshotConfig struct {
	// MCPServer is the MCP server instance to handle requests
	MCPServer *server.MCPServer
	// Reader is the input source (typically os.Stdin)
	Reader io.Reader
	// Writer is the output destination (typically os.Stdout)
	Writer io.Writer
	// MaxRequests is the maximum number of requests to process before exiting
	// Must be > 0
	MaxRequests int
}

// OneshotResult contains statistics from a oneshot run.
type OneshotResult struct {
	// Processed is the number of requests successfully processed
	Processed int
	// Errors is the number of requests that resulted in errors
	Errors int
	// Skipped is the number of empty lines skipped
	Skipped int
}

// NewOneshotTransport creates a new oneshot transport.
// Returns an error if configuration is invalid.
func NewOneshotTransport(cfg OneshotConfig) (*OneshotTransport, error) {
	if cfg.MCPServer == nil {
		return nil, fmt.Errorf("mcpServer is required")
	}
	if cfg.Reader == nil {
		return nil, fmt.Errorf("reader is required")
	}
	if cfg.Writer == nil {
		return nil, fmt.Errorf("writer is required")
	}
	if cfg.MaxRequests < 1 {
		return nil, fmt.Errorf("MaxRequests must be >= 1, got %d",
			cfg.MaxRequests)
	}

	return &OneshotTransport{
		mcpServer:   cfg.MCPServer,
		reader:      cfg.Reader,
		writer:      cfg.Writer,
		maxRequests: cfg.MaxRequests,
	}, nil
}

// scanResult holds the result of a single scan operation.
type scanResult struct {
	line string
	ok   bool
}

// Run processes requests until maxRequests is reached or stdin closes.
// Returns OneshotResult with statistics and any fatal error encountered.
//
// The scanner runs in a separate goroutine to allow context cancellation
// to interrupt blocking reads. This prevents goroutine leaks when the
// context is cancelled while waiting for input.
func (t *OneshotTransport) Run(ctx context.Context) (OneshotResult, error) {
	result := OneshotResult{}

	klog.InfoS("oneshot transport starting", "maxRequests", t.maxRequests)

	scanner := bufio.NewScanner(t.reader)

	// Channel for scan results - buffered to avoid goroutine leak
	lines := make(chan scanResult, 1)

	// Start scanner goroutine
	go func() {
		defer close(lines)
		for scanner.Scan() {
			select {
			case lines <- scanResult{line: scanner.Text(), ok: true}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Process lines with context awareness
	for {
		select {
		case <-ctx.Done():
			klog.InfoS("oneshot transport cancelled")
			return result, ctx.Err()

		case scan, ok := <-lines:
			if !ok {
				// Scanner closed (EOF or error)
				if err := scanner.Err(); err != nil {
					return result, fmt.Errorf("stdin read error: %w", err)
				}
				// Normal EOF
				klog.InfoS("oneshot transport completed",
					"processed", result.Processed,
					"errors", result.Errors,
					"skipped", result.Skipped)
				return result, nil
			}

			line := scan.line

			// Skip empty lines (don't count toward maxRequests)
			if line == "" {
				result.Skipped++
				continue
			}

			// Process the request
			if err := t.processRequest(ctx, line); err != nil {
				result.Errors++
				klog.V(2).InfoS("request processing error",
					"error", err, "processed", result.Processed)
			} else {
				result.Processed++
			}

			// Check if we've reached the limit
			if result.Processed >= t.maxRequests {
				klog.InfoS("oneshot transport completed",
					"processed", result.Processed,
					"errors", result.Errors,
					"skipped", result.Skipped)
				return result, nil
			}
		}
	}
}

// processRequest handles a single JSON-RPC request line.
func (t *OneshotTransport) processRequest(ctx context.Context, line string) error {
	// Parse request to extract method and ID for logging/error handling
	var req jsonRPCRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		// Write parse error response
		t.writeErrorResponse(nil, -32700, "Parse error: "+err.Error())
		return fmt.Errorf("parse error: %w", err)
	}

	klog.V(4).InfoS("processing request", "method", req.Method, "id", formatID(req.ID))

	// Handle the request through the MCP server
	// HandleMessage returns mcp.JSONRPCMessage (errors are embedded in response)
	response := t.mcpServer.HandleMessage(ctx, []byte(line))

	// Marshal the response to JSON
	respBytes, err := json.Marshal(response)
	if err != nil {
		t.writeErrorResponse(req.ID, -32603, "marshal error: "+err.Error())
		return fmt.Errorf("marshal error: %w", err)
	}

	// Write response (newline-terminated)
	if _, err := fmt.Fprintf(t.writer, "%s\n", respBytes); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// writeErrorResponse writes a JSON-RPC error response.
func (t *OneshotTransport) writeErrorResponse(id json.RawMessage, code int,
	message string) {

	resp := jsonRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: jsonRPCError{
			Code:    code,
			Message: message,
		},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		// Last resort: write a hardcoded error
		if _, writeErr := fmt.Fprintf(t.writer, `{"jsonrpc":"2.0","id":null,"error":`+
			`{"code":-32603,"message":"marshal error"}}`+"\n"); writeErr != nil {
			klog.ErrorS(writeErr, "failed to write fallback error")
		}
		return
	}

	if _, err := fmt.Fprintf(t.writer, "%s\n", respBytes); err != nil {
		klog.ErrorS(err, "failed to write error response")
	}
}

// jsonRPCRequest is a minimal JSON-RPC request structure for parsing.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCErrorResponse is a JSON-RPC error response.
type jsonRPCErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   jsonRPCError    `json:"error"`
}

// jsonRPCError is the error object in a JSON-RPC error response.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// formatID formats a JSON-RPC ID for logging.
func formatID(id json.RawMessage) string {
	if id == nil {
		return "null"
	}
	return string(id)
}
