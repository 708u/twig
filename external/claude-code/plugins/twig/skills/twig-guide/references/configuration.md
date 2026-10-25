# Configuration

twig reads configuration from TOML files in the `.twig/` directory.

## Files

| File                         | Purpose                                      |
|------------------------------|----------------------------------------------|
| `.twig/settings.toml`         | Project-level settings (commit to repository)|
| `.twig/settings.local.toml`   | Local settings (add to .gitignore)           |

## Fields

### worktree_destination_base_dir

Base directory where new worktrees are created.

```toml
worktree_destination_base_dir = "/path/to/worktrees"
```

Default: `../<repo-name>-worktree`

### default_source

Default branch to use as source when creating new worktrees.

```toml
default_source = "main"
```

Setting `default_source` ensures symlinks are always created from the same
worktree (e.g., main branch), preventing symlink chaining when creating
worktrees from derived branches.

Without `default_source`, symlinks are created from the current worktree.
For example, if you create `feat/api` from `main`, then `feat/api-v2` from
`feat/api`, the symlinks chain: `feat/api-v2 -> feat/api -> main`.
With `default_source = "main"`, symlinks always point directly to main.

See [add subcommand](commands/add.md#default-source-configuration) for details.

### symlinks

Glob patterns for files to symlink from source worktree to new worktrees.

```toml
symlinks = [".envrc", "config/**/*.toml"]
```

Since these symlinks are untracked, `git status` reports them the same way
as ordinary untracked files. The [clean](commands/clean.md) and
[remove](commands/remove.md) commands match `symlinks` (including
`extra_symlinks`) against each worktree's untracked files and exclude the
matches from uncommitted-changes detection, so a worktree with only
symlinked files is still treated as clean.

### extra_symlinks

Additional symlink patterns. Collected from both project and local configs.

```toml
extra_symlinks = [".tool-versions", ".claude"]
```

### init_submodules

Enable automatic submodule initialization when creating worktrees.

```toml
init_submodules = true
```

Default: `false` (disabled)

When enabled, `git submodule update --init --recursive` is run after
worktree creation. The CLI flag `--init-submodules` forces enable regardless
of this setting.

See [add subcommand](commands/add.md#submodule-initialization) for details.

### submodule_reference

Reuse objects from main worktree when initializing submodules.

```toml
submodule_reference = true
```

Default: `false` (disabled)

When enabled, submodule initialization reuses objects from the main worktree's
already-initialized submodules. This can significantly speed up initialization
for large submodules.

The CLI flag `--submodule-reference` forces enable regardless of this setting.

See [add subcommand](commands/add.md#submodule-reference) for details.

### clean_stale

Always enable `--stale` behavior for the clean command.

```toml
clean_stale = true
```

Default: `false` (disabled)

When enabled, `twig clean` treats merged or upstream-gone branches
as stale even if they have uncommitted changes or dirty submodules.
The CLI flag `--stale` forces enable regardless of this setting.

See [clean subcommand](commands/clean.md#stale-option) for details.

### hooks

Commands to run after worktree creation.

```toml
hooks = ["npm install", "direnv allow"]
```

Default: `[]` (no hooks)

Each command is executed via `sh -c` in the new worktree
directory, in order. If a hook fails, remaining hooks are
skipped and a warning is displayed, but the worktree creation
itself succeeds.

See [add subcommand](commands/add.md#post-create-hooks)
for details.

### lfs_fetch_exclude

Git LFS paths to leave as pointer files in new worktrees.

```toml
lfs_fetch_exclude = ["assets/images/**"]
```

Default: `[]` (all LFS files are checked out)

The patterns are joined with commas and passed to `git worktree add` as
`-c lfs.fetchexclude=<patterns>`. A pattern must not contain a comma,
because git-lfs splits the value on commas. When
`extensions.worktreeConfig` is enabled, the same value is also written
with `git config --worktree`, so later merges, rebases, and branch
switches in that worktree keep the exclusion. Otherwise only the initial
checkout excludes the files, and `twig add` prints a warning.

To fetch excluded files later, run in the worktree:

```bash
# Some paths, keeping the exclusion for later checkouts
git lfs pull -I '<path>' -X ''

# Every file, and stop excluding in this worktree
git config --worktree --unset-all lfs.fetchexclude
git lfs pull
```

`-X ''` is required because the worktree config would otherwise exclude
the path again. Worktrees share LFS objects, so `git lfs checkout` can
replace `git lfs pull` when the objects are already stored locally.

`twig add --lfs-fetch-exclude <pattern>` replaces the patterns and
`twig add --no-lfs-fetch-exclude` disables the setting for one run.
See [add subcommand](commands/add.md#checkout-settings) for details.

### sparse_checkout

Non-cone sparse checkout patterns applied to new worktrees.

```toml
sparse_checkout = ["/*", "!/docs/archive/"]
```

Default: `[]` (sparse checkout disabled)

Patterns use the gitignore-style syntax of `git sparse-checkout set
--no-cone`. Requires git 2.36 or later.

`git sparse-checkout set` enables `extensions.worktreeConfig` in the
shared repository config (`.git/config`). The setting applies to every
worktree of the repository, and git understands it since 2.20.

To check out excluded paths later, run in the worktree:

```bash
# One path; the added pattern overrides the earlier exclusion
git sparse-checkout add /docs/archive/

# Every path
git sparse-checkout disable
```

`twig add --sparse-checkout <pattern>` replaces the patterns and
`twig add --no-sparse-checkout` disables the setting for one run.
See [add subcommand](commands/add.md#checkout-settings) for details.

## Merge Rules

When both files exist, settings are merged:

| Field                           | Behavior                | Default                        |
|---------------------------------|-------------------------|--------------------------------|
| `worktree_destination_base_dir` | Local overrides project | `../<repo-name>-worktree`      |
| `default_source`                | Local overrides project | (current worktree)             |
| `symlinks`                      | Local overrides project | `[]`                           |
| `extra_symlinks`                | Collected from both     | `[]`                           |
| `init_submodules`               | Local overrides project | `false`                        |
| `submodule_reference`           | Local overrides project | `false`                        |
| `clean_stale`                   | Local overrides project | `false`                        |
| `hooks`                         | Local overrides project | `[]`                           |
| `lfs_fetch_exclude`             | Local overrides project | `[]`                           |
| `sparse_checkout`               | Local overrides project | `[]`                           |

## symlinks vs extra_symlinks

Use `symlinks` for base patterns shared with the team.
Use `extra_symlinks` to add personal patterns without overriding the base.

Example:

```toml
# .twig/settings.toml (shared)
symlinks = [".envrc", "config/**"]
```

```toml
# .twig/settings.local.toml (personal)
extra_symlinks = [".tool-versions"]
```

Result: `.envrc`, `config/**`, `.tool-versions` are all symlinked.

To completely replace project symlinks locally:

```toml
# .twig/settings.local.toml
symlinks = [".my-envrc"]
```

Result: Only `.my-envrc` is symlinked (project symlinks ignored).

## Example Configuration

```toml
# .twig/settings.toml
worktree_destination_base_dir = "/Users/dev/projects/myapp-worktree"
default_source = "main"
symlinks = [".envrc", ".tool-versions", "config/**"]
init_submodules = true
submodule_reference = true
clean_stale = true
hooks = ["npm install", "direnv allow"]
lfs_fetch_exclude = ["assets/images/**"]
sparse_checkout = ["/*", "!/docs/archive/"]
```

```toml
# .twig/settings.local.toml
default_source = "develop"
extra_symlinks = [".claude", ".local-config"]
```
