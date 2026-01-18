// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmDeployment_DaemonSetRunning(t *testing.T) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "daemonset",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=k8s-gpu-mcp-server",
		"-o", "jsonpath={.items[*].status.numberReady}")
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get DaemonSet status")

	readyStr := strings.TrimSpace(string(out))
	t.Logf("DaemonSet ready pods: %s", readyStr)

	// Should have at least 1 ready pod (worker nodes have GPU label)
	assert.NotEmpty(t, readyStr, "Expected at least 1 DaemonSet pod ready")
	assert.NotEqual(t, "0", readyStr, "Expected DaemonSet pods to be ready")
}

func TestHelmDeployment_GatewayRunning(t *testing.T) {
	deploymentName := helmReleaseName + "-k8s-gpu-mcp-server-gateway"
	cmd := exec.CommandContext(ctx, "kubectl", "get", "deployment",
		deploymentName,
		"-n", namespace,
		"-o", "jsonpath={.status.readyReplicas}")
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get gateway deployment status")

	readyStr := strings.TrimSpace(string(out))
	t.Logf("Gateway ready replicas: %s", readyStr)

	assert.Equal(t, "1", readyStr, "Gateway should have 1 replica ready")
}

func TestHelmDeployment_PodsPassReadinessProbe(t *testing.T) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=k8s-gpu-mcp-server",
		"-o", "jsonpath={.items[*].status.conditions[?(@.type=='Ready')].status}")
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get pod readiness status")

	statuses := strings.Fields(string(out))
	t.Logf("Pod readiness statuses: %v", statuses)

	require.NotEmpty(t, statuses, "Expected at least one pod")
	for i, status := range statuses {
		assert.Equal(t, "True", status, "Pod %d should pass readiness probe", i)
	}
}

func TestHelmDeployment_ServiceExists(t *testing.T) {
	// Check agent service
	agentSvcName := helmReleaseName + "-k8s-gpu-mcp-server"
	cmd := exec.CommandContext(ctx, "kubectl", "get", "service",
		agentSvcName,
		"-n", namespace,
		"-o", "jsonpath={.spec.ports[0].port}")
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get agent service")
	assert.Equal(t, "8080", strings.TrimSpace(string(out)), "Agent service port")

	// Check gateway service
	gatewaySvcName := helmReleaseName + "-k8s-gpu-mcp-server-gateway"
	cmd = exec.CommandContext(ctx, "kubectl", "get", "service",
		gatewaySvcName,
		"-n", namespace,
		"-o", "jsonpath={.spec.ports[0].port}")
	out, err = cmd.Output()
	require.NoError(t, err, "Failed to get gateway service")
	assert.Equal(t, "8080", strings.TrimSpace(string(out)), "Gateway service port")
}

func TestHelmDeployment_CorrectLabels(t *testing.T) {
	// Verify pods have expected labels
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=k8s-gpu-mcp-server",
		"-o", "jsonpath={.items[*].metadata.labels.app\\.kubernetes\\.io/name}")
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get pod labels")

	labels := strings.Fields(string(out))
	t.Logf("Pod app labels: %v", labels)

	require.NotEmpty(t, labels, "Expected pods with k8s-gpu-mcp-server label")
	for _, label := range labels {
		assert.Equal(t, "k8s-gpu-mcp-server", label,
			"All pods should have correct app label")
	}
}
