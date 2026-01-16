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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
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
		klog.V(2).InfoS("invalid EXEC_TIMEOUT",
			"value", value, "error", err, "usingDefault", fallback)
		return fallback
	}

	if d < minTimeout || d > maxTimeout {
		klog.V(2).InfoS("EXEC_TIMEOUT out of bounds",
			"value", d, "min", minTimeout, "max", maxTimeout,
			"usingDefault", fallback)
		return fallback
	}

	klog.InfoS("exec timeout configured", "timeout", d, "source", "env")
	return d
}

// Client wraps the Kubernetes clientset for GPU agent operations.
type Client struct {
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	namespace   string
	execTimeout time.Duration
	serviceName string
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithExecTimeout sets the timeout for pod exec operations.
func WithExecTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.execTimeout = d
	}
}

// WithServiceName sets the headless service name for DNS-based routing.
// This overrides the GPU_MCP_SERVICE_NAME environment variable.
func WithServiceName(name string) ClientOption {
	return func(c *Client) {
		if name != "" {
			c.serviceName = name
		}
	}
}

// AgentHTTPPort is the default port agents listen on in HTTP mode.
const AgentHTTPPort = 8080

// GPUNode represents a node with GPU agents.
type GPUNode struct {
	Name        string `json:"name"`
	PodName     string `json:"pod_name"`
	PodIP       string `json:"pod_ip"`
	Ready       bool   `json:"ready"`
	Namespace   string `json:"namespace,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// GetAgentHTTPEndpoint returns the HTTP endpoint for an agent pod.
// Returns empty string if the pod has no IP assigned.
// IPv6 addresses are wrapped in brackets per RFC 3986.
func (n GPUNode) GetAgentHTTPEndpoint() string {
	if n.PodIP == "" {
		return ""
	}
	// IPv6 addresses contain colons and must be bracketed in URLs
	if strings.Contains(n.PodIP, ":") {
		return fmt.Sprintf("http://[%s]:%d", n.PodIP, AgentHTTPPort)
	}
	return fmt.Sprintf("http://%s:%d", n.PodIP, AgentHTTPPort)
}

// GetAgentDNSEndpoint returns the DNS-based HTTP endpoint for an agent pod.
// Uses the headless service DNS format:
// <pod-name>.<service-name>.<namespace>.svc.cluster.local
// This is more reliable than direct Pod IPs when CNI has cross-node issues.
// Returns empty string if required fields are missing.
//
// Note: DNS-based routing only works when the Helm chart is deployed with
// service.headless=true and the pods have the subdomain field configured.
// See docs/troubleshooting/cross-node-networking.md for details.
func (n GPUNode) GetAgentDNSEndpoint() string {
	if n.PodName == "" || n.ServiceName == "" || n.Namespace == "" {
		return ""
	}
	return fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%d",
		n.PodName, n.ServiceName, n.Namespace, AgentHTTPPort)
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
		serviceName: DefaultServiceName,
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
		serviceName: DefaultServiceName,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// defaultServiceNameFallback is the fallback service name when not configured.
// This matches the Helm template: {{ include "k8s-gpu-mcp-server.fullname" . }}
// when using release name "gpu-mcp".
const defaultServiceNameFallback = "gpu-mcp-k8s-gpu-mcp-server"

// DefaultServiceName is the headless service name for agent pods.
// Can be overridden via GPU_MCP_SERVICE_NAME environment variable.
// This is needed for deployments with custom Helm release names or
// fullnameOverride.
var DefaultServiceName = getEnvOrDefault(
	"GPU_MCP_SERVICE_NAME",
	defaultServiceNameFallback,
)

// getEnvOrDefault returns the value of the environment variable if set,
// otherwise returns the default value.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
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
			Name:        pod.Spec.NodeName,
			PodName:     pod.Name,
			PodIP:       pod.Status.PodIP,
			Ready:       ready,
			Namespace:   c.namespace,
			ServiceName: c.serviceName,
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
			klog.ErrorS(err, "exec timeout",
				"pod", podName, "timeout", c.execTimeout,
				"durationSeconds", duration.Seconds())
			return nil, fmt.Errorf("exec timeout after %s", c.execTimeout)
		}
		return nil, fmt.Errorf("exec failed: %w (stderr: %s)",
			err, stderr.String())
	}

	klog.V(4).InfoS("exec completed",
		"pod", podName, "durationSeconds", duration.Seconds(),
		"stdoutSize", stdout.Len())

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
		Name:        pod.Spec.NodeName,
		PodName:     pod.Name,
		PodIP:       pod.Status.PodIP,
		Ready:       ready,
		Namespace:   c.namespace,
		ServiceName: c.serviceName,
	}, nil
}

// Namespace returns the configured namespace.
func (c *Client) Namespace() string {
	return c.namespace
}

// Clientset returns the underlying Kubernetes clientset.
//
// This method exposes direct access to the K8s API for handlers that need
// to perform operations not covered by the existing Client methods, such as:
//   - Querying node resources and conditions
//   - Listing pods with custom field selectors
//   - Accessing resource quota or limit range information
//
// Prefer using the existing Client methods (ListGPUNodes, GetPodForNode, etc.)
// when possible, as they provide GPU-specific abstractions. Use Clientset()
// only when you need direct API access for operations outside GPU management.
//
// Note: The returned clientset shares the same authentication and rate limiting
// configuration as the Client.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// ListNodes returns nodes matching the optional label selector.
// An empty labelSelector returns all nodes.
// Common GPU-related selectors:
//   - "nvidia.com/gpu.present=true"
//   - "node.kubernetes.io/instance-type=p4d.24xlarge"
func (c *Client) ListNodes(
	ctx context.Context,
	labelSelector string,
) ([]corev1.Node, error) {
	nodeList, err := c.clientset.CoreV1().Nodes().List(ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return nodeList.Items, nil
}

// GetNode returns a node by name.
// Returns an error if the node does not exist.
func (c *Client) GetNode(
	ctx context.Context,
	name string,
) (*corev1.Node, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", name, err)
	}
	return node, nil
}

// ListPods returns pods in the specified namespace matching the selectors.
// If namespace is empty, uses the client's configured namespace.
// If labelSelector is empty, returns all pods in the namespace.
// If fieldSelector is empty, no field filtering is applied.
//
// Common selectors:
//   - labelSelector: "app.kubernetes.io/name=my-app"
//   - fieldSelector: "spec.nodeName=gpu-node-1"
func (c *Client) ListPods(
	ctx context.Context,
	namespace string,
	labelSelector string,
	fieldSelector string,
) ([]corev1.Pod, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in %s: %w", namespace, err)
	}
	return podList.Items, nil
}

// GetPod returns a pod by namespace and name.
// If namespace is empty, uses the client's configured namespace.
func (c *Client) GetPod(
	ctx context.Context,
	namespace string,
	name string,
) (*corev1.Pod, error) {
	if namespace == "" {
		namespace = c.namespace
	}

	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name,
		metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w",
			namespace, name, err)
	}
	return pod, nil
}
