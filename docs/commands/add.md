# add subcommand

Create a new worktree with optional symlinks.

## Usage

```txt
twig add <name> [flags]
```

## Arguments

- `<name>`: Branch name (required)

## Flags

| Flag                  | Short | Description                                        |
|-----------------------|-------|----------------------------------------------------|
| `--sync`              | `-s`  | Sync uncommitted changes to new worktree           |
| `--carry [<branch>]`  | `-c`  | Carry uncommitted changes (optionally from branch) |
| `--file <pattern>`    | `-F`  | File patterns to sync/carry (requires `--sync` or `--carry`) |
| `--quiet`             | `-q`  | Output only the worktree path                      |
| `--verbose`           | `-v`  | Enable verbose output                              |
| `--source <branch>`   |       | Use specified branch's worktree as source          |
| `--lock`              |       | Lock the worktree after creation                   |
| `--reason <string>`   |       | Reason for locking (requires `--lock`)             |

## Behavior

- Creates worktree at `WorktreeDestBaseDir/<name>`
- Branch lookup follows this order:
  1. If the branch exists locally, uses that branch
  2. If the branch exists on a remote, fetches and creates tracking branch
  3. Otherwise, creates a new local branch
- Creates symlinks from source worktree to new worktree
  based on `symlinks` patterns (see [Configuration](../configuration.md))
- Warns when symlink patterns don't match any files

### Remote Branch Support

When the specified branch doesn't exist locally, twig checks local
remote-tracking branches (e.g., `refs/remotes/origin/<branch>`):

```bash
# If origin/feat/api exists locally (already fetched):
twig add feat/api
# Creates worktree with tracking branch
```

This behavior is similar to `git checkout <branch>` which auto-tracks
remote branches without network access.

To get the latest remote branches, run `git fetch` first:

```bash
git fetch origin
twig add feat/api
```

#### Multiple Remotes

When multiple remotes have the branch:

| Scenario                              | Behavior                            |
|---------------------------------------|-------------------------------------|
| Branch exists on one remote only      | Uses that remote                    |
| Branch exists on multiple remotes     | Error (ambiguous)                   |
| Branch exists on no remote            | Creates new local branch            |

```bash
# Branch exists on both origin and upstream
twig add feat/shared
# Error: branch "feat/shared" exists on multiple remotes: [origin upstream]
```

### Sync Option

With `--sync`, uncommitted changes are copied to the new worktree:

1. Stashes current changes
2. Creates the new worktree
3. Applies stash to new worktree
4. Restores changes in the source worktree

If worktree creation or stash apply fails, changes are restored
to the source worktree automatically.

#### Syncing Specific Files

Use `--file` to sync only matching files:

```bash
# Sync only Go files in root
twig add feat/new --sync --file "*.go"

# Sync all Go files recursively (globstar)
twig add feat/new --sync --file "**/*.go"

# Sync multiple patterns
twig add feat/new --sync --file "*.go" --file "cmd/**"
```

When `--file` is specified with `--sync`:

- Only matching files are stashed and synced to the new worktree
- Non-matching files remain only in the source worktree (not synced)
- Both worktrees have the matching files after operation

Without `--file`, all uncommitted changes are synced (default behavior).

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
twig add feat/new --carry

# Move changes from main worktree
twig add feat/new --carry=main

# Move changes from feat/a worktree
twig add feat/new --source main --carry=feat/a
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
twig add feat/new --carry --file "*.go"

# Carry all Go files recursively (globstar)
twig add feat/new --carry --file "**/*.go"

# Carry multiple patterns
twig add feat/new --carry --file "*.go" --file "cmd/**"

# Carry specific file from another worktree
twig add feat/new --carry=feat/a --file config.toml
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

- `--carry` cannot be used together with `--sync`

### Quiet Option

With `--quiet`, only the worktree path is output to stdout.
This is useful for piping to other commands.

```bash
cd $(twig add feat/x -q)
```

When `--quiet` is specified, `--verbose` is ignored.

### Source Option

With `--source`, uses the specified branch's worktree as the source.

```bash
# From a derived worktree, create a new worktree based on main
twig add feat/new --source main
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
- This allows running `twig add` from outside any worktree

```bash
# From any directory, create worktree using repo's settings
twig add feat/new -C /path/to/repo --source main
```

### Lock Option

With `--lock`, the worktree is locked after creation to prevent automatic
pruning by `git worktree prune`. This is useful for worktrees on portable
devices or network shares that are not always mounted.

```bash
# Create a locked worktree
twig add feat/usb-work --lock

# Create a locked worktree with a reason
twig add feat/usb-work --lock --reason "USB drive work"
```

The `--reason` option requires `--lock` and adds an explanation for why
the worktree is locked. This reason is displayed by `git worktree list`.

Locked worktrees require `--force` (or `-f -f`) to be moved or removed
with git commands.

### Default Source Configuration

The default source branch can be configured in `.twig/settings.toml`:

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
twig add feat/new -C /path/to/repo
# This is equivalent to:
twig add feat/new -C /path/to/repo --source main
```

The setting can be overridden in `.twig/settings.local.toml` for personal
preferences:

```toml
# settings.local.toml
default_source = "develop"
```

To bypass `default_source` and use the current worktree, specify the current
branch with `--source`:

```bash
# Explicitly use current worktree instead of default_source
twig add feat/x --source feat/a  # assuming you're on feat/a
```

## Configuration

See [Configuration](../configuration.md) for details on settings files,
available fields, and merge rules.
