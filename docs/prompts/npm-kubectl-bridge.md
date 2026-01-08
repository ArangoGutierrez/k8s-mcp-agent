# NPM Package: kubectl Port-Forward Bridge

## Issue Reference

- **Issue:** [#97 - NPM package should abstract kubectl port-forward](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/97)
- **Priority:** P1-High
- **Labels:** enhancement, area/npm-package
- **Milestone:** M3: Kubernetes Integration

## Background

Current UX for connecting Cursor to a remote k8s-gpu-mcp-server gateway requires
manual steps:

1. Run `kubectl port-forward` in a separate terminal
2. Edit `~/.cursor/mcp.json` with the local URL
3. Keep the port-forward running

This is clunky compared to other MCP servers like `kubernetes-mcp-server` which
handle connection setup transparently.

**Desired UX:**

```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server"]
    }
  }
}
```

The NPM package should:
1. Detect kubeconfig context
2. Auto-discover the gateway service in the cluster
3. Establish port-forward internally
4. Expose MCP over stdio (wrapping HTTP transport)

---

## Objective

Create a Node.js wrapper that bridges stdio ↔ HTTP, automatically managing
kubectl port-forward to the gateway service.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/npm-kubectl-bridge
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Cursor / Claude                              │
│                              ↓ stdio                                 │
├─────────────────────────────────────────────────────────────────────┤
│                    npm/bin/k8s-gpu-mcp-server                        │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Node.js Bridge                            │    │
│  │  1. Spawn kubectl port-forward (background)                  │    │
│  │  2. Wait for port to be ready                                │    │
│  │  3. Read JSON-RPC from stdin                                 │    │
│  │  4. POST to http://localhost:<port>/mcp                      │    │
│  │  5. Write response to stdout                                 │    │
│  │  6. Cleanup port-forward on exit                             │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              ↓ HTTP                                  │
├─────────────────────────────────────────────────────────────────────┤
│              kubectl port-forward (child process)                    │
│                              ↓                                       │
├─────────────────────────────────────────────────────────────────────┤
│                    Kubernetes Cluster                                │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  gpu-mcp-gateway Service (ClusterIP:8080)                    │    │
│  │       ↓                                                       │    │
│  │  Gateway Pod → DaemonSet Agents on GPU nodes                 │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Implementation Tasks

### Task 1: Create the Bridge Script

Create `npm/bin/k8s-gpu-mcp-server` as a Node.js script.

**File:** `npm/bin/k8s-gpu-mcp-server`

```javascript
#!/usr/bin/env node

const { spawn } = require('child_process');
const http = require('http');
const readline = require('readline');

// Configuration
const NAMESPACE = process.env.K8S_GPU_MCP_NAMESPACE || 'gpu-diagnostics';
const SERVICE = process.env.K8S_GPU_MCP_SERVICE || 'gpu-mcp-gateway';
const SERVICE_PORT = process.env.K8S_GPU_MCP_SERVICE_PORT || '8080';
const LOCAL_PORT = process.env.K8S_GPU_MCP_LOCAL_PORT || '0'; // 0 = auto-select
const KUBECONFIG = process.env.KUBECONFIG || '';
const CONTEXT = process.env.K8S_GPU_MCP_CONTEXT || '';

class KubectlBridge {
  constructor() {
    this.portForward = null;
    this.localPort = null;
    this.ready = false;
  }

  async start() {
    // Start port-forward
    this.localPort = await this.startPortForward();
    this.ready = true;

    // Set up stdio bridge
    await this.bridgeStdio();
  }

  startPortForward() {
    return new Promise((resolve, reject) => {
      const args = ['port-forward', '-n', NAMESPACE, `svc/${SERVICE}`];
      
      // Use random local port if not specified
      const localPort = LOCAL_PORT === '0' ? '' : LOCAL_PORT;
      args.push(`${localPort}:${SERVICE_PORT}`);

      // Add context if specified
      if (CONTEXT) {
        args.unshift('--context', CONTEXT);
      }

      // Add kubeconfig if specified
      if (KUBECONFIG) {
        args.unshift('--kubeconfig', KUBECONFIG);
      }

      console.error(`[k8s-gpu-mcp] Starting: kubectl ${args.join(' ')}`);

      this.portForward = spawn('kubectl', args, {
        stdio: ['ignore', 'pipe', 'pipe']
      });

      let resolved = false;

      // Parse stdout for the assigned port
      this.portForward.stdout.on('data', (data) => {
        const output = data.toString();
        console.error(`[k8s-gpu-mcp] ${output.trim()}`);

        // Parse: "Forwarding from 127.0.0.1:XXXXX -> 8080"
        const match = output.match(/Forwarding from 127\.0\.0\.1:(\d+)/);
        if (match && !resolved) {
          resolved = true;
          resolve(parseInt(match[1], 10));
        }
      });

      this.portForward.stderr.on('data', (data) => {
        console.error(`[k8s-gpu-mcp] stderr: ${data.toString().trim()}`);
      });

      this.portForward.on('error', (err) => {
        if (!resolved) {
          reject(new Error(`Failed to start kubectl: ${err.message}`));
        }
      });

      this.portForward.on('exit', (code) => {
        if (!resolved) {
          reject(new Error(`kubectl exited with code ${code}`));
        }
        console.error(`[k8s-gpu-mcp] Port-forward exited with code ${code}`);
        process.exit(code || 1);
      });

      // Timeout after 30 seconds
      setTimeout(() => {
        if (!resolved) {
          this.cleanup();
          reject(new Error('Timeout waiting for port-forward'));
        }
      }, 30000);
    });
  }

  async bridgeStdio() {
    const rl = readline.createInterface({
      input: process.stdin,
      terminal: false
    });

    // Handle each line as a JSON-RPC message
    for await (const line of rl) {
      if (!line.trim()) continue;

      try {
        const response = await this.sendRequest(line);
        console.log(response);
      } catch (err) {
        // Return JSON-RPC error
        const errorResponse = {
          jsonrpc: '2.0',
          id: null,
          error: {
            code: -32603,
            message: err.message
          }
        };
        console.log(JSON.stringify(errorResponse));
      }
    }

    // Stdin closed, cleanup
    this.cleanup();
  }

  sendRequest(jsonLine) {
    return new Promise((resolve, reject) => {
      const postData = jsonLine;

      const options = {
        hostname: '127.0.0.1',
        port: this.localPort,
        path: '/mcp',
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(postData)
        }
      };

      const req = http.request(options, (res) => {
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => {
          if (res.statusCode !== 200) {
            reject(new Error(`HTTP ${res.statusCode}: ${data}`));
          } else {
            resolve(data);
          }
        });
      });

      req.on('error', reject);
      req.write(postData);
      req.end();
    });
  }

  cleanup() {
    if (this.portForward) {
      console.error('[k8s-gpu-mcp] Cleaning up port-forward...');
      this.portForward.kill('SIGTERM');
      this.portForward = null;
    }
  }
}

// Handle signals
const bridge = new KubectlBridge();

process.on('SIGINT', () => bridge.cleanup());
process.on('SIGTERM', () => bridge.cleanup());
process.on('exit', () => bridge.cleanup());

// Start the bridge
bridge.start().catch((err) => {
  console.error(`[k8s-gpu-mcp] Error: ${err.message}`);
  process.exit(1);
});
```

**Acceptance criteria:**
- [ ] Script spawns kubectl port-forward
- [ ] Parses assigned local port from stdout
- [ ] Bridges stdin → HTTP POST → stdout
- [ ] Cleans up port-forward on exit
- [ ] Handles errors gracefully

> 💡 **Commit:** `feat(npm): add kubectl port-forward bridge script`

---

### Task 2: Update package.json

Update npm configuration to use the bridge script.

**File:** `npm/package.json`

```json
{
  "name": "k8s-gpu-mcp-server",
  "version": "0.2.0",
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
  "files": [
    "bin/",
    "README.md"
  ],
  "engines": {
    "node": ">=18.0.0"
  }
}
```

**Changes:**
- Remove `scripts/postinstall.js` (no binary download needed)
- Remove platform-specific fields (pure Node.js now)
- Bump version to 0.2.0

**Acceptance criteria:**
- [ ] package.json updated
- [ ] postinstall.js removed or repurposed
- [ ] Version bumped

> 💡 **Commit:** `feat(npm): update package.json for bridge mode`

---

### Task 3: Add Service Discovery

Add auto-discovery of gateway service if not explicitly configured.

**File:** `npm/bin/k8s-gpu-mcp-server` (add to class)

```javascript
async discoverGateway() {
  return new Promise((resolve, reject) => {
    const args = ['get', 'svc', '-A', '-l', 'app.kubernetes.io/name=k8s-gpu-mcp-server',
      '-o', 'jsonpath={.items[0].metadata.namespace}/{.items[0].metadata.name}'];
    
    if (CONTEXT) {
      args.unshift('--context', CONTEXT);
    }

    const kubectl = spawn('kubectl', args, { stdio: ['ignore', 'pipe', 'pipe'] });
    
    let stdout = '';
    kubectl.stdout.on('data', (data) => { stdout += data; });
    
    kubectl.on('exit', (code) => {
      if (code !== 0 || !stdout.trim()) {
        // Fall back to defaults
        resolve({ namespace: NAMESPACE, service: SERVICE });
      } else {
        const [namespace, service] = stdout.trim().split('/');
        console.error(`[k8s-gpu-mcp] Discovered gateway: ${namespace}/${service}`);
        resolve({ namespace, service });
      }
    });
  });
}
```

**Acceptance criteria:**
- [ ] Auto-discovers gateway service by label
- [ ] Falls back to environment variables if not found
- [ ] Logs discovery result

> 💡 **Commit:** `feat(npm): add gateway service discovery`

---

### Task 4: Update README

Update npm README with new usage instructions.

**File:** `npm/README.md`

```markdown
# k8s-gpu-mcp-server

MCP server for NVIDIA GPU diagnostics in Kubernetes clusters.

## Prerequisites

- Node.js 18+
- `kubectl` installed and configured with cluster access
- k8s-gpu-mcp-server gateway deployed in cluster

## Installation

```bash
npm install -g k8s-gpu-mcp-server
```

## Usage with Cursor

Add to `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "k8s-gpu-mcp": {
      "command": "npx",
      "args": ["-y", "k8s-gpu-mcp-server"]
    }
  }
}
```

Restart Cursor and the GPU diagnostics tools will be available.

## Configuration

Configure via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_GPU_MCP_NAMESPACE` | Gateway namespace | `gpu-diagnostics` |
| `K8S_GPU_MCP_SERVICE` | Gateway service name | `gpu-mcp-gateway` |
| `K8S_GPU_MCP_CONTEXT` | Kubernetes context | Current context |
| `KUBECONFIG` | Path to kubeconfig | `~/.kube/config` |

## How It Works

1. Spawns `kubectl port-forward` to the gateway service
2. Bridges stdin/stdout to HTTP requests
3. Cleans up port-forward on exit

## Manual Testing

```bash
# Test the bridge
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | npx k8s-gpu-mcp-server
```

## Troubleshooting

### "kubectl not found"

Ensure kubectl is installed and in your PATH.

### "No gateway service found"

Deploy the gateway:

```bash
helm install gpu-mcp oci://ghcr.io/arangogutierrez/charts/k8s-gpu-mcp-server \
  --set gateway.enabled=true
```

### Connection timeout

Check cluster connectivity:

```bash
kubectl get svc -n gpu-diagnostics
```
```

**Acceptance criteria:**
- [ ] README explains new usage
- [ ] Environment variables documented
- [ ] Troubleshooting section added

> 💡 **Commit:** `docs(npm): update README for kubectl bridge`

---

### Task 5: Remove Old Binary Download Logic

Remove or deprecate the postinstall script since we no longer download binaries.

**Options:**
1. Delete `npm/scripts/postinstall.js`
2. Keep it but make it optional (for users who want the Go binary)

**Recommended:** Delete it for simplicity.

```bash
rm npm/scripts/postinstall.js
rmdir npm/scripts
```

Update package.json to remove the `postinstall` script.

**Acceptance criteria:**
- [ ] postinstall.js removed
- [ ] scripts directory removed if empty
- [ ] package.json updated

> 💡 **Commit:** `chore(npm): remove binary download postinstall`

---

## Testing Requirements

### Local Testing

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server/npm

# Test the script directly
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}' | node bin/k8s-gpu-mcp-server

# Test with npx (after npm link)
npm link
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | npx k8s-gpu-mcp-server
```

### Cluster Testing

```bash
# Ensure gateway is deployed
kubectl get svc -n gpu-diagnostics gpu-mcp-gateway

# Test bridge
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}
{"jsonrpc":"2.0","method":"tools/list","id":2}' | node npm/bin/k8s-gpu-mcp-server
```

---

## Pre-Commit Checklist

```bash
# Since this is JavaScript, no Go checks needed
cd npm
node --check bin/k8s-gpu-mcp-server  # Syntax check

# Test locally
npm link && npx k8s-gpu-mcp-server --help || true
```

---

## Commit and Push

### Commit Sequence

```bash
git add npm/bin/k8s-gpu-mcp-server
git commit -s -S -m "feat(npm): add kubectl port-forward bridge script"

git add npm/package.json
git commit -s -S -m "feat(npm): update package.json for bridge mode"

git add npm/bin/k8s-gpu-mcp-server  # if modified for discovery
git commit -s -S -m "feat(npm): add gateway service discovery"

git add npm/README.md
git commit -s -S -m "docs(npm): update README for kubectl bridge"

git rm npm/scripts/postinstall.js
git commit -s -S -m "chore(npm): remove binary download postinstall"
```

### Push

```bash
git push -u origin feat/npm-kubectl-bridge
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(npm): abstract kubectl port-forward for remote cluster access" \
  --body "Fixes #97

## Summary
Replace binary download with a Node.js bridge that handles kubectl port-forward
internally, providing seamless cluster connectivity.

## Changes
- Add kubectl port-forward bridge script
- Auto-discover gateway service by label
- Bridge stdio ↔ HTTP for MCP protocol
- Clean up port-forward on exit
- Update README with new usage

## Testing
- [ ] Local testing with npm link
- [ ] Cluster testing with real gateway
- [ ] Cursor integration testing

## Checklist
- [ ] Bridge spawns port-forward correctly
- [ ] JSON-RPC messages flow through
- [ ] Cleanup on exit works
- [ ] Error handling is graceful" \
  --label "enhancement" \
  --label "area/npm-package"
```

---

## Related Files

- `npm/bin/k8s-gpu-mcp-server` - Main bridge script
- `npm/package.json` - Package configuration
- `npm/README.md` - User documentation

## Notes

- The bridge requires `kubectl` to be installed and configured
- Users must have cluster access to the gateway namespace
- The gateway must be deployed with HTTP transport enabled
- No Go binary is downloaded - pure Node.js implementation

---

**Reply "GO" when ready to start implementation.** 🚀
