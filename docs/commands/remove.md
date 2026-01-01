# remove subcommand

Remove a worktree and delete its associated branch.

## Usage

```txt
gwt remove <branch> [flags]
```

## Arguments

- `<branch>`: Branch name to remove (required)

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
