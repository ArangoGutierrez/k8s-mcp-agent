# GitHub Configuration

This directory contains GitHub-specific configuration files for the k8s-gpu-mcp-server project.

## 📁 Structure

```
.github/
├── ISSUE_TEMPLATE/          # Issue form templates
│   ├── bug_report.yml       # Bug report with GPU/K8s context
│   ├── feature_request.yml  # Feature proposals with milestone targeting
│   ├── tech_debt.yml        # Technical debt tracking
│   ├── security.yml         # Security vulnerability reports
│   └── config.yml           # Template configuration (disables blank issues)
├── workflows/               # GitHub Actions CI/CD
│   ├── ci.yml              # Lint, test, build, security scan
│   └── release.yml         # GoReleaser for tagged releases
├── PULL_REQUEST_TEMPLATE.md # PR template with DCO/GPG checklist
├── copilot-instructions.md  # GitHub Copilot guidance
├── dependabot.yml          # Automated dependency updates
└── README.md               # This file
```

## 🎫 Issue Templates

### Bug Report (`bug_report.yml`)
Structured form for reporting bugs with:
- Environment details (K8s version, GPU model, driver)
- Component area selection (MCP, NVML, K8s, etc.)
- Steps to reproduce
- Expected vs. actual behavior
- Relevant logs

### Feature Request (`feature_request.yml`)
Proposal template with:
- Problem statement and proposed solution
- Component area and milestone targeting
- Priority assessment (P0-P3)
- Technical considerations

### Technical Debt (`tech_debt.yml`)
Tracks code quality issues:
- Current vs. desired state
- Impact and severity assessment
- Effort estimation
- Blockers and dependencies

### Security Vulnerability (`security.yml`)
Security issue reporting with:
- Severity assessment
- Affected components
- Impact and mitigation suggestions
- Responsible disclosure checklist

## 🔄 Pull Request Template

The PR template enforces:
- **Git Protocol Compliance**: DCO (`-s`) and GPG (`-S`) signing
- **Code Quality**: Linting, testing, self-review checklist
- **Security**: Input validation, privilege auditing
- **Documentation**: Inline comments, README updates

## 🤖 Copilot Instructions

The `copilot-instructions.md` file provides context for GitHub Copilot:
- Project architecture and principles
- Go style conventions (Effective Go, 80-char docs)
- Error handling patterns (context wrapping)
- MCP-specific guidelines (stdio separation)
- NVML safety patterns (graceful degradation)
- Testing patterns (table-driven, mocking)

## 📦 Dependabot Configuration

Automated dependency updates for:
- **Go modules**: Weekly updates (Monday 09:00)
  - Groups minor/patch updates together
  - Allows major updates only for critical deps (NVML, mcp-go)
- **GitHub Actions**: Weekly workflow updates
- **Docker**: Base image updates (if Dockerfile present)

All updates are labeled and assigned to maintainers.

## ⚙️ GitHub Actions Workflows

### CI Workflow (`workflows/ci.yml`)

Runs on every push and pull request:

1. **Lint Job**
   - `gofmt -s` formatting check
   - `go vet` static analysis
   - `golangci-lint` comprehensive linting

2. **Test Job**
   - Unit tests with race detector
   - Coverage reporting (CodeCov integration)

3. **Build Job**
   - Multi-arch builds (linux/amd64, linux/arm64)
   - Binary size validation (warns if >50MB)

4. **Security Job**
   - Trivy vulnerability scanning
   - SARIF results uploaded to GitHub Security

5. **Verify Commits Job** (PRs only)
   - DCO signoff verification (`Signed-off-by`)
   - Commit message format check (`type(scope): description`)

### Release Workflow (`workflows/release.yml`)

Triggered on version tags (`v*`):
- GoReleaser for multi-platform builds
- Container image publishing to ghcr.io
- GitHub release with artifacts

## 🚀 Usage

### Creating an Issue

1. Go to [Issues](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/issues/new/choose)
2. Select the appropriate template
3. Fill out all required fields
4. Link to relevant milestone (M1-M4)

### Opening a Pull Request

1. Ensure branch follows naming convention: `feat/`, `fix/`, `chore/`, etc.
2. Commit with DCO and GPG: `git commit -s -S -m "type(scope): description"`
3. Run tests locally: `go test ./... -count=1`
4. Push and create PR - template will auto-populate
5. Fill out all checklist items
6. Link to issue: `Fixes #XX`
7. Wait for CI checks to pass

### Dependabot PRs

When Dependabot opens a dependency update PR:
1. Review the changelog/release notes
2. Check CI passes
3. Test locally if significant update
4. Approve and merge (auto-merge can be configured)

## 🔒 Security

- **Private vulnerability reporting**: Use [Security Advisories](https://github.com/ArangoGutierrez/k8s-gpu-mcp-server/security/advisories/new)
- **Public security issues**: Use `security.yml` template for lower-severity concerns
- **Trivy scanning**: Automated on every PR and push
- **Dependabot alerts**: Enabled for security vulnerabilities

## 📖 References

- [GitHub Issue Forms](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms)
- [Dependabot Configuration](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file)
- [GitHub Actions Workflow Syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
- [GitHub Copilot Custom Instructions](https://docs.github.com/en/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)

---

**Maintained by:** [@ArangoGutierrez](https://github.com/ArangoGutierrez)  
**Last Updated:** 2026-01-03

