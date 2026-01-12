// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the k8s-gpu-mcp-server MCP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/internal/info"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/mcp"
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
)

// ValidLogLevels are the accepted log levels.
var ValidLogLevels = []string{"debug", "info", "warn", "error"}

// resolveLogLevel determines the effective log level from env var and flag.
// Priority: LOG_LEVEL env var > --log-level flag > default ("info")
func resolveLogLevel(flagValue string) string {
	// Check environment variable first
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		level := strings.ToLower(strings.TrimSpace(envLevel))
		if isValidLogLevel(level) {
			return level
		}
		// Invalid env value - log warning and fall back to flag
		log.Printf(`{"level":"warn","msg":"invalid LOG_LEVEL env var",`+
			`"value":"%s","valid":%q,"using":"%s"}`,
			envLevel, ValidLogLevels, flagValue)
	}
	return flagValue
}

// isValidLogLevel checks if a log level is valid.
func isValidLogLevel(level string) bool {
	for _, valid := range ValidLogLevels {
		if level == valid {
			return true
		}
	}
	return false
}

const (
	// ModeReadOnly enables only read-only operations (default)
	ModeReadOnly = "read-only"
	// ModeOperator enables write operations (kill/reset)
	ModeOperator = "operator"
)

func main() {
	// Parse command-line flags
	var (
		mode     = flag.String("mode", ModeReadOnly, "Operation mode: read-only or operator")
		nvmlMode = flag.String("nvml-mode", "mock", "NVML mode: mock or real (requires GPU hardware)")
		showVer  = flag.Bool("version", false, "Show version information and exit")
		logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error")

		// HTTP transport flags
		port = flag.Int("port", 0, "HTTP port (0 = stdio mode, >0 = HTTP mode)")
		addr = flag.String("addr", "0.0.0.0", "HTTP listen address")

		// Gateway mode flags
		gatewayMode = flag.Bool("gateway", false,
			"Enable gateway mode (routes to node agents via K8s pod exec)")
		namespace = flag.String("namespace", "gpu-diagnostics",
			"Namespace for GPU agent pods (gateway mode)")
		routingMode = flag.String("routing-mode", "http",
			"Gateway routing mode: http (default, direct HTTP) or exec (legacy)")

		// Oneshot mode for exec-based invocations
		oneshot = flag.Int("oneshot", 0,
			"Exit after processing N requests (0=disabled, 2=init+tool)")
	)
	flag.Parse()

	// Show version and exit if requested
	if *showVer {
		buildInfo := info.GetInfo()
		fmt.Fprintf(os.Stderr, "k8s-gpu-mcp-server version %s (commit %s)\n",
			buildInfo.Version, buildInfo.GitCommit)
		os.Exit(0)
	}

	// Validate mode flag
	if *mode != ModeReadOnly && *mode != ModeOperator {
		log.Fatalf(`{"level":"fatal","msg":"invalid mode","mode":"%s",`+
			`"valid":["read-only","operator"]}`, *mode)
	}

	// Validate nvml-mode flag (only relevant in non-gateway mode)
	if !*gatewayMode && *nvmlMode != "mock" && *nvmlMode != "real" {
		log.Fatalf(`{"level":"fatal","msg":"invalid nvml-mode",`+
			`"nvml_mode":"%s","valid":["mock","real"]}`, *nvmlMode)
	}

	// Resolve log level from env var and flag
	effectiveLogLevel := resolveLogLevel(*logLevel)
	if !isValidLogLevel(effectiveLogLevel) {
		log.Fatalf(`{"level":"fatal","msg":"invalid log-level",`+
			`"log_level":"%s","valid":%q}`, effectiveLogLevel, ValidLogLevels)
	}

	// Validate routing mode if in gateway mode (fail fast before logging)
	if *gatewayMode {
		if *routingMode != "http" && *routingMode != "exec" {
			log.Fatalf(`{"level":"fatal","msg":"invalid routing-mode",`+
				`"routing_mode":"%s","valid":["http","exec"]}`, *routingMode)
		}
	}

	// Validate and configure transport mode
	var transport mcp.TransportType
	var httpAddr string

	if *port > 0 {
		if *port < 1 || *port > 65535 {
			log.Fatalf(`{"level":"fatal","msg":"invalid port","port":%d,`+
				`"valid":"1-65535 or 0 for stdio"}`, *port)
		}
		transport = mcp.TransportHTTP
		httpAddr = fmt.Sprintf("%s:%d", *addr, *port)
		log.Printf(`{"level":"info","msg":"HTTP mode enabled","addr":"%s"}`,
			httpAddr)
	} else {
		transport = mcp.TransportStdio
	}

	// Log startup information to stderr (structured JSON)
	if *gatewayMode {
		log.Printf(`{"level":"info","msg":"starting k8s-gpu-mcp-server",`+
			`"version":"%s","commit":"%s","mode":"%s","gateway":true,`+
			`"namespace":"%s","routing_mode":"%s","log_level":"%s"}`,
			info.Version(), info.GitCommit(), *mode, *namespace,
			*routingMode, effectiveLogLevel)
	} else {
		log.Printf(`{"level":"info","msg":"starting k8s-gpu-mcp-server",`+
			`"version":"%s","commit":"%s","mode":"%s","nvml_mode":"%s",`+
			`"log_level":"%s"}`,
			info.Version(), info.GitCommit(), *mode, *nvmlMode, effectiveLogLevel)
	}

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Channel to coordinate shutdown
	done := make(chan error, 1)

	// Build MCP server config
	buildInfo := info.GetInfo()
	mcpCfg := mcp.Config{
		Mode:        *mode,
		Version:     buildInfo.Version,
		GitCommit:   buildInfo.GitCommit,
		Transport:   transport,
		HTTPAddr:    httpAddr,
		GatewayMode: *gatewayMode,
		Namespace:   *namespace,
		Oneshot:     *oneshot,
		RoutingMode: *routingMode,
	}

	if *gatewayMode {
		// Gateway mode: initialize K8s client
		log.Printf(`{"level":"info","msg":"initializing K8s client",`+
			`"namespace":"%s"}`, *namespace)

		k8sClient, err := k8s.NewClient(*namespace)
		if err != nil {
			log.Printf(`{"level":"fatal","msg":"failed to create K8s client",`+
				`"error":"%s"}`, err)
			os.Exit(1)
		}
		mcpCfg.K8sClient = k8sClient
	} else {
		// Regular mode: initialize NVML client
		var nvmlClient nvml.Interface
		if *nvmlMode == "real" {
			log.Printf(`{"level":"info",` +
				`"msg":"initializing real NVML (requires GPU hardware)"}`)
			nvmlClient = nvml.NewReal()
		} else {
			log.Printf(`{"level":"info","msg":"initializing mock NVML",` +
				`"fake_gpus":2}`)
			nvmlClient = nvml.NewMock(2)
		}

		if err := nvmlClient.Init(ctx); err != nil {
			log.Printf(`{"level":"fatal","msg":"failed to initialize NVML",`+
				`"nvml_mode":"%s","error":"%s"}`, *nvmlMode, err)
			os.Exit(1)
		}
		defer func() {
			if err := nvmlClient.Shutdown(ctx); err != nil {
				log.Printf(`{"level":"error",`+
					`"msg":"failed to shutdown NVML","error":"%s"}`, err)
			}
		}()
		mcpCfg.NVMLClient = nvmlClient
	}

	// Initialize MCP server
	mcpServer, err := mcp.New(mcpCfg)
	if err != nil {
		log.Printf(`{"level":"fatal","msg":"failed to create MCP server",`+
			`"error":"%s"}`, err)
		os.Exit(1)
	}

	// Start the MCP server in a goroutine
	go func() {
		if err := mcpServer.Run(ctx); err != nil {
			log.Printf(`{"level":"error","msg":"MCP server error","error":"%s"}`, err)
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf(`{"level":"info","msg":"received signal","signal":"%s"}`, sig)
		cancel()
	case err := <-done:
		if err != nil {
			log.Printf(`{"level":"error","msg":"server error","error":"%s"}`, err)
			os.Exit(1)
		}
	}

	// Wait for graceful shutdown
	<-done
	log.Printf(`{"level":"info","msg":"shutdown complete"}`)
}
