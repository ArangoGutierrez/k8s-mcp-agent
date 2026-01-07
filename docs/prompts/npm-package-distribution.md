# npm Package Distribution

## Issue Reference

- **Issue:** [#74 - feat: npm package distribution](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/74)
- **Priority:** P0-Blocker
- **Labels:** kind/feature, ops/ci-cd
- **Milestone:** M4: Safety & Release
- **Depends On:** [#76 - Multi-platform binary releases](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/76)

## Background

The `containers/kubernetes-mcp-server` project (reference implementation) is
available via `npx kubernetes-mcp-server@latest`, providing one-liner
installation for Cursor and Claude Desktop users.

Currently, `k8s-gpu-mcp-server` requires:
1. Cloning the repo
2. Building from source
3. Manual binary installation

This is a significant adoption barrier. npm distribution enables:
- **One-liner installation:** `npx k8s-gpu-mcp-server@latest`
- **Version management:** Semantic versioning via npm
- **Cross-platform support:** Platform-specific binary download
- **Easy MCP configuration:** Simple Cursor/Claude Desktop setup

### Reference Implementation

See `containers/kubernetes-mcp-server`:
- [npm package](https://www.npmjs.com/package/kubernetes-mcp-server)
- [npm/ directory structure](https://github.com/containers/kubernetes-mcp-server/tree/main/npm)

---

## Objective

Publish an npm package that downloads the correct platform-specific binary and
enables `npx k8s-gpu-mcp-server` for instant MCP server access.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/npm-package
```

Verify:
```bash
git branch --show-current
# Should output: feat/npm-package
```

---

## Prerequisites

Before starting this task, ensure:

- [ ] **#76 is complete** - Multi-platform binaries available on GitHub Releases
- [ ] npm account with publish access
- [ ] `NPM_TOKEN` secret configured in GitHub repo settings

If #76 is not complete, work on that first or coordinate both together.

---

## Implementation Tasks

### Task 1: Create npm Package Structure

Create the `npm/` directory with package structure:

```bash
mkdir -p npm
```

**Files to create:**

#### `npm/package.json`

```json
{
  "name": "k8s-gpu-mcp-server",
  "version": "0.1.0",
  "description": "MCP server for NVIDIA GPU diagnostics in Kubernetes clusters",
  "keywords": [
    "mcp",
    "model-context-protocol",
    "nvidia",
    "gpu",
    "kubernetes",
    "diagnostics",
    "claude",
    "cursor"
  ],
  "homepage": "https://github.com/ArangoGutierrez/k8s-gpu-mcp-server",
  "bugs": {
    "url": "https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues"
  },
  "license": "Apache-2.0",
  "author": "Eduardo Arango <arangogutierrez@gmail.com>",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/ArangoGutierrez/k8s-gpu-mcp-server.git"
  },
  "bin": {
    "k8s-gpu-mcp-server": "./bin/k8s-gpu-mcp-server"
  },
  "scripts": {
    "postinstall": "node scripts/postinstall.js"
  },
  "files": [
    "bin/",
    "scripts/",
    "README.md"
  ],
  "engines": {
    "node": ">=18.0.0"
  },
  "os": [
    "darwin",
    "linux",
    "win32"
  ],
  "cpu": [
    "x64",
    "arm64"
  ]
}
```

#### `npm/README.md`

```markdown
# k8s-gpu-mcp-server

MCP server for NVIDIA GPU diagnostics in Kubernetes clusters.

## Installation

```bash
# Run directly with npx (recommended)
npx k8s-gpu-mcp-server@latest

# Or install globally
npm install -g k8s-gpu-mcp-server
```

## Usage with Cursor

Add to your Cursor MCP configuration:

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

## Usage with Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

## Available Tools

- `get_gpu_inventory` - Hardware inventory and telemetry
- `get_gpu_health` - GPU health monitoring with scoring
- `analyze_xid_errors` - Parse GPU XID errors from kernel logs
- `echo_test` - MCP protocol validation

## Documentation

- [Full Documentation](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server)
- [Architecture](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/architecture.md)
- [MCP Usage Guide](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/blob/main/docs/mcp-usage.md)

## License

Apache-2.0
```

---

### Task 2: Create Binary Download Script

The postinstall script downloads the correct platform binary from GitHub Releases.

#### `npm/scripts/postinstall.js`

```javascript
#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const REPO = 'ArangoGutierrez/k8s-gpu-mcp-server';
const BINARY_NAME = 'k8s-gpu-mcp-server';

// Map Node.js platform/arch to Go build targets
const PLATFORM_MAP = {
  'darwin-x64': 'darwin-amd64',
  'darwin-arm64': 'darwin-arm64',
  'linux-x64': 'linux-amd64',
  'linux-arm64': 'linux-arm64',
  'win32-x64': 'windows-amd64',
};

function getPlatformKey() {
  const platform = process.platform;
  const arch = process.arch;
  return `${platform}-${arch}`;
}

function getBinaryName(platformKey) {
  const goPlatform = PLATFORM_MAP[platformKey];
  if (!goPlatform) {
    throw new Error(`Unsupported platform: ${platformKey}`);
  }
  
  const ext = process.platform === 'win32' ? '.exe' : '';
  return `${BINARY_NAME}-${goPlatform}${ext}`;
}

function getPackageVersion() {
  const packageJson = require('../package.json');
  return packageJson.version;
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 5) {
        reject(new Error('Too many redirects'));
        return;
      }

      https.get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          follow(res.headers.location, redirects + 1);
          return;
        }

        if (res.statusCode !== 200) {
          reject(new Error(`Download failed with status: ${res.statusCode}`));
          return;
        }

        const file = fs.createWriteStream(dest);
        res.pipe(file);
        file.on('finish', () => {
          file.close();
          resolve();
        });
        file.on('error', reject);
      }).on('error', reject);
    };

    follow(url);
  });
}

async function main() {
  const platformKey = getPlatformKey();
  const binaryName = getBinaryName(platformKey);
  const version = getPackageVersion();
  
  console.log(`Installing k8s-gpu-mcp-server v${version} for ${platformKey}...`);

  // Create bin directory
  const binDir = path.join(__dirname, '..', 'bin');
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  // Download URL
  const downloadUrl = `https://github.com/${REPO}/releases/download/v${version}/${binaryName}`;
  const destPath = path.join(binDir, BINARY_NAME + (process.platform === 'win32' ? '.exe' : ''));

  try {
    console.log(`Downloading from: ${downloadUrl}`);
    await downloadFile(downloadUrl, destPath);
    
    // Make executable on Unix
    if (process.platform !== 'win32') {
      fs.chmodSync(destPath, 0o755);
    }

    console.log(`Successfully installed to: ${destPath}`);
  } catch (error) {
    console.error(`Failed to download binary: ${error.message}`);
    console.error('');
    console.error('You may need to:');
    console.error('1. Check if the release exists on GitHub');
    console.error('2. Build from source: https://github.com/ArangoGutierrez/k8s-gpu-mcp-server');
    process.exit(1);
  }
}

main();
```

---

### Task 3: Create Wrapper Script (for bin)

Create a placeholder that gets replaced by the downloaded binary.

#### `npm/bin/.gitkeep`

```
# This directory is populated by postinstall.js
```

The actual binary is downloaded during `npm install` and placed here.

---

### Task 4: Create GitHub Actions Workflow for npm Publish

#### `.github/workflows/npm-publish.yml`

```yaml
name: Publish npm Package

on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to publish (e.g., 0.1.0)'
        required: true

jobs:
  publish:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          registry-url: 'https://registry.npmjs.org'

      - name: Update package version
        working-directory: npm
        run: |
          VERSION="${{ github.event.inputs.version || github.event.release.tag_name }}"
          VERSION="${VERSION#v}"  # Remove 'v' prefix if present
          npm version "$VERSION" --no-git-tag-version

      - name: Verify package
        working-directory: npm
        run: |
          npm pack --dry-run

      - name: Publish to npm
        working-directory: npm
        run: npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

---

### Task 5: Add npm Package to .gitignore

Update `.gitignore` to exclude npm artifacts:

```gitignore
# npm package artifacts
npm/bin/
npm/node_modules/
npm/*.tgz
```

---

### Task 6: Update Main README

Add one-click install section to `README.md`:

```markdown
## 🚀 Quick Start

### One-Line Installation

```bash
# Using npx (recommended)
npx k8s-gpu-mcp-server@latest

# Or install globally
npm install -g k8s-gpu-mcp-server
```

### Use with Cursor

Add to your Cursor MCP configuration:

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

### Use with Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```
```

---

## File Structure

After completing all tasks:

```
npm/
├── package.json           # npm package manifest
├── README.md              # npm package README
├── bin/
│   └── .gitkeep           # Placeholder (binary downloaded at install)
└── scripts/
    └── postinstall.js     # Binary download script

.github/workflows/
└── npm-publish.yml        # npm publish workflow
```

---

## Testing

### Local Testing

```bash
cd npm

# Install dependencies (none currently)
npm install

# Test the postinstall script (requires release to exist)
node scripts/postinstall.js

# Verify binary works
./bin/k8s-gpu-mcp-server --help

# Pack locally to test
npm pack
```

### Test with npx (after publish)

```bash
# Clear npx cache
npx clear-npx-cache

# Test installation
npx k8s-gpu-mcp-server@latest --help
```

---

## Pre-Commit Checklist

- [ ] `npm/package.json` has correct version and metadata
- [ ] `npm/README.md` has clear usage instructions
- [ ] `npm/scripts/postinstall.js` handles all platforms
- [ ] `.github/workflows/npm-publish.yml` workflow configured
- [ ] `.gitignore` updated for npm artifacts
- [ ] Main `README.md` updated with npm install instructions
- [ ] `NPM_TOKEN` secret configured in GitHub repo

---

## Commit and Push

```bash
git add -A
git commit -s -S -m "feat(npm): add npm package distribution

- Add npm package structure in npm/
- Add postinstall script for binary download
- Add GitHub Actions workflow for npm publish
- Update README with npm installation instructions

Fixes #74"

git push -u origin feat/npm-package
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(npm): add npm package distribution" \
  --body "Fixes #74

## Summary
Adds npm package distribution for one-liner installation via \`npx k8s-gpu-mcp-server\`.

## Changes
- \`npm/\` directory with package structure
- \`npm/scripts/postinstall.js\` for platform-specific binary download
- \`.github/workflows/npm-publish.yml\` for automated publishing
- Updated README with npm installation instructions

## Testing
- [ ] \`npm pack\` produces valid package
- [ ] Postinstall script downloads correct binary
- [ ] Binary executes correctly after download
- [ ] Workflow syntax validated

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-reviewed the code
- [ ] Documentation updated" \
  --label "kind/feature" \
  --label "ops/ci-cd" \
  --label "prio/p0-blocker" \
  --milestone "M4: Safety & Release"
```

---

## Wait for CI Checks

```bash
gh pr checks <PR-NUMBER> --watch
```

Expected jobs:
- [ ] lint
- [ ] test
- [ ] build
- [ ] DCO

---

## Review Process

### Copilot Review Comments

Address any Copilot suggestions, especially for:
- JavaScript best practices in postinstall.js
- Security considerations for binary download
- Error handling edge cases

### After Addressing Feedback

```bash
git add -A
git commit -s -S -m "fix(npm): address review feedback"
git push
```

---

## Merge the PR

```bash
# After CI passes and reviews addressed
gh pr merge <PR-NUMBER> --merge --delete-branch

# Return to main
git checkout main
git pull origin main
```

---

## Post-Merge: First Publish

After merging and creating a release:

1. **Create a GitHub Release** with tag `v0.1.0`
2. **Ensure binaries are uploaded** (from #76 workflow)
3. **Trigger npm publish:**
   ```bash
   gh workflow run npm-publish.yml -f version=0.1.0
   ```

4. **Verify on npm:**
   ```bash
   npm view k8s-gpu-mcp-server
   npx k8s-gpu-mcp-server@latest --help
   ```

---

## Acceptance Criteria

**Must Have:**
- [ ] npm package structure in `npm/` directory
- [ ] Postinstall script downloads correct platform binary
- [ ] Supports darwin-amd64, darwin-arm64, linux-amd64, linux-arm64
- [ ] GitHub Actions workflow for npm publish
- [ ] README updated with npm installation instructions

**Should Have:**
- [ ] Error handling for download failures
- [ ] Clear error messages with fallback instructions
- [ ] Package validates with `npm pack --dry-run`

**Nice to Have:**
- [ ] Progress indicator during download
- [ ] Checksum verification
- [ ] Fallback to building from source

---

## Related Issues

- **#76** - Multi-platform binary releases (dependency)
- **#75** - PyPI package distribution (similar pattern)
- **#86** - Release workflow with semantic versioning

---

## Quick Reference

### Key Commands

```bash
# Branch
git checkout -b feat/npm-package

# Test npm package
cd npm && npm pack

# Test postinstall
node scripts/postinstall.js

# Commit
git commit -s -S -m "feat(npm): add npm package distribution"

# Create PR
gh pr create --title "..." --label "kind/feature" --milestone "M4: Safety & Release"

# Merge
gh pr merge <PR#> --merge --delete-branch
```

### MCP Config for Testing

```json
{
  "mcpServers": {
    "k8s-gpu": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server@latest"]
    }
  }
}
```

---

**Reply "GO" when ready to start implementation.** 🚀

