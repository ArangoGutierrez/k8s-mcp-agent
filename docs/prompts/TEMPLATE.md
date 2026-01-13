# Feature/Task Prompt Template

> **Instructions:** Copy this template and fill in the sections for your specific
> task. Delete this instruction block and any sections that don't apply.

---

# [Title: Brief Description of the Task]

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/<this-file>.md`, the agent MUST continue
> working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke the prompt:** `@docs/prompts/<file>.md`

### Iteration Rules (For the Agent)

1. **NEVER STOP EARLY** - If any task is `[TODO]` or `[WIP]`, keep working
2. **UPDATE STATUS** - Edit this file: mark tasks `[WIP]` → `[DONE]` as you go
3. **COMMIT PROGRESS** - Commit and push after each completed task
4. **SELF-CHECK** - Before ending your turn, verify ALL tasks show `[DONE]`
5. **REPORT STATUS** - End each turn with a status summary of remaining tasks

### Progress Tracker

<!-- UPDATE THIS SECTION AS YOU WORK -->
<!-- Edit this file directly to track progress between invocations -->

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[TODO]` | |
| 1 | Task 1 placeholder | `[TODO]` | |
| 2 | Task 2 placeholder | `[TODO]` | |
| 3 | Run tests and verify | `[TODO]` | |
| 4 | Create pull request | `[TODO]` | |
| 5 | Wait for Copilot review | `[TODO]` | ⏳ Takes 1-2 min |
| 6 | Address review comments | `[TODO]` | |
| 7 | Merge after reviews | `[TODO]` | |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[BLOCKED:reason]`

### How to Use (For Humans)

```
1. Copy this template: cp TEMPLATE.md my-feature.md
2. Fill in task details
3. Invoke in Cursor: @docs/prompts/my-feature.md
4. Let the agent work
5. If tasks remain, re-invoke: @docs/prompts/my-feature.md
6. Repeat until all tasks show [DONE]
```

Typical workflow requires **3-5 invocations** for a complete feature:
- Invocation 1: Branch + initial implementation
- Invocation 2: Tests + fixes
- Invocation 3: PR creation + CI wait
- Invocation 4: Copilot review + fixes
- Invocation 5: Final merge

### Agent Self-Check (Before Ending Each Turn)

Before you finish ANY response, perform this self-check:

```
┌─────────────────────────────────────────────────────────────┐
│ SELF-CHECK: Can I end this turn?                            │
├─────────────────────────────────────────────────────────────┤
│ □ Have I made progress on at least one task?                │
│ □ Did I update the Progress Tracker in this file?           │
│ □ Did I commit my changes? (if code was modified)           │
│ □ Are there any [TODO] tasks I can continue working on?     │
├─────────────────────────────────────────────────────────────┤
│ If tasks remain → Tell user to re-invoke: @prompt <file>    │
│ If ALL [DONE] → Congratulate and suggest archiving prompt   │
└─────────────────────────────────────────────────────────────┘
```

### End-of-Turn Status Report

**Always end your turn with this format:**

```markdown
## 📊 Status Report

**Completed this turn:**
- [x] Task X
- [x] Task Y

**Remaining tasks:**
- [ ] Task Z (next priority)
- [ ] Task W

**Next invocation will:** [describe what happens next]

➡️ **Re-invoke to continue:** `@docs/prompts/<this-file>.md`
```

> ⚠️ **IMPORTANT:** Copilot reviews take 1-2 minutes to appear after PR creation.
> Do NOT merge until Copilot review is complete and all comments are addressed.

---

## Issue Reference

- **Issue:** [#XX - Title](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/XX)
- **Priority:** P0-Blocker | P1-High | P2-Medium | P3-Low
- **Labels:** kind/feature, area/..., ops/...
- **Milestone:** M1 | M2 | M3 | M4
- **Autonomous Mode:** ✅ Enabled (max 10 iterations)

## Background

Provide context for why this work is needed:
- What problem does it solve?
- How does it fit into the project roadmap?
- Any relevant architecture decisions or dependencies?

---

## Objective

One clear sentence describing the goal of this task.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

Before making any changes, create a feature branch from latest `main`:

```bash
# Ensure you're on main and up to date
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main

# Create feature branch with semantic prefix
git checkout -b <prefix>/<short-description>
```

### Branch Naming Conventions

| Prefix | Use Case | Example |
|--------|----------|---------|
| `feat/` | New features | `feat/npm-package` |
| `fix/` | Bug fixes | `fix/nvml-timeout` |
| `chore/` | Maintenance | `chore/update-deps` |
| `docs/` | Documentation | `docs/quickstart-k8s` |
| `refactor/` | Code refactoring | `refactor/nvml-interface` |
| `infra/` | Infrastructure/CI | `infra/release-workflow` |
| `security/` | Security patches | `security/input-validation` |

### Verify Branch

```bash
# Confirm you're on the new branch
git branch --show-current

# Should output: <prefix>/<short-description>
```

---

## Implementation Tasks

<!-- 
NOTE: Update the Progress Tracker YAML above as you complete each task!
  - When starting a task: change status to "[WIP]"
  - When completing a task: change status to "[DONE]"
  - If blocked: change status to "[BLOCKED:reason]"
-->

### Task 1: [First Task Title] `[TODO]`

Description of what needs to be done.

**Files to create/modify:**
- `path/to/file.go` - Description
- `path/to/another.go` - Description

**Code example (if helpful):**
```go
// Example implementation
package example
```

**Acceptance criteria for this task:**
- [ ] Criterion 1
- [ ] Criterion 2

> 💡 **After completing:** Update Progress Tracker → `status: "[DONE]"` → Commit

---

### Task 2: [Second Task Title] `[TODO]`

Continue with subsequent tasks...

> 💡 **After completing:** Update Progress Tracker → `status: "[DONE]"` → Commit

---

## Testing Requirements

### Local Testing (Mock Mode)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Run tests with race detector
make test

# Build and test manually
make agent
./bin/agent --nvml-mode=mock < examples/your_example.json
```

### Integration Testing (if applicable)

Describe any integration tests needed, including:
- Remote machine access (if GPU testing required)
- Kubernetes cluster testing
- External service dependencies

---

## Pre-Commit Checklist

Before committing, verify all of the following pass:

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Full check suite
make all
```

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass
- [ ] `go test ./... -race` - No race conditions
- [ ] Documentation updated if needed

---

## Commit and Push

### Atomic Commits (IMPORTANT)

> **⚠️ Prefer many small commits over one large commit.**

Each commit should represent **one logical change**. A PR with 10-20 focused
commits is better than a single massive commit. This provides:

- **Better traceability** - Easy to find when/why a change was introduced
- **Easier reviews** - Reviewers can understand changes incrementally
- **Simpler rollbacks** - Revert specific changes without losing everything
- **Cleaner history** - `git log` and `git bisect` become useful tools

**Good commit practices:**
```bash
# Each commit is one logical change
git commit -s -S -m "feat(gateway): add ProxyHandler struct"
git commit -s -S -m "feat(gateway): implement buildMCPRequest helper"
git commit -s -S -m "feat(gateway): add response aggregation logic"
git commit -s -S -m "test(gateway): add ProxyHandler unit tests"
git commit -s -S -m "feat(mcp): register proxy handlers in gateway mode"
```

**Bad commit practices:**
```bash
# One massive commit with everything
git commit -s -S -m "feat(gateway): implement full gateway proxy with tests"

# Vague or meaningless messages
git commit -s -S -m "WIP"
git commit -s -S -m "fixes"
git commit -s -S -m "updates"
```

**When to commit:**
- After completing a function or method
- After adding tests for a component
- After fixing a specific bug
- After updating configuration or docs
- Before switching to a different logical task

---

### Commit Message Format

All commits MUST be signed with DCO (`-s`) and GPG (`-S`):

```bash
git add -A
git commit -s -S -m "type(scope): description"
```

**Commit Types:**
- `feat` - New features
- `fix` - Bug fixes
- `chore` - Maintenance tasks
- `docs` - Documentation only
- `refactor` - Code refactoring
- `test` - Test improvements
- `ci` - CI/CD changes
- `security` - Security fixes
- `perf` - Performance improvements

**Scope Examples:** `mcp`, `nvml`, `k8s`, `helm`, `ci`, `tools`, `npm`, `docs`

**Examples:**
```bash
git commit -s -S -m "feat(npm): add npm package distribution"
git commit -s -S -m "fix(nvml): handle missing GPU gracefully"
git commit -s -S -m "docs(readme): add one-click install buttons"
```

### Push to Remote

Push frequently - don't wait until everything is done. This provides:
- **Backup** - Your work is safe on the remote
- **Early CI feedback** - Catch issues sooner
- **Visibility** - Others can see progress

```bash
# First push (sets upstream)
git push -u origin <your-branch-name>

# Subsequent pushes
git push
```

> 💡 **Push after every 2-3 commits** or after completing a logical milestone.

---

## Create Pull Request

### PR Creation Command

```bash
gh pr create \
  --title "type(scope): Brief description" \
  --body "Fixes #XX

## Summary
Brief description of changes.

## Changes
- Change 1
- Change 2

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass (if applicable)
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-reviewed the code
- [ ] Documentation updated (if applicable)" \
  --label "kind/feature" \
  --label "area/..." \
  --milestone "M2: Hardware Introspection"
```

### Required PR Elements

| Element | Description | Required |
|---------|-------------|----------|
| **Title** | Follows `type(scope): description` format | ✅ |
| **Body** | Includes "Fixes #XX" or "Closes #XX" | ✅ |
| **Label** | At least one `kind/` label | ✅ |
| **Label** | At least one `area/` label | ✅ |
| **Milestone** | Assigned to appropriate milestone | ✅ |

### Available Labels

**Kind Labels:**
- `kind/feature` - New feature
- `kind/fix` - Bug fix
- `kind/docs` - Documentation
- `kind/refactor` - Refactoring
- `kind/test` - Test improvements

**Area Labels:**
- `area/mcp-protocol` - MCP server/protocol
- `area/nvml-binding` - NVML/GPU hardware
- `area/k8s-ephemeral` - Kubernetes integration
- `ops/ci-cd` - CI/CD pipelines
- `ops/security` - Security related

**Priority Labels:**
- `prio/p0-blocker` - Critical blocker
- `prio/p1-high` - High priority
- `prio/p2-medium` - Medium priority
- `prio/p3-low` - Low priority

---

## Wait for CI Checks

After creating the PR, wait for all CI checks to pass:

```bash
# Watch CI status
gh pr checks <PR-NUMBER> --watch

# Or check in browser
gh pr view <PR-NUMBER> --web
```

### Expected CI Jobs

- [ ] **lint** - gofmt, go vet, golangci-lint
- [ ] **test** - Unit tests with race detector
- [ ] **build** - Build verification
- [ ] **DCO** - Developer Certificate of Origin check

### If CI Fails

1. Read the failure logs carefully
2. Fix the issue locally
3. Commit the fix with appropriate message
4. Push - CI will re-run automatically

```bash
# Example: fixing a lint issue
git add -A
git commit -s -S -m "fix(scope): address lint issues"
git push
```

---

## Review Process

### Copilot/Bot Reviews

> ⚠️ **WAIT FOR COPILOT REVIEW** - Reviews take 1-2 minutes to appear after PR
> creation. Do NOT proceed to merge until you have checked for Copilot comments.

**After creating the PR, wait and check:**
```bash
# Wait 1-2 minutes, then check for Copilot review
gh pr view <PR-NUMBER> --json reviews

# Or check in browser (look for Copilot's review)
gh pr view <PR-NUMBER> --web
```

GitHub Copilot may leave automated review comments. For each comment:

1. **Read carefully** - Understand the suggestion
2. **Evaluate** - Does it improve the code?
3. **If valid** - Implement the change
4. **If not applicable** - Reply explaining why

```bash
# After addressing review feedback
git add -A
git commit -s -S -m "fix(scope): address review feedback"
git push
```

**Re-check after pushing fixes** - Copilot may add more comments on new code.

### Human Reviews (if required)

- Respond to all review comments
- Request re-review after addressing feedback
- Don't merge until approved

---

## Merge the PR

### Pre-Merge Checklist

> ⚠️ **WAIT FOR COPILOT REVIEW** - Do NOT merge immediately after PR creation!
> Copilot reviews take 1-2 minutes to appear. Wait and check for comments.

- [ ] All CI checks pass ✅
- [ ] **Copilot review has appeared** (wait 1-2 min after PR creation)
- [ ] **ALL Copilot review comments addressed** (fix issues, push, re-check)
- [ ] Human review approved (if required)
- [ ] No merge conflicts

### Waiting for Copilot Review

```bash
# Check if Copilot review has appeared (run after 1-2 minutes)
gh pr view <PR-NUMBER> --json reviews --jq '.reviews[] | select(.author.login | contains("copilot"))'

# Or check in browser
gh pr view <PR-NUMBER> --web
```

If no Copilot review appears after 2 minutes, you can proceed with merge.
If comments appear, address them before merging.

### Merge Command

```bash
# Merge with merge commit (preserves history)
gh pr merge <PR-NUMBER> --merge --delete-branch

# Or squash merge (single commit)
gh pr merge <PR-NUMBER> --squash --delete-branch
```

### Post-Merge

```bash
# Switch back to main
git checkout main

# Pull merged changes
git pull origin main

# Verify merge
git log --oneline -5
```

---

## Related Files

List any related files or documentation:
- `path/to/related/file.go` - Description
- `docs/related-doc.md` - Description

## Notes

Any additional notes, gotchas, or considerations for this task.

---

## Quick Reference

### Key Commands

```bash
# Branch creation
git checkout -b feat/my-feature

# Commit (ALWAYS with -s -S)
git commit -s -S -m "type(scope): description"

# Push
git push -u origin feat/my-feature

# Create PR
gh pr create --title "..." --label "..." --milestone "..."

# Watch CI
gh pr checks <PR#> --watch

# Merge
gh pr merge <PR#> --merge --delete-branch
```

### Workflow Summary

```
1. Create branch (Step 0) ─────────────────────────────► feat/<name>
2. Implement ONE logical change ───────────────────────► Code changes
3. Run pre-commit checks ──────────────────────────────► make all
4. Commit with DCO + GPG ──────────────────────────────► git commit -s -S
5. Repeat steps 2-4 for each logical change ───────────► Atomic commits
6. Push to remote ─────────────────────────────────────► git push
7. Create PR with labels + milestone ──────────────────► gh pr create
8. Wait for CI ────────────────────────────────────────► gh pr checks --watch
9. ⏳ WAIT 1-2 min for Copilot review ─────────────────► Don't rush!
10. Address ALL Copilot review comments ───────────────► Fix → Push
11. Merge PR (only after reviews done) ────────────────► gh pr merge
```

> 💡 **Remember:** 10-20 atomic commits > 1 massive commit
> ⚠️ **Never merge before Copilot review appears!** (takes 1-2 min)

---

## Completion Protocol

### When All Tasks Are Done

Once you have verified that:
- ✅ All tasks in the Progress Tracker show `[DONE]`
- ✅ All tests pass (`make all` succeeds)
- ✅ PR is created and CI is green
- ✅ **Copilot review has appeared** (waited 1-2 min after PR creation)
- ✅ **All Copilot review comments addressed**
- ✅ PR is merged (or ready for human review)

**Final status report:**
```markdown
## 🎉 ALL TASKS COMPLETE

All tasks in this prompt have been completed successfully.

**Summary:**
- Branch: `feat/xxx`
- PR: #XXX (merged)
- Tests: ✅ Passing

**Recommend:** Move this prompt to `archive/`
```

### If Tasks Remain Incomplete

If ANY task is not `[DONE]`:

1. **Update the Progress Tracker** in this file with current status
2. **Commit your progress** so the next invocation can continue
3. **End with a status report** telling the user what remains
4. **Prompt re-invocation:** Tell user to run `@docs/prompts/<file>.md`

### State Persistence Between Invocations

Between `@prompt` invocations, state persists via:
- **Git commits** - Code changes are saved
- **Progress Tracker table** - Updated in this prompt file
- **GitHub Issues/PR** - Progress visible externally

The agent reads this file on each invocation to know what's done and what remains.

---

## Quick Reference

### Cursor Invocation Commands

```
# In Cursor chat, invoke the prompt:
@docs/prompts/my-task.md

# Re-invoke to continue (after agent ends turn):
@docs/prompts/my-task.md

# Check progress in terminal:
grep -E '\[TODO\]|\[WIP\]|\[DONE\]' docs/prompts/my-task.md
```

### Key Commands

```bash
# Branch creation
git checkout -b feat/my-feature

# Commit (ALWAYS with -s -S)
git commit -s -S -m "type(scope): description"

# Push
git push -u origin feat/my-feature

# Create PR
gh pr create --title "..." --label "..." --milestone "..."

# Watch CI
gh pr checks <PR#> --watch

# Merge
gh pr merge <PR#> --merge --delete-branch
```

### Workflow Summary (Cursor Iterative Mode)

```
┌─────────────────────────────────────────────────────────────────────────┐
│              RALPH WIGGUM PATTERN (Cursor Native)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   Human: @docs/prompts/my-task.md                                       │
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
│     │  Human re-      │          │  🎉 Complete!   │                    │
│     │  invokes prompt │          │  Archive prompt │                    │
│     └────────┬────────┘          └─────────────────┘                    │
│              │                                                          │
│              └──────────► Next turn...                                  │
│                                                                         │
│   Typical: 3-5 invocations per feature                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Manual Workflow Summary

```
1. Create branch (Step 0) ─────────────────────────────► feat/<name>
2. Implement ONE logical change ───────────────────────► Code changes
3. Run pre-commit checks ──────────────────────────────► make all
4. Commit with DCO + GPG ──────────────────────────────► git commit -s -S
5. Repeat steps 2-4 for each logical change ───────────► Atomic commits
6. Push to remote ─────────────────────────────────────► git push
7. Create PR with labels + milestone ──────────────────► gh pr create
8. Wait for CI ────────────────────────────────────────► gh pr checks --watch
9. ⏳ WAIT 1-2 min for Copilot review ─────────────────► Don't rush!
10. Address ALL Copilot review comments ───────────────► Fix → Push
11. Merge PR (only after reviews done) ────────────────► gh pr merge
```

> 💡 **Remember:** 10-20 atomic commits > 1 massive commit

---

**Reply "GO" when ready to start implementation.** 🚀

<!-- 
COMPLETION MARKER - Do not output until ALL tasks are [DONE]:
<completion>ALL_TASKS_DONE</completion>
-->


