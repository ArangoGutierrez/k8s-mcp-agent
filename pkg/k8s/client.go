// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package k8s provides a Kubernetes client wrapper for GPU agent operations.
package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// DefaultExecTimeout is the default timeout for pod exec operations.
// Can be overridden via EXEC_TIMEOUT environment variable (e.g., "60s").
var DefaultExecTimeout = 60 * time.Second

func init() {
	if envTimeout := os.Getenv("EXEC_TIMEOUT"); envTimeout != "" {
		DefaultExecTimeout = parseExecTimeout(envTimeout, DefaultExecTimeout)
	}
}

// parseExecTimeout parses a duration string for exec timeout configuration.
// Returns the parsed duration on success, or the fallback on parse error.
// Validates that the duration is within reasonable bounds (1s to 300s).
func parseExecTimeout(value string, fallback time.Duration) time.Duration {
	const minTimeout = 1 * time.Second
	const maxTimeout = 300 * time.Second

	d, err := time.ParseDuration(value)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"invalid EXEC_TIMEOUT",`+
			`"value":"%s","error":"%v","using_default":"%s"}`,
			value, err, fallback)
		return fallback
	}

	if d < minTimeout || d > maxTimeout {
		log.Printf(`{"level":"warn","msg":"EXEC_TIMEOUT out of bounds",`+
			`"value":"%s","min":"%s","max":"%s","using_default":"%s"}`,
			d, minTimeout, maxTimeout, fallback)
		return fallback
	}

	log.Printf(`{"level":"info","msg":"exec timeout configured",`+
		`"timeout":"%s","source":"env"}`, d)
	return d
}

// Client wraps the Kubernetes clientset for GPU agent operations.
type Client struct {
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	namespace   string
	execTimeout time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithExecTimeout sets the timeout for pod exec operations.
func WithExecTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.execTimeout = d
	}
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
func NewClient(namespace string, opts ...ClientOption) (*Client, error) {
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

	c := &Client{
		clientset:   clientset,
		restConfig:  config,
		namespace:   namespace,
		execTimeout: DefaultExecTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// NewClientWithConfig creates a new Kubernetes client with provided config.
// Useful for testing with mock clients.
func NewClientWithConfig(
	clientset kubernetes.Interface,
	restConfig *rest.Config,
	namespace string,
	opts ...ClientOption,
) *Client {
	c := &Client{
		clientset:   clientset,
		restConfig:  restConfig,
		namespace:   namespace,
		execTimeout: DefaultExecTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ListGPUNodes returns all nodes running the GPU agent DaemonSet.
func (c *Client) ListGPUNodes(ctx context.Context) ([]GPUNode, error) {
	// List pods with the GPU agent label, excluding gateway pods
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server," +
				"app.kubernetes.io/component!=gateway",
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
//
// The exec operation uses a configurable timeout (default 30s) to prevent
// hanging on unresponsive pods. The timeout can be set via WithExecTimeout.
//
// Note: This function is tested via integration tests rather than unit tests
// because the fake K8s clientset does not support the exec subresource.
// See the integration testing section in docs/quickstart.md.
func (c *Client) ExecInPod(
	ctx context.Context,
	podName string,
	container string,
	stdin io.Reader,
) ([]byte, error) {
	// Apply exec timeout
	execCtx, cancel := context.WithTimeout(ctx, c.execTimeout)
	defer cancel()

	startTime := time.Now()

	execOpts := &corev1.PodExecOptions{
		Container: container,
		// Use --oneshot=2 to process exactly 2 requests (init + tool) then exit
		Command: []string{"/agent", "--nvml-mode=real", "--oneshot=2"},
		Stdin:   stdin != nil,
		Stdout:  true,
		Stderr:  true,
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
	err = exec.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})

	duration := time.Since(startTime)

	if err != nil {
		// Check if it was a timeout
		if execCtx.Err() == context.DeadlineExceeded {
			log.Printf(`{"level":"error","msg":"exec timeout","pod":"%s",`+
				`"timeout":"%s","duration":"%s"}`,
				podName, c.execTimeout, duration)
			return nil, fmt.Errorf("exec timeout after %s", c.execTimeout)
		}
		return nil, fmt.Errorf("exec failed: %w (stderr: %s)",
			err, stderr.String())
	}

	log.Printf(`{"level":"debug","msg":"exec completed","pod":"%s",`+
		`"duration":"%s","stdout_size":%d}`,
		podName, duration, stdout.Len())

	return stdout.Bytes(), nil
}

// ExecTimeout returns the configured exec timeout.
func (c *Client) ExecTimeout() time.Duration {
	return c.execTimeout
}

// GetPodForNode returns the GPU agent pod running on a specific node.
// Uses a field selector for efficient lookup in large clusters.
func (c *Client) GetPodForNode(
	ctx context.Context,
	nodeName string,
) (*GPUNode, error) {
	// Use field selector to query directly by node name for efficiency
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=k8s-gpu-mcp-server",
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
		})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for node %s: %w",
			nodeName, err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no GPU agent found on node %s", nodeName)
	}

	pod := pods.Items[0]
	ready := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady &&
			cond.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return &GPUNode{
		Name:    pod.Spec.NodeName,
		PodName: pod.Name,
		PodIP:   pod.Status.PodIP,
		Ready:   ready,
	}, nil
}

// Namespace returns the configured namespace.
func (c *Client) Namespace() string {
	return c.namespace
}
