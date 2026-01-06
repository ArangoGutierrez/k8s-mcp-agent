# Create DaemonSet Deployment Manifests for Kubernetes

## Issue Reference

- **Issue:** [#62 - feat: Add DaemonSet deployment manifests for Kubernetes](https://github.com/ArangoGutierrez/k8s-mcp-agent/issues/62)
- **Priority:** P1-High
- **Labels:** kind/feature, area/k8s-ephemeral, ops/security

## Background

Based on the [Architecture Decision Report](../reports/k8s-deploy-architecture-decision.md),
`kubectl debug` cannot access GPUs because ephemeral containers bypass the NVIDIA
device plugin. The recommended deployment pattern is a DaemonSet with
`runtimeClassName: nvidia` and `kubectl exec` for on-demand diagnostics.

## Objective

Create production-ready Kubernetes manifests for deploying k8s-mcp-agent to GPU
clusters with NVIDIA Device Plugin, GPU Operator, or DRA driver.

## Step 0: Create Feature Branch

```bash
git checkout main
git pull origin main
git checkout -b feat/daemonset-manifests
```

## Files to Create

### 1. `deployment/helm/k8s-mcp-agent/templates/namespace.yaml`

Dedicated namespace for GPU diagnostics:
- Name: `gpu-diagnostics` (or `k8s-mcp-agent`)
- Labels for identification

### 2. `deployment/helm/k8s-mcp-agent/templates/serviceaccount.yaml`

ServiceAccount and RBAC configuration:
- ServiceAccount for the DaemonSet
- Minimal permissions (read-only cluster info if needed)
- Consider future operator mode permissions

### 3. `deployment/helm/k8s-mcp-agent/templates/daemonset.yaml`

Primary DaemonSet manifest with RuntimeClass:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k8s-mcp-agent
  namespace: gpu-diagnostics
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: k8s-mcp-agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: k8s-mcp-agent
        app.kubernetes.io/component: gpu-diagnostics
    spec:
      runtimeClassName: nvidia  # CDI injection for GPU access
      nodeSelector:
        nvidia.com/gpu.present: "true"
      tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      serviceAccountName: k8s-mcp-agent
      containers:
      - name: agent
        image: ghcr.io/arangogutierrez/k8s-mcp-agent:latest
        command: ["sleep", "infinity"]
        stdin: true
        tty: true
        securityContext:
          runAsUser: 0
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]
        resources:
          requests:
            cpu: 1m
            memory: 10Mi
          limits:
            cpu: 100m
            memory: 50Mi
        # NO nvidia.com/gpu resource - monitors all GPUs without allocation
```

### 4. `deployment/helm/k8s-mcp-agent/values.yaml`

Kustomize base for easy customization:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- namespace.yaml
- rbac.yaml
- daemonset.yaml
```

## Security Requirements (Zero Trust)

Follow the principle of least privilege:

| Aspect | Requirement |
|--------|-------------|
| `privileged` | ❌ Never (use RuntimeClass instead) |
| `runAsNonRoot` | ❌ false (root may be needed for NVML) |
| `runAsUser` | 0 (root) |
| `allowPrivilegeEscalation` | ❌ false |
| `readOnlyRootFilesystem` | ✅ true |
| `capabilities` | Drop ALL, add only if proven necessary |
| `hostNetwork` | ❌ false |
| `hostPID` | ❌ false |
| `nvidia.com/gpu` | ❌ Do NOT request (don't block scheduler) |

## GPU Access Requirements

The DaemonSet requires one of the following cluster configurations:

### Option A: RuntimeClass (Preferred)

```yaml
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
```

The RuntimeClass must be configured by cluster admins (typically done by GPU
Operator or manual nvidia-container-toolkit setup).

### Option B: GPU Operator

If NVIDIA GPU Operator is installed, RuntimeClass is automatically configured.

### Option C: DRA Driver

For Kubernetes 1.32+ with DRA, the manifest may need adjustments for
ResourceClaim-based GPU access.

## Usage Pattern

Once deployed, users diagnose GPUs with:

```bash
# Find pod on target node
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-mcp-agent \
  --field-selector spec.nodeName=<node-name> \
  -o jsonpath='{.items[0].metadata.name}')

# Start diagnostic session
kubectl exec -it -n gpu-diagnostics $POD -- /agent --mode=read-only
```

Future `kubectl mcp` plugin will simplify this to:
```bash
kubectl mcp diagnose <node-name>
```

## Testing Checklist

- [ ] Deploy to cluster with NVIDIA Device Plugin
- [ ] Verify pod starts on GPU nodes only
- [ ] Verify `kubectl exec` successfully runs agent
- [ ] Verify agent can access all GPUs on node
- [ ] Verify agent exits cleanly on disconnect
- [ ] Verify no GPU resources consumed (scheduler sees all GPUs as available)
- [ ] Test with GPU Operator cluster
- [ ] Test toleration for tainted GPU nodes

## Documentation Updates

- [ ] Update README.md with deployment instructions
- [ ] Update docs/quickstart.md with Kubernetes section
- [ ] Reference manifests in docs/architecture.md

## Related Files

- `docs/reports/k8s-deploy-architecture-decision.md` — Architecture decision
- `docs/architecture.md` — System architecture
- `deployment/Containerfile` — Container image build

## Notes

- Start with minimal security context; add capabilities only if NVML requires
- Test on real GPU cluster before merging
- Consider Helm chart as future enhancement (not required for this issue)

