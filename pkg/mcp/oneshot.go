// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/mark3labs/mcp-go/server"
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

// Run processes requests until maxRequests is reached or stdin closes.
// Returns OneshotResult with statistics and any fatal error encountered.
func (t *OneshotTransport) Run(ctx context.Context) (OneshotResult, error) {
	result := OneshotResult{}

	log.Printf(`{"level":"info","msg":"oneshot transport starting",`+
		`"max_requests":%d}`, t.maxRequests)

	scanner := bufio.NewScanner(t.reader)

	for scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			log.Printf(`{"level":"info","msg":"oneshot transport cancelled"}`)
			return result, ctx.Err()
		default:
		}

		line := scanner.Text()

		// Skip empty lines (don't count toward maxRequests)
		if line == "" {
			result.Skipped++
			continue
		}

		// Process the request
		if err := t.processRequest(ctx, line); err != nil {
			result.Errors++
			log.Printf(`{"level":"warn","msg":"request processing error",`+
				`"error":"%v","processed":%d}`, err, result.Processed)
		} else {
			result.Processed++
		}

		// Check if we've reached the limit
		if result.Processed >= t.maxRequests {
			break
		}
	}

	// Check for scanner errors (I/O errors, not EOF)
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("stdin read error: %w", err)
	}

	log.Printf(`{"level":"info","msg":"oneshot transport completed",`+
		`"processed":%d,"errors":%d,"skipped":%d}`,
		result.Processed, result.Errors, result.Skipped)

	return result, nil
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

	log.Printf(`{"level":"debug","msg":"processing request",`+
		`"method":"%s","id":%s}`, req.Method, formatID(req.ID))

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
			log.Printf(`{"level":"error","msg":"failed to write fallback error",`+
				`"error":"%v"}`, writeErr)
		}
		return
	}

	if _, err := fmt.Fprintf(t.writer, "%s\n", respBytes); err != nil {
		log.Printf(`{"level":"error","msg":"failed to write error response",`+
			`"error":"%v"}`, err)
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
