// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"testing"

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
