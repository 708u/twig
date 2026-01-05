# remove subcommand

Remove worktrees and delete their associated branches.

## Usage

```txt
twig remove <branch>... [flags]
```

## Arguments

- `<branch>...`: One or more branch names to remove (required)

## Flags

| Flag        | Short | Description                                       |
|-------------|-------|---------------------------------------------------|
| `--force`   | `-f`  | Force removal (can be specified twice, see below) |
| `--dry-run` |       | Show what would be removed                        |
| `--verbose` | `-v`  | Enable verbose output                             |

## Behavior

- Finds the worktree path by looking up the branch name
- Prevents removal if current directory is inside the target worktree
- Cleans up empty parent directories after removal (see below)
- With `--dry-run`: prints what would be removed without making changes
- Without `--force`: fails if there are uncommitted changes,
  the branch is not merged, or the worktree is locked
- With `-f` (once): bypasses uncommitted changes and unmerged branch checks
- With `-ff` (twice): also bypasses locked worktree checks

This matches git's behavior where `git worktree remove -f` removes unclean
worktrees and `git worktree remove -f -f` also removes locked worktrees.

### Empty Directory Cleanup

After removing a worktree, twig automatically removes any empty parent
directories up to `WorktreeDestBaseDir`. This prevents orphan directories
from blocking future branch creation.

Example:

```txt
# Remove feat/test worktree
twig remove feat/test

# If feat/ directory is now empty, it is also removed
# This allows creating a 'feat' branch later
```

The cleanup is safe:

- Only removes empty directories
- Stops at `WorktreeDestBaseDir` boundary
- Preserves directories containing other worktrees or files
- Cleanup errors are non-fatal (main operation succeeds)

## Multiple Branches

When multiple branches are specified, errors on individual branches
do not stop processing of remaining branches. All results and errors
are reported at the end.

```txt
# Remove multiple worktrees
twig remove feature/a feature/b feature/c
```

## Exit Code

- 0: All branches removed successfully
- 1: One or more branches failed to remove
