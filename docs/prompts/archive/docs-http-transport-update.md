# Update Documentation for HTTP Transport Architecture

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/docs-http-transport-update.md`, the agent MUST
> continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke:** `@docs/prompts/docs-http-transport-update.md`

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[TODO]` | `docs/http-transport-update` |
| 1 | Verify real cluster is accessible | `[TODO]` | KUBECONFIG must be set |
| 2 | Test HTTP transport in real cluster | `[TODO]` | Verify gateway→agent HTTP works |
| 3 | Test observability features | `[TODO]` | Verify /metrics, circuit breaker |
| 4 | Update `.cursor/rules/01-mcp-server.mdc` | `[TODO]` | Remove "NO HTTP" claims |
| 5 | Update `.cursor/rules/03-k8s-constraints.mdc` | `[TODO]` | Update deployment patterns |
| 6 | Update `docs/architecture.md` | `[TODO]` | HTTP transport architecture |
| 7 | Update `docs/quickstart.md` | `[TODO]` | Deployment instructions |
| 8 | Update `SCRATCHPAD.md` | `[TODO]` | Refactor completion status |
| 9 | Update `README.md` | `[TODO]` | Architecture diagram if present |
| 10 | Run final verification | `[TODO]` | All docs consistent |
| 11 | Create pull request | `[TODO]` | |
| 12 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 13 | Address review comments | `[TODO]` | |
| 14 | Merge after reviews | `[TODO]` | |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** [#117 - docs: Update architecture documentation for HTTP transport](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/117)
- **Priority:** P2-Medium
- **Labels:** kind/docs, prio/p2-medium
- **Parent Epic:** #112 - HTTP transport refactor (Phase 5)
- **Autonomous Mode:** ✅ Enabled

## Background

The HTTP transport refactor (Epic #112) is complete:
- ✅ Phase 1: Timeout alignment (#113, PR #119)
- ✅ Phase 2: Agent HTTP mode (#114, PR #121)
- ✅ Phase 3: Gateway HTTP routing (#115, PR #122)
- ✅ Phase 4: Resilience & observability (#116, PR #123)

However, documentation and workspace rules have drifted from reality:

| Rule File | Current Claim | Reality |
|-----------|---------------|---------|
| `01-mcp-server.mdc` | "NO HTTP/WebSocket listeners" | HTTP transport implemented and used |
| `03-k8s-constraints.mdc` | "NO DaemonSets - ephemeral only" | DaemonSet is primary deployment |
| `03-k8s-constraints.mdc` | "No probes needed" | HTTP mode uses liveness/readiness |

---

## Objective

Update all documentation and workspace rules to accurately reflect the HTTP transport architecture, **after verifying the implementation works in a real cluster**.

---

<testing_first>
## ⚠️ TESTING-FIRST: Verify Before Documenting

**CRITICAL:** This is a documentation task, but we MUST verify the current implementation
works correctly in a real cluster BEFORE updating any documentation.

**Why?**
- Documentation should reflect actual behavior, not aspirational behavior
- Real cluster testing catches issues that mock tests miss
- We have KUBECONFIG ready - use it!

**Order of operations:**
1. First: Test in real cluster (Tasks 1-3)
2. Then: Update documentation (Tasks 4-9)
3. Finally: Verify consistency (Task 10)
</testing_first>

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b docs/http-transport-update
```

---

## Task 1: Verify Real Cluster Access `[TODO]`

> **⚠️ DO NOT SKIP** - Verify cluster access before any documentation work.

```bash
# Check KUBECONFIG is set
echo $KUBECONFIG

# Verify cluster connectivity
kubectl cluster-info
kubectl get nodes

# Check GPU diagnostics namespace
kubectl get ns gpu-diagnostics

# List GPU agent pods
kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/name=k8s-gpu-mcp-server

# List gateway pods
kubectl get pods -n gpu-diagnostics -l app.kubernetes.io/component=gateway
```

**Acceptance criteria:**
- [ ] KUBECONFIG is set and valid
- [ ] Cluster is accessible
- [ ] GPU agent pods are running
- [ ] Gateway pod is running (if deployed)

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 2: Test HTTP Transport in Real Cluster `[TODO]`

Verify the HTTP transport implementation works correctly:

### 2.1 Test Agent HTTP Mode

```bash
# Get an agent pod name
AGENT_POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/component=gpu-diagnostics \
  -o jsonpath='{.items[0].metadata.name}')

# Test agent healthz endpoint
kubectl exec -n gpu-diagnostics $AGENT_POD -- \
  curl -s http://localhost:8080/healthz

# Test agent readyz endpoint
kubectl exec -n gpu-diagnostics $AGENT_POD -- \
  curl -s http://localhost:8080/readyz

# Test MCP tools/list via agent HTTP
kubectl exec -n gpu-diagnostics $AGENT_POD -- \
  curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

### 2.2 Test Gateway HTTP Routing

```bash
# Get gateway pod name
GATEWAY_POD=$(kubectl get pods -n gpu-diagnostics \
  -l app.kubernetes.io/component=gateway \
  -o jsonpath='{.items[0].metadata.name}')

# Test gateway healthz
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s http://localhost:8080/healthz

# Test gateway MCP tools/list (should route to agents)
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Test get_gpu_inventory via gateway
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_gpu_inventory"}}'
```

### 2.3 Test Pod-to-Pod HTTP Communication

```bash
# Get agent pod IP
AGENT_IP=$(kubectl get pod -n gpu-diagnostics $AGENT_POD \
  -o jsonpath='{.status.podIP}')

# From gateway, test direct HTTP to agent
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s http://$AGENT_IP:8080/healthz
```

**Acceptance criteria:**
- [ ] Agent HTTP endpoints respond correctly
- [ ] Gateway routes requests to agents via HTTP
- [ ] Pod-to-pod HTTP communication works
- [ ] MCP tools return valid responses

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 3: Test Observability Features `[TODO]`

Verify the resilience and observability features from Phase 4:

### 3.1 Test Prometheus Metrics

```bash
# Test gateway /metrics endpoint
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s http://localhost:8080/metrics | head -50

# Check for expected metrics
kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
  curl -s http://localhost:8080/metrics | grep -E "mcp_requests_total|mcp_request_duration|mcp_node_health|mcp_circuit_breaker"
```

### 3.2 Test Circuit Breaker (if possible)

```bash
# Make multiple requests to see circuit breaker behavior
for i in {1..5}; do
  kubectl exec -n gpu-diagnostics $GATEWAY_POD -- \
    curl -s -X POST http://localhost:8080/mcp \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":'$i',"method":"tools/call","params":{"name":"get_gpu_health"}}'
  echo ""
done
```

### 3.3 Test NetworkPolicy (if enabled)

```bash
# Check if NetworkPolicy is deployed
kubectl get networkpolicy -n gpu-diagnostics

# Describe NetworkPolicy for agent
kubectl describe networkpolicy -n gpu-diagnostics gpu-mcp-agent 2>/dev/null || echo "NetworkPolicy not deployed"
```

**Acceptance criteria:**
- [ ] /metrics endpoint returns Prometheus metrics
- [ ] Expected metric names are present
- [ ] Circuit breaker metrics visible (if requests made)

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 4: Update `.cursor/rules/01-mcp-server.mdc` `[TODO]`

Read the current file and update to reflect HTTP transport reality:

**Changes needed:**
- Remove any "NO HTTP/WebSocket listeners" claims
- Add HTTP transport section
- Document transport options (http, stdio)
- Update MCP protocol section

**New content to add:**

```markdown
## Transport Options

The agent supports multiple transports:

### HTTP/SSE (Production Default)
- Long-running HTTP server on port 8080
- Used by gateway for pod-to-pod communication
- Supports MCP Streamable HTTP transport
- Endpoints: `/mcp`, `/healthz`, `/readyz`, `/metrics`

### Stdio (Debug/Direct Access)
- For kubectl exec access
- Oneshot mode for single tool calls
- Useful for SRE debugging
```

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 5: Update `.cursor/rules/03-k8s-constraints.mdc` `[TODO]`

Read the current file and update deployment patterns:

**Changes needed:**
- Remove "NO DaemonSets - ephemeral only" claims
- Document DaemonSet as primary deployment pattern
- Add HTTP mode probe configuration
- Update security context requirements

**New content to add:**

```markdown
## Deployment Patterns

### DaemonSet (Primary)
- One agent pod per GPU node
- HTTP transport mode (default)
- Liveness/readiness probes enabled
- Persistent connection for low latency

### Gateway Deployment
- Single replica deployment
- Routes requests to agent DaemonSet
- HTTP-based pod-to-pod communication
- Circuit breaker for resilience
```

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 6: Update `docs/architecture.md` `[TODO]`

Update the architecture documentation with HTTP transport model:

**Key sections to update:**
- System architecture diagram
- Gateway → Agent communication
- Transport options
- Observability (metrics, tracing)

**New architecture diagram:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        System Architecture                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Cursor/Claude                                                       │
│       │                                                              │
│       │ MCP over stdio                                               │
│       ▼                                                              │
│  ┌─────────────────┐                                                 │
│  │  NPM Bridge     │ (kubectl port-forward)                          │
│  │  (local)        │                                                 │
│  └────────┬────────┘                                                 │
│           │ HTTP                                                     │
│           ▼                                                          │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Kubernetes Cluster                        │    │
│  │  ┌─────────────────┐         ┌─────────────────────────┐    │    │
│  │  │  Gateway Pod    │  HTTP   │  Agent DaemonSet        │    │    │
│  │  │  :8080          │────────▶│  :8080 (per GPU node)   │    │    │
│  │  │  - Router       │         │  - NVML Client          │    │    │
│  │  │  - CircuitBreaker│        │  - GPU Tools            │    │    │
│  │  │  - Metrics      │         │  - Health Endpoints     │    │    │
│  │  └─────────────────┘         └─────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 7: Update `docs/quickstart.md` `[TODO]`

Update deployment instructions for HTTP transport:

**Changes needed:**
- Update Helm install commands
- Add HTTP mode verification steps
- Update troubleshooting section

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 8: Update `SCRATCHPAD.md` `[TODO]`

Update with HTTP transport refactor completion status:

**Add section:**

```markdown
## HTTP Transport Refactor (Epic #112) - COMPLETE ✅

| Phase | Issue | PR | Status |
|-------|-------|-----|--------|
| Phase 1: Timeout Fix | #113 | #119 | ✅ Merged |
| Phase 2: Agent HTTP Mode | #114 | #121 | ✅ Merged |
| Phase 3: Gateway HTTP Routing | #115 | #122 | ✅ Merged |
| Phase 4: Resilience & Observability | #116 | #123 | ✅ Merged |
| Phase 5: Documentation | #117 | #XXX | 🔄 In Progress |

### Key Changes
- Default transport: HTTP (was stdio)
- Gateway routing: HTTP (was kubectl exec)
- Added: Circuit breaker, Prometheus metrics, NetworkPolicy
```

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 9: Update `README.md` `[TODO]`

Check README.md for any architecture diagrams or deployment instructions that need updating.

**Review and update:**
- Architecture diagram (if present)
- Quick start instructions
- Feature list

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Task 10: Run Final Verification `[TODO]`

Verify all documentation is consistent:

```bash
# Search for outdated claims
grep -r "NO HTTP" .cursor/rules/ docs/
grep -r "ephemeral only" .cursor/rules/ docs/
grep -r "NO DaemonSet" .cursor/rules/ docs/

# Verify no broken links in docs
# (manual review)

# Run any doc linters if available
```

**Acceptance criteria:**
- [ ] No outdated "NO HTTP" claims remain
- [ ] No outdated "ephemeral only" claims remain
- [ ] All docs reference HTTP transport correctly
- [ ] Architecture diagrams are consistent

> 💡 **After completing:** Update Progress Tracker → `[DONE]` → Commit

---

## Create Pull Request

```bash
gh pr create \
  --title "docs: update architecture documentation for HTTP transport" \
  --body "Fixes #117

## Summary

Updates all documentation and workspace rules to reflect the HTTP transport architecture
implemented in Epic #112 (Phases 1-4).

## Changes

### Workspace Rules
- \`.cursor/rules/01-mcp-server.mdc\` - Add HTTP transport section
- \`.cursor/rules/03-k8s-constraints.mdc\` - Update deployment patterns

### Documentation
- \`docs/architecture.md\` - HTTP transport architecture
- \`docs/quickstart.md\` - Updated deployment instructions
- \`SCRATCHPAD.md\` - Refactor completion status
- \`README.md\` - Architecture diagram updates

## Testing

- [x] Verified HTTP transport in real cluster
- [x] Verified observability features (/metrics)
- [x] No outdated claims remain in docs

## Related

- Parent epic: #112
- Completes Phase 5 of HTTP transport refactor" \
  --label "kind/docs" \
  --label "prio/p2-medium"
```

---

## Completion Protocol

When all tasks show `[DONE]`:

```markdown
## 🎉 ALL TASKS COMPLETE

Phase 5 of HTTP transport refactor (#117) is complete.

**Summary:**
- Branch: `docs/http-transport-update`
- PR: #XXX (merged)
- Tests: ✅ Real cluster verification passed

**HTTP Transport Refactor Epic #112:** COMPLETE ✅

**Recommend:** Move this prompt to `archive/`
```
