# add subcommand

Create a new worktree with optional symlinks.

## Usage

```txt
gwt add <name> [flags]
```

## Arguments

- `<name>`: Branch name (required)

## Flags

| Flag              | Short | Description                               |
|-------------------|-------|-------------------------------------------|
| `--sync`          | `-s`  | Sync uncommitted changes to new worktree  |
| `--print <field>` |       | Print specific field (path)               |
| `--source <branch>` |     | Use specified branch's worktree as source |

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

### Print Option

With `--print`, only the specified field is output to stdout.
This is useful for piping to other commands.

```bash
cd $(gwt add feat/x --print path)
```

Available fields:

- `path`: Worktree path

When `--print` is specified, `--verbose` is ignored.

### Source Option

With `--source`, uses the specified branch's worktree as the source.
This is equivalent to `-C <path>` but accepts a branch name instead of a path.

```bash
# From a derived worktree, create a new worktree based on main
gwt add feat/new --source main
```

When `--source` is specified:

- Settings are loaded from the source branch's worktree
- Symlinks are created from the source branch's worktree
- With `--sync`, changes are stashed from the source branch's worktree

Constraints:

- Cannot be used together with `-C`
- The specified branch must have an existing worktree
