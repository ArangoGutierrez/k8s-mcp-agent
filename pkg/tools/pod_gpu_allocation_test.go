// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// makePodWithGPU creates a pod with GPU resource requests for testing.
func makePodWithGPU(name, namespace, nodeName string, gpuCount int64) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				gpuDeviceAnnotation: makeGPUUUIDs(int(gpuCount)),
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse(
								fmt.Sprintf("%d", gpuCount)),
						},
						Limits: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse(
								fmt.Sprintf("%d", gpuCount)),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

// makePodWithoutGPU creates a pod without GPU resources for testing.
func makePodWithoutGPU(name, namespace, nodeName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

// makeGPUUUIDs generates comma-separated GPU UUIDs for testing.
func makeGPUUUIDs(count int) string {
	if count == 0 {
		return ""
	}
	uuids := make([]string, count)
	for i := 0; i < count; i++ {
		uuids[i] = fmt.Sprintf("GPU-uuid-%d", i+1)
	}
	return strings.Join(uuids, ",")
}

func TestNewPodGPUAllocationHandler(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	handler := NewPodGPUAllocationHandler(clientset)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.clientset)
}

func TestPodGPUAllocationHandler_Handle(t *testing.T) {
	tests := []struct {
		name     string
		pods     []corev1.Pod
		nodeName string
		wantPods int
		wantGPUs int64
	}{
		{
			name:     "single pod with GPUs",
			nodeName: "gpu-node-1",
			pods: []corev1.Pod{
				makePodWithGPU("training-job", "ml", "gpu-node-1", 2),
			},
			wantPods: 1,
			wantGPUs: 2,
		},
		{
			name:     "multiple pods on same node",
			nodeName: "gpu-node-1",
			pods: []corev1.Pod{
				makePodWithGPU("job-1", "ns1", "gpu-node-1", 1),
				makePodWithGPU("job-2", "ns2", "gpu-node-1", 3),
			},
			wantPods: 2,
			wantGPUs: 4,
		},
		{
			name:     "filters by node",
			nodeName: "gpu-node-1",
			pods: []corev1.Pod{
				makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
				makePodWithGPU("job-2", "ns1", "gpu-node-2", 4), // different node
			},
			wantPods: 1,
			wantGPUs: 2,
		},
		{
			name:     "skips pods without GPUs",
			nodeName: "gpu-node-1",
			pods: []corev1.Pod{
				makePodWithGPU("gpu-job", "ns1", "gpu-node-1", 2),
				makePodWithoutGPU("cpu-job", "ns1", "gpu-node-1"),
			},
			wantPods: 1,
			wantGPUs: 2,
		},
		{
			name:     "no pods on node",
			nodeName: "empty-node",
			pods:     []corev1.Pod{},
			wantPods: 0,
			wantGPUs: 0,
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

			handler := NewPodGPUAllocationHandler(clientset)

			request := mcp.CallToolRequest{}
			request.Params.Arguments = map[string]interface{}{
				"node_name": tt.nodeName,
			}

			result, err := handler.Handle(context.Background(), request)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)

			// Parse response
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)

			var response PodGPUAllocationResponse
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tt.nodeName, response.NodeName)
			assert.Equal(t, tt.wantPods, response.Summary.TotalPods)
			assert.Equal(t, tt.wantGPUs, response.Summary.TotalGPUsAllocated)
			assert.Len(t, response.Pods, tt.wantPods)
		})
	}
}

func TestPodGPUAllocationHandler_NamespaceFilter(t *testing.T) {
	// Create pods in different namespaces
	pods := []corev1.Pod{
		makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
		makePodWithGPU("job-2", "ns2", "gpu-node-1", 3),
		makePodWithGPU("job-3", "ns1", "gpu-node-1", 1),
	}

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).
			Create(context.Background(), &pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	handler := NewPodGPUAllocationHandler(clientset)

	// Test filtering by namespace ns1
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
		"namespace": "ns1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response PodGPUAllocationResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Should only have pods from ns1
	assert.Equal(t, 2, response.Summary.TotalPods)
	assert.Equal(t, int64(3), response.Summary.TotalGPUsAllocated) // 2 + 1
	for _, pod := range response.Pods {
		assert.Equal(t, "ns1", pod.Namespace)
	}
}

func TestPodGPUAllocationHandler_GPUUUIDs(t *testing.T) {
	pod := makePodWithGPU("gpu-job", "ml", "gpu-node-1", 2)

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Pods(pod.Namespace).
		Create(context.Background(), &pod, metav1.CreateOptions{})
	require.NoError(t, err)

	handler := NewPodGPUAllocationHandler(clientset)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response PodGPUAllocationResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	require.Len(t, response.Pods, 1)
	require.Len(t, response.Pods[0].Containers, 1)

	container := response.Pods[0].Containers[0]
	assert.Equal(t, int64(2), container.GPURequest)
	assert.Equal(t, int64(2), container.GPULimit)
	assert.Len(t, container.GPUUUIDs, 2)
	assert.Equal(t, "GPU-uuid-1", container.GPUUUIDs[0])
	assert.Equal(t, "GPU-uuid-2", container.GPUUUIDs[1])
}

func TestPodGPUAllocationHandler_MissingNodeName(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	handler := NewPodGPUAllocationHandler(clientset)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		// Missing node_name
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestPodGPUAllocationHandler_EmptyNodeName(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	handler := NewPodGPUAllocationHandler(clientset)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestPodGPUAllocationHandler_MultipleContainers(t *testing.T) {
	// Create a pod with multiple containers, some with GPUs
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-container-job",
			Namespace: "ml",
		},
		Spec: corev1.PodSpec{
			NodeName: "gpu-node-1",
			Containers: []corev1.Container{
				{
					Name: "trainer",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse("2"),
						},
						Limits: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse("2"),
						},
					},
				},
				{
					Name: "logger",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
				{
					Name: "validator",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse("1"),
						},
						Limits: corev1.ResourceList{
							nvidiaGPUResource: resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Pods(pod.Namespace).
		Create(context.Background(), &pod, metav1.CreateOptions{})
	require.NoError(t, err)

	handler := NewPodGPUAllocationHandler(clientset)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response PodGPUAllocationResponse
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	require.Len(t, response.Pods, 1)
	// Should only include containers with GPU requests (trainer and validator)
	assert.Len(t, response.Pods[0].Containers, 2)
	// Total GPUs: 2 + 1 = 3
	assert.Equal(t, int64(3), response.Summary.TotalGPUsAllocated)
}

func TestGetPodGPUAllocationTool(t *testing.T) {
	tool := GetPodGPUAllocationTool()

	assert.Equal(t, "get_pod_gpu_allocation", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.Description, "GPU allocation")
	assert.Contains(t, tool.Description, "nvidia.com/gpu")
}
