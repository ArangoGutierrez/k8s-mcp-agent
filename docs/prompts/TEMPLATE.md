# Feature/Task Prompt Template

> **Instructions:** Copy this template and fill in the sections for your specific
> task. Delete this instruction block and any sections that don't apply.

---

# [Title: Brief Description of the Task]

## Issue Reference

- **Issue:** [#XX - Title](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/XX)
- **Priority:** P0-Blocker | P1-High | P2-Medium | P3-Low
- **Labels:** kind/feature, area/..., ops/...
- **Milestone:** M1 | M2 | M3 | M4

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

### Task 1: [First Task Title]

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

---

### Task 2: [Second Task Title]

Continue with subsequent tasks...

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

```bash
git push -u origin <your-branch-name>
```

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

### Human Reviews (if required)

- Respond to all review comments
- Request re-review after addressing feedback
- Don't merge until approved

---

## Merge the PR

### Pre-Merge Checklist

- [ ] All CI checks pass ✅
- [ ] Copilot review comments addressed
- [ ] Human review approved (if required)
- [ ] No merge conflicts

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
2. Implement tasks ────────────────────────────────────► Code changes
3. Run pre-commit checks ──────────────────────────────► make all
4. Commit with DCO + GPG ──────────────────────────────► git commit -s -S
5. Push to remote ─────────────────────────────────────► git push
6. Create PR with labels + milestone ──────────────────► gh pr create
7. Wait for CI ────────────────────────────────────────► gh pr checks --watch
8. Address Copilot/review comments ────────────────────► Fix → Push
9. Merge PR ───────────────────────────────────────────► gh pr merge
```

---

**Reply "GO" when ready to start implementation.** 🚀

