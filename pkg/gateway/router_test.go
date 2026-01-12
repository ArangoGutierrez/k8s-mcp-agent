// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRouterRouteToNode_NodeNotFound(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset is used for testing without apply config
	clientset := fake.NewSimpleClientset()
	k8sClient := k8s.NewClientWithConfig(clientset, nil, "gpu-diagnostics")
	router := NewRouter(k8sClient)

	_, err := router.RouteToNode(context.Background(), "missing-node", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
}

func TestRouterRouteToNode_NodeNotReady(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpu-agent-unready",
			Namespace: "gpu-diagnostics",
			Labels: map[string]string{
				"app.kubernetes.io/name": "k8s-gpu-mcp-server",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "unready-node",
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	//nolint:staticcheck // NewSimpleClientset is used for testing without apply config
	clientset := fake.NewSimpleClientset(pod)
	k8sClient := k8s.NewClientWithConfig(clientset, nil, "gpu-diagnostics")
	router := NewRouter(k8sClient)

	_, err := router.RouteToNode(context.Background(), "unready-node", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
}

func TestRouterRouteToAllNodes_NoNodes(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset is used for testing without apply config
	clientset := fake.NewSimpleClientset()
	k8sClient := k8s.NewClientWithConfig(clientset, nil, "gpu-diagnostics")
	router := NewRouter(k8sClient)

	results, err := router.RouteToAllNodes(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestRouterRouteToAllNodes_OnlyUnreadyNodes(t *testing.T) {
	// Test that RouteToAllNodes correctly skips all unready nodes
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gpu-agent-unready1",
				Namespace: "gpu-diagnostics",
				Labels: map[string]string{
					"app.kubernetes.io/name": "k8s-gpu-mcp-server",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "unready-node-1",
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.1",
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gpu-agent-unready2",
				Namespace: "gpu-diagnostics",
				Labels: map[string]string{
					"app.kubernetes.io/name": "k8s-gpu-mcp-server",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "unready-node-2",
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.2",
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
	}

	//nolint:staticcheck // NewSimpleClientset is used for testing without apply config
	clientset := fake.NewSimpleClientset()
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).
			Create(context.Background(), &pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	k8sClient := k8s.NewClientWithConfig(clientset, nil, "gpu-diagnostics")
	router := NewRouter(k8sClient)

	// All nodes are unready, so no exec should be attempted
	results, err := router.RouteToAllNodes(context.Background(), []byte("{}"))
	require.NoError(t, err)

	// No nodes should be in results since all are unready and skipped
	assert.Len(t, results, 0)
}

func TestNewRouter(t *testing.T) {
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")
	router := NewRouter(k8sClient)
	assert.NotNil(t, router)
}

func TestNewRouter_DefaultHTTPMode(t *testing.T) {
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")
	router := NewRouter(k8sClient)

	// Default routing mode should be HTTP
	assert.Equal(t, RoutingModeHTTP, router.RoutingMode())
	assert.NotNil(t, router.httpClient)
}

func TestWithRoutingMode(t *testing.T) {
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")

	// Test HTTP mode
	routerHTTP := NewRouter(k8sClient, WithRoutingMode(RoutingModeHTTP))
	assert.Equal(t, RoutingModeHTTP, routerHTTP.RoutingMode())

	// Test Exec mode
	routerExec := NewRouter(k8sClient, WithRoutingMode(RoutingModeExec))
	assert.Equal(t, RoutingModeExec, routerExec.RoutingMode())
}

func TestRoutingMode_Constants(t *testing.T) {
	// Ensure constants have expected values
	assert.Equal(t, RoutingMode("http"), RoutingModeHTTP)
	assert.Equal(t, RoutingMode("exec"), RoutingModeExec)
}

func TestRouter_HTTPClient_Initialized(t *testing.T) {
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")
	router := NewRouter(k8sClient)

	// HTTP client should be initialized even in exec mode
	// (for potential fallback scenarios)
	assert.NotNil(t, router.httpClient)
}

func TestRouter_ExecMode_Configuration(t *testing.T) {
	// Verify exec mode is correctly configured via option
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")
	router := NewRouter(k8sClient, WithRoutingMode(RoutingModeExec))

	// Verify routing mode is exec
	assert.Equal(t, RoutingModeExec, router.RoutingMode())

	// HTTP client should still be initialized (for potential future fallback)
	assert.NotNil(t, router.httpClient)
}

func TestRouter_HTTPMode_Configuration(t *testing.T) {
	// Verify HTTP mode is correctly configured via option
	k8sClient := k8s.NewClientWithConfig(nil, nil, "test-ns")
	router := NewRouter(k8sClient, WithRoutingMode(RoutingModeHTTP))

	// Verify routing mode is HTTP
	assert.Equal(t, RoutingModeHTTP, router.RoutingMode())
}
