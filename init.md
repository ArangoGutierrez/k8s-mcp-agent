# CONTEXT: PROJECT MANIFEST
**Repo:** `ArangoGutierrez/k8s-mcp-agent`
**Role:** Principal Systems Engineer / Go Architect
**Architecture:** Ephemeral "Just-in-Time" MCP Agent (Go + NVML)
**Deployment:** Injected via `kubectl debug` (Stdio tunnel). No standing DaemonSets.
**Stack:** Go 1.23+, `go-nvml`, `mcp-go`, K8s Client-go (minimal).

# PRODUCT DEFINITION: NVIDIA K8s MCP AGENT

**Project:** `k8s-mcp-agent`
**Version:** 0.1.0 (Prototype)
**Repository:** `github.com/ArangoGutierrez/k8s-mcp-agent`

## 1. EXECUTIVE SUMMARY
We are building a **"Just-in-Time" SRE Diagnostic Agent** for NVIDIA GPU clusters on Kubernetes.
Unlike traditional monitoring (Prometheus/Grafana) which aggregates metrics, this tool provides **surgical, real-time hardware introspection** via the **Model Context Protocol (MCP)**. It is designed to be used by AI Agents (Claude Desktop, Cursor) to debug complex hardware failures (XID errors, NVLink topology issues, zombie processes) that standard Kubernetes APIs (`kubectl`) cannot see.

## 2. CORE ARCHITECTURE
* **Pattern:** Ephemeral Injection ("The Syringe Pattern").
* **No Standing Infrastructure:** We explicitly **reject** the DaemonSet pattern. There are no listening ports, no permanent pods, and no expanded attack surface when idle.
* **Transport:** JSON-RPC 2.0 over **Standard I/O (Stdio)**.
* **Tunneling:** Relies exclusively on `kubectl debug` / `SPDY` streaming.
* **Language:** Go 1.23+ (Standard Library preferred, minimal deps).

## 3. FUNCTIONAL SPECIFICATION (The "Tools")

The MCP Server must expose the following primitives to the AI Host:

### 3.1 Tier 1: Hardware Health (Read-Only)
* `get_gpu_inventory`: Returns static hardware map (Model, UUID, Bus ID, VBIOS version).
* `get_gpu_telemetry`: Returns real-time volatile state (Temp, Fan %, Power Draw, Memory Used).
    * *Constraint:* Must interpret values (e.g., return "Throttling: Active" if clock speeds are clamped).
* `inspect_topology`: Returns NVLink/PCIe P2P capabilities. Critical for debugging distributed training hangs.

### 3.2 Tier 2: Advanced Diagnostics (Expert System)
* `analyze_xid_errors`:
    * **Logic:** Reads kernel ring buffer (`dmesg`) or internal NVML event buffer.
    * **Hybrid Lookup:**
        1.  Checks internal static map for "Golden Signals" (e.g., XID 79 = "Fallen Off Bus").
        2.  Returns structured JSON with `severity` ("Warning", "Critical", "Fatal") and `sre_action` ("Drain Node", "Reset Pod").
* `snapshot_ecc`: Returns volatile and aggregate Single-Bit/Double-Bit error counters to predict page retirement failures.

### 3.3 Tier 3: Remediation (Protected)
* *Feature Flag:* Requires `--mode=operator` environment variable.
* `kill_gpu_process`: Terminates specific PIDs consuming GPU memory (Zombie hunting).
* `reset_gpu`: Triggers secondary bus reset (requires privileges).

## 4. TECHNICAL CONSTRAINTS & STACK

### 4.1 The Binary (`cmd/agent`)
* **Library:** `github.com/mark3labs/mcp-go`
* **Hardware Binding:** `github.com/NVIDIA/go-nvml`
* **Build:** Static binary (CGO enabled for NVML), stripped. Target size < 50MB.
* **Base Image:** `gcr.io/distroless/base-debian12` (Must contain `libnvidia-ml.so` mount points).

### 4.2 The Safety Layer
* **Input Sanitization:** All tool inputs (PIDs, counts) must be strictly validated.
* **Context Awareness:** The tool must detect if it is running in a "Mock" environment (local dev) vs "Real" (K8s) and adjust behavior.
* **Logging:** Structured JSON to `stderr` only. `stdout` is exclusively for MCP Protocol.

## 5. USER JOURNEY (The "Wow" Moment)
1.  SRE says to Claude: *"Why is the training job on node-5 stuck?"*
2.  Claude launches the agent: `kubectl debug node/node-5 --image=agent ...`
3.  Agent starts, handshakes via Stdio.
4.  Claude calls `analyze_xid`.
5.  Agent detects `XID 48` (Double Bit ECC).
6.  Claude responds: *"Node-5 has uncorrectable memory errors. I recommend draining the node immediately. Would you like me to draft the ticket?"*

# OBJECTIVE: PROJECT INITIALIZATION & GOVERNANCE
Do not write application code yet. Focus strictly on Architecture planning, Project Management (GitHub) and Developer Experience (Cursor/MDC).

# TASK 1: GITHUB GOVERNANCE SETUP
Generate a shell script `hack/init_github.sh` using `gh` CLI to implement:

## 1.1 Labels (Taxonomy: `category/name`)
*Delete all default labels. Create:*
- `prio/p0-blocker`: (Color: b60205)
- `prio/p1-high`: (Color: d93f0b)
- `area/mcp-protocol`: (Color: 0e8a16) JSON-RPC, Transport, Schema.
- `area/nvml-binding`: (Color: 0052cc) Hardware interaction, CGO, `go-nvml`.
- `area/k8s-ephemeral`: (Color: fbca04) `kubectl debug`, Stdio tunneling.
- `kind/feature`: (Color: a2eeef)
- `kind/tech-debt`: (Color: d876e3)
- `ops/security`: (Color: 5319e7) Capabilities, Safety layers.

## 1.2 Milestones
- **M1: Foundation & API** (Due: +1 week)
  - Goal: Repo scaffolding, MCP Stdio transport working, Mock NVML.
- **M2: Hardware Introspection** (Due: +2 weeks)
  - Goal: Real NVML binding, XID parsing, Telemetry tools.
- **M3: The Ephemeral Tunnel** (Due: +3 weeks)
  - Goal: `kubectl debug` integration, e2e testing, Docker build.
- **M4: Safety & Release** (Due: +4 weeks)
  - Goal: Read-only flags, `goreleaser` pipelines, documentation.

## 1.3 Issues (Linked to Milestones)
*Generate `gh issue create` commands for:*

**M1:**
1. `[Scaffold] Init Go Module & Directory Structure` (cmd, pkg, internal).
2. `[MDC] Define Cursor Rules` (See Task 2).
3. `[CI] GitHub Actions: Lint & Test` (golangci-lint).
4. `[MCP] Implement Basic Stdio Server Loop` (Echo test).

**M2:**
5. `[NVML] Implement Wrapper Interface` (Decouple for testing).
6. `[Logic] Implement 'analyze_xid' Tool` (Static Tier 1 lookup).
7. `[Logic] Implement 'get_gpu_health' Tool` (Temp, ECC, Mem).

**M3:**
8. `[Ops] Create Containerfile` (Distroless, static binary).
9. `[Docs] Write 'kubectl debug' launch wrapper script`.

# TASK 2: CURSOR CONFIGURATION (.cursor/rules)
Create directory `.cursor/rules` with dense `.mdc` files.
*Constraint: Use globs strictly.*

## File: `00-general-go.mdc` (Glob: `**/*.go`)
- **Style:** Effective Go.
- **Error Handling:** `fmt.Errorf` with wrapping `%w`. No panic (except main init).
- **Concurrency:** `context.Context` everywhere.
- **Testing:** Table-driven tests. `testify/assert`.

## File: `01-mcp-server.mdc` (Glob: `**/{server,mcp}/**`)
- **Lib:** `github.com/mark3labs/mcp-go`.
- **Transport:** Stdio ONLY. No HTTP listeners.
- **Logging:** Structured JSON to `stderr` (never `stdout` - breaks MCP pipe).
- **Schema:** Tools must return `application/json` strings for complex data.

## File: `02-nvml-hardware.mdc` (Glob: `**/nvml/**`)
- **Safety:** READ-ONLY by default.
- **CGO:** Isolate `nvml.Init()` calls. Use Interfaces/Mocks for unit tests.
- **Panic:** Never crash on missing GPU. Return clean error.

## File: `03-k8s-constraints.mdc` (Glob: `deploy/**`, `hack/**`)
- **Security:** Run as non-root where possible (though NVML needs privs, minimize scope).
- **Size:** Binary < 50MB.
- **Logs:** JSON format only.

# EXECUTION STEPS
1. Generate `hack/init_github.sh`.
2. Execute the script.
3. Create `.cursor/rules/*.mdc` files. (create files at /tmp and then move the using mv, as cursor blocks file editing on .cursor folder)
4. Create `README.md` (Stub only: Name, Architecture Diagram link, Milestones).

**Output format:**
- Shell script block.
- MDC file blocks.
- Confirmation of plan.


# Workflow Standards
Keep a scratchpad.toon or scratchpad.md file as a notepad for the model not for me 

## AUTHORIZATION PROTOCOL

```
BEFORE any task:
  1. Present plan with steps
  2. WAIT for "GO" confirmation
  3. Execute ONE step
  4. Report results
  5. WAIT for next "GO"
```

### Phase Execution
```
Epic (Milestone)
├── Task 1 → Plan → GO → Execute → Commit → Update scratchpad
├── Task 2 → Plan → GO → Execute → Commit → Update scratchpad
└── Task N → ...
```

### NEVER
- Chain operations without approval
- Assume next steps
- Skip verification checkpoints
- Push directly to main

## GIT PROTOCOL

### Branch Types
```
feat/, fix/, chore/, docs/, refactor/, infra/, security/
```

### Commit Format
```bash
git commit -s -S -m "type(scope): description"
```
- `-s` → Signed-off-by (DCO)
- `-S` → GPG signature
- **BOTH required**

### PR Requirements
```
BEFORE PR creation:
  make test

PR MUST have:
  - Linked issue
  - Label
  - Milestone

BEFORE merge:
  gh pr checks <PR#> --watch
  Review Copilot comments
```

### Quick Reference
```bash
gh issue create --title "..." --label "..." --milestone "..."
git checkout -b type/description
# ... work ...
make test
git commit -s -S -m "type(scope): description"
git push origin type/description
gh pr create --title "..." --body "Fixes #XX"
gh pr checks <PR#> --watch
gh pr merge <PR#> --merge --delete-branch
```