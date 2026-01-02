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

- Review the command's documented behavior in `docs/commands/`
- Verify flag handling and argument parsing
- Check error handling and exit codes
- Understand the command's output format

### 2. Implement Changes

Based on the exploration:

- Add, modify, or delete code as needed
- Follow existing patterns and conventions in the codebase
- Keep changes focused and minimal

### 3. Update Tests

Review and update tests to ensure coverage:

- Check related unit tests (`*_test.go`)
- Check related integration tests (`*_integration_test.go`)
- If existing tests do not cover the changes:
  - Add new test cases
  - Modify existing tests
  - Remove obsolete tests

### 4. Verify All Tests Pass

Run the full test suite before completing:

```bash
go test -tags=integration ./...
```

This command runs both unit tests and integration tests.
All tests must pass before the modification is considered complete.
