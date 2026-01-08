# Real Cluster Integration Testing

## Issue Reference

- **Issue:** N/A (Testing checkpoint before M3 completion)
- **Priority:** P0-Blocker
- **Labels:** ops/testing, area/k8s-ephemeral, area/nvml-binding
- **Milestone:** M3: The Ephemeral Tunnel

## Background

After merging the Gateway mode feature (#72), we need to perform comprehensive
integration testing on a real Kubernetes cluster with actual GPU hardware.
This testing validates:

1. **DaemonSet deployment** - Agents run on all GPU nodes
2. **HTTP transport** - MCP server accessible via HTTP endpoint
3. **Gateway mode** - Single entry point routes to node agents
4. **NVML real mode** - Actual GPU telemetry from NVML
5. **All MCP tools** - `get_gpu_inventory`, `get_gpu_health`, `analyze_xid_errors`

---

## Objective

Validate all k8s-gpu-mcp-server features on a real Kubernetes cluster with
NVIDIA GPUs before proceeding with M3 completion.

---

## Prerequisites

### Cluster Requirements

- Kubernetes 1.26+ cluster with GPU nodes
- NVIDIA GPU Operator installed (or nvidia-container-toolkit configured)
- `nvidia` RuntimeClass available
- `kubectl` access to the cluster
- `helm` 3.x installed

### Verify Cluster Access

```bash
# Check cluster connection
kubectl cluster-info

# Verify GPU nodes exist
kubectl get nodes -l nvidia.com/gpu.present=true

# Check RuntimeClass
kubectl get runtimeclass nvidia

# Verify GPU Operator pods (if using)
kubectl get pods -n gpu-operator
```

---

## Step 0: Prepare Test Environment

### Clone and Build

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Ensure on latest main
git checkout main
git pull origin main

# Build the agent
make agent

# Verify version
./bin/agent --version
```

### Build Container Image

```bash
# Build container image for your registry
# Replace with your registry
REGISTRY=ghcr.io/arangogutierrez

# Build using Containerfile
podman build -t ${REGISTRY}/k8s-gpu-mcp-server:test \
  -f deployment/Containerfile .

# Push to registry
podman push ${REGISTRY}/k8s-gpu-mcp-server:test
```

---

## Test 1: DaemonSet Deployment (stdio mode)

### Deploy DaemonSet

```bash
# Install Helm chart with default stdio mode
helm upgrade --install gpu-agent ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --create-namespace \
  --set image.tag=test \
  --set gpu.runtimeClass.enabled=true \
  --set gpu.runtimeClass.name=nvidia
```

### Verify Deployment

```bash
# Check pods are running on GPU nodes
kubectl get pods -n gpu-diagnostics -o wide

# Expected: One pod per GPU node
# NAME                          READY   STATUS    RESTARTS   AGE   NODE
# gpu-agent-xxxxx               1/1     Running   0          1m    gpu-node-1
# gpu-agent-yyyyy               1/1     Running   0          1m    gpu-node-2
```

### Test stdio Mode via kubectl exec

```bash
# Pick any GPU node pod
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  -o jsonpath='{.items[0].metadata.name}')

echo "Testing pod: $POD"

# Test 1.1: GPU Inventory (REAL NVML)
kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
  'echo '"'"'{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real'
```

**Expected output:** Real GPU model names, UUIDs, temperatures, power usage

```bash
# Test 1.3: GPU Health
kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
  'echo '"'"'{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_health","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real'
```

**Expected output:** Health status, scores, temperature/memory/power checks

```bash
# Test 1.4: Analyze XID Errors
kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
  'echo '"'"'{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze_xid_errors","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real'
```

**Expected output:** XID error analysis (likely empty if no errors)

### Acceptance Criteria - Test 1

- [ ] Pods running on all GPU nodes (DaemonSet)
- [ ] `get_gpu_inventory` shows REAL GPU data (not mock)
- [ ] `get_gpu_health` returns health status
- [ ] `analyze_xid_errors` completes without error

---

## Test 2: HTTP Transport Mode

### Deploy with HTTP Transport

```bash
# Upgrade to enable HTTP transport
helm upgrade gpu-agent ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --set image.tag=test \
  --set transport.mode=http \
  --set transport.http.port=8080 \
  --set service.enabled=true \
  --set service.port=8080
```

### Verify HTTP Endpoint

```bash
# Wait for pods to restart
kubectl rollout status daemonset/gpu-agent-k8s-gpu-mcp-server -n gpu-diagnostics

# Check service
kubectl get svc -n gpu-diagnostics

# Port forward to test locally
kubectl port-forward -n gpu-diagnostics svc/k8s-gpu-mcp-server 8080:8080 &
PF_PID=$!

# Give it a moment
sleep 2
```

### Test HTTP Endpoints

```bash
# Test 2.1: Health check
curl -s http://localhost:8080/healthz
# Expected: {"status":"ok"}

# Test 2.2: Ready check
curl -s http://localhost:8080/readyz
# Expected: {"status":"ready"}

# Test 2.3: Version
curl -s http://localhost:8080/version | jq .
# Expected: {"version":"...","git_commit":"..."}

# Test 2.4: MCP call via HTTP (NOT SSE - direct JSON-RPC)
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "get_gpu_inventory",
      "arguments": {}
    },
    "id": 1
  }' | jq .
```

### Cleanup Port Forward

```bash
kill $PF_PID 2>/dev/null
```

### Acceptance Criteria - Test 2

- [ ] `/healthz` returns 200 OK
- [ ] `/readyz` returns 200 OK
- [ ] `/version` returns version info
- [ ] MCP calls work via HTTP POST

---

## Test 3: Gateway Mode

### Deploy Gateway

```bash
# Enable gateway alongside DaemonSet
helm upgrade gpu-agent ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics \
  --set image.tag=test \
  --set gateway.enabled=true \
  --set gateway.port=8080 \
  --set transport.mode=stdio
```

### Verify Gateway Deployment

```bash
# Check gateway deployment
kubectl get deployment -n gpu-diagnostics | grep gateway

# Check gateway service
kubectl get svc -n gpu-diagnostics | grep gateway

# Check gateway pods
kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway

# Check RBAC
kubectl get role,rolebinding -n gpu-diagnostics | grep gateway
```

### Test Gateway Endpoints

```bash
# Port forward to gateway
kubectl port-forward -n gpu-diagnostics \
  svc/gpu-agent-k8s-gpu-mcp-server-gateway 8080:8080 &
GW_PID=$!
sleep 2

# Test 3.1: Gateway health
curl -s http://localhost:8080/healthz
# Expected: {"status":"ok"}

# Test 3.2: List GPU nodes
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "list_gpu_nodes",
      "arguments": {}
    },
    "id": 1
  }' | jq .
```

**Expected output:**
```json
{
  "status": "success",
  "node_count": 2,
  "ready_count": 2,
  "nodes": [
    {
      "name": "gpu-node-1",
      "pod_name": "gpu-agent-xxxxx",
      "pod_ip": "10.244.1.5",
      "ready": true
    },
    {
      "name": "gpu-node-2",
      "pod_name": "gpu-agent-yyyyy",
      "pod_ip": "10.244.2.3",
      "ready": true
    }
  ]
}
```

### Cleanup Port Forward

```bash
kill $GW_PID 2>/dev/null
```

### Acceptance Criteria - Test 3

- [ ] Gateway deployment running
- [ ] Gateway service accessible
- [ ] `list_gpu_nodes` returns all GPU nodes
- [ ] All nodes show `ready: true`

---

## Test 4: Multi-Node Validation

### Test Each GPU Node

```bash
# Get list of GPU agent pods
kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.nodeName}{"\n"}{end}'

# For each pod, test GPU inventory
for POD in $(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  -o jsonpath='{.items[*].metadata.name}'); do
  
  echo "=== Testing $POD ==="
  
  NODE=$(kubectl get pod -n gpu-diagnostics $POD -o jsonpath='{.spec.nodeName}')
  echo "Node: $NODE"
  
  # Get GPU count on this node
  kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
    'echo '"'"'{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real' \
    2>/dev/null | grep -o '"device_count":[0-9]*'
  
  echo ""
done
```

### Acceptance Criteria - Test 4

- [ ] Each GPU node returns valid inventory
- [ ] GPU counts match expected hardware
- [ ] No pods in CrashLoopBackOff

---

## Test 5: Error Handling

### Test Invalid Inputs

```bash
POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  -o jsonpath='{.items[0].metadata.name}')

# Test 5.1: Invalid tool name
kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
  'echo '"'"'{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"nonexistent_tool","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real'

# Expected: Error response (not crash)

# Test 5.2: Missing initialize
kubectl exec -n gpu-diagnostics $POD -- /bin/sh -c \
  'echo '"'"'{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}'"'"' | /agent --nvml-mode=real'

# Expected: Error or tool result (implementation dependent)
```

### Acceptance Criteria - Test 5

- [ ] Invalid tool name returns error (not crash)
- [ ] Agent handles malformed input gracefully

---

## Test 6: Resource Usage

### Check Resource Consumption

```bash
# Check actual resource usage
kubectl top pods -n gpu-diagnostics

# Check for OOM kills
kubectl get events -n gpu-diagnostics --sort-by='.lastTimestamp' | grep -i oom

# Check container logs for errors
for POD in $(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/name=k8s-gpu-mcp-server \
  -o jsonpath='{.items[*].metadata.name}'); do
  echo "=== Logs: $POD ==="
  kubectl logs -n gpu-diagnostics $POD --tail=20
  echo ""
done
```

### Acceptance Criteria - Test 6

- [ ] CPU usage within limits
- [ ] Memory usage within limits
- [ ] No OOMKilled events
- [ ] No error logs

---

## Cleanup

```bash
# Remove test deployment
helm uninstall gpu-agent -n gpu-diagnostics

# Delete namespace
kubectl delete namespace gpu-diagnostics

# Verify cleanup
kubectl get all -n gpu-diagnostics
```

---

## Test Results Summary

Fill this out after running tests:

| Test | Description | Status | Notes |
|------|-------------|--------|-------|
| 1.1 | get_gpu_inventory (real NVML) | ⬜ | |
| 1.2 | get_gpu_health | ⬜ | |
| 1.3 | analyze_xid_errors | ⬜ | |
| 2.1 | /healthz HTTP | ⬜ | |
| 2.2 | /readyz HTTP | ⬜ | |
| 2.3 | /version HTTP | ⬜ | |
| 2.4 | MCP via HTTP POST | ⬜ | |
| 3.1 | Gateway health | ⬜ | |
| 3.2 | list_gpu_nodes | ⬜ | |
| 4.0 | Multi-node validation | ⬜ | |
| 5.1 | Invalid tool handling | ⬜ | |
| 5.2 | Missing initialize | ⬜ | |
| 6.0 | Resource usage | ⬜ | |

### Legend
- ✅ Pass
- ❌ Fail
- ⬜ Not tested

---

## Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod -n gpu-diagnostics <pod-name>

# Common issues:
# - RuntimeClass not found → Check nvidia RuntimeClass exists
# - ImagePullBackOff → Check image tag and registry access
# - CrashLoopBackOff → Check container logs
```

### NVML Initialization Failed

```bash
# Check if GPU is visible in container
kubectl exec -n gpu-diagnostics <pod> -- nvidia-smi

# If nvidia-smi works but agent fails:
# - Check /dev/nvidia* devices are mounted
# - Check NVIDIA_VISIBLE_DEVICES env var
```

### Gateway Cannot Reach Node Agents

```bash
# Check RBAC permissions
kubectl auth can-i create pods/exec \
  --as=system:serviceaccount:gpu-diagnostics:gpu-agent-k8s-gpu-mcp-server-gateway \
  -n gpu-diagnostics

# Should return: yes
```

### HTTP Endpoints Not Responding

```bash
# Check pod is using HTTP mode
kubectl exec -n gpu-diagnostics <pod> -- ps aux | grep agent

# Should show: /agent --port=8080 ...

# Check container port
kubectl get pod -n gpu-diagnostics <pod> -o jsonpath='{.spec.containers[0].ports}'
```

---

## Next Steps After Testing

1. **All tests pass:** Proceed with M3 completion
2. **Some tests fail:** Create issues for failures, fix before proceeding
3. **Critical failures:** Rollback if necessary

---

**Run tests and record results before proceeding with M3 milestone.**

