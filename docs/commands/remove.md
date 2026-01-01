# remove subcommand

Remove worktrees and delete their associated branches.

## Usage

```txt
gwt remove <branch>... [flags]
```

## Arguments

- `<branch>...`: One or more branch names to remove (required)

## Flags

| Flag        | Short | Description                            |
|-------------|-------|----------------------------------------|
| `--force`   | `-f`  | Force removal with uncommitted changes |
| `--dry-run` |       | Show what would be removed             |

## Behavior

- Finds the worktree path by looking up the branch name
- Prevents removal if current directory is inside the target worktree
- With `--dry-run`: prints what would be removed without making changes
- Without `--force`: fails if there are uncommitted changes
  or the branch is not merged
- With `--force`: bypasses uncommitted changes
  and unmerged branch checks

## Multiple Branches

When multiple branches are specified, errors on individual branches
do not stop processing of remaining branches. All results and errors
are reported at the end.

```txt
# Remove multiple worktrees
gwt remove feature/a feature/b feature/c
```

## Exit Code

- 0: All branches removed successfully
- 1: One or more branches failed to remove
