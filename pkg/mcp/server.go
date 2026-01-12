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

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/gateway"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/tools"
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
	oneshot     int
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
	// Oneshot exits after processing N requests (0=disabled)
	Oneshot int
	// RoutingMode specifies gateway routing: "http" (default) or "exec"
	RoutingMode string
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
		oneshot:     cfg.Oneshot,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"k8s-gpu-mcp-server",
		cfg.Version,
	)

	if cfg.GatewayMode {
		// Gateway mode: register GPU tools with proxy handlers
		// Note: list_gpu_nodes was consolidated into get_gpu_inventory
		// which now returns cluster summary with node info

		// Determine routing mode option
		var routerOpts []gateway.RouterOption
		if cfg.RoutingMode == "exec" {
			routerOpts = append(routerOpts,
				gateway.WithRoutingMode(gateway.RoutingModeExec))
		} else {
			// Default to HTTP
			routerOpts = append(routerOpts,
				gateway.WithRoutingMode(gateway.RoutingModeHTTP))
		}

		inventoryProxy := gateway.NewProxyHandler(cfg.K8sClient,
			"get_gpu_inventory", routerOpts...)
		mcpServer.AddTool(tools.GetGPUInventoryTool(), inventoryProxy.Handle)

		healthProxy := gateway.NewProxyHandler(cfg.K8sClient,
			"get_gpu_health", routerOpts...)
		mcpServer.AddTool(tools.GetGPUHealthTool(), healthProxy.Handle)

		xidProxy := gateway.NewProxyHandler(cfg.K8sClient,
			"analyze_xid_errors", routerOpts...)
		mcpServer.AddTool(tools.GetAnalyzeXIDTool(), xidProxy.Handle)

		log.Printf(`{"level":"info","msg":"MCP server initialized",`+
			`"mode":"%s","gateway":true,"namespace":"%s","routing_mode":"%s",`+
			`"tools":["get_gpu_inventory","get_gpu_health","analyze_xid_errors"],`+
			`"version":"%s","commit":"%s"}`,
			cfg.Mode, cfg.Namespace, cfg.RoutingMode, cfg.Version, cfg.GitCommit)
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

	// Oneshot mode: use OneshotTransport for deterministic exit
	if s.oneshot > 0 {
		transport, err := NewOneshotTransport(OneshotConfig{
			MCPServer:   s.mcpServer,
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			MaxRequests: s.oneshot,
		})
		if err != nil {
			return fmt.Errorf("failed to create oneshot transport: %w", err)
		}

		result, err := transport.Run(ctx)
		if err != nil {
			return err
		}

		log.Printf(`{"level":"info","msg":"oneshot completed",`+
			`"processed":%d,"errors":%d}`, result.Processed, result.Errors)
		return nil
	}

	// Standard mode: run server with stdio transport in a goroutine
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
