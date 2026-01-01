# add subcommand

Create a new worktree with optional symlinks.

## Usage

```txt
gwt add <name>
```

## Arguments

- `<name>`: Branch name (required)

## Behavior

- Creates worktree at `WorktreeDestBaseDir/<name>`
- If the branch already exists, uses that branch
- If the branch doesn't exist, creates a new branch with `-b` flag
- Creates symlinks from `WorktreeSourceDir` to worktree
  based on `Config.Symlinks` patterns
- Warns when symlink patterns don't match any files
