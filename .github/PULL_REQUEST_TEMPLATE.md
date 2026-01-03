## Description

<!-- Provide a clear and concise description of the changes -->

**What changed:**
- 

**Why:**
- 

**How:**
- 

## Related Issues

Fixes #<!-- issue number -->
Relates to #<!-- issue number if applicable -->

## Type of Change

<!-- Mark the relevant option with an 'x' -->

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New feature (non-breaking change which adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📝 Documentation update
- [ ] 🔧 Refactoring / Technical debt
- [ ] 🔒 Security fix
- [ ] ⚙️ CI/CD or build system change

## Component Area

<!-- Mark the relevant areas affected by this PR -->

- [ ] MCP Protocol / Transport
- [ ] NVML Hardware Binding
- [ ] Kubernetes Integration
- [ ] Tool Implementation
- [ ] Build / Container
- [ ] Documentation
- [ ] Testing Infrastructure

## Testing

**How has this been tested?**

<!-- Describe the tests you ran to verify your changes -->

- [ ] Unit tests pass locally (`go test ./... -count=1`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Integration tests (if applicable)
- [ ] Manual testing (describe below)

**Manual Testing Steps:**
```bash
# Example:
# 1. Build agent: go build ./cmd/agent
# 2. Run with: ./agent --mode=read-only
# 3. Test tool: echo '{"method":"tools/call"...}' | ./agent
```

**Test Environment:**
<!-- If testing with real hardware, provide details -->

- Kubernetes Version: 
- NVIDIA Driver Version: 
- GPU Model: 
- Node OS: 

## Code Quality Checklist

<!-- Ensure all items are checked before requesting review -->

- [ ] My code follows the Go style guidelines (`gofmt -s`, Effective Go)
- [ ] I have performed a self-review of my code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] Documentation comments are limited to 80 characters per line
- [ ] I have made corresponding changes to the documentation (README, godoc)
- [ ] My changes generate no new warnings or linter errors
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] I have checked for error handling and context cancellation support

## Git Protocol Compliance

<!-- Required by project workflow standards -->

- [ ] Commits are signed with DCO (`git commit -s`)
- [ ] Commits are GPG signed (`git commit -S`)
- [ ] Commit messages follow format: `type(scope): description`
- [ ] Branch name follows convention: `feat/`, `fix/`, `chore/`, etc.
- [ ] This PR is linked to an issue and milestone

## Security Considerations

<!-- For security-sensitive changes, answer the following -->

- [ ] This change handles user input (validated and sanitized)
- [ ] This change modifies privileged operations (audited)
- [ ] This change affects the security model (documented)
- [ ] No secrets or sensitive data are exposed in logs or errors

## Breaking Changes

<!-- If this is a breaking change, describe the impact and migration path -->

**Impact:**
- 

**Migration Guide:**
- 

## Screenshots / Logs

<!-- Add screenshots for UI changes, or logs for behavior changes -->

```
# Paste relevant logs or output here
```

## Additional Notes

<!-- Any other information that reviewers should know -->

---

**Reviewer Checklist** (for maintainers):
- [ ] Code quality and style compliance
- [ ] Test coverage is adequate
- [ ] Documentation is updated
- [ ] No security concerns
- [ ] Milestone and labels are correct
- [ ] CI checks pass

