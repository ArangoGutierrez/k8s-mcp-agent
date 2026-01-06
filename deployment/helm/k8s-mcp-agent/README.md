# k8s-mcp-agent Helm Chart

Deploys the k8s-mcp-agent as a DaemonSet for on-demand GPU diagnostics.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.0+
- NVIDIA GPU nodes with one of:
  - **RuntimeClass** configured (GPU Operator or nvidia-ctk) — *recommended*
  - **NVIDIA Device Plugin** deployed — *fallback*

## Installation

### Quick Start (RuntimeClass mode)

```bash
helm install k8s-mcp-agent ./deployment/helm/k8s-mcp-agent \
  --namespace gpu-diagnostics \
  --create-namespace
```

### Fallback Mode (Device Plugin)

For clusters without RuntimeClass configured:

```bash
helm install k8s-mcp-agent ./deployment/helm/k8s-mcp-agent \
  --namespace gpu-diagnostics \
  --create-namespace \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceRequest.enabled=true
```

## GPU Access Modes

| Mode | Value | GPU Access Method | K8s Version |
|------|-------|-------------------|-------------|
| **RuntimeClass** (default) | `gpu.runtimeClass.enabled=true` | `runtimeClassName` + `NVIDIA_VISIBLE_DEVICES=all` | 1.20+ |
| **Device Plugin** | `gpu.resourceRequest.enabled=true` | `nvidia.com/gpu: 1` limit | 1.8+ |
| **DRA** | `gpu.resourceClaim.enabled=true` | `ResourceClaim` | 1.26+ (beta 1.31+) |

### RuntimeClass Mode (Recommended)

Uses the nvidia RuntimeClass for CDI injection. This is the same pattern used
by dcgm-exporter and other NVIDIA monitoring tools.

**Requirements:**
- RuntimeClass `nvidia` must exist in the cluster
- Configured via GPU Operator or manually:
  ```bash
  sudo nvidia-ctk runtime configure --runtime=containerd
  sudo systemctl restart containerd
  kubectl apply -f - <<EOF
  apiVersion: node.k8s.io/v1
  kind: RuntimeClass
  metadata:
    name: nvidia
  handler: nvidia
  EOF
  ```

### Resource Request Mode (Device Plugin Fallback)

Requests `nvidia.com/gpu` resources from the NVIDIA Device Plugin. Use this
if RuntimeClass is not available (e.g., cri-dockerd clusters).

```bash
helm install k8s-mcp-agent ./deployment/helm/k8s-mcp-agent \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceRequest.enabled=true
```

**Requirements:**
- NVIDIA Device Plugin deployed
- ⚠️ **Warning:** Consumes GPU resources from the scheduler

### DRA Mode (Dynamic Resource Allocation)

Uses Kubernetes Dynamic Resource Allocation for fine-grained GPU access.
Requires K8s 1.26+ with DynamicResourceAllocation feature gate (beta in 1.31+).

```bash
# Using an existing ResourceClaimTemplate
helm install k8s-mcp-agent ./deployment/helm/k8s-mcp-agent \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceClaim.enabled=true \
  --set gpu.resourceClaim.templateName=gpu-template

# Using inline ResourceClaim spec
helm install k8s-mcp-agent ./deployment/helm/k8s-mcp-agent \
  --set gpu.runtimeClass.enabled=false \
  --set gpu.resourceClaim.enabled=true \
  --set-json 'gpu.resourceClaim.spec={"devices":{"requests":[{"name":"gpu","deviceClassName":"gpu.nvidia.com"}]}}'
```

**Requirements:**
- NVIDIA DRA Driver deployed
- DynamicResourceAllocation feature gate enabled
- ⚠️ **Warning:** Consumes GPU resources from the scheduler

## Configuration

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gpu.runtimeClass.enabled` | Use RuntimeClass for GPU access (recommended) | `true` |
| `gpu.runtimeClass.name` | RuntimeClass name | `nvidia` |
| `gpu.resourceRequest.enabled` | Request GPU via Device Plugin | `false` |
| `gpu.resourceRequest.resource` | GPU resource name | `nvidia.com/gpu` |
| `gpu.resourceRequest.count` | Number of GPUs to request | `1` |
| `gpu.resourceClaim.enabled` | Use DRA ResourceClaim | `false` |
| `gpu.resourceClaim.name` | Claim reference name | `gpu` |
| `gpu.resourceClaim.templateName` | ResourceClaimTemplate name | `""` |
| `gpu.resourceClaim.spec` | Inline ResourceClaim spec | `{}` |
| `namespace.create` | Create dedicated namespace | `true` |
| `namespace.name` | Namespace name | `gpu-diagnostics` |
| `nodeSelector` | Node selector for GPU nodes | `nvidia.com/gpu.present: "true"` |

### Full Values Reference

See [values.yaml](values.yaml) for all configuration options.

## Usage

### Find Agent Pod

```bash
NODE_NAME=<your-gpu-node>
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-mcp-agent \
  --field-selector spec.nodeName=$NODE_NAME \
  -o jsonpath='{.items[0].metadata.name}')
```

### Start Diagnostic Session

```bash
kubectl exec -it -n gpu-diagnostics $POD -- /agent --mode=read-only
```

### Query GPU Inventory

```bash
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}' | \
  kubectl exec -i -n gpu-diagnostics $POD -- /agent
```

## Uninstallation

```bash
helm uninstall k8s-mcp-agent -n gpu-diagnostics
kubectl delete namespace gpu-diagnostics
```

## Troubleshooting

### Pod stuck in ContainerCreating

**Symptom:** `RuntimeHandler "nvidia" not supported`

**Solution:** RuntimeClass is not configured. Either:
1. Configure RuntimeClass (see Prerequisites)
2. Use fallback mode: `--set gpu.runtimeClass.enabled=false --set gpu.resourceRequest.enabled=true`

### NVML fails to initialize

**Symptom:** `failed to initialize NVML: ERROR_LIBRARY_NOT_FOUND`

**Cause:** GPU libraries not injected into container.

**Solution:**
- Verify RuntimeClass exists: `kubectl get runtimeclass nvidia`
- Check `NVIDIA_VISIBLE_DEVICES` env var is set
- Ensure nvidia-container-runtime is configured

### No pods scheduled

**Symptom:** DaemonSet shows 0 desired pods

**Cause:** No nodes match the nodeSelector.

**Solution:**
- Label GPU nodes: `kubectl label node <node> nvidia.com/gpu.present=true`
- Or deploy GPU Feature Discovery to auto-label nodes

## License

Apache License 2.0

