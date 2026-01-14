// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/nvml"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// makeGPUNode creates a Kubernetes node with GPU capacity for testing.
func makeGPUNode(name string, gpuCount int64) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "g4dn.xlarge",
				"nvidia.com/gpu.product":           "Tesla-T4",
				"kubernetes.io/arch":               "amd64",
				"some-other-label":                 "value", // should be filtered
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "nvidia.com/gpu",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   corev1.NodeMemoryPressure,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   corev1.NodeDiskPressure,
					Status: corev1.ConditionFalse,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				nvidiaGPUResource:     *resource.NewQuantity(gpuCount, resource.DecimalSI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3920m"),
				corev1.ResourceMemory: resource.MustParse("15Gi"),
				nvidiaGPUResource:     *resource.NewQuantity(gpuCount, resource.DecimalSI),
			},
		},
	}
}

func TestNewDescribeGPUNodeHandler(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	mockNVML := nvml.NewMock(2)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.clientset)
	assert.NotNil(t, handler.nvmlClient)
}

func TestDescribeGPUNodeHandler_Handle(t *testing.T) {
	tests := []struct {
		name      string
		nodeName  string
		node      *corev1.Node
		pods      []corev1.Pod
		gpuCount  int
		wantError bool
	}{
		{
			name:     "healthy node with GPUs",
			nodeName: "gpu-node-1",
			node:     makeGPUNode("gpu-node-1", 4),
			pods: []corev1.Pod{
				makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
			},
			gpuCount: 4,
		},
		{
			name:      "node not found",
			nodeName:  "missing-node",
			wantError: true,
		},
		{
			name:     "node with no GPU pods",
			nodeName: "gpu-node-1",
			node:     makeGPUNode("gpu-node-1", 4),
			pods:     []corev1.Pod{},
			gpuCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:staticcheck // NewSimpleClientset used for testing
			clientset := fake.NewSimpleClientset()

			// Create node if provided
			if tt.node != nil {
				_, err := clientset.CoreV1().Nodes().Create(
					context.Background(), tt.node, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			// Create pods
			for _, pod := range tt.pods {
				_, err := clientset.CoreV1().Pods(pod.Namespace).
					Create(context.Background(), &pod, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			mockNVML := nvml.NewMock(tt.gpuCount)
			handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

			request := mcp.CallToolRequest{}
			request.Params.Arguments = map[string]interface{}{
				"node_name": tt.nodeName,
			}

			result, err := handler.Handle(context.Background(), request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantError {
				assert.True(t, result.IsError)
				return
			}

			require.False(t, result.IsError)

			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)

			var response GPUNodeDescription
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tt.nodeName, response.Node.Name)
			assert.Equal(t, tt.gpuCount, response.Summary.TotalGPUs)
		})
	}
}

func TestDescribeGPUNodeHandler_Labels(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 2)
	// Add more labels to test filtering
	node.Labels["feature.node.kubernetes.io/pci-10de.present"] = "true"
	node.Labels["kubernetes.io/hostname"] = "gpu-node-1"
	node.Labels["unrelated-label"] = "should-not-appear"

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	mockNVML := nvml.NewMock(2)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Should include GPU-related labels
	assert.Contains(t, response.Node.Labels, "nvidia.com/gpu.product")
	assert.Contains(t, response.Node.Labels, "node.kubernetes.io/instance-type")
	assert.Contains(t, response.Node.Labels,
		"feature.node.kubernetes.io/pci-10de.present")
	assert.Contains(t, response.Node.Labels, "kubernetes.io/arch")

	// Should NOT include unrelated labels
	assert.NotContains(t, response.Node.Labels, "unrelated-label")
	assert.NotContains(t, response.Node.Labels, "some-other-label")
}

func TestDescribeGPUNodeHandler_Conditions(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 2)

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	mockNVML := nvml.NewMock(2)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Verify conditions
	assert.True(t, response.Node.Conditions["Ready"])
	assert.False(t, response.Node.Conditions["MemoryPressure"])
	assert.False(t, response.Node.Conditions["DiskPressure"])
}

func TestDescribeGPUNodeHandler_Taints(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 2)
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    "dedicated",
		Value:  "ml-workloads",
		Effect: corev1.TaintEffectPreferNoSchedule,
	})

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	mockNVML := nvml.NewMock(2)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Verify taints
	require.Len(t, response.Node.Taints, 2)
	assert.Equal(t, "nvidia.com/gpu", response.Node.Taints[0].Key)
	assert.Equal(t, "NoSchedule", response.Node.Taints[0].Effect)
}

func TestDescribeGPUNodeHandler_CapacityAllocatable(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 4)

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	mockNVML := nvml.NewMock(4)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Verify capacity
	assert.Equal(t, "4", response.Node.Capacity.CPU)
	assert.Equal(t, "16Gi", response.Node.Capacity.Memory)
	assert.Equal(t, "4", response.Node.Capacity.NvidiaGPU)

	// Verify allocatable
	assert.Equal(t, "3920m", response.Node.Allocatable.CPU)
	assert.Equal(t, "15Gi", response.Node.Allocatable.Memory)
	assert.Equal(t, "4", response.Node.Allocatable.NvidiaGPU)
}

func TestDescribeGPUNodeHandler_GPUSummary(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 4)
	pods := []corev1.Pod{
		makePodWithGPU("job-1", "ns1", "gpu-node-1", 2),
		makePodWithGPU("job-2", "ns2", "gpu-node-1", 1),
	}

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	for _, pod := range pods {
		_, err := clientset.CoreV1().Pods(pod.Namespace).
			Create(context.Background(), &pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	mockNVML := nvml.NewMock(4)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Verify summary
	assert.Equal(t, 4, response.Summary.TotalGPUs)
	assert.Equal(t, int64(3), response.Summary.AllocatedGPUs)
	assert.Equal(t, int64(1), response.Summary.AvailableGPUs)
	assert.Equal(t, "healthy", response.Summary.OverallHealth)
}

func TestDescribeGPUNodeHandler_WithoutNVML(t *testing.T) {
	node := makeGPUNode("gpu-node-1", 4)

	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	_, err := clientset.CoreV1().Nodes().Create(
		context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	// Handler without NVML client (nil)
	handler := NewDescribeGPUNodeHandler(clientset, nil)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"node_name": "gpu-node-1",
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var response GPUNodeDescription
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	// Should fall back to K8s capacity for GPU count
	assert.Equal(t, 4, response.Summary.TotalGPUs)
	// GPUs slice should be empty without NVML
	assert.Empty(t, response.GPUs)
	// Health should be unknown without GPU data
	assert.Equal(t, "unknown", response.Summary.OverallHealth)
}

func TestDescribeGPUNodeHandler_MissingNodeName(t *testing.T) {
	//nolint:staticcheck // NewSimpleClientset used for testing
	clientset := fake.NewSimpleClientset()
	mockNVML := nvml.NewMock(2)
	handler := NewDescribeGPUNodeHandler(clientset, mockNVML)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		// Missing node_name
	}

	result, err := handler.Handle(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestDescribeGPUNodeHandler_HealthCalculation(t *testing.T) {
	handler := &DescribeGPUNodeHandler{}

	tests := []struct {
		name           string
		gpus           []GPUDescription
		expectedHealth string
	}{
		{
			name:           "no GPUs",
			gpus:           []GPUDescription{},
			expectedHealth: "unknown",
		},
		{
			name: "all healthy",
			gpus: []GPUDescription{
				{HealthScore: 95},
				{HealthScore: 92},
			},
			expectedHealth: "healthy",
		},
		{
			name: "degraded",
			gpus: []GPUDescription{
				{HealthScore: 75},
				{HealthScore: 70},
			},
			expectedHealth: "degraded",
		},
		{
			name: "critical",
			gpus: []GPUDescription{
				{HealthScore: 50},
				{HealthScore: 40},
			},
			expectedHealth: "critical",
		},
		{
			name: "mixed scores",
			gpus: []GPUDescription{
				{HealthScore: 100},
				{HealthScore: 50},
			},
			expectedHealth: "degraded", // avg = 75
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.calculateOverallHealth(tt.gpus)
			assert.Equal(t, tt.expectedHealth, result)
		})
	}
}

func TestGetDescribeGPUNodeTool(t *testing.T) {
	tool := GetDescribeGPUNodeTool()

	assert.Equal(t, "describe_gpu_node", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.Description, "Kubernetes metadata")
	assert.Contains(t, tool.Description, "NVML")
}
