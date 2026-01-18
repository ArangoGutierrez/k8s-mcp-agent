# Prompt Library

This directory contains the prompt template for AI-assisted development tasks.
The template provides structured instructions for implementing features, fixing
bugs, or completing other project work with Cursor IDE.

## Template

- **[TEMPLATE.md](TEMPLATE.md)** - Master template for creating task prompts

## How to Use

### Creating a New Task Prompt

1. **Copy the template:**
   ```bash
   cp docs/prompts/TEMPLATE.md docs/prompts/my-feature.md
   ```

2. **Fill in details:** Customize for your specific task/issue

3. **Invoke in Cursor:**
   ```
   @docs/prompts/my-feature.md
   ```

4. **Let the agent work:** It will update progress and commit changes

5. **Re-invoke if needed:** If tasks remain incomplete
   ```
   @docs/prompts/my-feature.md
   ```

6. **Repeat until all tasks show `[DONE]`**

### Iterative Execution (Ralph Wiggum Pattern)

The template is designed for multi-turn execution. The agent:
- Reads the Progress Tracker to find `[TODO]` tasks
- Works on tasks, updating status: `[TODO]` → `[WIP]` → `[DONE]`
- Commits progress after each task
- Ends turns with a status report
- Continues on re-invocation until everything is complete

**Typical invocations per task:**
| Complexity | Invocations |
|------------|-------------|
| Simple fix | 2-3 |
| Small feature | 3-5 |
| Medium feature | 5-8 |
| Large feature | 8-12 |

## Prompt Structure

The template follows this structure:

```
1. Issue Reference       - Link to GitHub issue
2. Background            - Context and motivation
3. Progress Tracker      - Task status table (auto-updated)
4. Step 0: Branch        - Always create branch first
5. Implementation Tasks  - Step-by-step instructions
6. Testing               - How to verify the work
7. Pre-Commit Checklist  - Quality gates
8. Commit and Push       - DCO + GPG signing
9. Create PR             - With labels and milestone
10. CI Checks            - Wait for green
11. Review Process       - Address Copilot/human feedback
12. Merge                - Final steps (requires human approval)
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

## Git Ignore

Prompt files (except `TEMPLATE.md` and `README.md`) are gitignored to keep
development artifacts out of the repository. Your working prompts remain local.

## Related Documentation

- [DEVELOPMENT.md](/DEVELOPMENT.md) - Full development guide
- [Architecture](../architecture.md) - System design
- [MCP Usage](../mcp-usage.md) - Protocol examples
