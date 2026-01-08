# MCP Usage Guide

Learn how to interact with `k8s-gpu-mcp-server` using the Model Context Protocol.

## Table of Contents

- [Introduction](#introduction)
- [MCP Protocol Basics](#mcp-protocol-basics)
- [Using with Claude Desktop](#using-with-claude-desktop)
- [Using with Cursor IDE](#using-with-cursor-ide)
- [Manual JSON-RPC](#manual-json-rpc)
- [Available Tools](#available-tools)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)

## Introduction

The Model Context Protocol (MCP) is an open protocol that enables AI
assistants to securely interact with external tools and data sources.

`k8s-gpu-mcp-server` implements MCP over **stdio** (standard input/output),
making it compatible with:

- **Claude Desktop** - Anthropic's AI assistant
- **Cursor IDE** - AI-powered code editor
- **Custom MCP clients** - Any tool that speaks JSON-RPC 2.0

## MCP Protocol Basics

### Protocol Version

**Current:** `2025-06-18`

Always specify the protocol version in initialization:

```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {
      "name": "your-client",
      "version": "1.0.0"
    }
  },
  "id": 0
}
```

### Message Format

All messages use **JSON-RPC 2.0**:

```json
{
  "jsonrpc": "2.0",          // Protocol version
  "method": "method_name",    // Method to call
  "params": {...},            // Parameters (optional)
  "id": 123                   // Request ID (for matching responses)
}
```

### Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 123,                  // Matches request ID
  "result": {...}             // Success response
}

// OR

{
  "jsonrpc": "2.0",
  "id": 123,
  "error": {                  // Error response
    "code": -32600,
    "message": "Error description"
  }
}
```

## Using with Claude Desktop

### Configuration

Add to your Claude Desktop MCP configuration:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

**Linux:** `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "k8s-gpu-agent": {
      "command": "kubectl",
      "args": [
        "debug",
        "node/gpu-node-5",
        "--image=ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest",
        "--profile=sysadmin",
        "--",
        "/agent",
        "--mode=read-only",
        "--nvml-mode=real"
      ]
    }
  }
}
```

### Usage in Claude

Once configured, you can ask Claude:

```
You: "Check the GPU temperatures on node gpu-node-5"

Claude: [Calls get_gpu_inventory tool]
        
        "GPU 0 (Tesla T4) is at 29°C - within normal range.
         Memory usage is at 3% (447MB / 15GB).
         The GPU is idle (0% utilization)."
```

### Example Prompts

```
"What GPUs are available on this node?"
"Show me the memory usage for all GPUs"
"Is GPU 2 thermal throttling?"
"Check for XID errors on the GPUs" (M2 Phase 2)
"What's the NVLink topology?" (M2 Phase 4)
```

## Using with Cursor IDE

### Setup

1. Add to Cursor MCP configuration
2. Set agent as available tool
3. Use in AI chat

### Configuration

```json
{
  "mcp": {
    "k8s-gpu-agent": {
      "command": "/path/to/bin/agent",
      "args": ["--mode=read-only", "--nvml-mode=mock"]
    }
  }
}
```

### Usage

```typescript
// Ask Cursor AI:
// "Query the GPU inventory using k8s-gpu-mcp-server"

// Cursor will:
// 1. Initialize MCP session
// 2. Call get_gpu_inventory tool
// 3. Display results in chat
```

## Manual JSON-RPC

For debugging or custom clients:

### 1. Initialize Session

```bash
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}},"id":0}' | ./bin/agent
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 0,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "tools": {
        "listChanged": true
      }
    },
    "serverInfo": {
      "name": "k8s-gpu-mcp-server",
      "version": "0.1.0-alpha"
    }
  }
}
```

### 2. List Available Tools

```bash
echo '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}' | ./bin/agent
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "get_gpu_inventory",
        "description": "Returns static hardware inventory for all GPU devices...",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      },
      {
        "name": "get_gpu_health",
        "description": "GPU health monitoring with scoring...",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      },
      {
        "name": "analyze_xid_errors",
        "description": "Parse GPU XID error codes from kernel logs...",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      }
    ]
  }
}
```

### 3. Call a Tool

```bash
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":2}' | ./bin/agent --nvml-mode=real
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"status\":\"success\",\"device_count\":1,\"devices\":[...]}"
      }
    ]
  }
}
```

## Available Tools

### get_gpu_inventory

**Purpose:** Get complete GPU hardware inventory with telemetry

**Arguments:** None

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_gpu_inventory",
    "arguments": {}
  },
  "id": 2
}
```

**Response:**
```json
{
  "status": "success",
  "device_count": 1,
  "devices": [
    {
      "Index": 0,
      "Name": "Tesla T4",
      "UUID": "GPU-d129fc5b-2d51-cec7-d985-49168c12716f",
      "BusID": "0000:00:1E.0",
      "MemoryTotal": 16106127360,
      "MemoryUsed": 469041152,
      "MemoryFree": 15637086208,
      "Temperature": 29,
      "PowerUsage": 13929,
      "GPUUtil": 0,
      "MemoryUtil": 0
    }
  ]
}
```

**Field Descriptions:**
- `MemoryTotal/Used/Free`: Bytes
- `Temperature`: Celsius
- `PowerUsage`: Milliwatts
- `GPUUtil`: Percentage (0-100)
- `MemoryUtil`: Percentage (0-100)

## Error Handling

### Error Types

**1. Protocol Errors** (JSON-RPC level)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32700,
    "message": "Parse error"
  }
}
```

**Common codes:**
- `-32700`: Parse error (invalid JSON)
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params

**2. Tool Errors** (Application level)
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"error\":\"NVML not initialized\"}"
      }
    ],
    "isError": true
  }
}
```

### Handling Errors in Clients

```python
# Python example
response = call_mcp_tool("get_gpu_inventory", {})

if response.get("error"):
    # JSON-RPC protocol error
    print(f"Protocol error: {response['error']['message']}")
elif response.get("result", {}).get("isError"):
    # Tool execution error
    content = response["result"]["content"][0]["text"]
    print(f"Tool error: {content}")
else:
    # Success
    data = json.loads(response["result"]["content"][0]["text"])
    print(f"GPU Count: {data['device_count']}")
```

## Best Practices

### 1. Always Initialize

Send `initialize` before any other requests:

```bash
# ✅ Good
echo '{"jsonrpc":"2.0","method":"initialize",...}' | ./bin/agent
echo '{"jsonrpc":"2.0","method":"tools/call",...}' | ./bin/agent

# ❌ Bad - will fail
echo '{"jsonrpc":"2.0","method":"tools/call",...}' | ./bin/agent
```

### 2. Use Timeouts

The agent blocks on stdin. Always use timeouts:

```bash
# With timeout
timeout 30s ./bin/agent --nvml-mode=real < requests.jsonl

# In Python
process = subprocess.Popen(..., stdin=PIPE, stdout=PIPE, timeout=30)
```

### 3. Handle Stdio Separation

```bash
# ✅ Good: Separate stdout (protocol) and stderr (logs)
./bin/agent 2>agent.log | process_responses.sh

# ✅ Good: View logs separately
cat agent.log | jq .

# ❌ Bad: Mixing stdout and stderr
./bin/agent 2>&1  # Logs corrupt JSON-RPC responses
```

### 4. Validate Responses

Always check for both JSON-RPC errors and tool errors:

```javascript
const response = JSON.parse(stdout);

if (response.error) {
  // Protocol error
  throw new Error(`MCP Error: ${response.error.message}`);
}

if (response.result?.isError) {
  // Tool error
  const errorText = response.result.content[0].text;
  throw new Error(`Tool Error: ${errorText}`);
}

// Success
const data = JSON.parse(response.result.content[0].text);
```

### 5. Use Mock for Development

```bash
# During development, use mock mode
./bin/agent --nvml-mode=mock

# Switch to real for production
./bin/agent --nvml-mode=real
```

## Integration Examples

### Python Client

```python
import json
import subprocess

def call_mcp_tool(tool_name, arguments):
    """Call an MCP tool and return the result."""
    
    # Start agent process
    proc = subprocess.Popen(
        ['./bin/agent', '--nvml-mode=real'],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Initialize
    init_req = {
        "jsonrpc": "2.0",
        "method": "initialize",
        "params": {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {"name": "python-client", "version": "1.0"}
        },
        "id": 0
    }
    proc.stdin.write(json.dumps(init_req) + '\n')
    proc.stdin.flush()
    
    # Read init response
    init_response = json.loads(proc.stdout.readline())
    
    # Call tool
    tool_req = {
        "jsonrpc": "2.0",
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": arguments
        },
        "id": 1
    }
    proc.stdin.write(json.dumps(tool_req) + '\n')
    proc.stdin.flush()
    
    # Read tool response
    tool_response = json.loads(proc.stdout.readline())
    
    proc.terminate()
    
    return tool_response

# Usage
response = call_mcp_tool("get_gpu_inventory", {})
data = json.loads(response["result"]["content"][0]["text"])
print(f"Found {data['device_count']} GPUs")
```

### Bash Client

```bash
#!/bin/bash
# mcp_client.sh - Simple MCP client in bash

AGENT="./bin/agent"
MODE="--nvml-mode=real"

# Function to call MCP tool
call_tool() {
    local tool_name="$1"
    local args="$2"
    
    {
        # Initialize
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\",\"capabilities\":{},\"clientInfo\":{\"name\":\"bash-client\",\"version\":\"1.0\"}},\"id\":0}"
        
        # Call tool
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"$tool_name\",\"arguments\":$args},\"id\":1}"
    } | $AGENT $MODE 2>/tmp/agent.log | tail -1
}

# Usage
result=$(call_tool "get_gpu_inventory" "{}")
echo "$result" | jq -r '.result.content[0].text' | jq .
```

### Go Client

```go
package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os/exec"
)

type MCPClient struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
}

func NewMCPClient(nvmlMode string) (*MCPClient, error) {
    cmd := exec.Command("./bin/agent", "--nvml-mode="+nvmlMode)
    
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    client := &MCPClient{cmd: cmd, stdin: stdin, stdout: stdout}
    
    // Initialize
    initReq := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "initialize",
        "params": map[string]interface{}{
            "protocolVersion": "2025-06-18",
            "capabilities":    map[string]interface{}{},
            "clientInfo": map[string]interface{}{
                "name": "go-client", "version": "1.0",
            },
        },
        "id": 0,
    }
    
    json.NewEncoder(client.stdin).Encode(initReq)
    
    // Read init response
    scanner := bufio.NewScanner(client.stdout)
    scanner.Scan()
    
    return client, nil
}

func (c *MCPClient) CallTool(name string, args map[string]interface{}) (map[string]interface{}, error) {
    req := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name":      name,
            "arguments": args,
        },
        "id": 1,
    }
    
    json.NewEncoder(c.stdin).Encode(req)
    
    scanner := bufio.NewScanner(c.stdout)
    scanner.Scan()
    
    var response map[string]interface{}
    json.Unmarshal(scanner.Bytes(), &response)
    
    return response, nil
}

// Usage
client, _ := NewMCPClient("real")
response, _ := client.CallTool("get_gpu_inventory", map[string]interface{}{})
fmt.Println(response)
```

## Advanced Usage

### Streaming Multiple Requests

Send multiple tool calls in sequence:

```bash
{
  echo '{"jsonrpc":"2.0","method":"initialize",...,"id":0}'
  echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory",...},"id":1}'
  echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_health",...},"id":2}'
} | ./bin/agent --nvml-mode=real
```

Each request gets a response with matching `id`.

### Context and Cancellation

The agent respects context cancellation:

```bash
# Agent stops gracefully on SIGINT/SIGTERM
./bin/agent & 
PID=$!

# Send requests...

# Stop agent
kill $PID  # Graceful shutdown
```

### Logging

Logs go to stderr in structured JSON format:

```bash
# Capture logs separately
./bin/agent --log-level=debug 2>agent.log | your_client.sh

# View logs
cat agent.log | jq '.level,.msg,.tool'
```

**Log Levels:**
- `debug`: Verbose (all tool calls, parameters)
- `info`: Normal (tool invocations, completion)
- `warn`: Warnings (slow operations, retries)
- `error`: Errors (NVML failures, invalid requests)

## Debugging

### Enable Debug Logging

```bash
./bin/agent --log-level=debug --nvml-mode=mock 2>&1 | tee debug.log
```

### Validate JSON-RPC Messages

```bash
# Pretty-print requests
echo '{"jsonrpc":"2.0",...}' | jq . | ./bin/agent

# Validate responses
./bin/agent < requests.jsonl | jq .
```

### Test Protocol Compliance

```bash
# Use MCP Inspector (if available)
mcp-inspector ./bin/agent --nvml-mode=mock

# Or manual validation
./bin/agent < test_requests.jsonl > responses.jsonl
cat responses.jsonl | jq -e '.jsonrpc == "2.0"'
```

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Claude Desktop MCP Configuration](https://docs.anthropic.com/claude/docs/model-context-protocol)
- [Project Examples](../examples/)

