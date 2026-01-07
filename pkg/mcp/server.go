// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides the MCP server implementation for stdio and HTTP
// transports.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/gateway"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TransportType defines the transport mode for the MCP server.
type TransportType string

const (
	// TransportStdio uses stdin/stdout for communication (default).
	TransportStdio TransportType = "stdio"
	// TransportHTTP uses HTTP/SSE for communication.
	TransportHTTP TransportType = "http"
)

// Server wraps the MCP server with configurable transport.
type Server struct {
	mcpServer   *server.MCPServer
	mode        string
	nvmlClient  nvml.Interface
	transport   TransportType
	httpAddr    string
	version     string
	gatewayMode bool
	k8sClient   *k8s.Client
}

// Config holds server configuration.
type Config struct {
	// Mode is the operation mode: "read-only" or "operator"
	Mode string
	// Version is the agent version
	Version string
	// GitCommit is the git commit hash
	GitCommit string
	// NVMLClient is the NVML interface implementation (nil in gateway mode)
	NVMLClient nvml.Interface
	// Transport is the transport mode: "stdio" or "http"
	Transport TransportType
	// HTTPAddr is the HTTP listen address (e.g., "0.0.0.0:8080")
	HTTPAddr string
	// GatewayMode enables routing to node agents via K8s pod exec
	GatewayMode bool
	// Namespace for GPU agent pods (gateway mode only)
	Namespace string
	// K8sClient is the Kubernetes client (gateway mode only)
	K8sClient *k8s.Client
}

// New creates a new MCP server instance.
func New(cfg Config) (*Server, error) {
	if cfg.Mode == "" {
		cfg.Mode = "read-only"
	}

	// Gateway mode requires K8s client, regular mode requires NVML client
	if cfg.GatewayMode {
		if cfg.K8sClient == nil {
			return nil, fmt.Errorf("K8sClient is required for gateway mode")
		}
	} else {
		if cfg.NVMLClient == nil {
			return nil, fmt.Errorf("NVMLClient is required")
		}
	}

	// Default to stdio transport
	if cfg.Transport == "" {
		cfg.Transport = TransportStdio
	}

	// Validate HTTPAddr is set when using HTTP transport
	if cfg.Transport == TransportHTTP && cfg.HTTPAddr == "" {
		return nil, fmt.Errorf("HTTPAddr is required for HTTP transport")
	}

	s := &Server{
		mode:        cfg.Mode,
		nvmlClient:  cfg.NVMLClient,
		transport:   cfg.Transport,
		httpAddr:    cfg.HTTPAddr,
		version:     cfg.Version,
		gatewayMode: cfg.GatewayMode,
		k8sClient:   cfg.K8sClient,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"k8s-gpu-mcp-server",
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

	if cfg.GatewayMode {
		// Gateway mode: register all tools with proxy handlers

		// list_gpu_nodes - handled directly by gateway (no proxy needed)
		listNodesHandler := tools.NewListGPUNodesHandler(cfg.K8sClient)
		mcpServer.AddTool(tools.GetListGPUNodesTool(), listNodesHandler.Handle)

		// GPU tools - proxied to node agents
		inventoryProxy := gateway.NewProxyHandler(cfg.K8sClient,
			"get_gpu_inventory")
		mcpServer.AddTool(tools.GetGPUInventoryTool(), inventoryProxy.Handle)

		healthProxy := gateway.NewProxyHandler(cfg.K8sClient, "get_gpu_health")
		mcpServer.AddTool(tools.GetGPUHealthTool(), healthProxy.Handle)

		xidProxy := gateway.NewProxyHandler(cfg.K8sClient, "analyze_xid_errors")
		mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidProxy.Handle)

		log.Printf(`{"level":"info","msg":"MCP server initialized",`+
			`"mode":"%s","gateway":true,"namespace":"%s",`+
			`"tools":["list_gpu_nodes","get_gpu_inventory",`+
			`"get_gpu_health","analyze_xid_errors"],`+
			`"version":"%s","commit":"%s"}`,
			cfg.Mode, cfg.Namespace, cfg.Version, cfg.GitCommit)
	} else {
		// Regular mode: register GPU tools with NVML
		gpuInventoryHandler := tools.NewGPUInventoryHandler(cfg.NVMLClient)
		mcpServer.AddTool(tools.GetGPUInventoryTool(),
			gpuInventoryHandler.Handle)

		xidHandler := tools.NewAnalyzeXIDHandler(cfg.NVMLClient)
		mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidHandler.Handle)

		healthHandler := tools.NewGPUHealthHandler(cfg.NVMLClient)
		mcpServer.AddTool(tools.GetGPUHealthTool(), healthHandler.Handle)

		log.Printf(`{"level":"info","msg":"MCP server initialized",`+
			`"mode":"%s","gateway":false,"version":"%s","commit":"%s"}`,
			cfg.Mode, cfg.Version, cfg.GitCommit)
	}

	s.mcpServer = mcpServer

	return s, nil
}

// Run starts the MCP server with the configured transport.
func (s *Server) Run(ctx context.Context) error {
	switch s.transport {
	case TransportHTTP:
		return s.runHTTP(ctx)
	default:
		return s.runStdio(ctx)
	}
}

// runStdio runs the server with stdio transport.
func (s *Server) runStdio(ctx context.Context) error {
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

// runHTTP runs the server with HTTP transport.
func (s *Server) runHTTP(ctx context.Context) error {
	log.Printf(`{"level":"info","msg":"MCP server starting",`+
		`"transport":"http","addr":"%s","mode":"%s"}`, s.httpAddr, s.mode)

	httpServer := NewHTTPServer(s.mcpServer, s.httpAddr, s.version)
	return httpServer.ListenAndServe(ctx)
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
