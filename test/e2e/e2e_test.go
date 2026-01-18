// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// Package e2e provides end-to-end tests for the k8s-gpu-mcp-server.
// These tests deploy the application via Helm in a Kind cluster and
// validate the full stack: DaemonSet agents, HTTP transport, and
// gateway routing.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	kindClusterName = "e2e-gpu-mcp"
	namespace       = "gpu-diagnostics"
	helmReleaseName = "e2e-test"
)

// Package-level state for E2E test infrastructure.
// Written by TestMain before any tests run, read-only during test execution.
// Safe: TestMain guarantees sequential initialization before parallel test execution.
var (
	// gatewayURL is the URL for accessing the gateway via port-forward.
	gatewayURL string

	// ctx is the test context with timeout.
	ctx context.Context

	// cancel cancels the test context.
	cancel context.CancelFunc

	// portForwardCmd holds the port-forward process for cleanup.
	portForwardCmd *exec.Cmd
)

// TestMain sets up and tears down the Kind cluster for E2E tests.
func TestMain(m *testing.M) {
	// Skip if not explicitly requested
	if os.Getenv("E2E_TEST") != "1" {
		fmt.Println("E2E tests skipped (set E2E_TEST=1 to run)")
		os.Exit(0)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Check if cluster already exists (for local development)
	if clusterExists() {
		fmt.Println("Using existing Kind cluster")
	} else {
		// Create Kind cluster
		fmt.Println("Creating Kind cluster...")
		if err := createKindCluster(); err != nil {
			fmt.Printf("Failed to create Kind cluster: %v\n", err)
			os.Exit(1)
		}
	}

	// Check if helm release already exists
	if !releaseExists() {
		// Deploy via Helm
		fmt.Println("Deploying via Helm...")
		if err := deployHelm(); err != nil {
			fmt.Printf("Failed to deploy Helm chart: %v\n", err)
			teardownKindCluster()
			os.Exit(1)
		}
	} else {
		fmt.Println("Using existing Helm release")
	}

	// Wait for pods to be ready
	fmt.Println("Waiting for pods to be ready...")
	if err := waitForPodsReady(); err != nil {
		fmt.Printf("Failed waiting for pods: %v\n", err)
		collectLogs()
		teardownKindCluster()
		os.Exit(1)
	}

	// Setup port-forward
	fmt.Println("Setting up port-forward...")
	if err := setupPortForward(); err != nil {
		fmt.Printf("Failed to setup port-forward: %v\n", err)
		collectLogs()
		teardownKindCluster()
		os.Exit(1)
	}

	// Run tests
	fmt.Println("Running E2E tests...")
	code := m.Run()

	// Cleanup
	if portForwardCmd != nil && portForwardCmd.Process != nil {
		_ = portForwardCmd.Process.Kill()
	}

	// Only teardown if we created the cluster (not for local dev)
	if os.Getenv("E2E_KEEP_CLUSTER") != "1" {
		teardownKindCluster()
	}

	os.Exit(code)
}

// clusterExists checks if the Kind cluster already exists.
func clusterExists() bool {
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, cluster := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(cluster) == kindClusterName {
			return true
		}
	}
	return false
}

// releaseExists checks if the Helm release already exists.
func releaseExists() bool {
	cmd := exec.CommandContext(ctx, "helm", "list",
		"-n", namespace,
		"-q",
		"--filter", helmReleaseName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == helmReleaseName
}

// createKindCluster creates the Kind cluster for testing.
func createKindCluster() error {
	configPath := filepath.Join("testdata", "kind-config.yaml")
	cmd := exec.CommandContext(ctx, "kind", "create", "cluster",
		"--name", kindClusterName,
		"--config", configPath,
		"--wait", "120s")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// deployHelm deploys the application via Helm.
func deployHelm() error {
	// Get absolute path to helm chart
	chartPath := filepath.Join("..", "..", "deployment", "helm", "k8s-gpu-mcp-server")
	cmd := exec.CommandContext(ctx, "helm", "install", helmReleaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--set", "namespace.create=false",
		"--set", "agent.nvmlMode=mock",
		"--set", "gateway.enabled=true",
		"--set", "gpu.runtimeClass.enabled=false",
		"--set", "gpu.resourceRequest.enabled=false",
		"--wait",
		"--timeout", "180s")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// waitForPodsReady waits for all pods in the namespace to be ready.
func waitForPodsReady() error {
	cmd := exec.CommandContext(ctx, "kubectl", "wait",
		"--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=k8s-gpu-mcp-server",
		"-n", namespace,
		"--timeout=180s")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// setupPortForward starts port-forwarding to the gateway service.
func setupPortForward() error {
	serviceName := helmReleaseName + "-k8s-gpu-mcp-server-gateway"

	// Kill any existing port-forward for this specific service on port 18080
	// Use a more specific pattern to avoid killing unrelated port-forwards
	// Note: Intentionally uses exec.Command (not CommandContext) - this is fire-and-forget
	// cleanup that should complete quickly regardless of test context state.
	_ = exec.Command("pkill", "-f",
		fmt.Sprintf("kubectl.*port-forward.*%s.*18080", serviceName)).Run()

	portForwardCmd = exec.CommandContext(ctx, "kubectl", "port-forward",
		"-n", namespace,
		"svc/"+serviceName,
		"18080:8080")

	if err := portForwardCmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	gatewayURL = "http://localhost:18080"

	// Verify connectivity with retry loop (no initial sleep needed)
	for i := 0; i < 15; i++ {
		checkCmd := exec.CommandContext(ctx, "curl", "-s", "-o", "/dev/null",
			"-w", "%{http_code}",
			"--connect-timeout", "2",
			gatewayURL+"/healthz")
		out, err := checkCmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "200" {
			fmt.Println("Port-forward established successfully")
			return nil
		}
		// Note: Using simple sleep rather than context-aware select.
		// Bounded by loop iteration (max 15 iterations = 15s worst case).
		// Context cancellation will be handled on next iteration's exec.CommandContext.
		time.Sleep(time.Second)
	}

	return fmt.Errorf("port-forward not responding after 15 attempts")
}

// teardownKindCluster deletes the Kind cluster.
func teardownKindCluster() {
	fmt.Println("Tearing down Kind cluster...")
	cmd := exec.Command("kind", "delete", "cluster", "--name", kindClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// collectLogs collects pod logs for debugging test failures.
func collectLogs() {
	fmt.Println("\n=== Collecting logs for debugging ===")

	// Get pod status
	statusCmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", namespace, "-o", "wide")
	statusCmd.Stdout = os.Stdout
	statusCmd.Stderr = os.Stderr
	_ = statusCmd.Run()

	// Get pod descriptions
	describeCmd := exec.CommandContext(ctx, "kubectl", "describe", "pods",
		"-n", namespace)
	describeCmd.Stdout = os.Stdout
	describeCmd.Stderr = os.Stderr
	_ = describeCmd.Run()

	// Get pod logs
	logsCmd := exec.CommandContext(ctx, "kubectl", "logs",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=k8s-gpu-mcp-server",
		"--all-containers",
		"--tail=100")
	logsCmd.Stdout = os.Stdout
	logsCmd.Stderr = os.Stderr
	_ = logsCmd.Run()
}
