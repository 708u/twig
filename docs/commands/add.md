# add subcommand

Create a new worktree with optional symlinks.

## Usage

```txt
gwt add <name> [flags]
```

## Arguments

- `<name>`: Branch name (required)

## Flags

| Flag                  | Short | Description                                        |
|-----------------------|-------|----------------------------------------------------|
| `--sync`              | `-s`  | Sync uncommitted changes to new worktree           |
| `--carry [<branch>]`  | `-c`  | Carry uncommitted changes (optionally from branch) |
| `--file <pattern>`    | `-F`  | File patterns to carry (requires `--carry`)        |
| `--quiet`             | `-q`  | Output only the worktree path                      |
| `--verbose`           | `-v`  | Enable verbose output                              |
| `--source <branch>`   |       | Use specified branch's worktree as source          |
| `--lock`              |       | Lock the worktree after creation                   |
| `--reason <string>`   |       | Reason for locking (requires `--lock`)             |

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

### Carry Option

With `--carry`, uncommitted changes are moved to the new worktree:

1. Stashes changes from the specified source
2. Creates the new worktree
3. Applies stash to new worktree
4. Drops the stash (source worktree becomes clean)

Unlike `--sync` which copies changes to both worktrees, `--carry` moves
changes so that only the new worktree has them.

```bash
# Move current work to a new branch
gwt add feat/new --carry

# Move changes from main worktree
gwt add feat/new --carry=main

# Move changes from feat/a worktree
gwt add feat/new --source main --carry=feat/a
```

The `--carry` option accepts an optional value to specify where to take
changes from:

| Value         | Description                                    |
|---------------|------------------------------------------------|
| (no value)    | Take changes from current worktree (default)   |
| `<branch>`    | Take changes from specified branch's worktree  |

#### Carrying Specific Files

Use `--file` to carry only matching files:

```bash
# Carry only Go files in root
gwt add feat/new --carry --file "*.go"

# Carry all Go files recursively (globstar)
gwt add feat/new --carry --file "**/*.go"

# Carry multiple patterns
gwt add feat/new --carry --file "*.go" --file "cmd/**"

# Carry specific file from another worktree
gwt add feat/new --carry=feat/a --file config.toml
```

Patterns support globstar (`**`) for recursive matching.

When `--file` is specified:

- Only matching files are stashed and carried to the new worktree
- Non-matching files remain in the source worktree
- The source worktree is not completely clean after carry

Without `--file`, all uncommitted changes are carried (default behavior).

If worktree creation or stash apply fails, changes are restored
to the source worktree automatically.

Constraints:

- Cannot be used together with `--sync`
- `--file` requires the `--carry` flag

### Quiet Option

With `--quiet`, only the worktree path is output to stdout.
This is useful for piping to other commands.

```bash
cd $(gwt add feat/x -q)
```

When `--quiet` is specified, `--verbose` is ignored.

### Source Option

With `--source`, uses the specified branch's worktree as the source.

```bash
# From a derived worktree, create a new worktree based on main
gwt add feat/new --source main
```

When `--source` is specified:

- Settings are loaded from the source branch's worktree
- Symlinks are created from the source branch's worktree
- With `--sync`, changes are stashed from the source branch's worktree
- With `--carry` (no value), changes are stashed from the current worktree
- With `--carry=<branch>`, changes are stashed from the specified branch's
  worktree

Constraints:

- The specified branch must have an existing worktree

When used with `-C`:

- `-C` sets the working directory and loads config from that location
- `--source` searches for the branch within that directory's worktree group
- This allows running `gwt add` from outside any worktree

```bash
# From any directory, create worktree using repo's settings
gwt add feat/new -C /path/to/repo --source main
```

### Lock Option

With `--lock`, the worktree is locked after creation to prevent automatic
pruning by `git worktree prune`. This is useful for worktrees on portable
devices or network shares that are not always mounted.

```bash
# Create a locked worktree
gwt add feat/usb-work --lock

# Create a locked worktree with a reason
gwt add feat/usb-work --lock --reason "USB drive work"
```

The `--reason` option requires `--lock` and adds an explanation for why
the worktree is locked. This reason is displayed by `git worktree list`.

Locked worktrees require `--force` (or `-f -f`) to be moved or removed
with git commands.

### Default Source Configuration

The default source branch can be configured in `.gwt/settings.toml`:

```toml
default_source = "main"
```

Priority:

1. CLI `--source` flag (highest)
2. Config `default_source`
3. Current worktree (lowest)

When `-C` is specified, `default_source` from that directory's config is
applied. This provides consistent behavior: the config loaded by `-C` is
fully respected.

```bash
# If /path/to/repo has default_source = "main" in its config:
gwt add feat/new -C /path/to/repo
# This is equivalent to:
gwt add feat/new -C /path/to/repo --source main
```

The setting can be overridden in `.gwt/settings.local.toml` for personal
preferences:

```toml
# settings.local.toml
default_source = "develop"
```

To bypass `default_source` and use the current worktree, specify the current
branch with `--source`:

```bash
# Explicitly use current worktree instead of default_source
gwt add feat/x --source feat/a  # assuming you're on feat/a
```
