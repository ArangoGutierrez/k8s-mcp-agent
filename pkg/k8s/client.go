// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package k8s provides a Kubernetes client wrapper for GPU agent operations.
package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// Client wraps the Kubernetes clientset for GPU agent operations.
type Client struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	namespace  string
}

// GPUNode represents a node with GPU agents.
type GPUNode struct {
	Name    string `json:"name"`
	PodName string `json:"pod_name"`
	PodIP   string `json:"pod_ip"`
	Ready   bool   `json:"ready"`
}

// NewClient creates a new Kubernetes client.
// Uses in-cluster config if available, falls back to kubeconfig.
func NewClient(namespace string) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get k8s config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset:  clientset,
		restConfig: config,
		namespace:  namespace,
	}, nil
}

// NewClientWithConfig creates a new Kubernetes client with provided config.
// Useful for testing with mock clients.
func NewClientWithConfig(
	clientset kubernetes.Interface,
	restConfig *rest.Config,
	namespace string,
) *Client {
	return &Client{
		clientset:  clientset,
		restConfig: restConfig,
		namespace:  namespace,
	}
}

// ListGPUNodes returns all nodes running the GPU agent DaemonSet.
func (c *Client) ListGPUNodes(ctx context.Context) ([]GPUNode, error) {
	// List pods with the GPU agent label
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server",
		})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	nodes := make([]GPUNode, 0, len(pods.Items))
	for _, pod := range pods.Items {
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady &&
				cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		nodes = append(nodes, GPUNode{
			Name:    pod.Spec.NodeName,
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
			Ready:   ready,
		})
	}

	return nodes, nil
}

// ExecInPod executes the agent binary in a pod with MCP request as stdin.
// Returns the stdout and any error encountered.
func (c *Client) ExecInPod(
	ctx context.Context,
	podName string,
	container string,
	stdin io.Reader,
) ([]byte, error) {
	execOpts := &corev1.PodExecOptions{
		Container: container,
		Command:   []string{"/agent", "--nvml-mode=real"},
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(c.namespace).
		SubResource("exec").
		VersionedParams(execOpts, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w (stderr: %s)",
			err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// GetPodForNode returns the GPU agent pod running on a specific node.
func (c *Client) GetPodForNode(
	ctx context.Context,
	nodeName string,
) (*GPUNode, error) {
	nodes, err := c.ListGPUNodes(ctx)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.Name == nodeName {
			return &node, nil
		}
	}

	return nil, fmt.Errorf("no GPU agent found on node %s", nodeName)
}

// Namespace returns the configured namespace.
func (c *Client) Namespace() string {
	return c.namespace
}
