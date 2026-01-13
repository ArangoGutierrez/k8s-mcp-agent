# Investigate Cross-Node Networking (Blocking HTTP Mode)

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/investigate-cross-node-networking.md`, the agent MUST continue
> working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke the prompt:** `@docs/prompts/investigate-cross-node-networking.md`

### Iteration Rules (For the Agent)

1. **NEVER STOP EARLY** - If any task is `[TODO]` or `[WIP]`, keep working
2. **UPDATE STATUS** - Edit this file: mark tasks `[WIP]` → `[DONE]` as you go
3. **COMMIT PROGRESS** - Commit and push after each completed task
4. **SELF-CHECK** - Before ending your turn, verify ALL tasks show `[DONE]`
5. **REPORT STATUS** - End each turn with a status summary of remaining tasks

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create investigation branch | `[DONE]` | fix/cross-node-networking |
| 1 | Diagnose cross-node connectivity | `[DONE]` | Root cause: Calico direct routing on AWS |
| 2 | Check AWS VPC CNI configuration | `[DONE]` | Cluster uses Calico, not AWS VPC CNI |
| 3 | Test pod-to-pod networking | `[DONE]` | Same-node works, cross-node fails |
| 4 | Document findings | `[DONE]` | docs/troubleshooting/cross-node-networking.md |
| 5 | Implement fix (if code change needed) | `[DONE]` | Added DNS routing + headless svc |
| 6 | Verify HTTP mode works across nodes | `[BLOCKED:infra]` | Needs Calico vxlanMode=Always |
| 7 | Create pull request (if code changes) | `[TODO]` | Ready for PR |
| 8 | Merge or document infrastructure fix | `[DONE]` | Documented 3 fix options |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** Not yet created - investigation task
- **Priority:** P1-High (blocks HTTP transport mode)
- **Labels:** kind/bug, area/k8s-ephemeral, ops/networking
- **Milestone:** M2: Hardware Introspection
- **Autonomous Mode:** ✅ Enabled

## Background

During E2E testing of the HTTP transport architecture (PR #125), we discovered that the gateway can only reach agent pods on the **same Kubernetes node**. Cross-node pod-to-pod HTTP communication times out.

### Observed Behavior

```
Gateway pod:    ip-10-0-0-153 (192.168.223.242)
Can reach:      Agent on ip-10-0-0-153 (192.168.223.241) ✅ 22ms
Cannot reach:   Agent on ip-10-0-0-81  (192.168.41.19)   ❌ timeout
Cannot reach:   Agent on ip-10-0-0-10  (192.168.180.212) ❌ timeout
Cannot reach:   Agent on ip-10-0-0-236 (192.168.30.145)  ❌ timeout
```

### Impact

- **HTTP routing mode** only works for single-node deployments
- **Exec routing mode** works perfectly (uses kubectl exec, not pod-to-pod HTTP)
- Multi-node GPU aggregation requires exec mode as a workaround

### Potential Causes

1. **AWS VPC CNI misconfiguration** - Pod networking not routing across nodes
2. **Security Groups** - EC2 security groups blocking pod CIDR traffic
3. **Network Policy** - Kubernetes NetworkPolicy blocking traffic (ruled out - none exist)
4. **Calico/Cilium** - If alternate CNI is in use, may need configuration
5. **Node firewall** - iptables rules on nodes blocking traffic

---

## Objective

Identify and resolve the root cause of cross-node pod-to-pod HTTP connectivity failure, enabling HTTP routing mode for multi-node GPU clusters.

---

## Step 0: Create Investigation Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b fix/cross-node-networking
```

---

## Investigation Tasks

### Task 1: Diagnose Cross-Node Connectivity `[TODO]`

Run comprehensive connectivity tests to pinpoint the failure.

**Commands to run:**

```bash
# 1. Check cluster networking setup
kubectl get nodes -o wide
kubectl cluster-info dump | grep -i "cluster-cidr\|pod-cidr\|service-cidr" | head -20

# 2. Check CNI plugin in use
kubectl get pods -n kube-system -l k8s-app=aws-node -o wide
kubectl get daemonset -n kube-system aws-node -o yaml | grep -A5 "image:"

# 3. Get pod IPs for testing
kubectl get pods -n gpu-diagnostics -o wide

# 4. Test connectivity from gateway pod to each agent
GATEWAY_POD=$(kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway -o jsonpath='{.items[0].metadata.name}')

# Test each agent's /healthz endpoint
for AGENT_IP in $(kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gpu-diagnostics -o jsonpath='{.items[*].status.podIP}'); do
    echo "Testing $AGENT_IP..."
    kubectl exec -n gpu-diagnostics $GATEWAY_POD -- /bin/busybox wget -q -O - --timeout=5 http://$AGENT_IP:8080/healthz 2>&1 || echo "FAILED"
done
```

**Expected output:** Document which agents are reachable and which fail.

---

### Task 2: Check AWS VPC CNI Configuration `[TODO]`

Investigate VPC CNI settings that could block cross-node traffic.

**Commands to run:**

```bash
# 1. Check aws-node DaemonSet configuration
kubectl get ds aws-node -n kube-system -o yaml | grep -A20 "env:"

# 2. Check ENI configuration on nodes
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}'

# 3. Check if SNAT is enabled (can cause issues)
kubectl logs -n kube-system -l k8s-app=aws-node --tail=50 | grep -i "snat\|nat\|masq"

# 4. Check VPC CNI version
kubectl describe ds aws-node -n kube-system | grep "Image:"

# 5. Verify pod CIDR allocation per node
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.podCIDR}{"\n"}{end}'
```

**Key settings to check:**
- `AWS_VPC_K8S_CNI_EXTERNALSNAT` - If true, may cause issues
- `ENABLE_POD_ENI` - Security group per pod can cause isolation
- Pod CIDR overlap or routing issues

---

### Task 3: Test Pod-to-Pod Networking `[TODO]`

Test raw network connectivity without application layer.

**Commands to run:**

```bash
# 1. Deploy a debug pod on the gateway's node
GATEWAY_NODE=$(kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway -o jsonpath='{.items[0].spec.nodeName}')
kubectl run nettest --rm -it --image=nicolaka/netshoot --overrides="{\"spec\":{\"nodeName\":\"$GATEWAY_NODE\"}}" -- bash

# Inside the pod, test:
# - ping to agent pod IPs
# - curl to agent pod IPs:8080
# - traceroute to agent pod IPs

# 2. Check if iptables rules block traffic
kubectl get nodes -o name | while read node; do
    echo "=== $node ==="
    kubectl debug node/${node#node/} -it --image=busybox -- cat /host/proc/net/ip_tables_names 2>/dev/null
done

# 3. Test TCP connectivity on port 8080
for AGENT_IP in $(kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gpu-diagnostics -o jsonpath='{.items[*].status.podIP}'); do
    echo "Testing TCP to $AGENT_IP:8080..."
    kubectl run tcptest-$(date +%s) --rm -it --restart=Never --image=busybox -- nc -zv -w5 $AGENT_IP 8080 2>&1 || echo "FAILED"
done
```

---

### Task 4: Check Security Groups (AWS Specific) `[TODO]`

If using AWS EKS, check EC2 security groups.

**Commands to run:**

```bash
# 1. Get node instance IDs
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.providerID}{"\n"}{end}'

# 2. Using AWS CLI, check security groups
# (Run these if aws CLI is configured)
aws ec2 describe-instances --instance-ids <INSTANCE_ID> --query 'Reservations[*].Instances[*].SecurityGroups' --output table

# 3. Check if security group allows pod CIDR traffic
# Look for rules allowing traffic from 192.168.0.0/16 or your pod CIDR
```

**Required security group rules for pod-to-pod:**
- Inbound: Allow all traffic from node security group (self-referencing)
- Inbound: Allow all traffic from pod CIDR range
- Outbound: Allow all traffic (or at least to pod CIDR)

---

### Task 5: Document Findings `[TODO]`

Create a findings document with:

1. **Root cause identified** (or list of potential causes)
2. **Evidence** (command outputs, logs)
3. **Impact assessment**
4. **Recommended fix**

**File to create:** `docs/troubleshooting/cross-node-networking.md`

---

### Task 6: Implement Fix `[TODO]`

Based on findings, implement the fix:

**If AWS VPC CNI issue:**
```bash
# Example: Disable external SNAT if causing issues
kubectl set env daemonset aws-node -n kube-system AWS_VPC_K8S_CNI_EXTERNALSNAT=false

# Or update VPC CNI configuration
kubectl edit configmap amazon-vpc-cni -n kube-system
```

**If Security Group issue:**
```bash
# Add rule to allow pod CIDR traffic
aws ec2 authorize-security-group-ingress \
    --group-id <SG_ID> \
    --protocol all \
    --source-group <SG_ID>
```

**If code change needed:**
- Implement fallback logic in gateway
- Add NetworkPolicy for explicit allow
- Document workaround in helm values

---

### Task 7: Verify HTTP Mode Works Across Nodes `[TODO]`

After implementing fix, verify HTTP routing works:

```bash
# 1. Deploy with HTTP mode
helm upgrade gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics \
  --set transport.mode=http \
  --set gateway.enabled=true \
  --set gateway.routingMode=http

# 2. Wait for pods
kubectl rollout status -n gpu-diagnostics daemonset/gpu-mcp-k8s-gpu-mcp-server

# 3. Test gateway HTTP routing to ALL nodes
kubectl port-forward -n gpu-diagnostics svc/gpu-mcp-k8s-gpu-mcp-server-gateway 8080:8080 &
sleep 5

curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}}}' | jq .

# 4. Check gateway logs - should show all nodes successful
GATEWAY_POD=$(kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway -o jsonpath='{.items[0].metadata.name}')
kubectl logs -n gpu-diagnostics $GATEWAY_POD --tail=20 | grep "routing complete"
# Expected: "success":4,"failed":0
```

**Acceptance criteria:**
- [ ] Gateway can reach ALL agent pods via HTTP
- [ ] `get_gpu_inventory` returns data from ALL nodes
- [ ] Gateway logs show 0 failed nodes
- [ ] Response time is <1s for 4-node cluster

---

### Task 8: Create PR or Document Infrastructure Fix `[TODO]`

**If code changes were made:**
```bash
gh pr create \
  --title "fix(networking): resolve cross-node pod connectivity" \
  --body "Fixes cross-node HTTP routing issue.

## Summary
[Description of the fix]

## Root Cause
[What was causing the issue]

## Testing
- [ ] HTTP routing works across all nodes
- [ ] Gateway aggregates data from all GPUs
- [ ] No regression in exec routing mode" \
  --label "kind/fix" \
  --label "area/k8s-ephemeral"
```

**If infrastructure-only fix:**
- Document the fix in `docs/troubleshooting/cross-node-networking.md`
- Update `docs/quickstart.md` with prerequisites
- Consider adding pre-flight check in Helm chart

---

## Related Files

- `pkg/gateway/router.go` - HTTP routing implementation
- `pkg/gateway/http_client.go` - HTTP client for agent communication
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml` - Agent deployment
- `deployment/helm/k8s-gpu-mcp-server/templates/gateway-deployment.yaml` - Gateway deployment

## Notes

### Workaround (Current)

Until this is fixed, use **exec routing mode** which works perfectly:

```bash
helm upgrade gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics \
  --set transport.mode=stdio \
  --set gateway.enabled=true \
  --set gateway.routingMode=exec
```

### Test Results from PR #125

| Routing Mode | Same Node | Cross Node | Status |
|--------------|-----------|------------|--------|
| HTTP         | ✅ 22ms   | ❌ timeout | Blocked |
| Exec         | ✅ 554ms  | ✅ 611ms   | Working |

---

## Quick Reference

### Key Commands

```bash
# Check pod connectivity
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- /bin/busybox wget -q -O - --timeout=5 http://$AGENT_IP:8080/healthz

# Check CNI logs
kubectl logs -n kube-system -l k8s-app=aws-node --tail=100

# Test with netshoot
kubectl run nettest --rm -it --image=nicolaka/netshoot -- bash

# Deploy HTTP mode
helm upgrade gpu-mcp deployment/helm/k8s-gpu-mcp-server --set transport.mode=http --set gateway.routingMode=http
```

---

**Reply "GO" when ready to start investigation.** 🔍
