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

## Best Practices

- Always use `t.Parallel()` for test isolation and performance
- Use `t.TempDir()` for automatic cleanup
- Use `t.Helper()` in helper functions for better error locations
- Test both success and error paths
- Verify actual side effects (files created, git state changed, etc.)
