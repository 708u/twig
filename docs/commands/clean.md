# clean subcommand

Remove merged worktrees that are no longer needed.

## Usage

```txt
gwt clean [flags]
```

## Flags

| Flag              | Short | Description                                    |
|-------------------|-------|------------------------------------------------|
| `--yes`           | `-y`  | Execute removal without confirmation           |
| `--check`         |       | Show candidates without prompting              |
| `--target`        |       | Target branch for merge check                  |
| `--force`         | `-f`  | Force clean (can be specified twice, see below)|
| `--verbose`       | `-v`  | Show skip reasons for skipped worktrees        |

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
  feat/old-branch
  fix/completed

Proceed? [y/N]:
```

Enter `y` or `yes` (case-insensitive) to proceed with removal.
Any other input aborts the operation without removing anything.

### Safety Checks

All conditions must pass for a worktree to be cleaned:

| Condition          | Description                       |
|--------------------|-----------------------------------|
| Merged             | Branch is merged to target        |
| No changes         | No uncommitted changes            |
| Not locked         | Worktree is not locked            |
| Not current        | Not the current directory         |
| Not main           | Not the main worktree             |

### Force Option

With `--force` (`-f`), some safety checks can be bypassed:

| Force Level | Bypassed Conditions                      |
|-------------|------------------------------------------|
| `-f`        | Uncommitted changes, not merged          |
| `-ff`       | Above + locked worktrees                 |

The following conditions are never bypassed:

- Current directory (dangerous to remove cwd)
- Detached HEAD (RemoveCommand requires branch name)

This matches `gwt remove` behavior where `-f` removes unclean worktrees
and `-ff` also removes locked worktrees.

```bash
# Force clean unmerged branches with uncommitted changes
gwt clean -f --yes

# Also force clean locked worktrees
gwt clean -ff --yes
```

### Target Branch Detection

If `--target` is not specified, auto-detects from the first
non-bare worktree (usually main).

### Additional Actions

The command also runs `git worktree prune` to clean up references
to worktrees that no longer exist.

## Output Format

Output is grouped by status with indentation:

```txt
clean:
  feat/old-branch
  fix/completed

skip:
  feat/wip (not merged)
  feat/active (has uncommitted changes)
```

- `clean:` shows worktrees that will be removed
- `skip:` shows skipped worktrees (verbose mode only)
- Each item is indented with 2 spaces
- A blank line separates groups

## Examples

```txt
# Show candidates with confirmation prompt (default)
gwt clean
clean:
  feature/old-branch
  fix/completed

Proceed? [y/N]: y
gwt clean: feature/old-branch
gwt clean: fix/completed

# Show with skip reasons
gwt clean -v
clean:
  feature/old-branch

skip:
  feature/active (has uncommitted changes)
  feature/wip (not merged)

Proceed? [y/N]:

# Remove without confirmation
gwt clean --yes
gwt clean: feature/old-branch
gwt clean: fix/completed

# Only check candidates (no prompt, no removal)
gwt clean --check
clean:
  feature/old-branch
  fix/completed

# Check against specific branch
gwt clean --target develop
```

## Exit Code

- 0: Success (or no candidates to clean)
- 1: Error occurred during cleanup
