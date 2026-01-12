// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildMCPRequest_Valid(t *testing.T) {
	data, err := BuildMCPRequest("get_gpu_inventory", map[string]interface{}{
		"filter": "all",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it ends with newline
	if data[len(data)-1] != '\n' {
		t.Errorf("request should end with newline")
	}

	// Verify it contains two objects separated by newline
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Verify first line is initialize
	var initReq MCPRequest
	if err := json.Unmarshal([]byte(lines[0]), &initReq); err != nil {
		t.Fatalf("failed to parse init request: %v", err)
	}
	if initReq.Method != "initialize" {
		t.Errorf("expected method 'initialize', got %s", initReq.Method)
	}
	if initReq.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %s", initReq.JSONRPC)
	}

	// Verify second line is tools/call
	var toolReq MCPRequest
	if err := json.Unmarshal([]byte(lines[1]), &toolReq); err != nil {
		t.Fatalf("failed to parse tool request: %v", err)
	}
	if toolReq.Method != "tools/call" {
		t.Errorf("expected method 'tools/call', got %s", toolReq.Method)
	}
}

func TestBuildMCPRequest_EmptyToolName(t *testing.T) {
	_, err := BuildMCPRequest("", nil)
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}
	if !strings.Contains(err.Error(), "toolName") {
		t.Errorf("error should mention toolName: %v", err)
	}
}

func TestBuildMCPRequest_NilArguments(t *testing.T) {
	data, err := BuildMCPRequest("test_tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still produce valid request
	if err := ValidateMCPRequest(data); err != nil {
		t.Errorf("request should be valid: %v", err)
	}
}

func TestBuildMCPRequest_HasTrailingNewline(t *testing.T) {
	data, err := BuildMCPRequest("test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(string(data), "\n") {
		t.Error("request must end with newline for stdio protocol")
	}
}

func TestBuildMCPRequest_HasNewlineBetweenMessages(t *testing.T) {
	data, err := BuildMCPRequest("test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count newlines - should be exactly 2 (one after each message)
	count := strings.Count(string(data), "\n")
	if count != 2 {
		t.Errorf("expected 2 newlines, got %d", count)
	}
}

func TestParseStdioResponse_ValidToolResult(t *testing.T) {
	// Simulate response from agent: init response + tool response
	response := `{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":"2025-06-18"}}
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"status\":\"ok\",\"count\":2}"}]}}`

	data, err := ParseStdioResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse the JSON content
	result, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", data)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
	if result["count"] != float64(2) {
		t.Errorf("expected count 2, got %v", result["count"])
	}
}

func TestParseStdioResponse_EmptyResponse(t *testing.T) {
	_, err := ParseStdioResponse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention 'empty': %v", err)
	}
}

func TestParseStdioResponse_NoJSONObjects(t *testing.T) {
	_, err := ParseStdioResponse([]byte("not json at all"))
	if err == nil {
		t.Fatal("expected error for non-JSON response")
	}
}

func TestParseStdioResponse_MalformedJSON(t *testing.T) {
	_, err := ParseStdioResponse([]byte(`{"incomplete`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseStdioResponse_MCPError(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"internal error"}}`

	_, err := ParseStdioResponse([]byte(response))
	if err == nil {
		t.Fatal("expected error for MCP error response")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("error should contain message: %v", err)
	}
	if !strings.Contains(err.Error(), "-32603") {
		t.Errorf("error should contain code: %v", err)
	}
}

func TestParseStdioResponse_ToolError(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"something went wrong"}],"isError":true}}`

	_, err := ParseStdioResponse([]byte(response))
	if err == nil {
		t.Fatal("expected error for tool error")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error should contain tool error message: %v", err)
	}
}

func TestParseStdioResponse_NonJSONContent(t *testing.T) {
	// Tool returns plain text, not JSON
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Hello, World!"}]}}`

	data, err := ParseStdioResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return as string
	str, ok := data.(string)
	if !ok {
		t.Fatalf("expected string, got %T", data)
	}
	if str != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", str)
	}
}

func TestParseStdioResponse_EmptyContent(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`

	data, err := ParseStdioResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data != nil {
		t.Errorf("expected nil for empty content, got %v", data)
	}
}

func TestSplitJSONObjects_MultipleObjects(t *testing.T) {
	input := `{"a":1}{"b":2}{"c":3}`

	objects := SplitJSONObjects([]byte(input))
	if len(objects) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objects))
	}

	// Verify each is valid JSON
	for i, obj := range objects {
		var m map[string]interface{}
		if err := json.Unmarshal(obj, &m); err != nil {
			t.Errorf("object %d is not valid JSON: %v", i, err)
		}
	}
}

func TestSplitJSONObjects_WithNewlines(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n"

	objects := SplitJSONObjects([]byte(input))
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
}

func TestSplitJSONObjects_NestedBraces(t *testing.T) {
	input := `{"outer":{"inner":{"deep":1}}}`

	objects := SplitJSONObjects([]byte(input))
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	var m map[string]interface{}
	if err := json.Unmarshal(objects[0], &m); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
}

func TestSplitJSONObjects_BracesInStrings(t *testing.T) {
	// Braces inside strings should not affect parsing
	input := `{"msg":"contains { and } chars"}`

	objects := SplitJSONObjects([]byte(input))
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	var m map[string]interface{}
	if err := json.Unmarshal(objects[0], &m); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if m["msg"] != "contains { and } chars" {
		t.Errorf("unexpected msg: %v", m["msg"])
	}
}

func TestSplitJSONObjects_EscapedQuotes(t *testing.T) {
	// Escaped quotes should not end the string
	input := `{"msg":"say \"hello\""}`

	objects := SplitJSONObjects([]byte(input))
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	var m map[string]interface{}
	if err := json.Unmarshal(objects[0], &m); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
}

func TestSplitJSONObjects_Empty(t *testing.T) {
	objects := SplitJSONObjects([]byte{})
	if len(objects) != 0 {
		t.Errorf("expected 0 objects for empty input, got %d", len(objects))
	}
}

func TestSplitJSONObjects_NoObjects(t *testing.T) {
	objects := SplitJSONObjects([]byte("just some text"))
	if len(objects) != 0 {
		t.Errorf("expected 0 objects for non-JSON, got %d", len(objects))
	}
}

func TestValidateMCPRequest_Valid(t *testing.T) {
	data, _ := BuildMCPRequest("test", nil)

	err := ValidateMCPRequest(data)
	if err != nil {
		t.Errorf("expected valid request, got error: %v", err)
	}
}

func TestValidateMCPRequest_Empty(t *testing.T) {
	err := ValidateMCPRequest([]byte{})
	if err == nil {
		t.Fatal("expected error for empty request")
	}
}

func TestValidateMCPRequest_NoTrailingNewline(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)

	err := ValidateMCPRequest(data)
	if err == nil {
		t.Fatal("expected error for missing newline")
	}
	if !strings.Contains(err.Error(), "newline") {
		t.Errorf("error should mention newline: %v", err)
	}
}

func TestValidateMCPRequest_InvalidJSON(t *testing.T) {
	data := []byte("{not valid json}\n")

	err := ValidateMCPRequest(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateMCPRequest_MissingMethod(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1}` + "\n")

	err := ValidateMCPRequest(data)
	if err == nil {
		t.Fatal("expected error for missing method")
	}
	if !strings.Contains(err.Error(), "method") {
		t.Errorf("error should mention method: %v", err)
	}
}

func TestValidateMCPRequest_WrongJSONRPCVersion(t *testing.T) {
	data := []byte(`{"jsonrpc":"1.0","method":"test","id":1}` + "\n")

	err := ValidateMCPRequest(data)
	if err == nil {
		t.Fatal("expected error for wrong version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention version: %v", err)
	}
}

// Tests for HTTP mode request/response functions

func TestBuildHTTPToolRequest_Valid(t *testing.T) {
	data, err := BuildHTTPToolRequest("get_gpu_inventory", map[string]interface{}{
		"filter": "all",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse and verify structure
	var req MCPRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to parse request: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got %s", req.JSONRPC)
	}
	if req.Method != "tools/call" {
		t.Errorf("expected method 'tools/call', got %s", req.Method)
	}
	if req.ID != float64(1) {
		t.Errorf("expected ID 1, got %v", req.ID)
	}
}

func TestBuildHTTPToolRequest_EmptyToolName(t *testing.T) {
	_, err := BuildHTTPToolRequest("", nil)
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}
	if !strings.Contains(err.Error(), "toolName") {
		t.Errorf("error should mention toolName: %v", err)
	}
}

func TestBuildHTTPToolRequest_NilArguments(t *testing.T) {
	data, err := BuildHTTPToolRequest("test_tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce valid JSON
	var req MCPRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Errorf("request should be valid JSON: %v", err)
	}
}

func TestBuildHTTPToolRequest_SingleObject(t *testing.T) {
	data, err := BuildHTTPToolRequest("test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// HTTP mode should be a single JSON object (no newline-delimited framing)
	objects := SplitJSONObjects(data)
	if len(objects) != 1 {
		t.Errorf("expected 1 JSON object, got %d", len(objects))
	}
}

func TestParseHTTPResponse_ValidToolResult(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"status\":\"ok\",\"count\":2}"}]}}`

	data, err := ParseHTTPResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", data)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
	if result["count"] != float64(2) {
		t.Errorf("expected count 2, got %v", result["count"])
	}
}

func TestParseHTTPResponse_EmptyResponse(t *testing.T) {
	_, err := ParseHTTPResponse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention 'empty': %v", err)
	}
}

func TestParseHTTPResponse_MCPError(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"internal error"}}`

	_, err := ParseHTTPResponse([]byte(response))
	if err == nil {
		t.Fatal("expected error for MCP error response")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("error should contain message: %v", err)
	}
}

func TestParseHTTPResponse_ToolError(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"something went wrong"}],"isError":true}}`

	_, err := ParseHTTPResponse([]byte(response))
	if err == nil {
		t.Fatal("expected error for tool error")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error should contain tool error message: %v", err)
	}
}

func TestParseHTTPResponse_NonJSONContent(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Hello, World!"}]}}`

	data, err := ParseHTTPResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := data.(string)
	if !ok {
		t.Fatalf("expected string, got %T", data)
	}
	if str != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", str)
	}
}

func TestParseHTTPResponse_EmptyContent(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`

	data, err := ParseHTTPResponse([]byte(response))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data != nil {
		t.Errorf("expected nil for empty content, got %v", data)
	}
}

func TestParseHTTPResponse_MalformedJSON(t *testing.T) {
	_, err := ParseHTTPResponse([]byte(`{"incomplete`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// Benchmark for performance-critical path
func BenchmarkSplitJSONObjects(b *testing.B) {
	// Typical MCP response
	input := []byte(`{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":"2025-06-18","capabilities":{}}}
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"devices\":[{\"name\":\"Tesla T4\"}]}"}]}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SplitJSONObjects(input)
	}
}

func BenchmarkParseStdioResponse(b *testing.B) {
	input := []byte(`{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":"2025-06-18"}}
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"status\":\"ok\"}"}]}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseStdioResponse(input)
	}
}

func BenchmarkBuildHTTPToolRequest(b *testing.B) {
	args := map[string]interface{}{"filter": "all"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BuildHTTPToolRequest("get_gpu_inventory", args)
	}
}

func BenchmarkParseHTTPResponse(b *testing.B) {
	input := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"status\":\"ok\"}"}]}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseHTTPResponse(input)
	}
}
