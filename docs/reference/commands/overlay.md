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
worktree (e.g., running docker compose from the main worktree).

### Apply

1. Resolves the target worktree (from `--target` or current worktree)
2. Checks that no overlay is already active
3. Checks that the target has no uncommitted changes (use `--force`
   to skip)
4. Resolves the source branch to a commit hash
5. Checks out source branch files onto the target: `git checkout
   <source> -- .`
6. Deletes files that exist in target HEAD but not in source
7. Unstages all changes: `git reset HEAD`
8. Writes a state file for later restore

### Restore

1. Reads the state file from the target's git directory
2. Checks that HEAD hasn't moved since overlay (use `--force` to
   skip)
3. Restores tracked files from HEAD: `git checkout HEAD -- .`
4. Removes only files that were added by the overlay
5. Removes the state file

Files created by the user after overlay are preserved during restore.
Only overlay-added files are removed.

### State File

The overlay state is stored at `<git-dir>/twig-overlay`:

- Main worktree: `.git/twig-overlay`
- Linked worktree: `.git/worktrees/<name>/twig-overlay`

```json
{
  "source_branch": "feat/x",
  "source_commit": "abc1234",
  "target_branch": "main",
  "target_commit": "def5678",
  "added_files": ["new-file.go"],
  "created_at": "2026-03-19T12:00:00Z"
}
```

### Docker Compose Workflow

```bash
# Overlay feature branch onto main worktree
twig overlay feat/x --target main

# Run services (main worktree is synced to docker)
docker compose up
# ... test ...
docker compose down

# Restore main worktree
twig overlay --restore --target main
```

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
