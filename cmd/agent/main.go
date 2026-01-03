// Copyright 2026 k8s-mcp-agent contributors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the k8s-mcp-agent MCP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ArangoGutierrez/k8s-mcp-agent/internal/info"
	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/mcp"
	"github.com/ArangoGutierrez/k8s-mcp-agent/pkg/nvml"
)

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
		showVer  = flag.Bool("version", false, "Show version information and exit")
		logLevel = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	// Show version and exit if requested
	if *showVer {
		buildInfo := info.GetInfo()
		fmt.Fprintf(os.Stderr, "k8s-mcp-agent version %s (commit %s)\n",
			buildInfo.Version, buildInfo.GitCommit)
		os.Exit(0)
	}

	// Validate mode flag
	if *mode != ModeReadOnly && *mode != ModeOperator {
		log.Fatalf(`{"level":"fatal","msg":"invalid mode","mode":"%s","valid":["read-only","operator"]}`, *mode)
	}

	// Log startup information to stderr (structured JSON)
	log.Printf(`{"level":"info","msg":"starting k8s-mcp-agent","version":"%s","commit":"%s","mode":"%s","log_level":"%s"}`,
		info.Version(), info.GitCommit(), *mode, *logLevel)

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Channel to coordinate shutdown
	done := make(chan error, 1)

	// Initialize NVML client (mock for M1)
	// TODO: Add flag to switch between mock and real NVML in M2
	nvmlClient := nvml.NewMock(2) // 2 fake GPUs
	if err := nvmlClient.Init(ctx); err != nil {
		log.Printf(`{"level":"fatal","msg":"failed to initialize NVML","error":"%s"}`, err)
		os.Exit(1)
	}
	defer func() {
		if err := nvmlClient.Shutdown(ctx); err != nil {
			log.Printf(`{"level":"error","msg":"failed to shutdown NVML","error":"%s"}`, err)
		}
	}()

	// Initialize MCP server
	buildInfo := info.GetInfo()
	mcpServer, err := mcp.New(mcp.Config{
		Mode:       *mode,
		Version:    buildInfo.Version,
		GitCommit:  buildInfo.GitCommit,
		NVMLClient: nvmlClient,
	})
	if err != nil {
		log.Printf(`{"level":"fatal","msg":"failed to create MCP server","error":"%s"}`, err)
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
