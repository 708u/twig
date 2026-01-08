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

### 2. TDD Cycle

Follow the Red-Green-Refactor cycle:

#### Red: Write Failing Tests

- Write tests that define expected behavior before implementation
- Focus on behavior, not implementation details
- Run tests to confirm they fail for the expected reason

#### Green: Implement Changes

- Write minimal code to make tests pass
- Follow existing patterns and conventions in the codebase
- Keep changes focused and minimal

#### Refactor

- Improve code structure while keeping tests green
- Remove duplication and improve readability
- Run tests after each refactoring step

#### Update Existing Tests

- Check related unit tests (`*_test.go`) and integration tests (`*_integration_test.go`)
- Modify or remove obsolete tests if needed

### 3. Update Documentation

If the changes affect subcommand behavior, update the documentation:

- Update the corresponding file in @docs/reference/commands/
- Document new flags, arguments, or behavior changes
- Update examples if needed

### 4. Sync Plugin Docs

If you modified files in `docs/reference/`, run:

```bash
make sync-plugin-docs
```

See @docs/development/plugin-docs-sync.md for details.

### 5. Verify All Tests Pass

Run the full test suite before completing:

```bash
go test -tags=integration ./...
```

This command runs both unit tests and integration tests.
All tests must pass before the modification is considered complete.
