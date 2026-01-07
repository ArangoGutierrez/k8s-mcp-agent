#!/usr/bin/env bash
set -euo pipefail

# NVIDIA K8s MCP Agent - GitHub Repository Initialization Script
# This script sets up labels, milestones, and initial issues for project governance.

REPO="ArangoGutierrez/k8s-gpu-mcp-server"

echo "🚀 Initializing GitHub repository: ${REPO}"
echo ""

# ============================================================================
# STEP 1: DELETE DEFAULT LABELS
# ============================================================================
echo "📦 Cleaning up default GitHub labels..."

DEFAULT_LABELS=(
  "bug"
  "documentation"
  "duplicate"
  "enhancement"
  "good first issue"
  "help wanted"
  "invalid"
  "question"
  "wontfix"
)

for label in "${DEFAULT_LABELS[@]}"; do
  if gh label list --repo "${REPO}" --json name --jq '.[].name' | grep -q "^${label}$"; then
    gh label delete "${label}" --repo "${REPO}" --yes 2>/dev/null || true
    echo "  ✓ Deleted: ${label}"
  fi
done

echo ""

# ============================================================================
# STEP 2: CREATE CUSTOM LABELS
# ============================================================================
echo "🏷️  Creating custom label taxonomy..."

# Priority Labels
gh label create "prio/p0-blocker" --color "b60205" --description "Critical blocker - immediate attention required" --repo "${REPO}" --force
gh label create "prio/p1-high" --color "d93f0b" --description "High priority - address soon" --repo "${REPO}" --force

# Area Labels
gh label create "area/mcp-protocol" --color "0e8a16" --description "JSON-RPC, Transport, Schema" --repo "${REPO}" --force
gh label create "area/nvml-binding" --color "0052cc" --description "Hardware interaction, CGO, go-nvml" --repo "${REPO}" --force
gh label create "area/k8s-ephemeral" --color "fbca04" --description "kubectl debug, Stdio tunneling" --repo "${REPO}" --force

# Kind Labels
gh label create "kind/feature" --color "a2eeef" --description "New feature or enhancement" --repo "${REPO}" --force
gh label create "kind/tech-debt" --color "d876e3" --description "Technical debt and refactoring" --repo "${REPO}" --force

# Ops Labels
gh label create "ops/security" --color "5319e7" --description "Capabilities, Safety layers" --repo "${REPO}" --force

echo "  ✓ Created 8 custom labels"
echo ""

# ============================================================================
# STEP 3: CREATE MILESTONES (Using GitHub API)
# ============================================================================
echo "🎯 Creating milestones..."

# Calculate due dates (relative to today, ISO 8601 format with time)
M1_DATE=$(date -v+1w -u +"%Y-%m-%dT23:59:59Z")
M2_DATE=$(date -v+2w -u +"%Y-%m-%dT23:59:59Z")
M3_DATE=$(date -v+3w -u +"%Y-%m-%dT23:59:59Z")
M4_DATE=$(date -v+4w -u +"%Y-%m-%dT23:59:59Z")

# M1: Foundation & API
gh api repos/"${REPO}"/milestones -f title="M1: Foundation & API" \
  -f state="open" \
  -f description="Repo scaffolding, MCP Stdio transport working, Mock NVML" \
  -f due_on="${M1_DATE}" 2>/dev/null && echo "  ✓ Created M1" || echo "  ⚠️  M1 already exists"

# M2: Hardware Introspection
gh api repos/"${REPO}"/milestones -f title="M2: Hardware Introspection" \
  -f state="open" \
  -f description="Real NVML binding, XID parsing, Telemetry tools" \
  -f due_on="${M2_DATE}" 2>/dev/null && echo "  ✓ Created M2" || echo "  ⚠️  M2 already exists"

# M3: The Ephemeral Tunnel
gh api repos/"${REPO}"/milestones -f title="M3: The Ephemeral Tunnel" \
  -f state="open" \
  -f description="kubectl debug integration, e2e testing, Docker build" \
  -f due_on="${M3_DATE}" 2>/dev/null && echo "  ✓ Created M3" || echo "  ⚠️  M3 already exists"

# M4: Safety & Release
gh api repos/"${REPO}"/milestones -f title="M4: Safety & Release" \
  -f state="open" \
  -f description="Read-only flags, goreleaser pipelines, documentation" \
  -f due_on="${M4_DATE}" 2>/dev/null && echo "  ✓ Created M4" || echo "  ⚠️  M4 already exists"

echo ""

# ============================================================================
# STEP 4: CREATE ISSUES (Linked to Milestones)
# ============================================================================
echo "📋 Creating initial issues..."

# M1 Issues
gh issue create \
  --repo "${REPO}" \
  --title "[Scaffold] Init Go Module & Directory Structure" \
  --body "Initialize Go module and create standard project layout (cmd, pkg, internal)." \
  --label "kind/feature,area/mcp-protocol" \
  --milestone "M1: Foundation & API" || true

gh issue create \
  --repo "${REPO}" \
  --title "[MDC] Define Cursor Rules" \
  --body "Create .cursor/rules/*.mdc files for Go, MCP, NVML, and K8s development patterns." \
  --label "kind/feature" \
  --milestone "M1: Foundation & API" || true

gh issue create \
  --repo "${REPO}" \
  --title "[CI] GitHub Actions: Lint & Test" \
  --body "Set up GitHub Actions workflow with golangci-lint and go test." \
  --label "kind/feature" \
  --milestone "M1: Foundation & API" || true

gh issue create \
  --repo "${REPO}" \
  --title "[MCP] Implement Basic Stdio Server Loop" \
  --body "Create basic MCP server with stdio transport using mcp-go library. Echo test to validate transport." \
  --label "kind/feature,area/mcp-protocol,prio/p1-high" \
  --milestone "M1: Foundation & API" || true

# M2 Issues
gh issue create \
  --repo "${REPO}" \
  --title "[NVML] Implement Wrapper Interface" \
  --body "Create abstraction layer around go-nvml to enable mocking and unit testing. Decouple hardware dependencies." \
  --label "kind/feature,area/nvml-binding,prio/p1-high" \
  --milestone "M2: Hardware Introspection" || true

gh issue create \
  --repo "${REPO}" \
  --title "[Logic] Implement 'analyze_xid' Tool" \
  --body "Build XID error analyzer with static lookup table (Tier 1). Parse dmesg/NVML events and return structured JSON with severity and SRE actions." \
  --label "kind/feature,area/nvml-binding" \
  --milestone "M2: Hardware Introspection" || true

gh issue create \
  --repo "${REPO}" \
  --title "[Logic] Implement 'get_gpu_health' Tool" \
  --body "Implement GPU telemetry tool (Temperature, ECC errors, Memory usage). Must interpret throttling states." \
  --label "kind/feature,area/nvml-binding" \
  --milestone "M2: Hardware Introspection" || true

# M3 Issues
gh issue create \
  --repo "${REPO}" \
  --title "[Ops] Create Containerfile" \
  --body "Build distroless-based container image with static Go binary. Target size < 50MB. Include NVML library mount points." \
  --label "kind/feature,area/k8s-ephemeral" \
  --milestone "M3: The Ephemeral Tunnel" || true

gh issue create \
  --repo "${REPO}" \
  --title "[Docs] Write 'kubectl debug' launch wrapper script" \
  --body "Create helper script to launch agent via kubectl debug with proper SPDY tunneling for stdio transport." \
  --label "kind/feature,area/k8s-ephemeral" \
  --milestone "M3: The Ephemeral Tunnel" || true

echo "  ✓ Created 9 issues across 3 milestones"
echo ""

# ============================================================================
# SUMMARY
# ============================================================================
echo "✅ GitHub repository initialized successfully!"
echo ""
echo "📊 Summary:"
LABEL_COUNT=$(gh label list --repo "${REPO}" --json name --jq 'length')
MILESTONE_COUNT=$(gh api repos/"${REPO}"/milestones --jq 'length')
ISSUE_COUNT=$(gh issue list --repo "${REPO}" --json number --jq 'length')
echo "  - Labels: ${LABEL_COUNT}"
echo "  - Milestones: ${MILESTONE_COUNT}"
echo "  - Issues: ${ISSUE_COUNT}"
echo ""
echo "🔗 Quick Links:"
echo "  - Labels: https://github.com/${REPO}/labels"
echo "  - Milestones: https://github.com/${REPO}/milestones"
echo "  - Issues: https://github.com/${REPO}/issues"

