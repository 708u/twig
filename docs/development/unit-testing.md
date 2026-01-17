---
paths: "**/*_test.go, **/internal/testutil/**"
---

# Testing Guidelines

## Mocking external dependencies

Operations involving I/O (filesystem, network) or external processes reduce
testability. Use interface + DI pattern to make them mockable:

What to mock:

- Filesystem operations (os.Stat, os.ReadFile, os.Symlink, etc.)
- External command execution (exec.Command)
- Network calls
- Time-dependent operations

Example: filesystem operations

```go
// Define interface with only needed operations
type FileSystem interface {
    Stat(name string) (fs.FileInfo, error)
    Symlink(oldname, newname string) error
    MkdirAll(path string, perm fs.FileMode) error
}

// Production implementation
type osFS struct{}

func (osFS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (osFS) Symlink(old, new string) error                { return os.Symlink(old, new) }
func (osFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

// Test mock
type mockFS struct {
    statErr    error
    symlinkErr error
}

func (m mockFS) Stat(name string) (fs.FileInfo, error) { return nil, m.statErr }
func (m mockFS) Symlink(old, new string) error         { return m.symlinkErr }
```

Usage pattern:

```go
func DoSomething(fs FileSystem, path string) error {
    if fs == nil {
        fs = osFS{}  // default to real OS
    }
    // use fs.Stat(), fs.Symlink(), etc.
}
```

## Using context.Context in tests

Prefer `t.Context()` (Go 1.21+) over `context.Background()` for tests that
require a context. `t.Context()` returns a context that is canceled when the
test completes, enabling proper cleanup and timeout handling.

Use `context.Background()` only when `t.Context()` is not available:

- Helper functions without access to `*testing.T`
- Benchmark functions (`*testing.B` lacks `Context()`)
- Table-driven tests where context needs to outlive subtests
