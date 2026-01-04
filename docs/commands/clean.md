# clean subcommand

Remove merged worktrees that are no longer needed.

## Usage

```txt
gwt clean [flags]
```

## Flags

| Flag              | Short | Description                                |
|-------------------|-------|--------------------------------------------|
| `--yes`           | `-y`  | Execute removal without confirmation       |
| `--dry-run`       |       | Show candidates without removing           |
| `--target`        |       | Target branch for merge check              |
| `--verbose`       | `-v`  | Show skip reasons for skipped worktrees    |

## Behavior

By default, only shows candidates without removing them.
Use `--yes` to actually remove the worktrees.

### Safety Checks

All conditions must pass for a worktree to be cleaned:

| Condition          | Description                       |
|--------------------|-----------------------------------|
| Merged             | Branch is merged to target        |
| No changes         | No uncommitted changes            |
| Not locked         | Worktree is not locked            |
| Not current        | Not the current directory         |
| Not main           | Not the main worktree             |

### Target Branch Detection

If `--target` is not specified, auto-detects from the first
non-bare worktree (usually main).

### Additional Actions

The command also runs `git worktree prune` to clean up references
to worktrees that no longer exist.

## Examples

```txt
# Show candidates only (default)
gwt clean
clean: feature/old-branch
clean: fix/completed

# Show with skip reasons
gwt clean -v
clean: feature/old-branch
skip: feature/active (has uncommitted changes)
skip: feature/wip (not merged)

# Actually remove worktrees
gwt clean --yes
gwt clean: feature/old-branch
gwt clean: fix/completed

# Check against specific branch
gwt clean --target develop

# Preview what would be removed
gwt clean --dry-run
```

## Exit Code

- 0: Success (or no candidates to clean)
- 1: Error occurred during cleanup
