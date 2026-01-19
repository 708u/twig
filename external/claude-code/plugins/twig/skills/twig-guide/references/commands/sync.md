# sync subcommand

Sync symlinks and submodules from source worktree to target worktrees.

## Usage

```txt
twig sync [<branch>...] [flags]
```

## Arguments

- `<branch>...`: Target branch names to sync (optional)

## Flags

| Flag              | Short | Description                                        |
|-------------------|-------|----------------------------------------------------|
| `--source`        |       | Source branch (default: `default_source` config)   |
| `--all`           | `-a`  | Sync all worktrees (except main)                   |
| `--check`         |       | Show what would be synced (dry-run)                |
| `--force`         | `-f`  | Replace existing symlinks                          |
| `--verbose`       | `-v`  | Enable verbose output (use `-vv` for debug)        |

## Behavior

Syncs symlinks and submodules from a source worktree to one or more target
worktrees. This is useful when configuration changes (symlinks, submodules)
need to be applied to existing worktrees.

### Source Resolution

The source worktree is determined in this order:

1. `--source` flag if specified
2. `default_source` configuration if set
3. Error if neither is available

### Target Resolution

Targets are determined based on arguments and flags:

| Arguments | `--all` | Behavior                           |
|-----------|---------|-------------------------------------|
| None      | No      | Sync current worktree               |
| None      | Yes     | Sync all worktrees (except main)    |
| Specified | No      | Sync specified worktrees            |
| Specified | Yes     | Error (mutually exclusive)          |

### What Gets Synced

The command syncs based on the source worktree's configuration:

| Configuration       | Action                                          |
|---------------------|-------------------------------------------------|
| `symlinks`          | Create symlinks from source to target           |
| `init_submodules`   | Initialize submodules in target worktrees       |

If neither `symlinks` nor `init_submodules` is configured, the command exits
early with a message indicating nothing to sync.

### Symlink Behavior

By default, existing files (including symlinks) at the destination are
skipped. Use `--force` to replace existing symlinks.

| Condition               | Default      | With `--force`     |
|-------------------------|--------------|--------------------|
| No file at destination  | Create       | Create             |
| Symlink exists          | Skip         | Replace            |
| Regular file exists     | Skip         | Skip (not replaced)|

The `--force` flag only replaces symlinks, not regular files. This prevents
accidental data loss when a tracked file exists at the destination.

### Check Mode

With `--check`, the command shows what would be synced without making changes.
This is useful for previewing the sync operation.

## Output Format

### Default Output

```txt
twig sync: feat/a (2 symlinks, 1 submodule(s))
twig sync: feat/b (skipped: up to date)
```

### Verbose Output

```txt
Syncing from main to feat/a
Created symlink: /repo/feat/a/.envrc -> /repo/main/.envrc
Created symlink: /repo/feat/a/.tool-versions -> /repo/main/.tool-versions
Initialized 1 submodule(s)
twig sync: feat/a (2 symlinks, 1 submodule(s))
```

### Check Mode Output

```txt
Would sync from main:

feat/a:
  Would create symlink: /repo/feat/a/.envrc
  Would create symlink: /repo/feat/a/.tool-versions
  Would initialize submodules

feat/b:
  (skipped: up to date)
```

### Debug Output

With `-vv`, debug logging traces internal operations:

```txt
2026-01-19 12:34:56.000 [DEBUG] [a1b2c3d4] sync: resolving source branch=main
2026-01-19 12:34:56.000 [DEBUG] [a1b2c3d4] sync: loading config from source
2026-01-19 12:34:56.000 [DEBUG] [a1b2c3d4] sync: syncing target branch=feat/a
twig sync: feat/a (2 symlinks)
```

## Examples

```bash
# Sync current worktree from default_source
twig sync

# Sync specific worktrees
twig sync feat/a feat/b

# Sync all worktrees (except main)
twig sync --all

# Sync from a specific source branch
twig sync --source develop

# Replace existing symlinks
twig sync --force

# Preview what would be synced
twig sync --check

# Sync all with verbose output
twig sync --all -v

# Sync with debug logging
twig sync -vv
```

## Configuration

The sync command uses configuration from the source worktree:

```toml
# .twig/settings.toml in source worktree
symlinks = [".envrc", ".tool-versions", "config/**"]
init_submodules = true
```

See [Configuration](../configuration.md) for details.

## Error Handling

| Condition                        | Behavior                              |
|----------------------------------|---------------------------------------|
| Source not specified + no config | Error with hint                       |
| Source worktree not found        | Error                                 |
| Target worktree not found        | Error for that target                 |
| Target is same as source         | Skipped                               |
| Symlink creation fails           | Error for that target, others proceed |
| Submodule init fails             | Warning (non-fatal)                   |

## Exit Code

- 0: Success (or nothing to sync)
- 1: One or more targets failed to sync
