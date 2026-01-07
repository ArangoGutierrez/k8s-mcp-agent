# Deploy k8s-gpu-mcp-server to Kubernetes Cluster

## Project Context

I'm working on `k8s-gpu-mcp-server` - an ephemeral GPU diagnostic agent that uses MCP
(Model Context Protocol) over stdio.

### Recently Completed
- PR #61 merged: Multi-stage Dockerfile (`deployment/Containerfile`)
- PR #61 merged: CI workflow for container builds (`.github/workflows/image.yml`)
- Container image builds successfully locally (tested with Docker)

### Current State
- Image is NOT yet in ghcr.io (CI builds on PR but only pushes on main/tags)
- Cluster: `holodeck-cluster` via kubectl (KUBECONFIG in env)
- Cluster has NO GPUs - will test with `--nvml-mode=mock`

## Objective

Test the k8s-gpu-mcp-server deployment pipeline on a real Kubernetes cluster:

1. Push container image to ghcr.io
2. Deploy via `kubectl debug` (ephemeral injection pattern)
3. Validate MCP protocol works through SPDY tunnel
4. Document the process

## Prerequisites

- [x] kubectl configured (context: `kubernetes-admin@holodeck-cluster`)
- [x] Dockerfile ready (`deployment/Containerfile`)
- [x] CI workflow ready (`.github/workflows/image.yml`)
- [ ] Container image in ghcr.io

## Step-by-Step Tasks

### Step 1: Push Container Image to ghcr.io

Option A: Create a release tag to trigger CI push

```bash
git tag v0.1.0-alpha
git push origin v0.1.0-alpha
```

Option B: Push manually using docker + ghcr.io login

```bash
# Login to ghcr.io
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Build and push
docker build -f deployment/Containerfile -t ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest .
docker push ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

### Step 2: Verify Image is Pullable

```bash
# Verify image exists
docker pull ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest

# Check image size
docker images ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest
```

### Step 3: Test kubectl debug Injection

```bash
# Inject agent into node (mock GPU mode since no real GPUs)
kubectl debug node/ip-10-0-0-194 \
  --image=ghcr.io/arangogutierrez/k8s-gpu-mcp-server:latest \
  -it --tty \
  -- /agent --nvml-mode=mock --mode=read-only
```

### Step 4: Test MCP Protocol

Once attached, send MCP JSON-RPC commands via stdin:

```json
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
```

Then test GPU inventory (will return mock data):

```json
{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_gpu_inventory","arguments":{}},"id":1}
```

### Step 5: Document Results

- Did the image pull successfully?
- Did kubectl debug injection work?
- Did MCP protocol respond correctly?
- What's the image size on amd64?

## Cluster Info

```
Node: ip-10-0-0-194
Kubernetes: v1.33.3
Runtime: docker://29.1.3
OS: Ubuntu 22.04.5 LTS
Resources: 4 CPU, 16GB RAM, No GPUs
```

## Files to Reference

- `deployment/Containerfile` - Production Dockerfile
- `.github/workflows/image.yml` - CI workflow
- `examples/initialize.json` - MCP init request
- `examples/gpu_inventory.json` - GPU inventory request

## Success Criteria

1. Image pushed to ghcr.io and pullable
2. `kubectl debug` successfully injects agent
3. Agent responds to MCP `initialize` request
4. Agent responds to `get_gpu_inventory` (mock data)
5. Clean exit on disconnect

## Troubleshooting

### Image pull fails

```bash
# Check if image is public or needs auth
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=USERNAME \
  --docker-password=$GITHUB_TOKEN
```

### kubectl debug hangs

```bash
# Check events
kubectl get events --sort-by='.lastTimestamp' | tail -20

# Check if image is being pulled
kubectl get pods -A | grep debug
```

### MCP protocol not responding

```bash
# Check agent logs (stderr)
# The agent should output JSON logs to stderr
```

## Related Issues

- Issue #33: Multi-stage Dockerfile (completed)
- Issue #36: CI workflow for images (completed)
- Issue #34: E2E kubectl debug integration tests (open)
