# Project Scratchpad - k8s-gpu-mcp-server

> Last updated: 2026-01-15

## 🔍 Project 360 Review Summary (2026-01-15)

**Overall Assessment:** Ready for v0.1.0 with minor improvements

| Category | Score | Notes |
|----------|-------|-------|
| Code Quality | ⭐⭐⭐⭐ | Clean, idiomatic Go; follows Effective Go |
| Test Coverage | ⭐⭐⭐ | 58-80% per package; gaps in metrics/nvml |
| Security | ⭐⭐⭐⭐ | Read-only default; proper validation |
| Documentation | ⭐⭐⭐⭐ | Comprehensive; minor staleness |
| Production Ready | ⭐⭐⭐⭐ | HTTP transport, circuit breaker, metrics |
| Community Value | ⭐⭐⭐⭐⭐ | Unique AI-native GPU diagnostics |

**Key Findings:**
- ✅ All tests pass with race detector
- ✅ No panics in production code
- ✅ All TODOs linked to GitHub issues (none P0/P1)
- ⚠️ 106 `log.Printf` calls → migrate to structured logging (M4)
- ⚠️ README milestone status outdated
- 📊 Test coverage: tools 80%, gateway 73%, mcp 66%, k8s 58%

**Full report:** `docs/reports/project-360-review-2026-01-15.md`

---

## 📊 Project Status Overview

### Recently Completed (Last 7 Days)

| PR | Description | Impact |
|----|-------------|--------|
| #127 | Gateway per-node latency metrics | Production observability |
| #126 | Cross-node HTTP routing fix (Calico/VXLAN) | 100% E2E success |
| #125 | Architecture docs for HTTP transport | Developer docs |
| #123 | Gateway resilience & observability | Circuit breaker, metrics |
| #122 | Gateway HTTP routing (not exec) | 150× latency improvement |
| #121 | Agent HTTP transport default | Persistent servers |
| #119 | Timeout alignment fix | Race condition fix |
| #109 | `/dev/kmsg` XID parsing for distroless | Enables XID analysis |
| #108 | Consolidated `list_gpu_nodes` into `get_gpu_inventory` | Simpler API |

### HTTP Transport Refactor (Epic #112) - COMPLETE ✅

**Status:** Epic closed on 2026-01-14

| Phase | Issue | PR | Status |
|-------|-------|-----|--------|
| Phase 1: Timeout Fix | #113 | #119 | ✅ Merged |
| Phase 2: Agent HTTP Mode | #114 | #121 | ✅ Merged |
| Phase 3: Gateway HTTP Routing | #115 | #122 | ✅ Merged |
| Phase 4: Resilience & Observability | #116 | #123 | ✅ Merged |
| Phase 5: Documentation | #117 | #125 | ✅ Merged |
| **Bonus:** Cross-Node Networking Fix | - | #126 | ✅ Merged |
| **Bonus:** Per-Node Latency Metrics | - | #127 | ✅ Merged |

**Key Achievements:**
- Default transport: HTTP (agents as persistent servers)
- Gateway routing: HTTP (not exec)
- Added: Circuit breaker, Prometheus metrics, NetworkPolicy support
- Memory footprint: ~15-20MB constant vs 200MB spikes
- Fixed: Calico CNI cross-node routing (VXLAN encapsulation)
- New metric: `mcp_gateway_request_duration_seconds{node,transport,status}`

**Results:** 100% E2E success rate (was ~10%), 150× latency improvement

### Current Milestone: M3 - Kubernetes Integration

**Progress:** ~80% complete (Epic #112 done, remaining: #97, #40, #30, #73)

---

## 🎯 Recommended Next Tasks (Priority Order)

### High Priority (P1) - Address Soon

| Issue | Title | Effort | Why Now |
|-------|-------|--------|---------|
| **#97** | NPM package abstracts kubectl port-forward | M | Critical UX for remote clusters - **PROMPT READY** |
| **#40** | `describe_gpu_node` tool | M | Comprehensive node diagnostics |
| **#30** | `get_pod_gpu_allocation` tool | M | GPU-to-pod correlation |
| **#73** | Multi-cluster support with context parameter | M | Enterprise use case |
| **#34** | kubectl debug E2E test suite | L | Ensure deployment works |

### Medium Priority (P2) - M4 Release Prep

| Issue | Title | Effort | Notes |
|-------|-------|--------|-------|
| **#86** | Release workflow with semantic versioning | M | Enable proper releases |
| **#77** | Publish Helm chart to GHCR OCI | S | Easy installation |
| **#84** | MCP integration tests | L | Protocol compliance |
| **#82** | AGENTS.md for AI assistants | S | Help AI tools navigate |
| **#78** | MCP Prompts support | L | Pre-defined SRE workflows |

### Low Priority (P3) - Nice to Have

| Issue | Title | Notes |
|-------|-------|-------|
| #89 | Demo videos and asciinema | Marketing |
| #88 | Healthz endpoint docs | Already works |
| #75 | PyPI package distribution | Python users |

---

## 📝 Active Prompts

| Prompt | Issue | Purpose | Status |
|--------|-------|---------|--------|
| `npm-kubectl-bridge.md` | #97 | NPM abstracts port-forward | 🟡 Ready to start |
| `investigate-cross-node-networking.md` | - | Debug CNI issues | ✅ Done (#126) |
| `docs-http-transport-update.md` | #117 | Architecture docs | ✅ Done (#125) |

---

## 🔧 Technical Debt & Production Readiness

### M4: Production Readiness (Tracking: #49)

| Issue | Area | Status | Priority |
|-------|------|--------|----------|
| #42 | Replace log.Printf with klog/v2 | 🔴 Not started | P2 |
| #43 | sync.WaitGroup for graceful shutdown | 🔴 Not started | P2 |
| #44 | Sentinel errors for NVML failures | 🔴 Not started | P3 |
| #56 | Flight recorder audit trail | 🔴 Not started | P3 |
| #59 | Graceful degradation for driver version skew | 🔴 Not started | P3 |

### Code Quality Notes (from 360 Review)

- **106 `log.Printf` calls** with manual JSON formatting → migrate to `slog`
- **3 TODOs in code** - all linked to issues (#68, #69), none critical
- **No panics** in production code (verified via grep)
- **Hardcoded `DefaultServiceName`** in `pkg/k8s/client.go:194` → make configurable

---

## 🧪 Testing Status

- **Unit Tests:** ✅ Passing (~7,257 test lines)
- **Race Detector:** ✅ All packages pass with `-race`
- **Integration Tests:** ⚠️ Need MCP protocol tests (#84)
- **E2E Tests:** ⚠️ Need kubectl debug tests (#34)
- **Real Cluster Testing:** ✅ AWS g4dn.xlarge (4-node cluster)
- **Gateway Metrics:** ✅ Per-node latency tracking (#127)

### Coverage by Package (2026-01-15)

| Package | Coverage | Status |
|---------|----------|--------|
| `pkg/tools` | 79.7% | ✅ Good |
| `pkg/xid` | 75.6% | ✅ Good |
| `pkg/gateway` | 72.9% | ✅ Good |
| `pkg/mcp` | 65.5% | ⚠️ Moderate |
| `pkg/k8s` | 58.0% | ⚠️ Moderate |
| `pkg/nvml` | 15.8% | ⚠️ Low (needs GPU) |
| `pkg/metrics` | 12.5% | 🔴 Low |

---

## 📋 Suggested Next Session Plan

### Option A: Complete M3 - Kubernetes Integration (Recommended)
1. **#97** - NPM kubectl port-forward abstraction ← **PROMPT READY**
2. **#40** - describe_gpu_node tool
3. **#30** - get_pod_gpu_allocation tool
4. Tag M3 complete, start M4

### Option B: Release Readiness Focus
1. **#86** - Release workflow with semantic versioning
2. **#77** - Helm chart to GHCR OCI
3. Tag v0.2.0 release

### Option C: Testing & Quality Focus
1. **#84** - MCP integration tests
2. **#34** - kubectl debug E2E tests
3. **#42** - Replace log.Printf with klog/v2

### Option D: Feature Focus
1. **#78** - MCP Prompts (SRE workflows)
2. **#73** - Multi-cluster support

---

## 📈 Issue Statistics

| Category | Count |
|----------|-------|
| Total Open Issues | 30 |
| P1 (High) | 10 |
| P2 (Medium) | ~10 |
| P3 (Low) | ~10 |

---

## 🗺️ Milestone Roadmap

| Milestone | Focus | Status |
|-----------|-------|--------|
| M1: Core NVML | GPU tools, XID analysis | ✅ Done |
| M2: Distribution | npm, Helm, container | ✅ Done |
| M3: K8s Integration | Gateway, multi-node | 🟡 80% (Epic #112 ✅) |
| M4: Safety & Release | Logging, lifecycle, release | 🔴 0% |
| M5: Quality & Testing | Integration tests, AI evals | 🔴 0% |
| M6: Polish & DX | Docs, demos, AGENTS.md | 🔴 0% |

---

## 🔍 Key Findings from Recent Work

### HTTP Transport Performance (Epic #112)

| Metric | Before (exec) | After (HTTP) | Improvement |
|--------|---------------|--------------|-------------|
| P50 Latency | ~30s | ~200ms | 150× faster |
| Success Rate | ~10% | 100% | Reliable |
| Memory | 200MB spikes | 15-20MB constant | 10× less |
| Timeout Handling | Race conditions | Clean cancellation | Stable |

### Cross-Node Networking (PR #126)

**Problem:** Gateway couldn't reach agent pods on other nodes via Pod IP.

**Root Cause:** Calico CNI defaults to IP-in-IP encapsulation which doesn't work across subnets in AWS VPC.

**Solution:** Configure Calico to use VXLAN encapsulation:
```yaml
# calico-config ConfigMap
vxlan: Always
```

### Gateway Observability (PR #127)

New Prometheus metric for production monitoring:
```promql
# P95 latency by node
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node)

# Slow nodes (p95 > 1s)
histogram_quantile(0.95, 
  rate(mcp_gateway_request_duration_seconds_bucket[5m])) by (node) > 1
```
