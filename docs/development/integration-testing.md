---
paths: "**/*_integration_test.go, **/internal/testutil/**"
---

# Integration Testing Guidelines

## Overview

Integration tests verify that components work correctly together using real I/O
operations (filesystem, git commands, etc.) instead of mocks.

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

### Why `-c protocol.file.allow=always` is Required

Git 2.38 introduced security restrictions that block local `file://` protocol
URLs by default. Since integration tests use local repositories as submodules,
the `-c protocol.file.allow=always` option must be passed to:

- `git submodule add` when adding a submodule
- `git submodule update` when initializing submodules

Without this option, tests fail on GitHub Actions and other CI environments
with errors like:

```txt
fatal: transport 'file' not allowed
```

### Setup Pattern

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

    // Step 3: Add submodule to main repo (MUST use -c protocol.file.allow=always)
    testutil.RunGit(t, mainDir, "-c", "protocol.file.allow=always",
        "submodule", "add", submoduleRepo, "mysub")
    testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

    // Now mainDir contains a submodule at "mysub"
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

1. **Always use `-c protocol.file.allow=always`** for `submodule add` and
   `submodule update` commands
2. **Submodule repo needs at least one commit** before it can be added
3. **Configure user.email and user.name** in the submodule repo before committing
4. **Use `testutil.RunGit`** helper for consistent error handling

## Best Practices

- Always use `t.Parallel()` for test isolation and performance
- Use `t.TempDir()` for automatic cleanup
- Use `t.Helper()` in helper functions for better error locations
- Test both success and error paths
- Verify actual side effects (files created, git state changed, etc.)
