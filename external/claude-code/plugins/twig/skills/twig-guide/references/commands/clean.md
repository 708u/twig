# clean subcommand

Remove merged worktrees and prunable branches that are no longer needed.

## Usage

```txt
twig clean [flags]
```

## Flags

| Flag              | Short | Description                                            |
|-------------------|-------|--------------------------------------------------------|
| `--yes`           | `-y`  | Execute removal without confirmation                   |
| `--check`         |       | Show candidates without prompting                      |
| `--target`        |       | Target branch for merge check                          |
| `--force`         | `-f`  | Force clean (can be specified twice, see below)        |
| `--verbose`       | `-v`  | Enable verbose output (use `-vv` for debug)            |

## Behavior

By default, shows candidates and prompts for confirmation before removing.

| Flag      | Behavior                                 |
|-----------|------------------------------------------|
| (none)    | Show candidates, prompt, then execute    |
| `--yes`   | Execute without confirmation             |
| `--check` | Show candidates only (no prompt)         |

### Interactive Confirmation

When run without `--yes` or `--check`, the command displays candidates
and prompts for confirmation:

```txt
clean:
  feat/old-branch (merged)
  fix/completed (upstream gone)

Proceed? [y/N]:
```

Enter `y` or `yes` (case-insensitive) to proceed with removal.
Any other input aborts the operation without removing anything.

### Safety Checks

All conditions must pass for a worktree to be cleaned:

| Condition          | Description                                      |
|--------------------|--------------------------------------------------|
| Merged             | Branch is merged to target or upstream is gone   |
| No changes         | No uncommitted changes                           |
| No dirty submodule | Submodules have no uncommitted changes           |
| Not locked         | Worktree is not locked                           |
| Not current        | Not the current directory                        |
| Not main           | Not the main worktree                            |

### Prunable Branches

When a worktree directory is deleted externally (via `rm -rf` or other means),
the branch remains but becomes prunable. The clean command detects these
prunable branches and includes them as cleanup candidates.

Prunable branches are identified by git's prunable status (from
`git worktree list --porcelain`). Only branches that were previously
associated with a worktree are detected - regular branches created with
`git branch` are not affected.

Safety checks for prunable branches:

| Condition | Description                                     |
|-----------|-------------------------------------------------|
| Merged    | Branch is merged to target or upstream is gone  |

Other checks (locked, changes, current directory) don't apply since
the worktree no longer exists.

### Upstream Gone Branches

Branches whose remote tracking branch has been deleted are detected as
"upstream gone" and cleaned without requiring `--force`.

### Force Option

With `--force` (`-f`), some safety checks can be bypassed:

| Force Level | Bypassed Conditions                              |
|-------------|--------------------------------------------------|
| `-f`        | Uncommitted changes, not merged, dirty submodule |
| `-ff`       | Above + locked worktrees                         |

The following conditions are never bypassed:

- Current directory (dangerous to remove cwd)
- Detached HEAD (RemoveCommand requires branch name)

This matches `twig remove` behavior where `-f` removes unclean worktrees
and `-ff` also removes locked worktrees.

```bash
# Force clean unmerged branches with uncommitted changes
twig clean -f --yes

# Also force clean locked worktrees
twig clean -ff --yes
```

### Target Branch Detection

If `--target` is not specified, auto-detects from the first
non-bare worktree (usually main).

### Additional Actions

The command also runs `git worktree prune` to clean up references
to worktrees that no longer exist.

## Output Format

Output is grouped by status with indentation. Each candidate shows the
reason why it is cleanable:

```txt
clean:
  feat/old-branch (merged)
  fix/completed (upstream gone)
  feat/stale-branch (prunable, merged)

skip:
  feat/wip (not merged)
  feat/active (has uncommitted changes)
  feat/submod (submodule has uncommitted changes)
```

- `clean:` shows worktrees and prunable branches that will be removed
- `skip:` shows skipped worktrees (verbose mode only)
- Each item is indented with 2 spaces
- A blank line separates groups

With `--verbose`, worktrees skipped due to uncommitted changes show the
list of changed files:

```txt
skip:
  feat/wip (has uncommitted changes)
     M src/main.go
    ?? tmp/debug.log
```

Clean reasons:

| Reason           | Description                                     |
|------------------|-------------------------------------------------|
| `merged`         | Branch is merged to target branch               |
| `upstream gone`  | Remote tracking branch was deleted              |
| `prunable, ...`  | Worktree directory was deleted externally       |

### Debug Output

With `-vv`, debug logging is enabled to trace internal operations:

```txt
twig clean --check -vv
2026-01-18 12:34:56.000 [DEBUG] [a1b2c3d4] clean: checking worktree branch=feat/old-branch
2026-01-18 12:34:56.000 [DEBUG] [a1b2c3d4] clean: check completed branch=feat/old-branch canRemove=true
clean:
  feat/old-branch (merged)
```

## Examples

```txt
# Show candidates with confirmation prompt (default)
twig clean
clean:
  feature/old-branch (merged)
  fix/completed (upstream gone)

Proceed? [y/N]: y

# Show with skip reasons and changed files
twig clean -v
clean:
  feature/old-branch (merged)

skip:
  feature/active (has uncommitted changes)
     M src/main.go
    ?? tmp/debug.log
  feature/wip (not merged)

Proceed? [y/N]: y
Removed worktree and branch: feature/old-branch

# Remove without confirmation (silent on success)
twig clean --yes

# Remove with verbose output
twig clean --yes -v
Removed worktree and branch: feature/old-branch
Removed worktree and branch: fix/completed

# Only check candidates (no prompt, no removal)
twig clean --check
clean:
  feature/old-branch (merged)
  fix/completed (upstream gone)

# Check against specific branch
twig clean --target develop

# Clean with prunable branches
twig clean --check
clean:
  feature/old-branch (merged)
  feature/deleted-worktree (prunable, merged)
```

## Exit Code

- 0: Success (or no candidates to clean)
- 1: Error occurred during cleanup
