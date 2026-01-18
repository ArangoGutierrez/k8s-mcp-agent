# Reports Directory

This directory is for internal development reports, analyses, and research documents.

## Purpose

- Architecture decision records
- Milestone completion reports
- Issue research and analysis
- Code review findings
- Performance assessments

## Usage

Reports are working documents used during development. They are **not intended for
end-users** and are excluded from the repository via `.gitignore`.

### Creating Reports

1. Copy this pattern for new reports:

```markdown
# [Report Title]

**Date:** YYYY-MM-DD
**Status:** Draft | Complete
**Author:** [Name or AI-Assisted]

## Summary

Brief overview of the report's purpose and findings.

## Details

[Report content...]

## Conclusion

Key takeaways and action items.
```

2. Use descriptive filenames: `topic-description.md`
3. Include dates for time-sensitive analysis

## .gitignore

All `.md` files in this directory (except `README.md`) are ignored by git:

```gitignore
docs/reports/*.md
!docs/reports/README.md
```

This keeps development artifacts local while preserving the directory structure.
