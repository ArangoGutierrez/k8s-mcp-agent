// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides the MCP server implementation for stdio transport.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with stdio transport.
type Server struct {
	mcpServer  *server.MCPServer
	mode       string
	nvmlClient nvml.Interface
}

// Config holds server configuration.
type Config struct {
	// Mode is the operation mode: "read-only" or "operator"
	Mode string
	// Version is the agent version
	Version string
	// GitCommit is the git commit hash
	GitCommit string
	// NVMLClient is the NVML interface implementation
	NVMLClient nvml.Interface
}

// New creates a new MCP server instance.
func New(cfg Config) (*Server, error) {
	if cfg.Mode == "" {
		cfg.Mode = "read-only"
	}

	if cfg.NVMLClient == nil {
		return nil, fmt.Errorf("NVMLClient is required")
	}

	s := &Server{
		mode:       cfg.Mode,
		nvmlClient: cfg.NVMLClient,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"k8s-mcp-agent",
		cfg.Version,
	)

	// Register the echo test tool
	echoTool := mcp.NewTool("echo_test",
		mcp.WithDescription("Echo test tool for validating MCP protocol"),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("Message to echo back"),
		),
	)
	mcpServer.AddTool(echoTool, s.handleEchoTest)

	// Register GPU inventory tool
	gpuInventoryHandler := tools.NewGPUInventoryHandler(cfg.NVMLClient)
	mcpServer.AddTool(tools.GetGPUInventoryTool(),
		gpuInventoryHandler.Handle)

	s.mcpServer = mcpServer

	log.Printf(`{"level":"info","msg":"MCP server initialized",`+
		`"mode":"%s","version":"%s","commit":"%s"}`,
		cfg.Mode, cfg.Version, cfg.GitCommit)

	return s, nil
}

// Run starts the MCP server and blocks until context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	log.Printf(`{"level":"info","msg":"MCP server starting",`+
		`"transport":"stdio","mode":"%s"}`, s.mode)

	// Run server with stdio transport in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := server.ServeStdio(s.mcpServer); err != nil {
			errCh <- fmt.Errorf("MCP server error: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Printf(`{"level":"info","msg":"MCP server stopping",` +
			`"reason":"context cancelled"}`)
		return s.Shutdown()
	case err := <-errCh:
		log.Printf(`{"level":"error","msg":"MCP server error","error":"%s"}`,
			err)
		return err
	}
}

// Shutdown gracefully shuts down the MCP server.
func (s *Server) Shutdown() error {
	log.Printf(`{"level":"info","msg":"MCP server shutdown initiated"}`)

	// The mcp-go library doesn't expose a shutdown method,
	// so we just log the shutdown

	log.Printf(`{"level":"info","msg":"MCP server shutdown complete"}`)
	return nil
}

// handleEchoTest implements the echo_test tool handler.
func (s *Server) handleEchoTest(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// Extract arguments as map
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("arguments must be an object"), nil
	}

	// Extract message from arguments
	message, ok := args["message"].(string)
	if !ok {
		return mcp.NewToolResultError("message parameter must be a string"),
			nil
	}

	log.Printf(`{"level":"debug","msg":"echo_test invoked",`+
		`"message":"%s"}`, message)

	// Create response
	response := map[string]interface{}{
		"echo":      message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"mode":      s.mode,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf(`{"level":"error","msg":"failed to marshal response",`+
			`"error":"%s"}`, err)
		return mcp.NewToolResultError(
			fmt.Sprintf("failed to marshal response: %s", err)), nil
	}

	log.Printf(`{"level":"info","msg":"echo_test completed",`+
		`"response_size":%d}`, len(jsonBytes))

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// LogToStderr logs a structured message to stderr.
// This is a helper to ensure logs never go to stdout.
func LogToStderr(level, msg string, fields map[string]interface{}) {
	logEntry := map[string]interface{}{
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		logEntry[k] = v
	}

	jsonBytes, err := json.Marshal(logEntry)
	if err != nil {
		// Fallback to simple log
		fmt.Fprintf(os.Stderr, `{"level":"error","msg":"log marshal failed"}`+"\n")
		return
	}

	fmt.Fprintf(os.Stderr, "%s\n", jsonBytes)
}
