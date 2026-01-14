// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListGPUNodes(t *testing.T) {
	tests := []struct {
		name      string
		pods      []corev1.Pod
		wantNodes int
		wantErr   bool
	}{
		{
			name:      "no pods",
			pods:      []corev1.Pod{},
			wantNodes: 0,
		},
		{
			name: "single ready pod",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-agent-abc123",
						Namespace: "gpu-diagnostics",
						Labels: map[string]string{
							"app.kubernetes.io/name": "k8s-gpu-mcp-server",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "gpu-node-1",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			wantNodes: 1,
		},
		{
			name: "multiple pods mixed ready status",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-agent-node1",
						Namespace: "gpu-diagnostics",
						Labels: map[string]string{
							"app.kubernetes.io/name": "k8s-gpu-mcp-server",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "gpu-node-1",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-agent-node2",
						Namespace: "gpu-diagnostics",
						Labels: map[string]string{
							"app.kubernetes.io/name": "k8s-gpu-mcp-server",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "gpu-node-2",
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
			},
			wantNodes: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with test pods
			//nolint:staticcheck // NewSimpleClientset used for testing
			clientset := fake.NewSimpleClientset()
			for _, pod := range tt.pods {
				_, err := clientset.CoreV1().Pods(pod.Namespace).
					Create(context.Background(), &pod, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			client := NewClientWithConfig(clientset, nil, "gpu-diagnostics")
			nodes, err := client.ListGPUNodes(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, nodes, tt.wantNodes)

			// Verify node data for single ready pod test
			if tt.name == "single ready pod" && len(nodes) > 0 {
				assert.Equal(t, "gpu-node-1", nodes[0].Name)
				assert.Equal(t, "gpu-agent-abc123", nodes[0].PodName)
				assert.Equal(t, "10.0.0.1", nodes[0].PodIP)
				assert.True(t, nodes[0].Ready)
			}

			// Verify mixed ready status
			if tt.name == "multiple pods mixed ready status" {
				readyCount := 0
				for _, n := range nodes {
					if n.Ready {
						readyCount++
					}
				}
				assert.Equal(t, 1, readyCount)
			}
		})
	}
}

func TestGetPodForNode(t *testing.T) {
	tests := []struct {
		name     string
		pods     []corev1.Pod
		nodeName string
		wantPod  string
		wantErr  bool
	}{
		{
			name:     "node not found",
			pods:     []corev1.Pod{},
			nodeName: "missing-node",
			wantErr:  true,
		},
		{
			name: "node found",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-agent-target",
						Namespace: "gpu-diagnostics",
						Labels: map[string]string{
							"app.kubernetes.io/name": "k8s-gpu-mcp-server",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "target-node",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.5",
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			nodeName: "target-node",
			wantPod:  "gpu-agent-target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:staticcheck // NewSimpleClientset used for testing
			clientset := fake.NewSimpleClientset()
			for _, pod := range tt.pods {
				_, err := clientset.CoreV1().Pods(pod.Namespace).
					Create(context.Background(), &pod, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			client := NewClientWithConfig(clientset, nil, "gpu-diagnostics")
			node, err := client.GetPodForNode(context.Background(), tt.nodeName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPod, node.PodName)
			assert.Equal(t, tt.nodeName, node.Name)
		})
	}
}

func TestNamespace(t *testing.T) {
	client := NewClientWithConfig(nil, nil, "test-namespace")
	assert.Equal(t, "test-namespace", client.Namespace())
}

func TestClientOptions_DefaultExecTimeout(t *testing.T) {
	client := NewClientWithConfig(nil, nil, "test-namespace")
	assert.Equal(t, DefaultExecTimeout, client.ExecTimeout())
}

func TestClientOptions_WithExecTimeout(t *testing.T) {
	customTimeout := 60 * time.Second
	client := NewClientWithConfig(nil, nil, "test-namespace",
		WithExecTimeout(customTimeout))
	assert.Equal(t, customTimeout, client.ExecTimeout())
}

func TestClientOptions_MultipleOptions(t *testing.T) {
	customTimeout := 45 * time.Second
	client := NewClientWithConfig(nil, nil, "test-namespace",
		WithExecTimeout(customTimeout))

	assert.Equal(t, "test-namespace", client.Namespace())
	assert.Equal(t, customTimeout, client.ExecTimeout())
}

func TestParseExecTimeout(t *testing.T) {
	fallback := 60 * time.Second

	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{
			name:     "valid seconds",
			value:    "45s",
			expected: 45 * time.Second,
		},
		{
			name:     "valid minutes",
			value:    "2m",
			expected: 2 * time.Minute,
		},
		{
			name:     "valid complex duration",
			value:    "1m30s",
			expected: 90 * time.Second,
		},
		{
			name:     "valid max boundary",
			value:    "300s",
			expected: 300 * time.Second,
		},
		{
			name:     "valid min boundary",
			value:    "1s",
			expected: 1 * time.Second,
		},
		{
			name:     "invalid duration returns fallback",
			value:    "not-a-duration",
			expected: fallback,
		},
		{
			name:     "empty string returns fallback",
			value:    "",
			expected: fallback,
		},
		{
			name:     "number without unit returns fallback",
			value:    "45",
			expected: fallback,
		},
		{
			name:     "zero duration returns fallback",
			value:    "0s",
			expected: fallback,
		},
		{
			name:     "negative duration returns fallback",
			value:    "-10s",
			expected: fallback,
		},
		{
			name:     "exceeds max returns fallback",
			value:    "999999h",
			expected: fallback,
		},
		{
			name:     "below min returns fallback",
			value:    "500ms",
			expected: fallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseExecTimeout(tt.value, fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultExecTimeout_Value(t *testing.T) {
	// Verify the default timeout is 60 seconds as per the fix
	assert.Equal(t, 60*time.Second, DefaultExecTimeout)
}

func TestGPUNode_GetAgentHTTPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		node     GPUNode
		expected string
	}{
		{
			name: "pod with IP",
			node: GPUNode{
				Name:    "gpu-node-1",
				PodName: "gpu-agent-abc123",
				PodIP:   "10.0.0.5",
				Ready:   true,
			},
			expected: "http://10.0.0.5:8080",
		},
		{
			name: "pod without IP",
			node: GPUNode{
				Name:    "gpu-node-2",
				PodName: "gpu-agent-pending",
				PodIP:   "",
				Ready:   false,
			},
			expected: "",
		},
		{
			name: "pod with IPv6",
			node: GPUNode{
				Name:    "gpu-node-3",
				PodName: "gpu-agent-ipv6",
				PodIP:   "fd00::1",
				Ready:   true,
			},
			expected: "http://[fd00::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.GetAgentHTTPEndpoint()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAgentHTTPPort(t *testing.T) {
	// Verify the port constant matches the Helm default
	assert.Equal(t, 8080, AgentHTTPPort)
}

func TestGPUNode_GetAgentDNSEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		node     GPUNode
		expected string
	}{
		{
			name: "complete DNS fields",
			node: GPUNode{
				Name:        "gpu-node-1",
				PodName:     "gpu-mcp-k8s-gpu-mcp-server-abc123",
				PodIP:       "10.0.0.5",
				Ready:       true,
				Namespace:   "gpu-diagnostics",
				ServiceName: "gpu-mcp-k8s-gpu-mcp-server",
			},
			expected: "http://gpu-mcp-k8s-gpu-mcp-server-abc123." +
				"gpu-mcp-k8s-gpu-mcp-server.gpu-diagnostics.svc.cluster.local:8080",
		},
		{
			name: "missing namespace",
			node: GPUNode{
				Name:        "gpu-node-2",
				PodName:     "gpu-agent-abc",
				PodIP:       "10.0.0.6",
				Ready:       true,
				ServiceName: "gpu-mcp-k8s-gpu-mcp-server",
			},
			expected: "",
		},
		{
			name: "missing service name",
			node: GPUNode{
				Name:      "gpu-node-3",
				PodName:   "gpu-agent-xyz",
				PodIP:     "10.0.0.7",
				Ready:     true,
				Namespace: "gpu-diagnostics",
			},
			expected: "",
		},
		{
			name: "missing pod name",
			node: GPUNode{
				Name:        "gpu-node-4",
				PodIP:       "10.0.0.8",
				Ready:       true,
				Namespace:   "gpu-diagnostics",
				ServiceName: "gpu-mcp-k8s-gpu-mcp-server",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.GetAgentDNSEndpoint()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGPUNode_GetAgentDNSEndpoint_FormatValidation(t *testing.T) {
	// Test that DNS endpoint follows Kubernetes DNS naming conventions
	// Format: http://<pod-name>.<service-name>.<namespace>.svc.cluster.local:<port>
	tests := []struct {
		name        string
		podName     string
		serviceName string
		namespace   string
	}{
		{
			name:        "realistic Helm-generated names",
			podName:     "gpu-mcp-k8s-gpu-mcp-server-b97p2",
			serviceName: "gpu-mcp-k8s-gpu-mcp-server",
			namespace:   "gpu-diagnostics",
		},
		{
			name:        "custom release name",
			podName:     "my-release-k8s-gpu-mcp-server-xyz12",
			serviceName: "my-release-k8s-gpu-mcp-server",
			namespace:   "custom-ns",
		},
		{
			name:        "default namespace",
			podName:     "agent-abc123",
			serviceName: "gpu-agent-svc",
			namespace:   "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := GPUNode{
				Name:        "test-node",
				PodName:     tt.podName,
				PodIP:       "10.0.0.1",
				Ready:       true,
				Namespace:   tt.namespace,
				ServiceName: tt.serviceName,
			}

			result := node.GetAgentDNSEndpoint()

			// Verify format: http://<pod>.<svc>.<ns>.svc.cluster.local:8080
			expectedFormat := "http://%s.%s.%s.svc.cluster.local:%d"
			expected := fmt.Sprintf(expectedFormat,
				tt.podName, tt.serviceName, tt.namespace, AgentHTTPPort)
			assert.Equal(t, expected, result)

			// Verify it starts with http://
			assert.True(t, strings.HasPrefix(result, "http://"))

			// Verify it ends with the correct port
			assert.True(t, strings.HasSuffix(result, ":8080"))

			// Verify it contains svc.cluster.local
			assert.Contains(t, result, ".svc.cluster.local")

			// Verify the components are in correct order
			assert.Contains(t, result, tt.podName+"."+tt.serviceName)
			assert.Contains(t, result, tt.serviceName+"."+tt.namespace)
		})
	}
}
