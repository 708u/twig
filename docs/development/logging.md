---
paths: "**/*.go"
---

# Logging Guidelines

## Use xxxContext Methods

Use context-aware methods (`DebugContext`, `InfoContext`, `WarnContext`,
`ErrorContext`) instead of non-context methods.

Reasons:

- Propagates context-scoped values (trace ID, request ID) to log output
- Enables future context-based logging features (e.g., structured tracing)
- Follows slog standard patterns

```go
// Good
c.Log.DebugContext(ctx, "run started",
    "category", LogCategoryRemove,
    "branch", branch)

// Avoid
c.Log.Debug("run started",
    "category", LogCategoryRemove,
    "branch", branch)
```

## Log Start and Completion

Log both the start and completion of operations:

```go
func (c *RemoveCommand) Run(ctx context.Context, ...) {
    c.Log.DebugContext(ctx, "run started", ...)

    // ... operation ...

    c.Log.DebugContext(ctx, "run completed", ...)
    return result, nil
}
```
