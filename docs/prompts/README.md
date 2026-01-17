# Prompt Library

This directory contains structured prompts for AI-assisted development tasks.
Each prompt provides detailed instructions for implementing features, fixing
bugs, or completing other project work.

## How to Use

### Manual Mode (Human-in-the-Loop)

1. **Copy the template:** Start with `TEMPLATE.md` for new tasks
2. **Fill in details:** Customize for your specific task/issue
3. **Follow the workflow:** Execute steps in order
4. **Wait for "GO":** Human approval required between major phases

### Iterative Mode (Ralph Wiggum Pattern)

For complex multi-step tasks, prompts are designed to be invoked multiple
times until all tasks are complete. This is the **Ralph Wiggum pattern** -
the agent keeps working across invocations until everything is done.

#### Quick Start

```
# In Cursor chat, invoke the prompt:
@docs/prompts/my-task.md

# Agent works, updates progress, ends turn with status report
# If tasks remain, re-invoke:
@docs/prompts/my-task.md

# Repeat until all tasks show [DONE]
```

#### How It Works

```
┌─────────────────────────────────────────────────────────────────────────┐
│              RALPH WIGGUM PATTERN (Cursor Native)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   You: @docs/prompts/my-task.md                                         │
│                    │                                                    │
│                    ▼                                                    │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │  AGENT TURN                                                     │   │
│   │                                                                 │   │
│   │  1. Read Progress Tracker → Find [TODO] tasks                   │   │
│   │  2. Work on next task                                           │   │
│   │  3. Update tracker: [TODO] → [WIP] → [DONE]                     │   │
│   │  4. Commit progress                                             │   │
│   │  5. Self-check: more [TODO]s?                                   │   │
│   │                                                                 │   │
│   └─────────────────────────┬──────────────────────────────────────┘   │
│                             │                                           │
│                             ▼                                           │
│                  ┌─────────────────────┐                                │
│                  │   Status Report     │                                │
│                  │   "Re-invoke to     │                                │
│                  │    continue..."     │                                │
│                  └──────────┬──────────┘                                │
│                             │                                           │
│              ┌──────────────┴──────────────┐                            │
│              │                             │                            │
│              ▼                             ▼                            │
│     ┌─────────────────┐          ┌─────────────────┐                    │
│     │  Tasks remain   │          │  ALL [DONE]     │                    │
│     │                 │          │                 │                    │
│     │  You re-invoke  │          │  🎉 Complete!   │                    │
│     │  the prompt     │          │  Archive prompt │                    │
│     └────────┬────────┘          └─────────────────┘                    │
│              │                                                          │
│              └──────────► Next turn...                                  │
│                                                                         │
│   Typical: 3-5 invocations per feature                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Progress Tracker

Every prompt includes a Progress Tracker table that the agent updates:

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[TODO]` | |
| 1 | Implement feature | `[TODO]` | |
| 2 | Run tests | `[TODO]` | |
| 3 | Create PR | `[TODO]` | |
| 4 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 5 | Address review comments | `[TODO]` | |
| 6 | Merge after reviews | `[TODO]` | |

**Status values:**
- `[TODO]` → Task not started
- `[WIP]` → Currently working on it
- `[DONE]` → Task completed
- `[BLOCKED:reason]` → Cannot proceed (with explanation)

#### Example: Running a Feature Prompt

```
# 1. Create your prompt from template
cp docs/prompts/TEMPLATE.md docs/prompts/feat-new-feature.md

# 2. Edit the prompt with your task details

# 3. In Cursor chat, invoke:
@docs/prompts/feat-new-feature.md

# 4. Agent works, updates Progress Tracker, commits changes

# 5. Agent ends turn with status report:
#    "Completed: branch creation, initial impl
#     Remaining: tests, PR, review
#     Re-invoke: @docs/prompts/feat-new-feature.md"

# 6. Re-invoke to continue:
@docs/prompts/feat-new-feature.md

# 7. Repeat until all [DONE]
```

#### Typical Invocation Count

| Task Complexity | Invocations | Example |
|-----------------|-------------|---------|
| Simple fix | 2-3 | Typo fix, config change |
| Small feature | 3-5 | New tool, API endpoint |
| Medium feature | 5-8 | Multi-file refactor |
| Large feature | 8-12 | New subsystem |

#### Key Behaviors

- **Progress persists:** Agent updates the prompt file itself between turns
- **Git commits:** Changes are committed, so progress survives re-invocation
- **Status reports:** Each turn ends with clear "what's done / what's next"
- **Copilot wait:** Agent must wait for Copilot review before merging

## Active Prompts

| Prompt | Issue | Status | Description |
|--------|-------|--------|-------------|
| [docs-http-transport-update.md](docs-http-transport-update.md) | #117 | 🟡 P2 | Update docs for HTTP transport (Phase 5) |
| [npm-kubectl-bridge.md](npm-kubectl-bridge.md) | #97 | 🟡 P1 | NPM package kubectl port-forward bridge |

## Template

- **[TEMPLATE.md](TEMPLATE.md)** - Master template for creating new prompts

## Archived Prompts (Completed)

Completed prompts are moved to `archive/` for reference:

| Prompt | Related Issue/PR | Completed |
|--------|-----------------|-----------|
| mcp-prompts-implementation.md | #78, PR #140 | Jan 2026 |
| gateway-resilience-observability.md | #116, PR #123 | Jan 2026 |
| gateway-http-routing.md | #115, PR #122 | Jan 2026 |
| agent-http-default.md | #114, PR #121 | Jan 2026 |
| fix-timeout-alignment.md | #113, PR #119 | Jan 2026 |
| consolidate-gpu-inventory.md | #99, PR #108 | Jan 2026 |
| remove-echo-test.md | #100, PR #105 | Jan 2026 |
| one-click-install-buttons.md | #87, PR #106 | Jan 2026 |
| gateway-tool-proxy.md | #98, PR #101 | Jan 2026 |
| gateway-mode.md | #72, PR #94 | Jan 2026 |
| real-cluster-integration-testing.md | Testing guide | Jan 2026 |
| http-sse-transport.md | #71, PR #93 | Jan 2026 |
| npm-package-distribution.md | #74, PR #92 | Jan 2026 |
| daemonset-manifests.md | PR #67 | Jan 2026 |
| gpu-health-monitoring.md | Issue #7, M2 | Jan 2026 |
| gpu-inventory-enhancement.md | Issue #5, M2 | Jan 2026 |
| m2-xid-implementation.md | Issue #6, M2 | Jan 2026 |
| m2-xid-task.md | Issue #6, M2 | Jan 2026 |
| nvml-interface-extension.md | Issue #5, M2 | Jan 2026 |
| xid-error-analysis.md | Issue #6, M2 | Jan 2026 |
| deploy-k8s-testing.md | Testing reference | Jan 2026 |

## Prompt Structure

Every prompt follows this structure:

```
1. Issue Reference       - Link to GitHub issue
2. Background            - Context and motivation
3. Step 0: Branch        - ⚠️ ALWAYS create branch first
4. Implementation Tasks  - Step-by-step instructions
5. Testing               - How to verify the work
6. Pre-Commit Checklist  - Quality gates
7. Commit and Push       - DCO + GPG signing
8. Create PR             - With labels and milestone
9. CI Checks             - Wait for green
10. Review Process       - Address Copilot/human feedback
11. Merge                - Final steps
```

## Key Principles

### Branch First
Always create a feature branch before making changes:
```bash
git checkout main && git pull
git checkout -b feat/my-feature
```

### Commit Requirements
All commits must be signed:
```bash
git commit -s -S -m "type(scope): description"
```

### PR Requirements
Every PR needs:
- Linked issue (`Fixes #XX`)
- At least one label
- Assigned milestone

## Creating New Prompts

1. Copy `TEMPLATE.md` to a new file: `my-feature.md`
2. Fill in all sections
3. Remove instructions and unused sections
4. Add to the "Active Prompts" table in this README

## Workflow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         PROMPT WORKFLOW                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   1. CREATE PROMPT                                                      │
│      cp TEMPLATE.md my-feature.md                                       │
│      Edit with task details                                             │
│                            │                                            │
│                            ▼                                            │
│   2. INVOKE IN CURSOR ─────────────────────────────┐                    │
│      @docs/prompts/my-feature.md                   │                    │
│                            │                       │                    │
│                            ▼                       │                    │
│   ┌────────────────────────────────────────────┐   │                    │
│   │  AGENT TURN                                │   │                    │
│   │                                            │   │                    │
│   │  • Read Progress Tracker                   │   │                    │
│   │  • Work on [TODO] tasks                    │   │                    │
│   │  • Update tracker → [DONE]                 │   │                    │
│   │  • Commit & push progress                  │   │                    │
│   │  • End with Status Report                  │   │                    │
│   │                                            │   │                    │
│   └─────────────────────┬──────────────────────┘   │                    │
│                         │                          │                    │
│                         ▼                          │                    │
│   3. CHECK STATUS ──────┴──────────────────────────┤                    │
│                         │                          │                    │
│          ┌──────────────┴──────────────┐           │                    │
│          │                             │           │                    │
│          ▼                             ▼           │                    │
│   ┌─────────────────┐          ┌─────────────────┐ │                    │
│   │  Tasks remain   │          │  ALL [DONE]     │ │                    │
│   │  [TODO] exists  │          │                 │ │                    │
│   │                 │          │  🎉 Complete!   │ │                    │
│   │  Re-invoke ─────┼──────────┼►Archive prompt  │ │                    │
│   └────────┬────────┘          └─────────────────┘ │                    │
│            │                                       │                    │
│            └───────────────────────────────────────┘                    │
│                                                                         │
│   Typical: 3-5 invocations │ Complex: 8-12 invocations                  │
└─────────────────────────────────────────────────────────────────────────┘
```

## Related Documentation

- [Workflow & Git Protocol](/.cursor/rules/04-workflow-git.mdc) - Git standards
- [Go Development Standards](/.cursor/rules/00-general-go.mdc) - Code style
- [DEVELOPMENT.md](/DEVELOPMENT.md) - Full development guide
- [Architecture](../architecture.md) - System design

