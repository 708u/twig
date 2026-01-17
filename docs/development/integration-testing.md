---
paths: "**/*_integration_test.go, **/internal/testutil/**"
---

# Integration Testing Guidelines

## Overview

Integration tests verify that components work correctly together using real I/O
operations (filesystem, git commands, etc.) instead of mocks.

## When to Write Integration Tests

Unit tests with mocks verify that code behaves correctly **given certain
inputs**. Integration tests verify that **the inputs themselves are correct**
when obtained from real external systems.

### Required: Data Acquisition and Propagation

Write integration tests when external data is acquired and propagated through
multiple layers:

```txt
External System → Acquisition → Intermediate Structure → Final Result
```

Unit tests can verify each component in isolation with mocked data, but cannot
guarantee:

1. The acquisition layer correctly parses external system output
2. The data is correctly stored in intermediate structures
3. The data is correctly propagated to final output structures

Unit tests mock external calls and verify logic. Integration tests verify that
data flows correctly from the real external system through all layers.

### Not Required: Pure Transformation

Integration tests are unnecessary for logic that only transforms
already-acquired data without external system interaction.

### Decision Checklist

Write an integration test if **any** of these apply:

- New field populated from external system output
- Data propagated across struct boundaries
- Parser for external command output format
- Multiple external calls combined into one operation

Skip integration test if **all** of these apply:

- Logic operates on already-acquired data
- Unit test covers all code paths with mocked inputs
- No external system interaction

## Build Tag

Use `//go:build integration` to separate integration tests from unit tests:

```go
//go:build integration

package mypackage
```

## File Naming

Use `*_integration_test.go` suffix:

```txt
add.go
add_test.go              # unit tests
add_integration_test.go  # integration tests
```

## Running Tests

```bash
# Unit tests only (default)
go test ./...

# Integration tests only
go test -tags=integration ./...

# Both
go test -tags=integration ./... && go test ./...
```

## Architecture

Integration tests use real resources in temporary directories:

1. Create temporary directory with `t.TempDir()`
2. Set up real git repository if needed
3. Execute the code under test
4. Verify results on the actual filesystem

## Example: Testing a File Processor

```go
//go:build integration

package processor

import (
    "os"
    "path/filepath"
    "testing"
)

func TestFileProcessor_Integration(t *testing.T) {
    t.Parallel()

    t.Run("ProcessesFilesInDirectory", func(t *testing.T) {
        t.Parallel()

        // Setup: create temporary directory structure
        tmpDir := t.TempDir()
        inputDir := filepath.Join(tmpDir, "input")
        outputDir := filepath.Join(tmpDir, "output")

        if err := os.MkdirAll(inputDir, 0755); err != nil {
            t.Fatal(err)
        }

        // Create test input file
        inputFile := filepath.Join(inputDir, "data.txt")
        if err := os.WriteFile(inputFile, []byte("test data"), 0644); err != nil {
            t.Fatal(err)
        }

        // Execute
        processor := NewFileProcessor(inputDir, outputDir)
        if err := processor.Run(); err != nil {
            t.Fatalf("Run failed: %v", err)
        }

        // Verify: check output file exists
        outputFile := filepath.Join(outputDir, "data.txt")
        if _, err := os.Stat(outputFile); os.IsNotExist(err) {
            t.Errorf("output file does not exist: %s", outputFile)
        }

        // Verify: check content
        content, err := os.ReadFile(outputFile)
        if err != nil {
            t.Fatalf("failed to read output: %v", err)
        }
        expected := "processed: test data"
        if string(content) != expected {
            t.Errorf("content = %q, want %q", string(content), expected)
        }
    })
}
```

## Example: Testing with Git Repository

```go
//go:build integration

package vcs

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func TestGitOperation_Integration(t *testing.T) {
    t.Parallel()

    t.Run("CreatesNewBranch", func(t *testing.T) {
        t.Parallel()

        // Setup: create temporary git repository
        repoDir := t.TempDir()
        runGit(t, repoDir, "init")
        runGit(t, repoDir, "config", "user.email", "test@example.com")
        runGit(t, repoDir, "config", "user.name", "Test User")
        runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

        // Execute
        op := NewGitOperation(repoDir)
        if err := op.CreateBranch("feature/new"); err != nil {
            t.Fatalf("CreateBranch failed: %v", err)
        }

        // Verify: branch exists
        out := runGit(t, repoDir, "branch", "--list", "feature/new")
        if out == "" {
            t.Error("branch feature/new was not created")
        }
    })
}

func runGit(t *testing.T, dir string, args ...string) string {
    t.Helper()

    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("git %v failed: %v\n%s", args, err, out)
    }
    return string(out)
}
```

## Example: Testing with Git Submodules

Git submodules require special handling in integration tests due to security
restrictions introduced in Git 2.38+.

### Why `protocol.file.allow=always` is Required

Git 2.38 introduced security restrictions that block local `file://` protocol
URLs by default. Since integration tests use local repositories as submodules,
`protocol.file.allow=always` must be configured.

Without this configuration, tests fail on GitHub Actions and other CI
environments with errors like:

```txt
fatal: transport 'file' not allowed
```

### Configuration Approaches

Choose based on whether **production code** executes submodule commands:

#### When production code runs submodule commands (e.g., `submodule update`)

Use `t.Setenv` to propagate config to child processes:

```go
// At the start of the test function (before any subtests)
t.Setenv("GIT_CONFIG_COUNT", "1")
t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
t.Setenv("GIT_CONFIG_VALUE_0", "always")
```

- Applies to all git commands including those run by production code
- Requires sequential test execution (no `t.Parallel()`)

#### When only test setup runs submodule commands

Use `-c` flag directly on `testutil.RunGit` calls:

```go
testutil.RunGit(t, dir, "-c", "protocol.file.allow=always",
    "submodule", "add", submoduleRepo, "mysub")
```

- Allows `t.Parallel()` for faster test execution
- Only affects the specific git command

### Setup Pattern: Using `-c` Flag

For tests where only test setup runs submodule commands:

```go
//go:build integration

package twig

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSubmoduleOperation_Integration(t *testing.T) {
    t.Parallel()

    t.Run("WithSubmodule", func(t *testing.T) {
        t.Parallel()

        repoDir, mainDir := testutil.SetupTestRepo(t)

        // Step 1: Create a submodule repository (separate git repo)
        submoduleRepo := filepath.Join(repoDir, "submodule-repo")
        if err := os.MkdirAll(submoduleRepo, 0755); err != nil {
            t.Fatal(err)
        }
        testutil.RunGit(t, submoduleRepo, "init")
        testutil.RunGit(t, submoduleRepo, "config", "user.email", "test@example.com")
        testutil.RunGit(t, submoduleRepo, "config", "user.name", "Test")

        // Step 2: Add content and commit in submodule repo
        subFile := filepath.Join(submoduleRepo, "file.txt")
        if err := os.WriteFile(subFile, []byte("submodule content"), 0644); err != nil {
            t.Fatal(err)
        }
        testutil.RunGit(t, submoduleRepo, "add", ".")
        testutil.RunGit(t, submoduleRepo, "commit", "-m", "initial")

        // Step 3: Add submodule to main repo (use -c flag)
        testutil.RunGit(t, mainDir, "-c", "protocol.file.allow=always",
            "submodule", "add", submoduleRepo, "mysub")
        testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

        // Now mainDir contains a submodule at "mysub"
    })
}
```

### Setup Pattern: Using `t.Setenv`

For tests where production code runs submodule commands (e.g., `submodule update`):

```go
//go:build integration

package twig

import (
    "os"
    "path/filepath"
    "testing"
)

// Not parallel: uses t.Setenv for file:// protocol in local submodule URLs.
func TestSubmoduleInit_Integration(t *testing.T) {
    // Allow file:// protocol for local submodule URLs in tests
    t.Setenv("GIT_CONFIG_COUNT", "1")
    t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
    t.Setenv("GIT_CONFIG_VALUE_0", "always")

    t.Run("InitializesSubmodules", func(t *testing.T) {
        repoDir, mainDir := testutil.SetupTestRepo(t)

        // Setup submodule (same as above)
        submoduleRepo := filepath.Join(repoDir, "submodule-repo")
        // ... create submodule repo ...

        // Add submodule (env var handles protocol)
        testutil.RunGit(t, mainDir, "submodule", "add", submoduleRepo, "mysub")
        testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

        // Production code runs submodule update (env var propagates)
        cmd := NewCommand(mainDir)
        result, err := cmd.Run()  // internally calls git submodule update
        // ...
    })
}
```

### Creating Dirty Submodule States

For testing removal/clean operations that check submodule status:

```go
// Pattern 1: Uncommitted changes in submodule (dirty working tree)
submodulePath := filepath.Join(wtPath, "sub")
if err := os.WriteFile(filepath.Join(submodulePath, "dirty.txt"),
    []byte("uncommitted"), 0644); err != nil {
    t.Fatal(err)
}
// git submodule status shows: " " (space) prefix = unmodified

// Pattern 2: Modified commit (submodule at different commit than recorded)
testutil.RunGit(t, submodulePath, "config", "user.email", "test@example.com")
testutil.RunGit(t, submodulePath, "config", "user.name", "Test")
testutil.RunGit(t, submodulePath, "add", ".")
testutil.RunGit(t, submodulePath, "commit", "-m", "advance")
// git submodule status shows: "+" prefix = modified commit
```

### Key Points

1. **Choose the right configuration approach** - use `t.Setenv` when production
   code runs submodule commands, use `-c` flag when only test setup does
2. **Submodule repo needs at least one commit** before it can be added
3. **Configure user.email and user.name** in the submodule repo before
   committing
4. **Use `testutil.RunGit`** helper for consistent error handling

## Test Naming

Name tests based on external behavior, not internal implementation.

Test names should describe what a user observes from outside the system. Avoid
naming tests after internal mechanisms that users don't interact with directly.

For example, if a feature allows deletion without the `--force` flag, name the
test to reflect that no force is required, rather than describing internal
implementation details like which git flag is used internally.

## Best Practices

- Use `t.Parallel()` for test isolation and performance (except when using
  `t.Setenv` for submodule tests)
- Use `t.TempDir()` for automatic cleanup
- Use `t.Helper()` in helper functions for better error locations
- Test both success and error paths
- Verify actual side effects (files created, git state changed, etc.)
- Use `t.Context()` instead of `context.Background()` when context is needed
  (see below)

## Using context.Context in tests

Prefer `t.Context()` (Go 1.21+) over `context.Background()` for tests that
require a context. `t.Context()` returns a context that is canceled when the
test completes, enabling proper cleanup and timeout handling.

Use `context.Background()` only when `t.Context()` is not available:

- Helper functions without access to `*testing.T`
- Benchmark functions (`*testing.B` lacks `Context()`)
- Table-driven tests where context needs to outlive subtests
