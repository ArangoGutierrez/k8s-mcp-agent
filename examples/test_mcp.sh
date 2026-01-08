#!/bin/bash
# Copyright 2026 k8s-gpu-mcp-server contributors
# SPDX-License-Identifier: Apache-2.0

# Test script for MCP protocol validation
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_BIN="${SCRIPT_DIR}/../bin/agent"

echo "=== Testing k8s-gpu-mcp-server MCP Protocol ==="
echo

# Check if agent binary exists
if [ ! -f "$AGENT_BIN" ]; then
    echo "Error: Agent binary not found at $AGENT_BIN"
    echo "Run 'make agent' first"
    exit 1
fi

echo "Testing MCP protocol with JSON-RPC requests..."
echo

# Pipe all requests to agent in one go to avoid race conditions
{
    echo "==> Sending initialize request"
    cat "${SCRIPT_DIR}/initialize.json"
    echo
    
    echo "==> Sending get_gpu_inventory request"
    cat "${SCRIPT_DIR}/gpu_inventory.json"
    echo
} | "$AGENT_BIN" 2>&1 | tee /tmp/mcp_test_output.log

echo
echo "=== Test Complete ==="
echo "Output saved to: /tmp/mcp_test_output.log"
echo
echo "Tip: To test interactively, run:"
echo "  cat examples/gpu_inventory.json | ./bin/agent"
