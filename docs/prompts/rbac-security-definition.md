# Prompt: Define RBAC Requirements for Agent

> **Issue**: [#32 - [Security] Define RBAC requirements for agent](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/32)
> **Milestone**: M3: The Ephemeral Tunnel
> **Priority**: P1-High
> **Labels**: `kind/feature`, `ops/security`

## Objective

Define and document the minimal RBAC permissions required for the agent to function in a Kubernetes cluster, with separate configurations for read-only and operator modes.

## Context

### Current State

The project has **partial RBAC** implemented:

| Component | RBAC Status | File |
|-----------|-------------|------|
| Gateway | ✅ Complete | `deployment/helm/.../gateway-rbac.yaml` |
| Agent DaemonSet | ❌ Missing | ServiceAccount exists, no Role/Binding |
| Standalone manifests | ❌ Missing | Only Helm templates exist |
| Security documentation | ❌ Missing | No `docs/security.md` |

### Architecture Understanding

```
┌─────────────────────────────────────────────────────────────────┐
│                     RBAC Architecture                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Direct Access Path (agent needs RBAC):                         │
│  ┌─────────┐      ┌─────────────────┐      ┌────────────────┐  │
│  │ kubectl │─────▶│ Agent DaemonSet │─────▶│ K8s API Server │  │
│  │  exec   │      │  (needs RBAC)   │      │                │  │
│  └─────────┘      └─────────────────┘      └────────────────┘  │
│                                                                  │
│  Gateway Path (gateway has RBAC, agent doesn't need it):        │
│  ┌─────────┐      ┌─────────┐      ┌──────┐      ┌──────────┐  │
│  │ Client  │─────▶│ Gateway │─────▶│Agent │─────▶│ NVML/HW  │  │
│  │         │      │(has RBAC)│     │(no K8s)│    │          │  │
│  └─────────┘      └─────────┘      └──────┘      └──────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Tools and Permission Requirements

| Tool | K8s API Access | Permissions Required |
|------|----------------|---------------------|
| `get_gpu_inventory` | None | — (NVML only) |
| `get_gpu_health` | None | — (NVML only) |
| `analyze_xid_errors` | None | — (reads `/dev/kmsg`) |
| `describe_gpu_node` | Yes | `nodes: get` |
| `get_pod_gpu_allocation` | Yes | `pods: list, get` (cluster-wide) |

### Existing Gateway RBAC (Reference)

```yaml
# From deployment/helm/k8s-gpu-mcp-server/templates/gateway-rbac.yaml
# Namespace-scoped Role (for pod discovery)
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["list", "get"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]

# ClusterRole (for cluster-wide operations)
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["list", "get"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["list", "get"]
```

## Requirements

### Files to Create

1. **`deployment/rbac/agent-rbac.yaml`** — Standalone agent RBAC for non-Helm deployments
2. **`deployment/rbac/agent-rbac-readonly.yaml`** — Minimal read-only permissions
3. **`deployment/rbac/agent-rbac-namespaced.yaml`** — Namespace-scoped variant
4. **`docs/security.md`** — Security model documentation
5. **Helm template update** — Add agent RBAC to existing Helm chart

### RBAC Definitions

#### Read-Only Mode (Default)

```yaml
# deployment/rbac/agent-rbac-readonly.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-gpu-mcp-server-agent-readonly
  labels:
    app.kubernetes.io/name: k8s-gpu-mcp-server
    app.kubernetes.io/component: agent
rules:
# Node information for describe_gpu_node
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
# Pod information for get_pod_gpu_allocation
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
# NO: pods/exec - agent doesn't exec into other pods
# NO: secrets - agent never needs secrets
# NO: configmaps - agent reads config from flags/env only
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-gpu-mcp-server-agent-readonly
  labels:
    app.kubernetes.io/name: k8s-gpu-mcp-server
    app.kubernetes.io/component: agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k8s-gpu-mcp-server-agent-readonly
subjects:
- kind: ServiceAccount
  name: k8s-gpu-mcp-server
  namespace: gpu-diagnostics
```

#### Operator Mode (Future)

```yaml
# deployment/rbac/agent-rbac-operator.yaml (for future --mode=operator)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-gpu-mcp-server-agent-operator
rules:
# Include all read-only permissions
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
# Additional: Pod eviction for kill_gpu_process (future)
- apiGroups: [""]
  resources: ["pods/eviction"]
  verbs: ["create"]
# Additional: Node status patch for GPU health reporting (future)
# - apiGroups: [""]
#   resources: ["nodes/status"]
#   verbs: ["patch"]
```

#### Namespace-Scoped (Multi-Tenant)

```yaml
# deployment/rbac/agent-rbac-namespaced.yaml
# For environments where cluster-wide access is not permitted
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: k8s-gpu-mcp-server-agent
  namespace: gpu-diagnostics
rules:
# Limited to pods in the same namespace
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: k8s-gpu-mcp-server-agent
  namespace: gpu-diagnostics
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: k8s-gpu-mcp-server-agent
subjects:
- kind: ServiceAccount
  name: k8s-gpu-mcp-server
  namespace: gpu-diagnostics
```

### Helm Chart Update

Add to `deployment/helm/k8s-gpu-mcp-server/templates/agent-rbac.yaml`:

```yaml
{{/*
Copyright 2026 k8s-gpu-mcp-server contributors
SPDX-License-Identifier: Apache-2.0
*/}}
{{- if .Values.agent.rbac.create }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-agent
  labels:
    {{- include "k8s-gpu-mcp-server.labels" . | nindent 4 }}
    app.kubernetes.io/component: agent
rules:
# Node information for describe_gpu_node tool
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
# Pod information for get_pod_gpu_allocation tool
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
{{- if eq .Values.agent.mode "operator" }}
# Operator mode: allow pod eviction for kill_gpu_process
- apiGroups: [""]
  resources: ["pods/eviction"]
  verbs: ["create"]
{{- end }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-agent
  labels:
    {{- include "k8s-gpu-mcp-server.labels" . | nindent 4 }}
    app.kubernetes.io/component: agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "k8s-gpu-mcp-server.fullname" . }}-agent
subjects:
- kind: ServiceAccount
  name: {{ include "k8s-gpu-mcp-server.serviceAccountName" . }}
  namespace: {{ include "k8s-gpu-mcp-server.namespace" . }}
{{- end }}
```

Update `values.yaml`:

```yaml
# Agent configuration
agent:
  # -- Operation mode: read-only or operator
  mode: "read-only"
  
  # RBAC configuration for agent DaemonSet
  rbac:
    # -- Create RBAC resources for agent
    # Enable if agents need direct K8s API access (describe_gpu_node, get_pod_gpu_allocation)
    # Disable if all access is through gateway (gateway already has RBAC)
    create: true
```

### Security Documentation

Create `docs/security.md`:

```markdown
# Security Model

This document describes the security architecture, RBAC requirements, and
best practices for deploying k8s-gpu-mcp-server.

## Table of Contents

- [Principle of Least Privilege](#principle-of-least-privilege)
- [Component Permissions](#component-permissions)
- [RBAC Configuration](#rbac-configuration)
- [Security Contexts](#security-contexts)
- [Network Security](#network-security)
- [Capability Requirements](#capability-requirements)

## Principle of Least Privilege

The agent follows strict least-privilege principles:

| Principle | Implementation |
|-----------|----------------|
| **No Secrets Access** | Agent never reads K8s secrets |
| **No Exec** | Agent doesn't exec into other pods |
| **No Mutations** | Read-only mode is default |
| **Minimal Network** | No listening ports in stdio mode |
| **No Privilege Escalation** | `allowPrivilegeEscalation: false` |

## Component Permissions

### Agent DaemonSet

The agent needs K8s API access for two tools:

| Tool | Resources | Verbs | Scope |
|------|-----------|-------|-------|
| `describe_gpu_node` | `nodes` | `get`, `list` | Cluster |
| `get_pod_gpu_allocation` | `pods` | `get`, `list` | Cluster |

All other tools (`get_gpu_inventory`, `get_gpu_health`, `analyze_xid_errors`)
use only local NVML or `/dev/kmsg` access.

### Gateway

The gateway requires additional permissions:

| Purpose | Resources | Verbs |
|---------|-----------|-------|
| Discover agent pods | `pods` | `get`, `list` |
| Exec routing (stdio mode) | `pods/exec` | `create` |
| Node info aggregation | `nodes` | `get`, `list` |

## RBAC Configuration

### Helm (Recommended)

```bash
# Default: agent RBAC enabled
helm install gpu-mcp ./deployment/helm/k8s-gpu-mcp-server

# Gateway-only mode (agents don't need RBAC)
helm install gpu-mcp ./deployment/helm/k8s-gpu-mcp-server \
  --set agent.rbac.create=false \
  --set gateway.enabled=true
```

### Standalone Manifests

```bash
# Read-only mode (default)
kubectl apply -f deployment/rbac/agent-rbac-readonly.yaml

# Namespace-scoped (multi-tenant)
kubectl apply -f deployment/rbac/agent-rbac-namespaced.yaml

# Operator mode (future)
kubectl apply -f deployment/rbac/agent-rbac-operator.yaml
```

## Security Contexts

### Agent Container

```yaml
securityContext:
  runAsUser: 0                    # May be required for NVML
  allowPrivilegeEscalation: false # Prevent privilege escalation
  readOnlyRootFilesystem: true    # Immutable container
  capabilities:
    drop: ["ALL"]
    add:
      - SYSLOG                    # Required for /dev/kmsg (XID errors)
```

### Gateway Container

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534                # nobody
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
```

## Network Security

### NetworkPolicy (Optional)

```yaml
# Restrict agent communication
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: k8s-gpu-mcp-server-agent
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: k8s-gpu-mcp-server
      app.kubernetes.io/component: gpu-diagnostics
  policyTypes:
  - Ingress
  ingress:
  # Only allow gateway to connect
  - from:
    - podSelector:
        matchLabels:
          app.kubernetes.io/component: gateway
    ports:
    - port: 8080
```

## Capability Requirements

| Capability | Required For | Mode |
|------------|--------------|------|
| `CAP_SYSLOG` | `/dev/kmsg` read for XID errors | Always |
| `CAP_SYS_ADMIN` | GPU profiling metrics (optional) | Optional |

## Graceful Permission Failures

The agent handles missing permissions gracefully:

```go
// describe_gpu_node returns partial data if K8s access fails
if err := h.getNodeInfo(ctx, nodeName); err != nil {
    klog.V(2).InfoS("K8s node access failed, returning NVML-only data",
        "node", nodeName, "error", err)
    // Return GPU hardware data without K8s metadata
}
```

## Verification

Test RBAC permissions:

```bash
# Check agent permissions
kubectl auth can-i get nodes \
  --as=system:serviceaccount:gpu-diagnostics:k8s-gpu-mcp-server

kubectl auth can-i list pods \
  --as=system:serviceaccount:gpu-diagnostics:k8s-gpu-mcp-server

# Should return "no" for secrets
kubectl auth can-i get secrets \
  --as=system:serviceaccount:gpu-diagnostics:k8s-gpu-mcp-server
```
```

### Graceful Permission Failure

Update `pkg/tools/describe_gpu_node.go` to handle permission errors:

```go
// Handle returns GPU node description with graceful K8s fallback.
func (h *DescribeGPUNodeHandler) Handle(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    // ... existing code ...
    
    // Try to get K8s node info, but don't fail if RBAC denies access
    var nodeInfo *NodeInfo
    if h.clientset != nil {
        node, err := h.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
        if err != nil {
            klog.V(2).InfoS("K8s node access unavailable",
                "node", nodeName,
                "error", err,
                "hint", "Agent may lack RBAC permissions, returning NVML-only data")
            // Continue without K8s metadata
        } else {
            nodeInfo = extractNodeInfo(node)
        }
    }
    
    // Always return NVML data, K8s metadata is optional
    // ...
}
```

### CI Validation

Add to `Makefile`:

```makefile
.PHONY: validate-rbac
validate-rbac: ## Validate RBAC manifests
	@echo "Validating RBAC manifests..."
	kubectl apply --dry-run=client -f deployment/rbac/
	@echo "Checking Helm RBAC templates..."
	helm template test ./deployment/helm/k8s-gpu-mcp-server \
		--set agent.rbac.create=true | kubectl apply --dry-run=client -f -
	helm template test ./deployment/helm/k8s-gpu-mcp-server \
		--set gateway.enabled=true | kubectl apply --dry-run=client -f -
```

Add to CI workflow:

```yaml
# .github/workflows/ci.yaml
- name: Validate RBAC manifests
  run: |
    # Install kubectl if not present
    if ! command -v kubectl &> /dev/null; then
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
      chmod +x kubectl
      sudo mv kubectl /usr/local/bin/
    fi
    make validate-rbac
```

## Implementation Steps

### Phase 1: Standalone RBAC Manifests

1. Create `deployment/rbac/` directory
2. Create `agent-rbac-readonly.yaml`
3. Create `agent-rbac-namespaced.yaml`
4. Create `agent-rbac-operator.yaml` (placeholder for future)

### Phase 2: Helm Chart Update

1. Create `templates/agent-rbac.yaml`
2. Update `values.yaml` with `agent.rbac.create`
3. Update `templates/NOTES.txt` with RBAC info

### Phase 3: Documentation

1. Create `docs/security.md`
2. Update `docs/architecture.md` security section
3. Update `README.md` with security highlights

### Phase 4: Graceful Failures

1. Update `describe_gpu_node.go` for permission errors
2. Update `pod_gpu_allocation.go` for permission errors
3. Add error context for RBAC troubleshooting

### Phase 5: CI Validation

1. Add `validate-rbac` Makefile target
2. Add CI workflow step
3. Test with `--dry-run=client`

## Progress Tracker

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[TODO]` | `feat/rbac-security-definition` |
| 1 | Create `deployment/rbac/` directory | `[TODO]` | |
| 2 | Create agent-rbac-readonly.yaml | `[TODO]` | |
| 3 | Create agent-rbac-namespaced.yaml | `[TODO]` | |
| 4 | Create agent-rbac-operator.yaml | `[TODO]` | Placeholder |
| 5 | Create Helm agent-rbac.yaml template | `[TODO]` | |
| 6 | Update values.yaml with agent.rbac | `[TODO]` | |
| 7 | Create docs/security.md | `[TODO]` | |
| 8 | Update describe_gpu_node.go graceful failure | `[TODO]` | |
| 9 | Update pod_gpu_allocation.go graceful failure | `[TODO]` | |
| 10 | Add validate-rbac Makefile target | `[TODO]` | |
| 11 | Run full test suite | `[TODO]` | `make all` |
| 12 | Test in real cluster | `[TODO]` | If KUBECONFIG available |
| 13 | Create pull request | `[TODO]` | Closes #32 |
| 14 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 15 | Address review comments | `[TODO]` | |
| 16 | **Merge after reviews** | `[WAIT]` | ⚠️ **Requires human approval** |

## Acceptance Criteria

From Issue #32:

- [ ] RBAC manifests created and tested
- [ ] Agent works with minimal permissions
- [ ] Agent fails gracefully without permissions
- [ ] Security documentation complete
- [ ] CI validates RBAC manifests

## Testing Commands

```bash
# Create feature branch
git checkout -b feat/rbac-security-definition

# Validate standalone manifests
kubectl apply --dry-run=client -f deployment/rbac/

# Validate Helm templates
helm template test ./deployment/helm/k8s-gpu-mcp-server \
  --set agent.rbac.create=true | kubectl apply --dry-run=client -f -

# Run tests
make all

# Test RBAC in real cluster (if available)
helm upgrade --install gpu-mcp ./deployment/helm/k8s-gpu-mcp-server \
  --set agent.rbac.create=true \
  --namespace gpu-diagnostics

# Verify permissions
kubectl auth can-i get nodes \
  --as=system:serviceaccount:gpu-diagnostics:gpu-mcp-k8s-gpu-mcp-server

kubectl auth can-i list pods \
  --as=system:serviceaccount:gpu-diagnostics:gpu-mcp-k8s-gpu-mcp-server

# Should be "no"
kubectl auth can-i get secrets \
  --as=system:serviceaccount:gpu-diagnostics:gpu-mcp-k8s-gpu-mcp-server
```

## PR Template

```markdown
## Summary

Implements RBAC requirements for the agent per issue #32.

## Changes

- Add standalone RBAC manifests in `deployment/rbac/`
- Add agent RBAC to Helm chart with `agent.rbac.create` toggle
- Create security documentation in `docs/security.md`
- Implement graceful permission failure handling
- Add CI validation for RBAC manifests

## RBAC Summary

| Component | ClusterRole | Permissions |
|-----------|-------------|-------------|
| Agent (read-only) | `k8s-gpu-mcp-server-agent` | nodes: get/list, pods: get/list |
| Agent (operator) | `k8s-gpu-mcp-server-agent-operator` | + pods/eviction: create |
| Gateway | `k8s-gpu-mcp-server-gateway` | nodes: get/list, pods: get/list/exec |

## Security Considerations

- Follows least-privilege principle
- No secrets access
- No exec into other pods (agent)
- Graceful degradation when permissions unavailable
- Mode-based permission separation (read-only vs operator)

## Testing

- [x] `make all` passes
- [x] RBAC manifests validate with `--dry-run=client`
- [x] Helm template renders correctly
- [ ] Tested in real cluster (optional)

Closes #32
```

## References

- [Issue #32](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/32)
- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Existing gateway-rbac.yaml](../deployment/helm/k8s-gpu-mcp-server/templates/gateway-rbac.yaml)
- [Architecture docs](./architecture.md)
