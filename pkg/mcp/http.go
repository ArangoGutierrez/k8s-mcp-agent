// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// HTTPServer wraps the MCP server with HTTP transport.
type HTTPServer struct {
	mcpServer  *server.MCPServer
	httpServer *http.Server
	addr       string
	version    string
	ready      chan struct{}
}

// NewHTTPServer creates an HTTP transport server.
func NewHTTPServer(mcpServer *server.MCPServer, addr, version string) *HTTPServer {
	return &HTTPServer{
		mcpServer: mcpServer,
		addr:      addr,
		version:   version,
		ready:     make(chan struct{}),
	}
}

// ListenAndServe starts the HTTP server.
func (h *HTTPServer) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()

	// MCP endpoint - Streamable HTTP transport
	streamableServer := server.NewStreamableHTTPServer(h.mcpServer)
	mux.Handle("/mcp", streamableServer)

	// Health check endpoints
	mux.HandleFunc("/healthz", h.handleHealthz)
	mux.HandleFunc("/readyz", h.handleReadyz)

	// Version endpoint
	mux.HandleFunc("/version", h.handleVersion)

	// Create server before starting goroutine to avoid race condition.
	// WriteTimeout (90s) must exceed exec timeout (60s) plus response
	// marshaling buffer (30s buffer) to prevent race conditions causing
	// "socket hang up" errors.
	// IdleTimeout (120s) exceeds WriteTimeout to avoid prematurely closing
	// keep-alive connections during long-running operations.
	h.httpServer = &http.Server{
		Addr:              h.addr,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf(`{"level":"info","msg":"HTTP server starting","addr":"%s"}`, h.addr)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		close(h.ready) // Signal that server is ready to accept connections
		if err := h.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return h.Shutdown()
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPServer) Shutdown() error {
	if h.httpServer == nil {
		return nil
	}

	log.Printf(`{"level":"info","msg":"HTTP server shutting down"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return h.httpServer.Shutdown(ctx)
}

// handleHealthz handles liveness probe.
func (h *HTTPServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	}); err != nil {
		log.Printf(`{"level":"error","msg":"failed to encode healthz response",`+
			`"error":"%s"}`, err)
	}
}

// handleReadyz handles readiness probe.
func (h *HTTPServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	// TODO: Check NVML initialization status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	}); err != nil {
		log.Printf(`{"level":"error","msg":"failed to encode readyz response",`+
			`"error":"%s"}`, err)
	}
}

// handleVersion returns version information.
func (h *HTTPServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"server":  "k8s-gpu-mcp-server",
		"version": h.version,
	}); err != nil {
		log.Printf(`{"level":"error","msg":"failed to encode version response",`+
			`"error":"%s"}`, err)
	}
}
