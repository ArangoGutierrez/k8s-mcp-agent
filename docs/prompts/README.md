# Prompt Library

This directory contains structured prompts for AI-assisted development tasks.
Each prompt provides detailed instructions for implementing features, fixing
bugs, or completing other project work.

## How to Use

1. **Copy the template:** Start with `TEMPLATE.md` for new tasks
2. **Fill in details:** Customize for your specific task/issue
3. **Follow the workflow:** Execute steps in order
4. **Wait for "GO":** Human approval required between major phases

## Active Prompts

| Prompt | Issue | Status | Description |
|--------|-------|--------|-------------|
| [remove-echo-test.md](remove-echo-test.md) | #100 | 🟡 P2 | Remove echo_test tool from production |
| [one-click-install-buttons.md](one-click-install-buttons.md) | #87 | 🟢 P3 | One-click install buttons for Cursor/VSCode |
| [gateway-mode.md](gateway-mode.md) | #72 | ✅ Done | Gateway mode for multi-node clusters |
| [gateway-tool-proxy.md](gateway-tool-proxy.md) | #98 | ✅ Done | Proxy all GPU tools via gateway |
| [real-cluster-integration-testing.md](real-cluster-integration-testing.md) | - | ✅ Done | Integration testing guide |

## Template

- **[TEMPLATE.md](TEMPLATE.md)** - Master template for creating new prompts

## Archived Prompts (Completed)

Completed prompts are moved to `archive/` for reference:

| Prompt | Related Issue/PR | Completed |
|--------|-----------------|-----------|
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
                    ┌─────────────────────────┐
                    │   Read Prompt / Issue   │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  Step 0: Create Branch  │
                    │  git checkout -b feat/  │
                    └───────────┬─────────────┘
                                │
           ┌────────────────────┼────────────────────┐
           │                    │                    │
           ▼                    ▼                    ▼
    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
    │   Task 1     │    │   Task 2     │    │   Task N     │
    │   Implement  │───►│   Implement  │───►│   Implement  │
    └──────────────┘    └──────────────┘    └──────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Pre-Commit Checks     │
                    │   make all              │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Commit & Push         │
                    │   git commit -s -S      │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Create PR             │
                    │   gh pr create          │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Wait for CI           │
                    │   gh pr checks --watch  │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Address Reviews       │
                    │   Copilot + Human       │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Merge PR              │
                    │   gh pr merge           │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │   Move prompt to        │
                    │   archive/              │
                    └─────────────────────────┘
```

## Related Documentation

- [Workflow & Git Protocol](/.cursor/rules/04-workflow-git.mdc) - Git standards
- [Go Development Standards](/.cursor/rules/00-general-go.mdc) - Code style
- [DEVELOPMENT.md](/DEVELOPMENT.md) - Full development guide
- [Architecture](../architecture.md) - System design

