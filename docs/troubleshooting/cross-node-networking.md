# Cross-Node Pod Networking Troubleshooting

## Issue Summary

Cross-node HTTP communication between gateway and agent pods times out while 
same-node communication works correctly.

**Observed Behavior:**
```
Gateway on ip-10-0-0-153:
  ✅ Same-node agent (192.168.223.x) - responds in ~20ms
  ❌ Cross-node agents (192.168.x.x)  - timeout after 3-5s
```

## Root Cause

**Infrastructure-level networking issue** - not a code bug.

### Technical Details

The cluster uses **Calico CNI** with the following configuration:

```yaml
# IPPool configuration
spec:
  cidr: 192.168.0.0/16
  ipipMode: Never        # No IPIP tunneling
  vxlanMode: CrossSubnet # VXLAN only across different subnets
```

Since all nodes are in the **same AWS subnet** (10.0.0.x/24), Calico uses 
**direct routing** instead of VXLAN encapsulation. Direct routing on AWS 
requires additional configuration that is missing.

### Why Same-Node Works

On the same node, pod traffic goes through the local veth interfaces and 
Linux bridge - no cross-node routing needed.

### Why Cross-Node Fails

Cross-node pod traffic requires packets with pod source/destination IPs 
(192.168.x.x) to traverse the VPC network. AWS VPC by default:

1. **Drops packets** where source/dest IP doesn't match the ENI's assigned IPs
2. **Has no routes** for the pod CIDR (192.168.0.0/16)

## Solutions

### Option A: Enable VXLAN Always (Recommended)

Change Calico to **always use VXLAN** encapsulation, regardless of subnet:

```bash
# Edit the IPPool
kubectl edit ippool default-ipv4-ippool

# Change:
#   vxlanMode: CrossSubnet
# To:
#   vxlanMode: Always
```

This encapsulates pod traffic in VXLAN packets with node IPs as outer 
headers, which VPC routing handles correctly.

**Pros:** No AWS infrastructure changes needed
**Cons:** ~5-10% performance overhead due to encapsulation

### Option B: Disable EC2 Source/Destination Check

For each EC2 instance in the cluster:

```bash
# Using AWS CLI
for INSTANCE_ID in $(kubectl get nodes -o jsonpath='{.items[*].spec.providerID}' | tr ' ' '\n' | sed 's|aws:///[^/]*/||'); do
  aws ec2 modify-instance-attribute --instance-id $INSTANCE_ID --no-source-dest-check
done
```

**Pros:** No encapsulation overhead
**Cons:** Requires AWS permissions, must be done for each new node

### Option C: Add VPC Route Table Entries

Add routes for each node's pod CIDR to the VPC route table:

```bash
# For each node, add route:
# 192.168.X.0/24 -> eni-xxxxx (node's ENI)

aws ec2 create-route \
  --route-table-id rtb-xxxxx \
  --destination-cidr-block 192.168.180.0/24 \
  --network-interface-id eni-xxxxx
```

**Pros:** Direct routing, best performance
**Cons:** Route table limit (50 routes), manual management

## Verification

After applying a fix, verify connectivity:

```bash
# Get gateway pod
GATEWAY_POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/component=gateway \
  -o jsonpath='{.items[0].metadata.name}')

# Test each agent
for AGENT_IP in $(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/component=gpu-diagnostics \
  -o jsonpath='{.items[*].status.podIP}'); do
  kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
    /bin/busybox wget -q -O - --timeout=3 http://$AGENT_IP:8080/healthz
done
```

All agents should return `{"status":"healthy"}`.

## Workaround

Until the infrastructure is fixed, use **exec routing mode** which uses 
`kubectl exec` instead of pod-to-pod HTTP:

```bash
helm upgrade gpu-mcp deployment/helm/k8s-gpu-mcp-server \
  -n gpu-diagnostics \
  --set transport.mode=stdio \
  --set gateway.enabled=true \
  --set gateway.routingMode=exec
```

This works because `kubectl exec` goes through the API server, not direct 
pod networking.

## Related

- [Calico VXLAN documentation](https://docs.tigera.io/calico/latest/networking/vxlan-ipip)
- [AWS VPC CNI vs Calico on AWS](https://docs.tigera.io/calico/latest/getting-started/kubernetes/managed-public-cloud/eks)
- [EC2 Source/Destination Check](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#eni-attribute-src-dest-check)
