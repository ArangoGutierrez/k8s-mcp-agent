// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/k8s"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListGPUNodesHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		pods           []corev1.Pod
		wantNodeCount  int
		wantReadyCount int
		wantStatus     string
	}{
		{
			name:           "no pods",
			pods:           []corev1.Pod{},
			wantNodeCount:  0,
			wantReadyCount: 0,
			wantStatus:     "success",
		},
		{
			name: "single ready pod",
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
			},
			wantNodeCount:  1,
			wantReadyCount: 1,
			wantStatus:     "success",
		},
		{
			name: "multiple pods mixed status",
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-agent-node3",
						Namespace: "gpu-diagnostics",
						Labels: map[string]string{
							"app.kubernetes.io/name": "k8s-gpu-mcp-server",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "gpu-node-3",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.3",
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			wantNodeCount:  3,
			wantReadyCount: 2,
			wantStatus:     "success",
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

			k8sClient := k8s.NewClientWithConfig(
				clientset, nil, "gpu-diagnostics")
			handler := NewListGPUNodesHandler(k8sClient)

			// Create tool request
			request := mcp.CallToolRequest{}
			request.Params.Name = "list_gpu_nodes"

			result, err := handler.Handle(context.Background(), request)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text),
				&response)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, response["status"])
			assert.Equal(t, float64(tt.wantNodeCount), response["node_count"])
			assert.Equal(t, float64(tt.wantReadyCount), response["ready_count"])

			nodes := response["nodes"].([]interface{})
			assert.Len(t, nodes, tt.wantNodeCount)
		})
	}
}

func TestGetListGPUNodesTool(t *testing.T) {
	tool := GetListGPUNodesTool()
	assert.Equal(t, "list_gpu_nodes", tool.Name)
	assert.Contains(t, tool.Description, "Gateway mode")
}
