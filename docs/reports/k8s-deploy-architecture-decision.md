# Architecture Decision Report: k8s-gpu-mcp-server Kubernetes Deployment

**Date:** January 6, 2026  
**Branch:** `feat/k8s-deploy-test`  
**Status:** Analysis Complete - Decision Required

---

## Executive Summary

The original "Syringe Pattern" architecture using `kubectl debug` for ephemeral GPU
diagnostics **does not work** because ephemeral containers cannot access GPUs. This
report analyzes alternative deployment patterns for clusters with NVIDIA Device Plugin
or DRA driver, with or without the GPU Operator.

**Key Finding:** GPU access in Kubernetes requires either:
1. CDI injection via nvidia-container-toolkit (runtimeClassName or device plugin)
2. Privileged mode with hostPath mounts (fallback)

---

## Table of Contents

1. [Test Environment](#test-environment)
2. [Discovery: kubectl debug Limitation](#discovery-kubectl-debug-limitation)
3. [How GPU Access Works in Kubernetes](#how-gpu-access-works-in-kubernetes)
4. [Analysis: dcgm-exporter Approach](#analysis-dcgm-exporter-approach)
5. [Deployment Scenarios](#deployment-scenarios)
6. [Architecture Options](#architecture-options)
7. [Security Analysis](#security-analysis)
8. [Recommendation](#recommendation)
9. [Next Steps](#next-steps)

---

## 1. Test Environment

| Component | Version/Details |
|-----------|-----------------|
| Kubernetes | v1.33.3 |
| Node | `ip-10-0-0-194` (AWS EC2) |
| GPU | Tesla T4 (15GB) |
| NVIDIA Driver | 575.57.08 |
| CUDA | 12.9 |
| Container Runtime | Docker 29.1.3 |
| nvidia-container-toolkit | 1.18.1 |
| CDI | Configured (`/var/run/cdi/nvidia.yaml`) |
| Device Plugin | **Not installed** |

---

## 2. Discovery: kubectl debug Limitation

### Original Design (Syringe Pattern)

```
kubectl debug node/gpu-node-5 \
  --image=ghcr.io/.../k8s-gpu-mcp-server:latest \
  -- /agent --nvml-mode=real
```

**Expected:** Ephemeral container with GPU access  
**Actual:** Container starts but **NVML fails to initialize**

### Root Cause

`kubectl debug` creates ephemeral containers that:
- Cannot request `nvidia.com/gpu` resources
- Cannot specify volume mounts
- Cannot use runtimeClassName
- Bypass the nvidia device plugin entirely

### Verification

```bash
# kubectl debug - FAILS
kubectl debug node/ip-10-0-0-194 \
  --image=ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest \
  -- /agent --nvml-mode=real
# Result: "failed to initialize NVML: Unknown Error"

# docker run --gpus all - WORKS
docker run --rm -i --gpus all \
  ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
# Result: Tesla T4 detected, MCP protocol functional
```

**Conclusion:** The Syringe Pattern via `kubectl debug` is not viable for GPU access.

---

## 3. How GPU Access Works in Kubernetes

### CDI (Container Device Interface)

Modern NVIDIA GPU access uses CDI, which injects:

```yaml
# From /var/run/cdi/nvidia.yaml
containerEdits:
  deviceNodes:
    - path: /dev/nvidia-modeset
    - path: /dev/nvidia-uvm
    - path: /dev/nvidia-uvm-tools
    - path: /dev/nvidiactl
    - path: /dev/nvidia0
  hooks:
    - hookName: createContainer
      path: /usr/bin/nvidia-cdi-hook
      args:
        - nvidia-cdi-hook
        - create-symlinks
        - --link
        - libnvidia-ml.so.575.57.08::/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.1
        # ... 30+ library symlinks
```

### How Pods Get GPU Access

| Method | Mechanism | When Used |
|--------|-----------|-----------|
| `--gpus all` (Docker) | nvidia runtime + CDI | Direct Docker usage |
| `nvidia.com/gpu: 1` | Device Plugin + CDI | Standard K8s GPU pods |
| `runtimeClassName: nvidia` | RuntimeClass + CDI | Alternative to device plugin |
| DRA ResourceClaim | k8s-dra-driver + CDI | Dynamic Resource Allocation |

### What Simple hostPath Mounts Lack

Tested approach that **failed**:
```yaml
volumes:
- name: nvidia-ctl
  hostPath:
    path: /dev/nvidiactl
    type: CharDevice
```

**Why it fails:**
- No cgroup device allowlist configuration
- No library symlinks created
- No NVIDIA environment variables set
- NVML needs the full CDI injection, not just device files

---

## 4. Analysis: dcgm-exporter Approach

NVIDIA's official GPU monitoring solution provides a reference implementation:

### dcgm-exporter Security Context

```yaml
securityContext:
  runAsNonRoot: false
  runAsUser: 0
  capabilities:
    add: ["SYS_ADMIN"]  # Required for profiling metrics
    drop: ["ALL"]
  allowPrivilegeEscalation: false
```

### Key Observations

| Aspect | dcgm-exporter | Implication for k8s-gpu-mcp-server |
|--------|---------------|-------------------------------|
| Runs as root | Yes | May be required for NVML |
| Privileged | No | Good - can avoid full privileged |
| Capabilities | SYS_ADMIN | May be needed for some metrics |
| GPU resource request | No | Monitors all GPUs without allocation |
| runtimeClassName | Optional | Can use nvidia runtime |
| DRA support | Yes (v4.4+) | Explicit `kubernetesDRA.enabled` |

### How dcgm-exporter Gets GPU Access

1. **With Device Plugin:** Uses device plugin's CDI injection
2. **With RuntimeClass:** Uses nvidia runtime directly
3. **With GPU Operator:** Operator configures everything automatically

**Critical:** dcgm-exporter does NOT request `nvidia.com/gpu` resources.
It monitors ALL GPUs without blocking the scheduler.

---

## 5. Deployment Scenarios

### Scenario A: Cluster with NVIDIA Device Plugin

**Prerequisites:**
- nvidia-container-toolkit on nodes
- nvidia device plugin DaemonSet deployed
- Optional: RuntimeClass configured

**How it works:**
- Device plugin advertises `nvidia.com/gpu` resources
- Pods requesting GPUs get CDI injection automatically
- Monitoring pods can use `runtimeClassName: nvidia` without resource request

### Scenario B: Cluster with GPU Operator

**Prerequisites:**
- NVIDIA GPU Operator installed

**How it works:**
- Operator deploys driver, toolkit, device plugin, runtime
- All GPU access handled automatically
- Most straightforward for GPU clusters

### Scenario C: Cluster with DRA Driver (k8s-dra-driver)

**Prerequisites:**
- Kubernetes 1.32+ with DRA feature gate
- nvidia k8s-dra-driver installed

**How it works:**
- ResourceClaim defines GPU requirements
- DRA scheduler handles placement
- More flexible than device plugin
- dcgm-exporter has explicit DRA support

### Scenario D: Bare Cluster (No GPU Infrastructure)

**Prerequisites:**
- nvidia-container-toolkit on nodes
- Docker configured with nvidia runtime

**How it works:**
- Only `docker run --gpus all` works
- Kubernetes needs privileged mode for GPU access
- **This is our test environment**

---

## 6. Architecture Options

### Option 1: DaemonSet with RuntimeClass (Preferred)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k8s-gpu-mcp-server
spec:
  selector:
    matchLabels:
      app: k8s-gpu-mcp-server
  template:
    spec:
      runtimeClassName: nvidia  # Uses nvidia runtime for CDI injection
      nodeSelector:
        nvidia.com/gpu.present: "true"
      containers:
      - name: agent-shell
        image: ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
        command: ["sleep", "infinity"]
        securityContext:
          runAsUser: 0
          capabilities:
            add: ["SYS_ADMIN"]
            drop: ["ALL"]
          allowPrivilegeEscalation: false
```

**Usage:**
```bash
kubectl mcp diagnose gpu-node-5
# Plugin internally runs:
# kubectl exec -it $(find-pod-on-node gpu-node-5) -- /agent
```

**Pros:**
- No `nvidia.com/gpu` resource request (doesn't block scheduler)
- Minimal privileges (matches dcgm-exporter)
- Agent runs only during diagnosis (exec pattern)
- Works with Device Plugin, Operator, or RuntimeClass

**Cons:**
- Requires `runtimeClassName: nvidia` to be available
- DaemonSet means 100+ pods on large clusters (but minimal footprint)

### Option 2: DaemonSet with Device Plugin Passthrough

For clusters where device plugin handles CDI but no RuntimeClass:

```yaml
spec:
  containers:
  - name: agent-shell
    resources:
      limits:
        nvidia.com/gpu: 0  # Special: gets CDI injection but no GPU allocation
```

**Note:** Requires device plugin v0.16+ with `--pass-device-specs` flag.

### Option 3: DaemonSet with Privileged Mode (Fallback)

> ⚠️ **SECURITY WARNING**: This option grants full root-level access to the host.
> Only use in tightly controlled, admin-only clusters where RuntimeClass is not
> available. A compromise of the agent or misuse of `kubectl exec` would allow
> full node takeover. **This is NOT recommended for production or multi-tenant
> environments.**

For clusters without proper GPU infrastructure:

```yaml
spec:
  containers:
  - name: agent-shell
    securityContext:
      privileged: true
    volumeMounts:
    - name: nvidia-ctl
      mountPath: /dev/nvidiactl
    - name: nvidia0
      mountPath: /dev/nvidia0
    - name: nvidia-ml
      mountPath: /usr/lib/x86_64-linux-gnu/libnvidia-ml.so.1
  volumes:
  - name: nvidia-ctl
    hostPath:
      path: /dev/nvidiactl
      type: CharDevice
  # ... additional mounts
```

**Pros:**
- Works on any cluster with GPUs
- No dependency on device plugin/operator

**Cons:**
- Requires `privileged: true` — **major security risk**
- Manual hostPath mounts
- Driver-version-specific library paths

**Required Mitigations (if used):**
- Deploy in dedicated namespace with strict RBAC
- Limit `kubectl exec` access to cluster admins only
- Use NetworkPolicy to isolate pods
- Enable audit logging for all exec operations
- Consider this a temporary measure until RuntimeClass is configured

### Option 4: On-Demand Pod (Ephemeral Alternative)

Instead of DaemonSet, create pods on-demand:

```bash
kubectl mcp diagnose gpu-node-5
# Plugin creates:
#   kubectl run gpu-diag-gpu-node-5 --image=... \
#     --overrides='{"spec":{"runtimeClassName":"nvidia",...}}' \
#     -it -- /agent
# Then deletes pod on exit
```

**Pros:**
- Zero footprint when not in use
- Truly ephemeral (closer to original vision)

**Cons:**
- 5-10s startup overhead
- Complex kubectl plugin logic
- No instant access

---

## 7. Security Analysis

### Privilege Comparison

| Approach | Privileged | Capabilities | Risk Level |
|----------|------------|--------------|------------|
| dcgm-exporter | No | SYS_ADMIN | Medium |
| RuntimeClass + SYS_ADMIN | No | SYS_ADMIN | Medium |
| RuntimeClass + no caps | No | None | Low |
| Privileged mode | Yes | ALL | High |

### What SYS_ADMIN Enables

- Access to certain NVML profiling APIs
- Some XID error retrieval methods
- Not required for basic GPU inventory/health

### Recommended Security Posture

```yaml
securityContext:
  runAsUser: 0              # Root may be required for NVML
  runAsNonRoot: false
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
    # Add SYS_ADMIN only if profiling metrics needed:
    # add: ["SYS_ADMIN"]
```

### Zero Trust Considerations

1. **Network Isolation:** No hostNetwork, no exposed ports
2. **RBAC:** Strict ServiceAccount with minimal permissions
3. **Audit Trail:** Pod creation/exec logged in audit log
4. **Time-Bounded:** Agent runs only during diagnosis

---

## 8. Recommendation

### Primary Deployment: DaemonSet with RuntimeClass

**Target:** Clusters with NVIDIA Device Plugin, GPU Operator, or DRA driver

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k8s-gpu-mcp-server
  namespace: gpu-diagnostics
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: k8s-gpu-mcp-server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: k8s-gpu-mcp-server
    spec:
      runtimeClassName: nvidia
      nodeSelector:
        nvidia.com/gpu.present: "true"
      tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      containers:
      - name: agent
        image: ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
        command: ["sleep", "infinity"]
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
        # NO nvidia.com/gpu resource request - monitors all GPUs
```

### Usage Pattern

```bash
# kubectl plugin handles pod discovery
kubectl mcp diagnose <node-name>

# Under the hood:
POD=$(kubectl get pods -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  --field-selector spec.nodeName=<node-name> -o name)
kubectl exec -it $POD -- /agent --mode=read-only
```

### Fallback for Bare Clusters

Document privileged mode as fallback with clear security trade-off warnings.

---

## 9. Next Steps

### Immediate Actions

1. [ ] Update `docs/architecture.md` with new deployment pattern
2. [x] Create `deployment/helm/k8s-gpu-mcp-server/` Helm chart
3. [x] Support RuntimeClass, Device Plugin, and DRA modes
4. [ ] Update README.md deployment instructions
5. [ ] Start kubectl plugin development (`kubectl-mcp`)

### Future Enhancements

1. [ ] Test with NVIDIA GPU Operator
2. [ ] Test with k8s-dra-driver (DRA)
3. [ ] Add RuntimeClass auto-detection in kubectl plugin
4. [ ] Explore non-root execution (may require upstream NVML changes)

### Open Questions

1. **Non-root NVML:** Can we run NVML as non-root with proper capabilities?
2. **DRA specifics:** How does monitoring work with DRA ResourceClaims?
3. **Air-gapped:** Additional requirements for air-gapped environments?

---

## Appendix: Test Results

### Docker (Direct) - WORKS

```bash
docker run --rm -i --gpus all \
  ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

```json
{
  "cuda_version": "12.9",
  "device_count": 1,
  "devices": [{
    "name": "Tesla T4",
    "uuid": "GPU-d129fc5b-2d51-cec7-d985-49168c12716f",
    "temperature": {"current_celsius": 28},
    "power": {"current_mw": 13837, "limit_mw": 70000},
    "ecc": {"enabled": true}
  }],
  "driver_version": "575.57.08"
}
```

### Docker Minimal Privileges - WORKS

```bash
docker run --rm -i \
  --user 65532:65532 \
  --cap-drop ALL \
  --device /dev/nvidiactl \
  --device /dev/nvidia0 \
  -v /usr/.../libnvidia-ml.so.1:/usr/.../libnvidia-ml.so.1:ro \
  ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

Result: **Full GPU access with non-root, no capabilities**

### Kubernetes hostPath - FAILS

```yaml
volumes:
- name: nvidia-ctl
  hostPath:
    path: /dev/nvidiactl
    type: CharDevice
```

Result: **"failed to initialize NVML: Unknown Error"**

### Kubernetes Privileged - WORKS

```yaml
securityContext:
  privileged: true
```

Result: **Full GPU access**

---

## 10. Future Enhancement: eBPF Integration

### Overview

eBPF (extended Berkeley Packet Filter) could complement NVML for specific use cases
where kernel-level tracing provides advantages over userspace APIs.

### What eBPF Can Do

| Use Case | eBPF Capability | Benefit |
|----------|-----------------|---------|
| **XID Error Detection** | Trace `printk`/dmesg in real-time | Lower latency than polling dmesg |
| **Process GPU Access** | Trace ioctls to `/dev/nvidia*` | Better process correlation |
| **Driver Call Tracing** | Trace NVML's underlying syscalls | Debug driver issues |
| **Real-time Event Streaming** | Stream GPU events as they happen | Proactive alerting |

### What eBPF Cannot Replace

Core GPU metrics **require NVML** — eBPF cannot access:
- Temperature, power, memory usage
- Utilization counters
- ECC error counts
- Clock speeds, throttling status

These are hardware registers accessed via the NVIDIA driver's proprietary API.

### Potential Architecture

```
┌─────────────────────────────────────────────────────────────┐
│               eBPF-Enhanced Architecture (Future)           │
├─────────────────────────────────────────────────────────────┤
│  MCP Tools                                                   │
│  ├─ get_gpu_inventory ──────────► NVML API                  │
│  ├─ get_gpu_health ─────────────► NVML API                  │
│  ├─ analyze_xid_errors ─────────► eBPF: trace XID events ⚡ │
│  ├─ get_gpu_processes ──────────► eBPF: trace device opens │
│  └─ watch_gpu_events ───────────► eBPF: real-time stream   │
└─────────────────────────────────────────────────────────────┘
```

### Requirements for eBPF

| Requirement | Impact |
|-------------|--------|
| CAP_BPF + CAP_PERFMON | Additional capabilities needed |
| Kernel 5.8+ | For full CO-RE support |
| BTF enabled | For portable eBPF programs |
| libbpf | Additional dependency (~2MB) |

### Recommendation

**Defer to M4+** — Current NVML + dmesg parsing approach is sufficient for v1.
eBPF should be explored for:
- Real-time XID error streaming
- Advanced process-to-pod correlation
- Deep driver debugging tools

### Related Research

- [eACGM: eBPF-assisted GPU monitoring](https://arxiv.org/abs/2506.02007)
- [FOSDEM 2025: Auto-instrumentation for GPU using eBPF](https://fosdem.org/2025/)
- [bpftime: Userspace eBPF runtime](https://github.com/eunomia-bpf/bpftime)

---

## References

- [NVIDIA Container Toolkit](https://github.com/NVIDIA/nvidia-container-toolkit)
- [NVIDIA Device Plugin](https://github.com/NVIDIA/k8s-device-plugin)
- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator)
- [NVIDIA DRA Driver](https://github.com/NVIDIA/k8s-dra-driver)
- [dcgm-exporter](https://github.com/NVIDIA/dcgm-exporter)
- [Container Device Interface (CDI)](https://github.com/cncf-tags/container-device-interface)
- [Kubernetes Device Plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)

