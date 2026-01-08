# Project Scratchpad - k8s-gpu-mcp-server

> Last updated: 2026-01-08

## 📊 Project Status Overview

### Recently Completed (Last 7 Days)

| PR | Description | Impact |
|----|-------------|--------|
| #109 | `/dev/kmsg` XID parsing for distroless containers | Enables XID analysis without dmesg |
| #108 | Consolidated `list_gpu_nodes` into `get_gpu_inventory` | Simpler API, fewer tools |
| #106 | One-click install button for Cursor | Better UX |
| #105 | Removed `echo_test` tool | Cleaner tool list |
| #101 | Gateway proxies all GPU tools to agents | Multi-node support |
| #94 | Gateway mode with node routing | Cluster-wide diagnostics |
| #93 | HTTP/SSE transport | Remote MCP access |
| #92 | npm package distribution | Easy installation |

### Current Milestone: M3 - Kubernetes Integration

**Progress:** ~60% complete

---

## 🎯 Recommended Next Tasks (Priority Order)

### High Priority (P1) - Address Soon

| Issue | Title | Effort | Why Now |
|-------|-------|--------|---------|
| **#97** | NPM package abstracts kubectl port-forward | M | Critical UX improvement for remote clusters |
| **#84** | MCP integration tests | L | Ensure protocol compliance |
| **#82** | AGENTS.md for AI assistants | S | Help AI tools navigate codebase |
| **#86** | Release workflow with semantic versioning | M | Enable proper releases |
| **#77** | Publish Helm chart to GHCR OCI | S | Easy installation |

### Medium Priority (P2) - Address When Possible

| Issue | Title | Effort | Notes |
|-------|-------|--------|-------|
| **#78** | MCP Prompts support | L | Pre-defined diagnostic workflows |
| **#85** | AI evals with gevals framework | L | Automated AI testing |
| **#79** | Toolsets with enable/disable | M | Reduce context size |
| **#40** | `describe_gpu_node` tool | M | Comprehensive node view |
| **#30** | `get_pod_gpu_allocation` tool | M | GPU-to-pod correlation |

### Low Priority (P3) - Nice to Have

| Issue | Title | Notes |
|-------|-------|-------|
| #89 | Demo videos and asciinema | Marketing |
| #88 | Healthz endpoint for HTTP mode | Already works, needs docs |
| #41 | Helm chart for DaemonSet | Already have it |

---

## 🔧 Technical Debt & Production Readiness

### M4: Production Readiness (Tracking: #49)

| Issue | Area | Status |
|-------|------|--------|
| #42 | Replace log.Printf with klog/v2 | 🔴 Not started |
| #43 | sync.WaitGroup for graceful shutdown | 🔴 Not started |
| #44 | Sentinel errors for NVML failures | 🔴 Not started |
| #56 | Flight recorder audit trail | 🔴 Not started |
| #59 | Graceful degradation for driver version skew | 🔴 Not started |

---

## 📝 Active Prompts

| Prompt | Purpose | Status |
|--------|---------|--------|
| `consolidate-gpu-inventory.md` | Merge list_gpu_nodes into get_gpu_inventory | ✅ Done (#108) |
| `read-kmsg-xid-parsing.md` | /dev/kmsg for distroless | ✅ Done (#109) |

---

## 🧪 Testing Status

- **Unit Tests:** ✅ Passing (~1,500 lines)
- **Integration Tests:** ⚠️ Need MCP protocol tests (#84)
- **E2E Tests:** ⚠️ Need kubectl debug tests (#34)
- **Real Cluster Testing:** ✅ Manual testing on AWS g4dn.xlarge

---

## 🔍 Key Findings from Recent Work

### cgroup v2 and /dev/kmsg Access (PR #109)

**Discovery:** Reading `/dev/kmsg` in Kubernetes requires `privileged: true` due to cgroup v2 BPF device controller, not just CAP_SYSLOG.

**Security Layers Tested:**
| Layer | Sufficient? |
|-------|-------------|
| CAP_SYSLOG | ❌ No |
| + Seccomp=Unconfined | ❌ No |
| + AppArmor=Unconfined | ❌ No |
| + privileged=true | ✅ Yes |

**Root Cause:** cgroup v2 uses eBPF programs to control device access, and `/dev/kmsg` is not in the default allowlist.

---

## 📋 Suggested Next Session Plan

### Option A: Developer Experience Focus
1. **#97** - NPM kubectl port-forward abstraction (biggest UX win)
2. **#82** - AGENTS.md for AI assistants

### Option B: Release Readiness Focus
1. **#86** - Release workflow with semantic versioning
2. **#77** - Helm chart to GHCR OCI
3. Tag v0.2.0 release

### Option C: Testing & Quality Focus
1. **#84** - MCP integration tests
2. **#83** - Migrate to testify/suite
3. **#42** - Replace log.Printf with klog/v2

### Option D: Feature Focus
1. **#78** - MCP Prompts (SRE workflows)
2. **#40** - describe_gpu_node tool
3. **#30** - get_pod_gpu_allocation tool

---

## 📈 Issue Statistics

| Category | Count |
|----------|-------|
| Total Open Issues | 46 |
| P0 (Blocker) | 1 (#49 tracking) |
| P1 (High) | 14 |
| P2 (Medium) | 14 |
| P3 (Low) | 10 |
| Kind: Feature | 30 |
| Kind: Test | 5 |
| Kind: Docs | 4 |
| Kind: Refactor | 4 |

---

## 🗺️ Milestone Roadmap

| Milestone | Focus | Status |
|-----------|-------|--------|
| M1: Core NVML | GPU tools, XID analysis | ✅ Done |
| M2: Distribution | npm, Helm, container | ✅ Done |
| M3: K8s Integration | Gateway, multi-node | 🟡 60% |
| M4: Production Ready | Logging, lifecycle, errors | 🔴 0% |
| M5: Advanced | MIG, eBPF, multi-cluster | 🔴 0% |
