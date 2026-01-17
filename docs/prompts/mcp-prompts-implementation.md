# MCP Prompts Implementation

> **Issue:** [#78 - feat: MCP Prompts support](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/78)
> **Related:** [#58 - Implement Prompts library with SRE SOPs](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/58)

---

## Autonomous Mode (Ralph Wiggum Pattern)

> **🔁 KEEP WORKING UNTIL DONE - READ THIS FIRST**
>
> This prompt is designed for iterative execution in Cursor. When you invoke
> this prompt with `@docs/prompts/mcp-prompts-implementation.md`, the agent
> MUST continue working until ALL tasks reach `[DONE]` status.
>
> **If tasks remain incomplete, re-invoke the prompt.**

### Progress Tracker

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0 | Create feature branch | `[DONE]` | `feat/mcp-prompts` |
| 1 | Create `pkg/prompts/` package with types | `[DONE]` | Core data structures |
| 2 | Implement prompt library with 3 built-in prompts | `[DONE]` | gpu-health-check, diagnose-xid-errors, gpu-triage |
| 3 | Add prompt handlers to MCP server | `[DONE]` | `prompts/list`, `prompts/get` |
| 4 | Unit tests for prompts package | `[DONE]` | |
| 5 | Integration tests for MCP prompts protocol | `[DONE]` | |
| 6 | Update documentation | `[DONE]` | mcp-usage.md |
| 7 | Run full test suite | `[DONE]` | `make all` |
| 8 | Create pull request | `[DONE]` | PR #140 |
| 9 | Wait for Copilot review | `[TODO]` | ⏳ 1-2 min |
| 10 | Address review comments | `[TODO]` | |
| 11 | **Merge after reviews** | `[WAIT]` | ⚠️ Requires human approval |

**Status Legend:** `[TODO]` | `[WIP]` | `[DONE]` | `[WAIT]` (human approval) | `[BLOCKED:reason]`

---

## Issue Reference

- **Issue:** [#78 - feat: MCP Prompts support](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/78)
- **Related:** [#58 - Implement Prompts library with SRE SOPs](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/58)
- **Priority:** P1-High
- **Labels:** `kind/feature`, `area/mcp-protocol`
- **Milestone:** M4: Safety & Release
- **Autonomous Mode:** ✅ Enabled

---

## Background

### What Are MCP Prompts?

MCP Prompts are reusable, parameterized workflow templates that AI assistants can invoke to execute guided diagnostic procedures. Unlike tools (which perform single actions), prompts orchestrate multi-step workflows and provide structured conversation templates.

**Current state:** The server only exposes tools (`get_gpu_inventory`, `get_gpu_health`, etc.). SREs must manually construct tool call sequences.

**Target state:** Provide pre-built diagnostic workflows as MCP Prompts that encode institutional knowledge.

### Reference Implementation

The `containers/kubernetes-mcp-server` supports prompts via TOML config:

```toml
[[prompts]]
name = "cluster-health-check"
description = "Check overall cluster health"

[[prompts.arguments]]
name = "namespace"
required = false

[[prompts.messages]]
role = "user"
content = "Check cluster health in {{namespace}}"
```

### Library Support

The `github.com/mark3labs/mcp-go` library provides:

```go
// Enable prompts capability
server.NewMCPServer("name", "version",
    server.WithPromptCapabilities(true),
)

// Register a prompt
p := mcp.NewPrompt("name",
    mcp.WithPromptDescription("description"),
    mcp.WithArgument("arg", mcp.ArgumentDescription("desc")),
)

server.AddPrompt(p, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
    // Build and return messages
    return mcp.NewGetPromptResult("description", messages), nil
})
```

---

## Objective

Implement MCP Prompts capability with 3 built-in GPU diagnostic workflows:

1. **`gpu-health-check`** - Comprehensive GPU health assessment
2. **`diagnose-xid-errors`** - XID error analysis workflow
3. **`gpu-triage`** - Standard SRE triage procedure (inventory → health → XID)

---

## Step 0: Create Feature Branch

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b feat/mcp-prompts
```

---

## Implementation Tasks

### Task 1: Create `pkg/prompts/` Package `[TODO]`

Create the core prompts package with type definitions.

**Files to create:**

#### `pkg/prompts/prompts.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package prompts provides MCP prompt definitions for GPU diagnostic workflows.
package prompts

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// PromptDef defines a prompt with its metadata and handler.
type PromptDef struct {
	// Name is the unique identifier for the prompt.
	Name string
	// Description is a human-readable description.
	Description string
	// Arguments defines the parameters the prompt accepts.
	Arguments []ArgumentDef
	// Template is the Go template for generating messages.
	Template string
}

// ArgumentDef defines a prompt argument.
type ArgumentDef struct {
	Name        string
	Description string
	Required    bool
	Default     string
}

// Handler is the signature for prompt handler functions.
type Handler func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// ToMCPPrompt converts a PromptDef to an mcp.Prompt.
func (p *PromptDef) ToMCPPrompt() mcp.Prompt {
	opts := []mcp.PromptOption{
		mcp.WithPromptDescription(p.Description),
	}

	for _, arg := range p.Arguments {
		argOpts := []mcp.ArgumentOption{
			mcp.ArgumentDescription(arg.Description),
		}
		if arg.Required {
			argOpts = append(argOpts, mcp.RequiredArgument())
		}
		opts = append(opts, mcp.WithArgument(arg.Name, argOpts...))
	}

	return mcp.NewPrompt(p.Name, opts...)
}

// RenderTemplate renders the prompt template with provided arguments.
func (p *PromptDef) RenderTemplate(args map[string]string) string {
	result := p.Template
	for key, value := range args {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	// Replace any remaining placeholders with defaults or empty
	for _, arg := range p.Arguments {
		placeholder := "{{" + arg.Name + "}}"
		if strings.Contains(result, placeholder) {
			if arg.Default != "" {
				result = strings.ReplaceAll(result, placeholder, arg.Default)
			} else {
				result = strings.ReplaceAll(result, placeholder, "")
			}
		}
	}
	return strings.TrimSpace(result)
}

// BuildHandler creates a standard handler for a PromptDef.
func (p *PromptDef) BuildHandler() Handler {
	return func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Extract arguments
		args := make(map[string]string)
		for key, value := range req.Params.Arguments {
			args[key] = value
		}

		// Validate required arguments
		for _, arg := range p.Arguments {
			if arg.Required {
				if _, ok := args[arg.Name]; !ok {
					return nil, fmt.Errorf("missing required argument: %s", arg.Name)
				}
			}
		}

		// Render template
		content := p.RenderTemplate(args)

		// Build messages
		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		}

		return mcp.NewGetPromptResult(p.Description, messages), nil
	}
}
```

**Acceptance criteria:**
- [ ] Package compiles without errors
- [ ] `PromptDef` type defined with all fields
- [ ] `ToMCPPrompt()` converts to mcp-go types correctly
- [ ] `RenderTemplate()` handles variable substitution
- [ ] `BuildHandler()` creates valid prompt handlers

---

### Task 2: Implement Prompt Library `[TODO]`

Create the built-in prompts library with 3 GPU diagnostic workflows.

**Files to create:**

#### `pkg/prompts/library.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package prompts

// Library contains all built-in prompt definitions.
var Library = []PromptDef{
	GPUHealthCheck,
	DiagnoseXIDErrors,
	GPUTriage,
}

// GPUHealthCheck provides comprehensive GPU health assessment.
var GPUHealthCheck = PromptDef{
	Name:        "gpu-health-check",
	Description: "Comprehensive GPU health assessment with recommendations",
	Arguments: []ArgumentDef{
		{
			Name:        "node",
			Description: "Optional: specific node name to check (default: all nodes)",
			Required:    false,
			Default:     "all nodes",
		},
	},
	Template: `## GPU Health Check Request

Please perform a comprehensive GPU health assessment on {{node}}.

### Workflow

1. **Inventory Check**
   - Use the \`get_gpu_inventory\` tool to list all GPUs
   - Note GPU models, memory sizes, and current utilization

2. **Health Assessment**
   - Use the \`get_gpu_health\` tool to get health scores
   - Check temperature, power, memory, and ECC status
   - Flag any GPUs with health score below 90

3. **Analysis**
   - Identify any thermal throttling (temperature > 80°C)
   - Check for memory pressure (> 90% utilization)
   - Review any health warnings or recommendations

### Expected Output

Provide a summary including:
- Total GPU count and models
- Overall cluster GPU health status
- Any GPUs requiring attention
- Specific recommendations for remediation`,
}

// DiagnoseXIDErrors provides XID error analysis workflow.
var DiagnoseXIDErrors = PromptDef{
	Name:        "diagnose-xid-errors",
	Description: "Analyze NVIDIA XID errors from kernel logs with remediation guidance",
	Arguments: []ArgumentDef{
		{
			Name:        "time_range",
			Description: "Time range to analyze (e.g., '1h', '24h', '7d')",
			Required:    false,
			Default:     "24h",
		},
	},
	Template: `## XID Error Diagnosis Request

Please analyze NVIDIA XID errors from the last {{time_range}}.

### Workflow

1. **Error Collection**
   - Use the \`analyze_xid_errors\` tool to parse kernel logs
   - Collect all XID errors with timestamps

2. **Error Classification**
   - Group errors by XID code
   - Identify severity levels (info, warning, critical)
   - Note affected GPU indices

3. **Root Cause Analysis**

   For each error type found, provide:
   - XID code meaning and typical causes
   - Whether it indicates hardware vs software issue
   - Correlation with workload patterns if visible

4. **Remediation Guidance**

   Based on findings, recommend:
   - Immediate actions (drain node, restart workload, etc.)
   - Monitoring changes needed
   - Escalation criteria (when to replace hardware)

### XID Quick Reference

| XID | Severity | Meaning |
|-----|----------|---------|
| 13  | Warning  | Graphics Engine Exception |
| 31  | Critical | GPU memory page fault |
| 43  | Warning  | GPU stopped processing |
| 45  | Critical | Preemptive cleanup |
| 48  | Critical | Double Bit ECC Error (DBE) |
| 63  | Warning  | ECC page retirement/row remap |
| 64  | Critical | ECC page retirement failed |
| 74  | Warning  | NVLink error |
| 79  | Warning  | GPU fallen off bus |
| 94  | Critical | Contained ECC error |
| 95  | Critical | Uncontained ECC error |

### Expected Output

Provide:
- Summary of errors found (count by XID, by GPU)
- Timeline of error occurrence
- Risk assessment (can workloads continue safely?)
- Concrete next steps`,
}

// GPUTriage provides standard SRE triage procedure.
var GPUTriage = PromptDef{
	Name:        "gpu-triage",
	Description: "Standard GPU triage workflow: inventory → health → XID analysis",
	Arguments: []ArgumentDef{
		{
			Name:        "node",
			Description: "Node name to triage (required for targeted diagnosis)",
			Required:    false,
			Default:     "cluster-wide",
		},
		{
			Name:        "incident_id",
			Description: "Optional incident/ticket ID for tracking",
			Required:    false,
			Default:     "",
		},
	},
	Template: `## GPU Triage Report {{incident_id}}

Performing standard GPU triage for: {{node}}

### Step 1: Hardware Inventory

Use \`get_gpu_inventory\` to collect:
- GPU count and models
- Memory capacity and current usage
- Temperature and power readings
- Current utilization

### Step 2: Health Assessment

Use \`get_gpu_health\` to evaluate:
- Health scores for each GPU
- Temperature status (normal/warning/critical)
- Power consumption vs limits
- ECC error counts (correctable/uncorrectable)
- Memory health status

### Step 3: Error Analysis

Use \`analyze_xid_errors\` to check:
- Recent XID errors in kernel logs
- Error frequency and patterns
- Correlation with specific GPUs

### Step 4: Workload Correlation (if K8s available)

Use \`get_pod_gpu_allocation\` to identify:
- Pods currently using GPUs
- Resource requests vs actual usage
- Any pods in error state

### Triage Decision Matrix

| Condition | Action |
|-----------|--------|
| All GPUs healthy, no XID errors | ✅ No action needed |
| Health score < 90, no critical errors | ⚠️ Monitor closely |
| XID 48/64/95 detected | 🔴 Drain node, escalate |
| GPU fallen off bus (XID 79) | 🔴 Immediate node restart |
| Thermal throttling (>83°C) | ⚠️ Check cooling, reduce load |
| Memory errors accumulating | ⚠️ Schedule maintenance |

### Expected Output

Provide a triage report including:
1. **Summary:** One-line status (healthy/degraded/critical)
2. **Findings:** Key observations from each step
3. **Risk Assessment:** Can workloads continue safely?
4. **Recommendations:** Prioritized action items
5. **Escalation:** If needed, who to contact and why`,
}

// GetPromptByName returns a prompt definition by name.
func GetPromptByName(name string) (*PromptDef, bool) {
	for i := range Library {
		if Library[i].Name == name {
			return &Library[i], true
		}
	}
	return nil, false
}

// GetAllPromptNames returns the names of all available prompts.
func GetAllPromptNames() []string {
	names := make([]string, len(Library))
	for i, p := range Library {
		names[i] = p.Name
	}
	return names
}
```

**Acceptance criteria:**
- [ ] 3 prompts defined: `gpu-health-check`, `diagnose-xid-errors`, `gpu-triage`
- [ ] Each prompt has clear workflow steps
- [ ] Arguments are properly defined with defaults
- [ ] `GetPromptByName()` works correctly
- [ ] Templates reference actual tool names

---

### Task 3: Add Prompt Handlers to MCP Server `[TODO]`

Integrate prompts into the MCP server.

**Files to modify:**

#### `pkg/mcp/server.go`

Add prompt registration in the `New()` function:

```go
import (
	// ... existing imports
	"github.com/ArangoGutierrez/k8s-gpu-mcp-server/pkg/prompts"
)

func New(cfg Config) (*Server, error) {
	// ... existing code ...

	// Create MCP server with prompt capabilities
	mcpServer := server.NewMCPServer(
		"k8s-gpu-mcp-server",
		cfg.Version,
		server.WithPromptCapabilities(true), // ADD THIS
	)

	// ... existing tool registration ...

	// Register prompts
	for _, promptDef := range prompts.Library {
		p := promptDef.ToMCPPrompt()
		handler := promptDef.BuildHandler()
		mcpServer.AddPrompt(p, handler)
	}

	klog.InfoS("MCP server initialized",
		"mode", cfg.Mode,
		"gateway", cfg.GatewayMode,
		"tools", []string{...},
		"prompts", prompts.GetAllPromptNames(), // ADD THIS
		"version", cfg.Version,
		"commit", cfg.GitCommit)

	// ... rest of function ...
}
```

**Acceptance criteria:**
- [ ] Server creates with `server.WithPromptCapabilities(true)`
- [ ] All prompts from library are registered
- [ ] Log output includes prompt names
- [ ] `prompts/list` returns all prompts
- [ ] `prompts/get` returns rendered prompt content

---

### Task 4: Unit Tests for Prompts Package `[TODO]`

**Files to create:**

#### `pkg/prompts/prompts_test.go`

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package prompts

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestPromptDef_ToMCPPrompt(t *testing.T) {
	def := PromptDef{
		Name:        "test-prompt",
		Description: "Test prompt description",
		Arguments: []ArgumentDef{
			{Name: "arg1", Description: "First arg", Required: true},
			{Name: "arg2", Description: "Second arg", Required: false, Default: "default"},
		},
		Template: "Test {{arg1}} and {{arg2}}",
	}

	p := def.ToMCPPrompt()

	if p.Name != "test-prompt" {
		t.Errorf("expected name 'test-prompt', got %q", p.Name)
	}
	if p.Description != "Test prompt description" {
		t.Errorf("expected description, got %q", p.Description)
	}
	if len(p.Arguments) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(p.Arguments))
	}
}

func TestPromptDef_RenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		def      PromptDef
		args     map[string]string
		expected string
	}{
		{
			name: "basic substitution",
			def: PromptDef{
				Template: "Hello {{name}}!",
			},
			args:     map[string]string{"name": "World"},
			expected: "Hello World!",
		},
		{
			name: "multiple substitutions",
			def: PromptDef{
				Template: "Check {{node}} for {{issue}}",
			},
			args:     map[string]string{"node": "gpu-1", "issue": "errors"},
			expected: "Check gpu-1 for errors",
		},
		{
			name: "default value",
			def: PromptDef{
				Arguments: []ArgumentDef{
					{Name: "node", Default: "all nodes"},
				},
				Template: "Check {{node}}",
			},
			args:     map[string]string{},
			expected: "Check all nodes",
		},
		{
			name: "missing arg without default",
			def: PromptDef{
				Template: "Check {{node}}",
			},
			args:     map[string]string{},
			expected: "Check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.def.RenderTemplate(tt.args)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPromptDef_BuildHandler(t *testing.T) {
	def := PromptDef{
		Name:        "test",
		Description: "Test prompt",
		Arguments: []ArgumentDef{
			{Name: "node", Description: "Node name", Required: false, Default: "all"},
		},
		Template: "Check {{node}} GPUs",
	}

	handler := def.BuildHandler()

	t.Run("with argument", func(t *testing.T) {
		req := mcp.GetPromptRequest{}
		req.Params.Name = "test"
		req.Params.Arguments = map[string]string{"node": "gpu-worker-1"}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if len(result.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(result.Messages))
		}
	})

	t.Run("without argument uses default", func(t *testing.T) {
		req := mcp.GetPromptRequest{}
		req.Params.Name = "test"
		req.Params.Arguments = map[string]string{}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
	})
}

func TestPromptDef_BuildHandler_RequiredArg(t *testing.T) {
	def := PromptDef{
		Name:        "test",
		Description: "Test prompt",
		Arguments: []ArgumentDef{
			{Name: "node", Description: "Node name", Required: true},
		},
		Template: "Check {{node}}",
	}

	handler := def.BuildHandler()

	req := mcp.GetPromptRequest{}
	req.Params.Name = "test"
	req.Params.Arguments = map[string]string{} // Missing required arg

	_, err := handler(context.Background(), req)
	if err == nil {
		t.Error("expected error for missing required argument")
	}
}

func TestGetPromptByName(t *testing.T) {
	tests := []struct {
		name     string
		lookup   string
		expected bool
	}{
		{"existing prompt", "gpu-health-check", true},
		{"another existing", "diagnose-xid-errors", true},
		{"third existing", "gpu-triage", true},
		{"non-existing", "not-a-prompt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, found := GetPromptByName(tt.lookup)
			if found != tt.expected {
				t.Errorf("GetPromptByName(%q) found=%v, expected=%v", tt.lookup, found, tt.expected)
			}
			if found && p.Name != tt.lookup {
				t.Errorf("expected name %q, got %q", tt.lookup, p.Name)
			}
		})
	}
}

func TestGetAllPromptNames(t *testing.T) {
	names := GetAllPromptNames()

	if len(names) != 3 {
		t.Errorf("expected 3 prompts, got %d", len(names))
	}

	expected := map[string]bool{
		"gpu-health-check":    true,
		"diagnose-xid-errors": true,
		"gpu-triage":          true,
	}

	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected prompt name: %q", name)
		}
	}
}

func TestLibraryPrompts(t *testing.T) {
	// Ensure all library prompts are valid
	for _, p := range Library {
		t.Run(p.Name, func(t *testing.T) {
			if p.Name == "" {
				t.Error("prompt name is empty")
			}
			if p.Description == "" {
				t.Error("prompt description is empty")
			}
			if p.Template == "" {
				t.Error("prompt template is empty")
			}

			// Test that ToMCPPrompt doesn't panic
			mcpPrompt := p.ToMCPPrompt()
			if mcpPrompt.Name != p.Name {
				t.Errorf("MCP prompt name mismatch")
			}

			// Test that handler can be built
			handler := p.BuildHandler()
			if handler == nil {
				t.Error("handler is nil")
			}
		})
	}
}
```

**Acceptance criteria:**
- [ ] All tests pass
- [ ] Template rendering tested with various inputs
- [ ] Handler behavior tested (with/without args)
- [ ] Required argument validation tested
- [ ] Library prompts validated

---

### Task 5: MCP Integration Tests `[TODO]`

**Files to modify:**

#### `pkg/mcp/server_test.go`

Add tests for prompts functionality:

```go
func TestServer_PromptsCapability(t *testing.T) {
	// Create server with mock NVML
	mockNVML := nvml.NewMock()
	cfg := Config{
		Mode:       "read-only",
		Version:    "test",
		NVMLClient: mockNVML,
		Transport:  TransportStdio,
	}

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Verify prompts are registered
	// This is a smoke test - full protocol tests are separate
	if s.mcpServer == nil {
		t.Fatal("MCP server is nil")
	}
}

func TestServer_PromptsList(t *testing.T) {
	// Test prompts/list JSON-RPC method
	// This requires testing the full MCP protocol flow
	t.Skip("requires full protocol test harness")
}
```

**Acceptance criteria:**
- [ ] Server initializes with prompts capability
- [ ] Prompts are registered correctly
- [ ] Integration with mcp-go library verified

---

### Task 6: Update Documentation `[TODO]`

**Files to modify:**

#### `docs/mcp-usage.md`

Add a new section for prompts after the "Available Tools" section:

```markdown
## Available Prompts

MCP Prompts provide guided diagnostic workflows. Unlike tools (which perform
single actions), prompts orchestrate multi-step workflows with contextual
instructions for AI assistants.

### Listing Prompts

```bash
echo '{"jsonrpc":"2.0","method":"prompts/list","params":{},"id":1}' | ./bin/agent
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "prompts": [
      {
        "name": "gpu-health-check",
        "description": "Comprehensive GPU health assessment with recommendations",
        "arguments": [
          {"name": "node", "description": "Optional: specific node name", "required": false}
        ]
      },
      {
        "name": "diagnose-xid-errors",
        "description": "Analyze NVIDIA XID errors from kernel logs",
        "arguments": [
          {"name": "time_range", "description": "Time range (e.g., '24h')", "required": false}
        ]
      },
      {
        "name": "gpu-triage",
        "description": "Standard GPU triage workflow: inventory → health → XID analysis",
        "arguments": [
          {"name": "node", "description": "Node name to triage", "required": false},
          {"name": "incident_id", "description": "Optional incident ID", "required": false}
        ]
      }
    ]
  }
}
```

### Getting a Prompt

```bash
echo '{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"gpu-health-check","arguments":{"node":"gpu-worker-1"}},"id":2}' | ./bin/agent
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "description": "Comprehensive GPU health assessment with recommendations",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "## GPU Health Check Request\n\nPlease perform a comprehensive GPU health assessment on gpu-worker-1.\n\n### Workflow\n..."
        }
      }
    ]
  }
}
```

### gpu-health-check

**Purpose:** Comprehensive GPU health assessment

**Arguments:**
- `node` (optional): Specific node name to check. Default: "all nodes"

**Workflow:**
1. Inventory check using `get_gpu_inventory`
2. Health assessment using `get_gpu_health`
3. Analysis of thermal and memory status
4. Summary with recommendations

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "gpu-health-check",
    "arguments": {"node": "gpu-node-5"}
  },
  "id": 1
}
```

### diagnose-xid-errors

**Purpose:** Analyze NVIDIA XID errors with remediation guidance

**Arguments:**
- `time_range` (optional): Time range to analyze. Default: "24h"

**Workflow:**
1. Error collection using `analyze_xid_errors`
2. Classification by XID code and severity
3. Root cause analysis
4. Remediation guidance

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "diagnose-xid-errors",
    "arguments": {"time_range": "7d"}
  },
  "id": 1
}
```

### gpu-triage

**Purpose:** Standard SRE triage procedure

**Arguments:**
- `node` (optional): Node name to triage. Default: "cluster-wide"
- `incident_id` (optional): Incident/ticket ID for tracking

**Workflow:**
1. Hardware inventory
2. Health assessment
3. Error analysis
4. Workload correlation (if K8s available)
5. Triage decision based on findings

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "gpu-triage",
    "arguments": {
      "node": "gpu-worker-42",
      "incident_id": "INC-12345"
    }
  },
  "id": 1
}
```

### Using Prompts with Claude Desktop

When prompts are available, Claude can execute guided workflows:

```
You: "Run the GPU triage workflow for node gpu-worker-5"

Claude: [Calls prompts/get for gpu-triage]
        [Follows the workflow steps]
        [Calls get_gpu_inventory, get_gpu_health, analyze_xid_errors]
        
        "## GPU Triage Report
         
         Node: gpu-worker-5
         Status: ⚠️ Degraded
         
         ### Findings:
         - GPU 0: Healthy (score: 98)
         - GPU 1: Warning (score: 75) - elevated temperature
         
         ### Recommendations:
         1. Monitor GPU 1 temperature
         2. Check cooling system
         ..."
```
```

**Acceptance criteria:**
- [ ] Prompts section added to mcp-usage.md
- [ ] All 3 prompts documented
- [ ] JSON-RPC examples provided
- [ ] Claude Desktop usage example included

---

### Task 7: Run Full Test Suite `[TODO]`

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Full suite
make all
```

**Acceptance criteria:**
- [ ] `make fmt` passes
- [ ] `make lint` passes
- [ ] `make test` passes (including new tests)
- [ ] `make all` succeeds

---

## Testing Requirements

### Unit Testing

```bash
# Run prompts package tests
go test -v ./pkg/prompts/...

# Run MCP server tests
go test -v ./pkg/mcp/...

# Run with race detector
go test -race ./pkg/prompts/... ./pkg/mcp/...
```

### Manual Testing

```bash
# Build agent
make agent

# Test prompts/list
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"prompts/list","params":{},"id":1}' | ./bin/agent --nvml-mode=mock 2>/dev/null

# Test prompts/get
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":0}
{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"gpu-health-check","arguments":{"node":"test-node"}},"id":1}' | ./bin/agent --nvml-mode=mock 2>/dev/null
```

---

## Pre-Commit Checklist

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass
- [ ] `go test ./... -race` - No race conditions
- [ ] Documentation updated

---

## Commit Strategy

Use atomic commits for each logical change:

```bash
git commit -s -S -m "feat(prompts): add prompts package with types"
git commit -s -S -m "feat(prompts): implement 3 built-in GPU diagnostic prompts"
git commit -s -S -m "feat(mcp): register prompts with MCP server"
git commit -s -S -m "test(prompts): add unit tests for prompts package"
git commit -s -S -m "docs(mcp): add prompts documentation to mcp-usage.md"
```

---

## Create Pull Request

```bash
gh pr create \
  --title "feat(mcp): implement MCP Prompts for GPU diagnostics" \
  --body "Fixes #78

## Summary
Implements MCP Prompts capability with 3 built-in GPU diagnostic workflows.

## Changes
- Add \`pkg/prompts/\` package with prompt types and library
- Implement 3 prompts: gpu-health-check, diagnose-xid-errors, gpu-triage
- Register prompts with MCP server using \`server.WithPromptCapabilities(true)\`
- Add comprehensive unit tests

## Prompts Added
| Prompt | Description |
|--------|-------------|
| \`gpu-health-check\` | Comprehensive GPU health assessment |
| \`diagnose-xid-errors\` | XID error analysis with remediation |
| \`gpu-triage\` | Standard SRE triage workflow |

## Testing
- [x] Unit tests pass
- [x] Manual testing with \`prompts/list\` and \`prompts/get\`
- [x] \`make all\` succeeds

## Checklist
- [x] Code follows project style guidelines
- [x] Self-reviewed the code
- [x] Documentation updated" \
  --label "kind/feature" \
  --label "area/mcp-protocol" \
  --milestone "M4: Safety & Release"
```

---

## Related Files

- `pkg/mcp/server.go` - MCP server implementation
- `pkg/tools/*.go` - Tool implementations (referenced by prompts)
- `docs/mcp-usage.md` - MCP usage documentation
- `.cursor/rules/01-mcp-server.mdc` - MCP development standards

---

## Notes

### Future Enhancements (Out of Scope)

These items are tracked separately and not part of this PR:

1. **Config-based prompts** (#80) - Define custom prompts in TOML config
2. **SIGHUP hot-reload** (#81) - Reload prompts without restart
3. **Additional prompts** (#58) - More SRE workflows:
   - `investigate_thermal_throttling`
   - `investigate_memory_errors`
   - `pre_maintenance_checklist`
   - `post_incident_review`

### Dependencies

- `github.com/mark3labs/mcp-go` - MCP library (already in go.mod)
- No new dependencies required

---

**Reply "GO" when ready to start implementation.** 🚀
