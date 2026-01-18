---
paths: "**/*.go"
---

# Development Workflow

## Overview

When making code modifications, follow this workflow to ensure quality
and maintain test coverage.

## Workflow Steps

### 1. Explore Related Code

Before modifying code, @agent-Explore the codebase to understand the context:

- Use the built-in Explore agent to investigate related code
- Identify dependencies and side effects
- Understand existing patterns and conventions

When modifying a subcommand, pay special attention to its behavior:

- Review the command's documented behavior in @docs/reference/commands/
- Verify flag handling and argument parsing
- Check error handling and exit codes
- Understand the command's output format

### 2. Update Documentation

If the changes affect subcommand behavior, update the documentation:

- Update the corresponding file in @docs/reference/commands/
- Document new flags, arguments, or behavior changes
- Update examples if needed

### 3. Sync Plugin Docs

If you modified files in `docs/reference/`, run:

```bash
make sync-plugin-docs
```

See @docs/development/plugin-docs-sync.md for details.

### 4. Verify All Tests Pass

Run the full test suite before completing:

```bash
go test -tags=integration ./...
```

This command runs both unit tests and integration tests.
All tests must pass before the modification is considered complete.

### 5. Run Linter

Run golangci-lint to check code quality:

```bash
make lint
make fmt
```

- `make lint`: Check for lint errors
- `make fmt`: Auto-fix formatting issues

All lint errors must be resolved before the modification is considered complete.
