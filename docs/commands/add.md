# add subcommand

Create a new worktree with optional symlinks.

## Usage

```txt
gwt add <name> [flags]
```

## Arguments

- `<name>`: Branch name (required)

## Flags

| Flag     | Short | Description                               |
|----------|-------|-------------------------------------------|
| `--sync` | `-s`  | Sync uncommitted changes to new worktree  |

## Behavior

- Creates worktree at `WorktreeDestBaseDir/<name>`
- If the branch already exists, uses that branch
- If the branch doesn't exist, creates a new branch with `-b` flag
- Creates symlinks from `WorktreeSourceDir` to worktree
  based on `Config.Symlinks` patterns
- Warns when symlink patterns don't match any files

### Sync Option

With `--sync`, uncommitted changes are copied to the new worktree:

1. Stashes current changes
2. Creates the new worktree
3. Applies stash to new worktree
4. Restores changes in the source worktree

If worktree creation or stash apply fails, changes are restored
to the source worktree automatically.
