# One-Click Install Buttons for Cursor/VSCode

## Issue Reference

- **Issue:** [#87 - docs: One-click install buttons for Cursor/VSCode](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/87)
- **Priority:** P3-Low
- **Labels:** kind/docs
- **Milestone:** M3: Kubernetes Integration

## Background

Reference: `containers/kubernetes-mcp-server` has install buttons:

```markdown
[![Install MCP Server](https://cursor.com/deeplink/mcp-install-dark.svg)](https://cursor.com/en/install-mcp?...)
```

One-click install buttons:
- Reduce friction for new users
- Improve adoption rates
- Give professional appearance
- Match other MCP servers' UX

---

## Objective

Add one-click install badges to the README for easy MCP setup in Cursor, VS
Code, and Claude Desktop.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b docs/one-click-install
```

---

## Implementation Tasks

### Task 1: Research Deep Link Format

Understand the deep link format for each IDE.

**Cursor deep link format:**
```
https://cursor.com/install-mcp?name=<name>&config=<base64-encoded-config>
```

**Config to encode:**
```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "@arangogutierrez/k8s-gpu-mcp-server"]
    }
  }
}
```

**VS Code format:**
```
vscode://anysphere.cursor-mcp/install?config=<base64>
```

**Acceptance criteria:**
- [ ] Understand Cursor deep link format
- [ ] Understand VS Code MCP extension format (if available)
- [ ] Document the config structure

> 💡 **Commit after completing this task**

---

### Task 2: Create Install Button Assets

Create or source the badge images.

**Options:**
1. Use Cursor's official badge SVG (if available)
2. Use shields.io custom badges
3. Create simple markdown-based buttons

**Shields.io examples:**
```markdown
![Install in Cursor](https://img.shields.io/badge/Install-Cursor-blue?logo=cursor&logoColor=white)
![Install in VS Code](https://img.shields.io/badge/Install-VS%20Code-007ACC?logo=visualstudiocode&logoColor=white)
```

**Files to create:**
- `docs/images/install-cursor.svg` (optional - can use shields.io)

**Acceptance criteria:**
- [ ] Badge style decided (shields.io or custom)
- [ ] Consistent with project branding

> 💡 **Commit after completing this task**

---

### Task 3: Generate Base64 Config

Create the base64-encoded configuration for deep links.

**Script to generate:**
```bash
# For npx installation
echo '{"mcpServers":{"k8s-gpu-mcp":{"command":"npx","args":["-y","@arangogutierrez/k8s-gpu-mcp-server"]}}}' | base64

# For kubectl exec (advanced)
echo '{"mcpServers":{"k8s-gpu-mcp":{"command":"kubectl","args":["exec","-i","deploy/gpu-mcp-gateway","-n","gpu-diagnostics","--","/agent"]}}}' | base64
```

**Acceptance criteria:**
- [ ] Base64 config generated for npx method
- [ ] Base64 config generated for kubectl method (optional)
- [ ] Configs tested and working

> 💡 **Commit after completing this task**

---

### Task 4: Update README with Install Section

Add a new "Quick Install" section to the README.

**Files to modify:**
- `README.md` - Add install buttons section

**Proposed README addition:**

```markdown
## Quick Install

### One-Click Install

[![Install in Cursor](https://img.shields.io/badge/Install-Cursor-black?logo=data:image/svg+xml;base64,...&logoColor=white)](https://cursor.com/install-mcp?name=k8s-gpu-mcp&config=eyJtY3BTZXJ2ZXJzIjp7Ims4cy1ncHUtbWNwIjp7ImNvbW1hbmQiOiJucHgiLCJhcmdzIjpbIi15IiwiQGFyYW5nb2d1dGllcnJlei9rOHMtZ3B1LW1jcC1zZXJ2ZXIiXX19fQ==)

### Manual Installation

<details>
<summary>Cursor / VS Code</summary>

Add to `~/.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "@arangogutierrez/k8s-gpu-mcp-server"]
    }
  }
}
```
</details>

<details>
<summary>Claude Desktop</summary>

Add to Claude Desktop config:
```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "@arangogutierrez/k8s-gpu-mcp-server"]
    }
  }
}
```
</details>
```

**Acceptance criteria:**
- [ ] Install button with working deep link
- [ ] Manual installation instructions in collapsible sections
- [ ] Both Cursor and Claude Desktop covered
- [ ] Instructions are copy-paste ready

> 💡 **Commit after completing this task**

---

### Task 5: Test Deep Links

Verify the deep links work correctly.

**Testing steps:**
1. Click the Cursor install button
2. Verify it opens Cursor with the MCP config dialog
3. Verify the config is correct
4. Test the MCP connection works

**Acceptance criteria:**
- [ ] Cursor deep link opens correctly
- [ ] Config is properly applied
- [ ] MCP server connects successfully

> 💡 **Commit after completing this task**

---

## Testing Requirements

### Link Validation

```bash
# Decode and verify the config
echo "eyJtY3BTZXJ2ZXJzIjp7Ims4cy1ncHUtbWNwIjp7ImNvbW1hbmQiOiJucHgiLCJhcmdzIjpbIi15IiwiQGFyYW5nb2d1dGllcnJlei9rOHMtZ3B1LW1jcC1zZXJ2ZXIiXX19fQ==" | base64 -d | jq .
```

### Manual Testing

1. Open the README in GitHub
2. Click the install button
3. Verify Cursor opens with correct config

---

## Pre-Commit Checklist

- [ ] Links are valid and working
- [ ] README renders correctly on GitHub
- [ ] Images load (if using custom images)
- [ ] No broken markdown

---

## Commit and Push

### Commits

```bash
git commit -s -S -m "docs: add one-click install button for Cursor"
git commit -s -S -m "docs: add manual installation instructions"
```

### Push

```bash
git push -u origin docs/one-click-install
```

---

## Create Pull Request

```bash
gh pr create \
  --title "docs: add one-click install buttons for Cursor/VSCode" \
  --body "Fixes #87

## Summary
Adds one-click install buttons to README for easy MCP setup.

## Changes
- Add Cursor install deep link button
- Add collapsible manual installation sections
- Cover Cursor, VS Code, and Claude Desktop

## Screenshots
[Add screenshot of the install button]

## Testing
- [x] Deep link opens Cursor correctly
- [x] Config is properly formatted
- [x] README renders correctly on GitHub" \
  --label "kind/docs"
```

---

## Quick Reference

**Estimated Time:** 30-45 minutes

**Complexity:** Easy - documentation only

**Files Changed:**
- `README.md`
- `docs/images/` (optional)

**Dependencies:**
- NPM package must be published (`@arangogutierrez/k8s-gpu-mcp-server`)

---

**Reply "GO" when ready to start implementation.** 🚀
