# overlay subcommand

Overlay file contents from a source branch onto a target worktree.

## Usage

```txt
twig overlay <source-branch> [flags]   # Apply overlay
twig overlay --restore [flags]          # Restore original state
```

## Arguments

- `<source-branch>`: Source branch to overlay (required for apply)

## Flags

| Flag        | Short | Description                                     |
|-------------|-------|-------------------------------------------------|
| `--restore` |       | Restore target worktree to original state       |
| `--target`  |       | Target worktree branch (default: current)       |
| `--check`   |       | Show what would be done (dry-run)               |
| `--force`   | `-f`  | Proceed even if target is dirty or HEAD moved   |
| `--quiet`   | `-q`  | Suppress output                                 |
| `--verbose` | `-v`  | Verbose output (use `-vv` for debug)            |

## Behavior

Overlays file contents from a source branch onto a target worktree
without changing the target's checked-out branch. This is useful for
testing changes from a feature branch in the context of another
worktree.

### Apply

1. Resolves the target worktree (from `--target` or current worktree)
2. Checks that no overlay is already active
3. Checks that the target has no uncommitted changes (use `--force`
   to skip)
4. Overlays source branch files onto the target working tree
5. Deletes files that exist in target HEAD but not in source

### Restore

1. Checks that HEAD hasn't moved since overlay (use `--force` to
   skip)
2. Restores all tracked files to their original state
3. Removes only files that were added by the overlay

Files created by the user after overlay are preserved during restore.
Only overlay-added files are removed.

### Safety: Commit Prevention

Overlay modifies the working tree without changing the branch.
Committing in an overlaid worktree would commit the feature branch's
content to the target branch (e.g., main).

**Apply time:** A warning is printed to stderr:

```txt
warning: do not commit in the overlaid worktree.
         Use 'twig overlay --restore' when done.
```

**Restore time:** HEAD movement is detected. If commits were made
after overlay, restore fails with a hint:

```txt
error: HEAD has moved since overlay was applied
hint: commits were made on the overlaid worktree
hint: use 'git log --oneline <saved-commit>..HEAD' to review
hint: use 'twig overlay --restore --force' to restore anyway
```

Use `--force` to restore anyway. Commits made after overlay remain
in the git history (user can `git reset` if needed).

### Force Option

| Context         | `--force` behavior                      |
|-----------------|-----------------------------------------|
| Apply           | Proceeds even with uncommitted changes  |
| Restore         | Proceeds even if HEAD has moved         |
| Overlay stacked | Always refused (restore first)          |

**Warning:** With `--force` on apply, uncommitted changes in tracked
files are overwritten and cannot be recovered by `--restore`.
Restore returns files to the last committed state, not the dirty
state before overlay.

### Check Mode

With `--check`, shows what would happen without making changes:

```txt
Would overlay main with feat/x:
  42 file(s) would change
  2 file(s) would be deleted
  1 file(s) would be added
```

## Output Format

### Apply (default)

```txt
Overlaid main with feat/x (42 files changed, 2 deleted, 1 added)
```

### Restore (default)

```txt
Restored main (removed overlay from feat/x)
```

### Verbose

Verbose mode shows deleted and added file lists.

### Quiet

With `--quiet`, no output is produced.

## Edge Cases

| Case                          | Behavior                            |
|-------------------------------|-------------------------------------|
| Target has uncommitted changes | Refuse (use `--force`)             |
| Overlay already active        | Refuse (restore first)             |
| Source branch not found       | Error                               |
| Source == target (same commit)| Error                               |
| Restore when not overlaid     | Error                               |
| User-created files on restore | Preserved                           |
| Commits during overlay        | Detected on restore                 |
| Submodules                    | Pointers updated, no init/deinit   |
| Binary files                  | Handled by git checkout             |
| Detached HEAD on target       | Works (recorded as "HEAD")          |

## Examples

```bash
# Overlay feat/x onto main worktree
twig overlay feat/x --target main

# Overlay onto current worktree
cd /path/to/main-worktree
twig overlay feat/x

# Preview changes
twig overlay feat/x --target main --check

# Force overlay on dirty worktree
twig overlay feat/x --target main --force

# Restore target worktree
twig overlay --restore --target main

# Force restore after accidental commit
twig overlay --restore --target main --force
```

## Exit Code

- 0: Success
- 1: Error (dirty target, no overlay active, etc.)
