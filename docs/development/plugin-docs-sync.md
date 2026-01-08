# Plugin Docs Sync

Synchronize documentation to the Claude Code plugin.

## When to Run

After modifying files in `docs/reference/`.

## Command

```bash
make sync-plugin-docs
```

## What It Does

Copies `docs/reference/` to the plugin's references directory.

CI will fail if plugin docs are out of sync.
