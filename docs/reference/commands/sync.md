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
| `--verbose`       | `-v`  | Enable verbose output (use `-vv` for debug)        |

## Behavior

Syncs symlinks and submodules from a source worktree to one or more target
worktrees. This is useful when configuration changes (symlinks, submodules)
need to be applied to existing worktrees.

### Source Resolution

The source worktree is determined in this order:

1. `--source` flag if specified
2. `default_source` configuration if set
3. Current worktree (fallback)

### Target Resolution

Targets are determined based on arguments and flags:

| Arguments | `--all` | Behavior                            |
|-----------|---------|-------------------------------------|
| None      | No      | Sync current worktree (see note)    |
| None      | Yes     | Sync all worktrees (except main)    |
| Specified | No      | Sync specified worktrees            |
| Specified | Yes     | Error (mutually exclusive)          |

**Note:** When no targets are specified and source falls back to current
worktree (no `--source` flag and no `default_source` config), this results
in syncing current worktree to itself, which is an error.

### What Gets Synced

The command syncs based on the source worktree's configuration:

| Configuration       | Action                                          |
|---------------------|-------------------------------------------------|
| `symlinks`          | Create symlinks from source to target           |
| `init_submodules`   | Initialize submodules in target worktrees       |

If neither `symlinks` nor `init_submodules` is configured, the command exits
early with a message indicating nothing to sync.

### Symlink Behavior

Symlinks are synchronized to match the source worktree. Existing symlinks are
replaced to ensure synchronization. Regular files are never overwritten.

| Condition               | Behavior                                |
|-------------------------|-----------------------------------------|
| No file at destination  | Create symlink                          |
| Symlink exists          | Replace with new symlink                |
| Regular file exists     | Skip (not replaced, prevents data loss) |

### Check Mode

With `--check`, the command shows what would be synced without making changes.
This is useful for previewing the sync operation.

## Output Format

### Default Output

```txt
Synced feat/a from main: 2 symlinks created, 1 submodule(s) initialized
Skipped feat/b: up to date
```

### Verbose Output

```txt
Syncing from main to feat/a
Created symlink: /repo/feat/a/.envrc -> /repo/main/.envrc
Created symlink: /repo/feat/a/.tool-versions -> /repo/main/.tool-versions
Initialized 1 submodule(s)
Synced feat/a from main: 2 symlinks created, 1 submodule(s) initialized
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
Synced feat/a: 2 symlinks created
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
| No source + no targets specified | Error with hint                       |
| Source worktree not found        | Error                                 |
| Target worktree not found        | Error for that target                 |
| Target is same as source         | Skipped                               |
| Symlink creation fails           | Error for that target, others proceed |
| Submodule init fails             | Warning (non-fatal)                   |

## Exit Code

- 0: Success (or nothing to sync)
- 1: One or more targets failed to sync
