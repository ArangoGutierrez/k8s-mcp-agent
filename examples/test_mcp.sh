#!/bin/bash
# Copyright 2026 k8s-mcp-agent contributors
# SPDX-License-Identifier: Apache-2.0

# Test script for MCP protocol validation
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_BIN="${SCRIPT_DIR}/../bin/agent"

echo "=== Testing k8s-mcp-agent MCP Protocol ==="
echo

# Check if agent binary exists
if [ ! -f "$AGENT_BIN" ]; then
    echo "Error: Agent binary not found at $AGENT_BIN"
    echo "Run 'make agent' first"
    exit 1
fi

echo "1. Testing initialize..."
cat "${SCRIPT_DIR}/initialize.json" | timeout 2s "$AGENT_BIN" 2>&1 | tee /tmp/mcp_test_output.log &
AGENT_PID=$!

sleep 1

echo
echo "2. Testing echo_test tool..."
cat "${SCRIPT_DIR}/echo_test.json"

# Kill agent after test
sleep 1
kill $AGENT_PID 2>/dev/null || true

echo
echo "=== Test Complete ==="
echo "Check /tmp/mcp_test_output.log for full output"

