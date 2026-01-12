// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MCPProtocolVersion is the MCP protocol version used by the gateway.
const MCPProtocolVersion = "2025-06-18"

// MCPRequest represents a JSON-RPC request for MCP.
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// MCPToolCallParams represents parameters for a tools/call request.
type MCPToolCallParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// MCPInitializeParams represents parameters for an initialize request.
type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      MCPClientInfo          `json:"clientInfo"`
}

// MCPClientInfo identifies the client in initialize requests.
type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPResponse represents a JSON-RPC response from MCP.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPToolResult represents the result of a tool call.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in a tool result.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// BuildMCPRequest creates a newline-delimited JSON-RPC request for agent
// invocation. The request contains two messages:
//  1. initialize - establishes the MCP session
//  2. tools/call - invokes the specified tool
//
// Each message is terminated with a newline as required by the stdio protocol.
// Returns the request bytes and any error encountered during marshaling.
func BuildMCPRequest(toolName string, arguments interface{}) ([]byte, error) {
	if toolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}

	// Build initialize request
	initReq := MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: MCPInitializeParams{
			ProtocolVersion: MCPProtocolVersion,
			Capabilities:    map[string]interface{}{},
			ClientInfo: MCPClientInfo{
				Name:    "gateway-proxy",
				Version: "1.0",
			},
		},
		ID: 0,
	}

	// Build tool call request
	toolReq := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: MCPToolCallParams{
			Name:      toolName,
			Arguments: arguments,
		},
		ID: 1,
	}

	// Marshal both requests
	initBytes, err := json.Marshal(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	toolBytes, err := json.Marshal(toolReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool request: %w", err)
	}

	// Concatenate with newlines (stdio protocol requires line-delimited JSON)
	// Format: <init>\n<tool>\n
	var buf bytes.Buffer
	buf.Write(initBytes)
	buf.WriteByte('\n')
	buf.Write(toolBytes)
	buf.WriteByte('\n')

	return buf.Bytes(), nil
}

// ParseStdioResponse extracts the tool result from a multi-line MCP response.
// The response typically contains:
//  1. initialize response (ignored)
//  2. tools/call response (extracted)
//
// Returns the parsed tool result data and any error encountered.
func ParseStdioResponse(response []byte) (interface{}, error) {
	if len(response) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// Split response into individual JSON objects
	objects := SplitJSONObjects(response)
	if len(objects) == 0 {
		return nil, fmt.Errorf("no JSON objects found in response")
	}

	// Use the last object (tool call response)
	lastObj := objects[len(objects)-1]

	// Parse as MCP response
	var mcpResp MCPResponse
	if err := json.Unmarshal(lastObj, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	// Check for JSON-RPC error
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s",
			mcpResp.Error.Code, mcpResp.Error.Message)
	}

	// Parse the result as tool result
	var toolResult MCPToolResult
	if err := json.Unmarshal(mcpResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	// Check for tool error
	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return nil, fmt.Errorf("tool error: %s", toolResult.Content[0].Text)
		}
		return nil, fmt.Errorf("tool error: unknown")
	}

	// Extract text content
	if len(toolResult.Content) == 0 {
		return nil, nil // Empty result
	}

	// Try to parse content as JSON
	text := toolResult.Content[0].Text
	var data interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		// Return as string if not valid JSON
		return text, nil
	}

	return data, nil
}

// SplitJSONObjects splits a byte slice containing multiple JSON objects into
// individual objects. Uses brace counting to find object boundaries.
//
// Note: This parser handles string boundaries and escape sequences, only
// counting braces that appear outside of string literals. It works correctly
// for well-formed MCP JSON-RPC responses. For production use with untrusted
// input, consider a streaming JSON decoder.
func SplitJSONObjects(data []byte) [][]byte {
	var objects [][]byte
	var current []byte
	depth := 0
	inString := false
	escaped := false

	for _, b := range data {
		current = append(current, b)

		// Handle escape sequences in strings
		if escaped {
			escaped = false
			continue
		}

		if b == '\\' && inString {
			escaped = true
			continue
		}

		// Handle string boundaries
		if b == '"' {
			inString = !inString
			continue
		}

		// Only count braces outside strings
		if !inString {
			switch b {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 && len(current) > 0 {
					// Found complete object
					objects = append(objects, bytes.TrimSpace(current))
					current = nil
				}
			}
		}
	}

	return objects
}

// BuildHTTPToolRequest creates a JSON-RPC request for HTTP mode agents.
// Unlike BuildMCPRequest, this does not include init framing since HTTP
// agents maintain persistent sessions.
func BuildHTTPToolRequest(toolName string, arguments interface{}) ([]byte, error) {
	if toolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: MCPToolCallParams{
			Name:      toolName,
			Arguments: arguments,
		},
		ID: 1,
	}

	return json.Marshal(req)
}

// ParseHTTPResponse extracts the tool result from an HTTP mode response.
// HTTP responses contain a single JSON-RPC response (no multi-line parsing).
func ParseHTTPResponse(response []byte) (interface{}, error) {
	if len(response) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(response, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s",
			mcpResp.Error.Code, mcpResp.Error.Message)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(mcpResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return nil, fmt.Errorf("tool error: %s", toolResult.Content[0].Text)
		}
		return nil, fmt.Errorf("tool error: unknown")
	}

	if len(toolResult.Content) == 0 {
		return nil, nil
	}

	text := toolResult.Content[0].Text
	var data interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return text, nil
	}

	return data, nil
}

// ValidateMCPRequest checks if a byte slice contains a valid MCP request.
// Returns an error describing any validation failures.
func ValidateMCPRequest(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty request")
	}

	// Check for trailing newline
	if data[len(data)-1] != '\n' {
		return fmt.Errorf("request must end with newline")
	}

	// Split into objects
	objects := SplitJSONObjects(data)
	if len(objects) == 0 {
		return fmt.Errorf("no JSON objects found")
	}

	// Validate each object is valid JSON-RPC
	for i, obj := range objects {
		var req MCPRequest
		if err := json.Unmarshal(obj, &req); err != nil {
			return fmt.Errorf("object %d: invalid JSON: %w", i, err)
		}
		if req.JSONRPC != "2.0" {
			return fmt.Errorf("object %d: invalid jsonrpc version: %s",
				i, req.JSONRPC)
		}
		if req.Method == "" {
			return fmt.Errorf("object %d: missing method", i)
		}
	}

	return nil
}
