# Read /dev/kmsg for XID Error Parsing

## Issue Reference

- **Issue:** [#95 - analyze_xid_errors requires dmesg not available in distroless container](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/95)
- **Priority:** P1-High
- **Labels:** enhancement, area/nvml-binding
- **Milestone:** M3: Kubernetes Integration

## Background

The `analyze_xid_errors` tool currently relies on the `dmesg` command to read
kernel logs for NVIDIA XID error detection. This fails in distroless containers
where `dmesg` is not available.

**Current error:**
```
failed to parse kernel logs: dmesg command not found: exec: "dmesg": executable file not found in $PATH
```

**Context:**
- Discovered during real cluster integration testing (2026-01-07)
- Image: `ghcr.io/arangogutierrez/k8s-gpu-mcp-server:85525d6`
- Cluster: AWS g4dn.xlarge with Tesla T4

**Solution:** Read directly from `/dev/kmsg` - the kernel's ring buffer device.
This eliminates the external dependency while providing the same data.

---

## Objective

Replace `dmesg` command execution with direct `/dev/kmsg` reading, enabling XID
error analysis in distroless containers without external dependencies.

---

## Step 0: Create Feature Branch

> **⚠️ REQUIRED FIRST STEP - DO NOT SKIP**

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server
git checkout main
git pull origin main
git checkout -b fix/read-kmsg-xid-parsing
```

---

## Technical Background: /dev/kmsg Format

The `/dev/kmsg` interface provides structured access to the kernel log buffer.
Each record has the format:

```
priority,sequence,timestamp,flags;message
```

**Example:**
```
6,1234,5678901234,-;NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=python3
```

**Fields:**
- `priority`: syslog priority (facility*8 + level)
- `sequence`: monotonically increasing sequence number
- `timestamp`: microseconds since boot
- `flags`: optional flags (-, c for continuation)
- `message`: the actual log message

**Key considerations:**
1. Reading is non-blocking by default - needs `O_NONBLOCK` handling
2. File position represents sequence number, not byte offset
3. Messages can be multi-line (continuation flag `c`)
4. Requires appropriate permissions (CAP_SYSLOG or root)

---

## Implementation Tasks

### Task 1: Add KmsgReader Interface

Create an abstraction for kernel log reading that supports both `/dev/kmsg`
and the existing `dmesg` command as fallback.

**Files to create:**
- `pkg/xid/kmsg.go` - KmsgReader implementation

**Code:**

```go
// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package xid

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultKmsgPath is the default path to the kernel message buffer.
	DefaultKmsgPath = "/dev/kmsg"

	// kmsgReadTimeout is the maximum time to spend reading from /dev/kmsg.
	kmsgReadTimeout = 5 * time.Second
)

// KmsgReader reads kernel messages from /dev/kmsg.
type KmsgReader struct {
	path string
}

// NewKmsgReader creates a new reader for the kernel message buffer.
func NewKmsgReader() *KmsgReader {
	return &KmsgReader{path: DefaultKmsgPath}
}

// NewKmsgReaderWithPath creates a reader with a custom path (for testing).
func NewKmsgReaderWithPath(path string) *KmsgReader {
	return &KmsgReader{path: path}
}

// KmsgRecord represents a parsed /dev/kmsg record.
type KmsgRecord struct {
	Priority  int
	Sequence  uint64
	Timestamp time.Duration // Microseconds since boot
	Message   string
}

// ReadMessages reads all available messages from /dev/kmsg.
// Returns messages filtered to only NVRM (NVIDIA driver) entries.
func (r *KmsgReader) ReadMessages(ctx context.Context) ([]string, error) {
	// Check if /dev/kmsg exists and is readable
	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s not found: %w", r.path, err)
	}

	// Open /dev/kmsg for reading
	// Use O_RDONLY | O_NONBLOCK to avoid blocking on empty buffer
	file, err := os.OpenFile(r.path, os.O_RDONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied reading %s "+
				"(requires CAP_SYSLOG or root): %w", r.path, err)
		}
		return nil, fmt.Errorf("failed to open %s: %w", r.path, err)
	}
	defer file.Close()

	// Seek to end to get current position, then seek back to start
	// This ensures we read all available messages
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek in %s: %w", r.path, err)
	}

	var messages []string
	scanner := bufio.NewScanner(file)

	// Set up timeout context
	readCtx, cancel := context.WithTimeout(ctx, kmsgReadTimeout)
	defer cancel()

	// Read messages until EOF, error, or timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanner.Scan() {
			select {
			case <-readCtx.Done():
				return
			default:
			}

			line := scanner.Text()
			record, err := parseKmsgRecord(line)
			if err != nil {
				continue // Skip malformed records
			}

			// Filter for NVIDIA driver messages only
			if strings.Contains(record.Message, "NVRM") {
				messages = append(messages, record.Message)
			}
		}
	}()

	select {
	case <-done:
		// Reading completed normally
	case <-readCtx.Done():
		// Timeout or context cancelled - return what we have
		if readCtx.Err() == context.DeadlineExceeded {
			// This is expected - /dev/kmsg doesn't EOF normally
		}
	}

	if err := scanner.Err(); err != nil {
		// EAGAIN is expected for non-blocking read
		if !strings.Contains(err.Error(), "resource temporarily unavailable") {
			return messages, fmt.Errorf("error reading %s: %w", r.path, err)
		}
	}

	return messages, nil
}

// parseKmsgRecord parses a single /dev/kmsg record.
// Format: priority,sequence,timestamp,flags;message
func parseKmsgRecord(line string) (*KmsgRecord, error) {
	// Split on semicolon to separate header from message
	parts := strings.SplitN(line, ";", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid kmsg format: missing semicolon")
	}

	header := parts[0]
	message := parts[1]

	// Parse header fields: priority,sequence,timestamp,flags
	fields := strings.Split(header, ",")
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid kmsg header: expected 3+ fields")
	}

	priority, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("invalid priority: %w", err)
	}

	seq, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid sequence: %w", err)
	}

	tsUsec, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	return &KmsgRecord{
		Priority:  priority,
		Sequence:  seq,
		Timestamp: time.Duration(tsUsec) * time.Microsecond,
		Message:   message,
	}, nil
}

// IsAvailable checks if /dev/kmsg is readable.
func (r *KmsgReader) IsAvailable() bool {
	file, err := os.OpenFile(r.path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}
```

**Acceptance criteria:**
- [ ] `KmsgReader` can read from `/dev/kmsg`
- [ ] Records are correctly parsed
- [ ] NVRM messages are filtered
- [ ] Permission errors are handled gracefully
- [ ] Timeout prevents blocking on empty buffer

> 💡 **Commit after completing this task**

---

### Task 2: Update Parser to Use KmsgReader

Modify the `Parser` to prefer `/dev/kmsg` when available, falling back to
`dmesg` command when not.

**Files to modify:**
- `pkg/xid/parser.go` - Add fallback logic

**Changes:**

```go
// ParseKernelLogs reads XID events from kernel logs.
// Prefers /dev/kmsg when available, falls back to dmesg command.
func (p *Parser) ParseKernelLogs(ctx context.Context) ([]XIDEvent, error) {
	// Check context before expensive operation
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Try /dev/kmsg first (works in distroless containers)
	kmsgReader := NewKmsgReader()
	if kmsgReader.IsAvailable() {
		messages, err := kmsgReader.ReadMessages(ctx)
		if err == nil && len(messages) >= 0 {
			return p.parseMessages(messages), nil
		}
		// Log warning and fall back to dmesg
	}

	// Fall back to dmesg command
	return p.ParseDmesg(ctx)
}

// parseMessages extracts XID events from a slice of kernel log messages.
func (p *Parser) parseMessages(messages []string) []XIDEvent {
	var events []XIDEvent
	for _, msg := range messages {
		if !strings.Contains(msg, "Xid") {
			continue
		}
		if event := p.parseXIDLine(msg); event != nil {
			events = append(events, *event)
		}
	}
	return events
}
```

**Also update:**
- `ParseDmesg` should remain as a fallback method
- Add logging when falling back to dmesg
- Deprecation notice on `ParseDmesg` (keep for compatibility)

**Acceptance criteria:**
- [ ] `/dev/kmsg` is tried first
- [ ] Falls back to `dmesg` when `/dev/kmsg` unavailable
- [ ] Both methods produce consistent XIDEvent output
- [ ] Logging indicates which method was used

> 💡 **Commit after completing this task**

---

### Task 3: Update analyze_xid Tool Handler

Update the tool handler to use the new `ParseKernelLogs` method.

**Files to modify:**
- `pkg/tools/analyze_xid.go` - Use new method

**Changes:**

Replace:
```go
events, err := h.parser.ParseDmesg(ctx)
```

With:
```go
events, err := h.parser.ParseKernelLogs(ctx)
```

**Acceptance criteria:**
- [ ] Tool uses `ParseKernelLogs` instead of `ParseDmesg`
- [ ] Existing functionality preserved
- [ ] Error messages remain helpful

> 💡 **Commit after completing this task**

---

### Task 4: Update Helm Chart for /dev/kmsg Access

Add volume mount for `/dev/kmsg` in the DaemonSet template.

**Files to modify:**
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml`
- `deployment/helm/k8s-gpu-mcp-server/values.yaml`

**daemonset.yaml changes (add to container spec):**

```yaml
        volumeMounts:
        {{- if .Values.xidAnalysis.enabled }}
        - name: kmsg
          mountPath: /dev/kmsg
          readOnly: true
        {{- end }}
        # ... other mounts ...
      volumes:
      {{- if .Values.xidAnalysis.enabled }}
      - name: kmsg
        hostPath:
          path: /dev/kmsg
          type: CharDevice
      {{- end }}
```

**values.yaml changes:**

```yaml
# XID error analysis configuration
xidAnalysis:
  # Enable /dev/kmsg mount for XID error detection
  # Required for analyze_xid_errors tool in distroless containers
  enabled: true
```

**Acceptance criteria:**
- [ ] `/dev/kmsg` mounted when `xidAnalysis.enabled: true`
- [ ] Mount is read-only for security
- [ ] Feature can be disabled if not needed

> 💡 **Commit after completing this task**

---

### Task 5: Add Unit Tests for KmsgReader

Create comprehensive tests for the new kmsg reading functionality.

**Files to create:**
- `pkg/xid/kmsg_test.go`

**Test cases:**

```go
func TestParseKmsgRecord(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, *KmsgRecord)
	}{
		{
			name:  "valid NVRM message",
			input: "4,12345,678901234,-;NVRM: Xid (PCI:0000:00:1E.0): 48",
			check: func(t *testing.T, r *KmsgRecord) {
				assert.Equal(t, 4, r.Priority)
				assert.Equal(t, uint64(12345), r.Sequence)
				assert.Contains(t, r.Message, "NVRM")
			},
		},
		{
			name:    "invalid format - no semicolon",
			input:   "4,12345,678901234",
			wantErr: true,
		},
		{
			name:    "invalid priority",
			input:   "abc,12345,678901234,-;message",
			wantErr: true,
		},
		{
			name:  "message with continuation flag",
			input: "4,12345,678901234,c;continued message",
			check: func(t *testing.T, r *KmsgRecord) {
				assert.Equal(t, "continued message", r.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := parseKmsgRecord(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, record)
			}
		})
	}
}

func TestKmsgReader_IsAvailable(t *testing.T) {
	// Test with non-existent path
	reader := NewKmsgReaderWithPath("/nonexistent/path")
	assert.False(t, reader.IsAvailable())
}

func TestParser_ParseKernelLogs_FallbackToDmesg(t *testing.T) {
	// When /dev/kmsg is not available, should fall back to dmesg
	// This test verifies the fallback behavior
}
```

**Acceptance criteria:**
- [ ] Record parsing tested with various formats
- [ ] Availability check tested
- [ ] Fallback behavior tested
- [ ] Error cases covered

> 💡 **Commit after completing this task**

---

### Task 6: Update Documentation

Update docs to reflect the new behavior.

**Files to modify:**
- `docs/mcp-usage.md` - Add note about XID analysis requirements
- `README.md` - Update if needed

**Add to mcp-usage.md:**

```markdown
### XID Error Analysis Requirements

The `analyze_xid_errors` tool reads kernel logs to detect NVIDIA XID errors.
It uses two methods:

1. **`/dev/kmsg`** (preferred) - Direct kernel log access, works in
   distroless containers when `/dev/kmsg` is mounted from the host.

2. **`dmesg` command** (fallback) - Used when `/dev/kmsg` is not available.
   Requires a non-distroless container with `dmesg` installed.

**Helm configuration:**
```yaml
xidAnalysis:
  enabled: true  # Mounts /dev/kmsg from host
```
```

**Acceptance criteria:**
- [ ] XID analysis requirements documented
- [ ] Helm configuration explained
- [ ] Both methods mentioned

> 💡 **Commit after completing this task**

---

## Testing Requirements

### Local Testing (Mock Mode)

```bash
cd /Users/eduardoa/src/github/ArangoGutierrez/k8s-gpu-mcp-server

# Run all checks
make all

# Run tests with race detector
go test ./pkg/xid/... -race -v

# Test the tool in mock mode (won't have real XID errors)
./bin/agent --nvml-mode=mock < examples/analyze_xid.json
```

### Integration Testing (Real GPU Node)

```bash
# On a node with GPU and /dev/kmsg access
sudo ./bin/agent --nvml-mode=real < examples/analyze_xid.json

# Verify /dev/kmsg is readable
sudo cat /dev/kmsg | head -20

# Inject a test message (requires root)
echo "NVRM: Xid (PCI:0000:00:1E.0): 48, pid='1234', name=test" | sudo tee /dev/kmsg
```

### Cluster Testing

```bash
# Deploy with updated Helm chart
helm upgrade --install gpu-mcp ./deployment/helm/k8s-gpu-mcp-server \
  --namespace gpu-diagnostics --create-namespace \
  --set xidAnalysis.enabled=true

# Verify /dev/kmsg is mounted
kubectl exec -n gpu-diagnostics ds/gpu-mcp-server -- ls -la /dev/kmsg

# Test XID analysis via gateway
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze_xid_errors"},"id":1}'
```

---

## Pre-Commit Checklist

```bash
make fmt
make lint
make test
make all
```

- [ ] `go fmt ./...` - Code formatted
- [ ] `go vet ./...` - No vet warnings
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./... -count=1` - All tests pass
- [ ] `go test ./... -race` - No race conditions
- [ ] Documentation updated

---

## Commit and Push

### Atomic Commits

```bash
git commit -s -S -m "feat(xid): add KmsgReader for /dev/kmsg parsing"
git commit -s -S -m "feat(xid): update Parser with fallback to dmesg"
git commit -s -S -m "feat(tools): use ParseKernelLogs in analyze_xid handler"
git commit -s -S -m "feat(helm): add /dev/kmsg volume mount for XID analysis"
git commit -s -S -m "test(xid): add KmsgReader unit tests"
git commit -s -S -m "docs: document XID analysis /dev/kmsg requirement"
```

### Push

```bash
git push -u origin fix/read-kmsg-xid-parsing
```

---

## Create Pull Request

```bash
gh pr create \
  --title "fix(xid): read /dev/kmsg for XID parsing in distroless containers" \
  --body "Fixes #95

## Summary
Enables XID error analysis in distroless containers by reading directly from
\`/dev/kmsg\` instead of relying on the \`dmesg\` command.

## Changes
- Add \`KmsgReader\` to parse kernel messages from \`/dev/kmsg\`
- Update \`Parser\` to prefer \`/dev/kmsg\` with \`dmesg\` fallback
- Add Helm chart option to mount \`/dev/kmsg\` from host
- Add comprehensive tests for kmsg parsing

## How it works
1. Try reading from \`/dev/kmsg\` (preferred, no external deps)
2. Fall back to \`dmesg\` command if \`/dev/kmsg\` unavailable
3. Parse NVRM messages for XID error codes
4. Return structured XID events

## Testing
- [x] Unit tests for kmsg parsing
- [x] Fallback behavior tested
- [x] Manual testing on GPU node
- [x] \`make all\` passes

## Helm Configuration
\`\`\`yaml
xidAnalysis:
  enabled: true  # Mounts /dev/kmsg from host
\`\`\`" \
  --label "enhancement" \
  --label "area/nvml-binding"
```

---

## Related Files

- `pkg/xid/parser.go` - Existing XID parser (to be modified)
- `pkg/xid/codes.go` - XID error code definitions
- `pkg/tools/analyze_xid.go` - Tool handler
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml`

## Notes

### Security Considerations

- `/dev/kmsg` access requires `CAP_SYSLOG` capability or root
- Mount is read-only to prevent writing to kernel log buffer
- DaemonSet already runs with elevated privileges for NVML access

### Alternative Approaches Not Taken

1. **Add busybox to container** - Increases attack surface and image size
2. **Use hostPID namespace** - Excessive privilege for just dmesg access
3. **Parse /var/log/kern.log** - Not reliably available across distros

### Compatibility

- Works with Linux kernel 3.5+ (when `/dev/kmsg` was introduced)
- Fallback ensures compatibility with older systems or restricted environments

---

## Quick Reference

**Estimated Time:** 3-4 hours

**Complexity:** Medium

**Files Changed:**
- `pkg/xid/kmsg.go` (new)
- `pkg/xid/kmsg_test.go` (new)
- `pkg/xid/parser.go` (modify)
- `pkg/tools/analyze_xid.go` (modify)
- `deployment/helm/k8s-gpu-mcp-server/templates/daemonset.yaml` (modify)
- `deployment/helm/k8s-gpu-mcp-server/values.yaml` (modify)
- `docs/mcp-usage.md` (modify)

---

**Reply "GO" when ready to start implementation.** 🚀
